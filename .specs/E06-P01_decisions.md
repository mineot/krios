# E06-P01 — Decisões: Valores e resultados de escrita

Data: 2026-10-19. Resolve as pendências de E06-P01 em [0002_execution_plan.md](0002_execution_plan.md) e a seção §6.6 de [0001_planning.md](0001_planning.md), sob o RF-07. Contratos de consumidor decididos pelo consumidor em 2026-10-19.

## 1. Contexto

E05 entregou os construtores de leitura (`db.Select(ent)`/`tx.Select(ent)`), a distinção coluna × valor, o `Where` e a fronteira construção × execução. E04 fixou a entidade por reflexão, com ponteiro/`sql.Null*` como representação de `NULL`, e deixou a regra fina de escrita para esta entrega. Restava especificar os construtores de escrita: a assinatura de `SetValues`, a política de `nil`/omitido/`0`/`false`/`NULL`, a validade de `Where` vazio em `UPDATE`/`DELETE`, as informações de execução e a solicitação de campos gerados ou alterados (`RETURNING`). Este documento fixa esses contratos antes da implementação em E06-P02.

## 2. Escopo e adiamentos

Decidido aqui: construtores de escrita e sua fronteira de construção/execução; `SetValues` e a política de valores (§4); `Where` obrigatório (§5); informações de execução por banco (§6); retorno de campos e capacidade de dialeto (§7); validação (§8); erros (§9).

| Item                                                                                                                                       | Destino                                  |
| ------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------- |
| `INSERT` com múltiplas linhas (lote)                                                                                                       | Extensão futura registrada (§2)          |
| Seleção de colunas ou expressões em `RETURNING` (padrão: colunas todas)                                                                    | Extensão futura registrada (§7)          |
| Preenchimento automático do struct de destino com valores como chave gerada fora do `RETURNING` (ex.: `LastInsertId` aplicado ao campo pk) | Consumidor, sem apoio da biblioteca (§6) |
| `INSERT` com `SELECT` (inserção a partir de consulta)                                                                                      | Fora do escopo inicial                   |

Multi-row `INSERT` não entra nesta entrega: o RF-07 define que o `INSERT` "deve receber valores associados aos nomes dos campos, representando uma linha". A menção a "múltiplas linhas" em E06-P03 refere-se a `UPDATE`/`DELETE` que atingem várias linhas.

## 3. Construtores e fronteira construção × execução

Mesmo molde de `db.Select(ent)` de [E05-P01_decisions.md](E05-P01_decisions.md) §8: receptores **não genéricos**, ancorados no acesso, guardando o contrato interno `executor` de [E03-P01_decisions.md](E03-P01_decisions.md) (satisfeito por `*DB` e `*Tx`) e o dialeto retido em `DB`/`Tx` (§3 de E05-P01):

```go
func (db *DB) Insert(ent *Entity) *Insert   // também em *Tx
func (db *DB) Update(ent *Entity) *Update   // também em *Tx
func (db *DB) Delete(ent *Entity) *Delete   // também em *Tx
```

- **Construção** (encadeável, pura, não executa):
  - `SetValues(record any)` em `*Insert` e `*Update` (`*Delete` não tem valores); `Omit(names ...string)` em `*Insert` e `*Update`; `Where(...Condition)` em `*Update` e `*Delete`; `Returning()` nos três.
  - Builder mutável em lugar, padrão Go, não seguro para reuso concorrente — cada escrita monta a sua.
- **Renderização**: `Build() (query string, args []any, err error)` público, como em `Select`, para testes de geração, inspeção e log.
- **Execução**: `Exec(ctx) (Result, error)` para a escrita sem retorno de campos; com `Returning()`, `One(ctx, dest)` em `*Insert` e `All(ctx, dest)` em `*Update`/`*Delete` (§7). Guards de E03 (`ErrClosed`, `ErrTxDone`, contexto, serialização da `Tx`, bloqueio de `Commit`) são preservados sem mecanismo novo: os métodos passam pelo mesmo `executor`.
- Toda falha de construção aparece em `Build()` **e** é interceptada antes de tocar o banco. Como em `SetValues` e `Omit` não há canal de erro na cadeia, o **primeiro erro é memorizado no builder** (first-wins) e devolvido por `Build()`/métodos de execução — mesma filosofia de E05 (erro de construção só se manifesta em `Build()`/execução), sem erro perdido e sem panics.
- A ordem textual dos parâmetros segue `SET` → `WHERE`; a renderização das condições reutiliza `Condition.RenderInto`, com numeração contínua de placeholders (inclusive `$n` no PostgreSQL).

Esboço de consumo:

```go
user := must(krios.EntityOf[User]())

// INSERT com coluna de default omitida
res, err := db.Insert(user).
    Omit("created_at").
    SetValues(&User{Name: "ana", Bio: nil}). // Bio → NULL
    Exec(ctx)

// UPDATE: campos nil não são atualizados; Where obrigatório
err = db.Update(user).
    SetValues(&User{Status: "active", Name: ""}). // Name (não-anulável) → ""
    Where(user.Col("id").Eq(1)).
    Exec(ctx)

// DELETE com filtro
err = db.Delete(user).
    Where(user.Col("status").IsNull()).
    Exec(ctx)

// Retorno de campos (SQLite/PostgreSQL; MySQL → erro antes de executar)
var created User
err = db.Insert(user).
    SetValues(&User{Name: "bia"}).
    Returning().
    One(ctx, &created)

var updated []User
err = db.Update(user).
    SetValues(&User{Status: "banned"}).
    Where(user.Col("status").Eq("active")).
    Returning().
    All(ctx, &updated)
```

## 4. `SetValues` e política de valores

### 4.1 Assinatura

- `SetValues(record any)` recebe **uma linha**: `T` ou `*T` do mesmo tipo struct da entidade (mesma normalização de `recordValue` usada por `Entity.Validate`).
- O tipo do registro deve corresponder à entidade do construtor; `nil` ou tipo divergente é erro memorizado (`ErrInvalidEntity`), sem tocar no banco.
- Lote (`[]T`) fica como extensão futura (§2).

### 4.2 Política de escrita

A distinção entre omitido, zero, `false` e `NULL` usa o modelo de ponteiro/`sql.Null*` de [E04-P01_decisions.md](E04-P01_decisions.md) §3, fixado aqui:

| Valor no struct                          | `INSERT`                                      | `UPDATE`                       |
| ---------------------------------------- | --------------------------------------------- | ------------------------------ |
| Campo não-anulável (inclui `0`, `false`) | Grava o valor                                 | Grava o valor (inclusive zero) |
| `*T` não-`nil`                           | Grava o valor                                 | Grava o valor                  |
| `*T` `nil`                               | Grava `NULL`                                  | **Coluna não é atualizada**    |
| `sql.Null*` com `Valid: true`            | Grava o valor                                 | Grava o valor                  |
| `sql.Null*` com `Valid: false`           | Grava `NULL`                                  | Grava `NULL`                   |
| Coluna listada em `Omit(...)`            | Coluna ausente do `INSERT` (default do banco) | Coluna não é atualizada        |

Decisões do consumidor (2026-10-19) refletidas acima:

- **`nil` em `INSERT` grava `NULL`** (não omite), preservando o roundtrip: um struct lido do banco com `NULL` e regravado produz `NULL`, não o default da coluna. O default do banco (timestamp automático, chave gerada) é obtido **somente** por omissão explícita com `Omit("coluna")`.
- **`nil` em `UPDATE` não atualiza a coluna** (convenção preliminar de E04-P01 §3, confirmada).
- `Omit` é a única via de "usar default/não tocar", sempre explícita na cadeia; não há omissão implícita por valor zero nem por tag.

Consequências documentadas como contrato:

- Em `UPDATE`, todo campo não-anulável é gravado, inclusive zero — quem precisa omitir usa campo ponteiro/`sql.Null*` ou `.Omit(...)`. Nada é inferido.
- Para usar o default do banco em uma coluna de chave gerada, o consumidor chama `.Omit("id")` (ou usa `Returning()` para ler o valor gerado de volta).
- `Omit` com nome desconhecido ou duplicado é erro de construção (`ErrInvalidQuery`); chamadas múltiplas acumulam.
- `UPDATE` que termina sem nenhuma coluna a gravar (todos os campos `nil`/omitidos, ou `SetValues` ausente) é erro de construção (`ErrInvalidQuery`) antes de executar.
- `INSERT` sem `SetValues` é erro de construção (`ErrInvalidQuery`).

### 4.3 Interação com validação e formatação

- O valor enviado ao banco é o **original** do campo; `Format` (apresentação) não participa da escrita (contrato de [E04-P03_decisions.md](E04-P03_decisions.md)).

## 5. `Where` obrigatório em `UPDATE` e `DELETE`

- `UPDATE` e `DELETE` exigem **pelo menos uma condição** em `Where` antes de executar. Ausente, vazio ou composto apenas por grupos vazios → erro de construção (`ErrInvalidQuery`) em `Build()` e antes de qualquer execução.
- Não existe atalho para "atualizar/excluir tudo": é exigência do RF-07. Quem precisa de varredura total exprime o filtro (por exemplo, `ent.Col("id").IsNotNull()`).
- As condições combinam com `AND` e acumulam entre chamadas, exatamente como em `Select` (§6 de E05-P01); a validação de entidades referenciadas é a mesma de `renderCondition`.
- `Where` em `INSERT` não existe (sem filtro de seleção).

## 6. Informações de execução

- A escrita sem `Returning()` retorna `krios.Result` (interface de [E03-P01_decisions.md](E03-P01_decisions.md)): `RowsAffected() (int64, bool)` e `LastInsertId() (int64, bool)`.

| Informação     | SQLite                   | MySQL                                   | PostgreSQL                       |
| -------------- | ------------------------ | --------------------------------------- | -------------------------------- |
| `RowsAffected` | disponível (`ok == true`) | disponível (`ok == true`)              | disponível (`ok == true`)         |
| `LastInsertId` | disponível em `INSERT`   | disponível em `INSERT` (auto-increment) | **indisponível** (`ok == false`) |

- A tabela descreve o caso esperado; a flag é o contrato real. `result.go` devolve `ok == false` sempre que o driver não fornece o dado **ou** falha ao reportá-lo (erro não exposto), inclusive para `RowsAffected`. Portanto `RowsAffected` **não** é "garantido" de forma absoluta: é comunicado pela flag, com disponibilidade esperada conforme a coluna acima. Ver [C05-P01_decisions.md](C05-P01_decisions.md) D28.

- Ausência de informação é comunicada pela flag `false`, nunca por zero ambíguo — contrato já fixado em E03. A aplicação da chave gerada ao struct do consumidor é responsabilidade do consumidor (`Result` + pk, ou `Returning()`); a biblioteca não escreve no struct depois da execução.
- Com `Returning()` ativo, o resultado **são as linhas retornadas**: `Exec` não está disponível nesse modo (§7) e `Result` não é devolvido — a contagem de linhas afetadas é o próprio conjunto lido (`All`) ou a presença da linha (`One`).
- A indisponibilidade de `LastInsertId` no PostgreSQL é a motivação clássica do `RETURNING`; documentada como limitação por banco, sem tradução de sintaxe.

## 7. Retorno de campos gerados ou alterados (`RETURNING`)

O RF-07 exige solicitação explícita, suporte documentado por banco e erro **antes** de executar quando não suportado.

- **Solicitação:** `Returning()` na cadeia (método sem argumentos, muta o builder). É sempre explícito — nenhuma escrita retorna campos sem pedir.
- **Capacidade do dialeto:** interface segregada no **mesmo tipo concreto** de `internal/dialect`, no padrão da `MigrationDialect` de [E08-P01_decisions.md](E08-P01_decisions.md) e da extensão prevista em E05-P01 §3:

  ```go
  type ReturningDialect interface {
      SupportsReturning() bool
  }
  ```

  | Banco      | `RETURNING`                                           |
  | ---------- | ----------------------------------------------------- |
  | SQLite     | sim (versão ≥ 3.35; incorporada pelos testes: 3.53.4) |
  | PostgreSQL | sim                                                   |
  | MySQL      | **não**                                               |

  MySQL não possui `RETURNING` (escopo: `Config.Type` `mysql`; MariaDB fora do escopo). Pedir retorno num dialeto sem suporte gera erro de construção (`ErrInvalidQuery`, com o nome do dialeto na mensagem) em `Build()` e antes de qualquer execução — a escrita nunca chega a acontecer.

- **Destinos** (contrato de E04; tipagem no destino, como em `Rows.All`):
  - `INSERT` atinge uma linha → `One(ctx, dest any) error`, destino `*T`, preenchido como `Row.Scan`.
  - `UPDATE`/`DELETE` podem atingir várias linhas → `All(ctx, dest any) error`, destino `*[]T`, preenchido como `Rows.All`.
  - Falha de `Build` é interceptada antes de tocar no executor; sem linha em `One`, `sql.ErrNoRows` é preservado; erros de conversão preservam a causa (E04).
- **Campos retornados:** todas as colunas mapeadas da entidade, na ordem de declaração dos campos. `UPDATE ... RETURNING` devolve o estado **pós-atualização**; `DELETE ... RETURNING` devolve o estado anterior à remoção (semântica do banco). Seleção de subconjunto de colunas ou expressões em `RETURNING` é extensão futura (§2).
- **Modos mutuamente exclusivos:** a superfície de execução depende do modo — sem `Returning()`: apenas `Exec`; com `Returning()`: `One` (`*Insert`) ou `All` (`*Update`/`*Delete`), e `Exec` passa a ser rejeitado (`ErrInvalidQuery`) antes de executar. O SQL gerado inclui `RETURNING` somente quando solicitado.

## 8. Validação automática na escrita

- `SetValues` executa `Entity.Validate(record)` no momento da chamada (fail-fast, primeiro erro), reaproveitando o motor de [E04-P03_decisions.md](E04-P03_decisions.md): erro `ErrValidation` com tabela/coluna e causa preservada, memorizado no builder e devolvido por `Build()`/execução — nunca após tocar no banco.
- Decisão do consumidor (2026-10-19): validação **automática**, sem opt-out. O consumidor não precisa chamar `Entity.Validate` antes de gravar; chamá-la de novo não causa efeito adverso.
- Campos sem `validator` são pulados pelo motor, como hoje. `Format` não participa (§4.3).

## 9. Erros

Nenhuma sentinela nova. Reuso, no padrão de `internal/errs`:

- `ErrInvalidQuery` — construção inválida: `UPDATE`/`DELETE` sem `Where` (ou com lista vazia), `INSERT` sem `SetValues`, `UPDATE` sem coluna a gravar, `Omit` com nome desconhecido/duplicado, `Returning()` não suportado pelo dialeto, `Exec` com `Returning()` ativo (ou `One`/`All` sem ele), entidade/dialeto ausente. A mensagem contextualiza a causa; embrulhada com `%w`.
- `ErrInvalidEntity` — registro em `SetValues` que não é `T`/`*T` da entidade, inclusive `nil`.
- `ErrValidation` — falha de validação de campo (§8).
- Erros de destino/conversão em `One`/`All` seguem o contrato de E04 (`Row.Scan`/`Rows.All`).
- Erros de execução continuam sendo os de E02/E03 (`ErrClosed`, `ErrTxDone`, contexto, driver), repassados sem conversão.

## 10. Diferenças por banco na versão inicial

| Ponto                 | SQLite          | MySQL                        | PostgreSQL          |
| --------------------- | --------------- | ---------------------------- | ------------------- |
| Marcador de parâmetro | `?`             | `?`                          | `$1..$n`            |
| `RETURNING`           | sim (≥ 3.35)    | não (erro antes de executar) | sim                 |
| `RowsAffected`        | sim             | sim                          | sim                 |
| `LastInsertId`        | sim em `INSERT` | sim em `INSERT`              | não (`ok == false`) |

## 11. Itens resolvidos e permanentes

Resolvidos nesta entrega:

- [x] Assinatura de `SetValues` (§6.6): entrada por struct/ponteiro de uma linha, `T`/`*T`, erro de tipo em `ErrInvalidEntity`.
- [x] Política de `nil`/omitido/zero/`false`/`NULL` em `INSERT` e `UPDATE` (§6.6; pendência de [E04-P01_decisions.md](E04-P01_decisions.md) §3): tabela da §4.2, com `Omit` como única via de default.
- [x] Validade de `Where` vazio para `UPDATE`/`DELETE` (§6.6): sempre exigido, sem atalho.
- [x] Informações de execução e indisponibilidade de `RowsAffected`/`LastInsertId` (§6.6): `krios.Result` com flags, matriz por banco na §6.
- [x] Solicitação, suporte e destinos de campos gerados/alterados (§6.6 e RF-07): `Returning()` + `One`/`All`, capacidade por `ReturningDialect` no mesmo tipo concreto (adiamento de [E05-P01_decisions.md](E05-P01_decisions.md) §2), erro antes de executar.

Permanecem em outras entregas: implementação dos construtores, `Where` obrigatório e rejeição de retorno não suportado (E06-P02); validação de efeitos, múltiplas linhas e rollback do grupo com construtores públicos (E06-P03); lote de `INSERT` e seleção de colunas em `RETURNING` (extensão futura, §2); evidência manual MySQL/PostgreSQL (E09).

## 12. Referências

- RF-07 de [0001_planning.md](0001_planning.md); §6.6 (escritas).
- [E03-P01_decisions.md](E03-P01_decisions.md): contrato `executor`, `Result` com flags.
- [E04-P01_decisions.md](E04-P01_decisions.md): entidade por reflexão, representação de `NULL` por ponteiro/`sql.Null*` (§3).
- [E04-P03_decisions.md](E04-P03_decisions.md): motor `Validate`/`Format` reutilizado na escrita.
- [E05-P01_decisions.md](E05-P01_decisions.md): molde do construtor ancorado no acesso, `Where`/condições reutilizadas, adiamentos com destino em E06-P01 (§2).
- [E08-P01_decisions.md](E08-P01_decisions.md): padrão de interface segregada no mesmo tipo concreto do dialeto.
- Passos E06-P01–E06-P03 em [0002_execution_plan.md](0002_execution_plan.md).
