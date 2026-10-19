# E05-P01 — Decisões: Expressões e contratos dos dialetos

Data: 2026-10-19. Resolve as pendências de E05-P01 em [0002_execution_plan.md](0002_execution_plan.md) e as seções §6.2, §6.3 (construtores) e §6.5 de [0001_planning.md](0001_planning.md). Contratos de consumidor decididos pelo consumidor em 2026-10-19.

## 1. Contexto

E04 entregou o mapeamento por reflexão e E03 o contrato de execução compartilhado por `*DB` e `*Tx`. Restava definir a camada de construção de consultas: como o dialeto adapta o SQL gerado, como uma expressão distingue coluna de valor parametrizado, como `Where` combina condições, quais junções e operadores existem e onde termina a construção e começa a execução. Este documento fixa esses contratos antes da implementação em E05-P02.

## 2. Escopo e adiamentos

Decidido aqui: contrato e seleção do dialeto; modelo de expressões (coluna × valor); comparadores e busca textual; combinação de condições em `Where`; tipos de junção; fronteira construção × execução e vínculo com `*DB`/`*Tx`; erro sentinela de construção.

Adiado com destino definido:

| Item                                                                                 | Destino                                            |
| ------------------------------------------------------------------------------------ | -------------------------------------------------- |
| `krios.Paginate`, total e estatísticas                                               | E05-P03                                            |
| Agregações (`Count`, `Sum`, `Max`, `Min`), `OrderBy`, `GroupBy`, `Distinct`, `Limit` | E05-P03 (o nó de função do AST, §4, já os reserva) |
| `RIGHT`/`FULL` join, alias e esquema como superfície pública                         | Extensão futura registrada (§7)                    |
| `SetValues`, validade de `Where` vazio em `UPDATE`/`DELETE`, retornos de escrita     | E06-P01                                            |
| Capacidade de dialeto para retornos (`RETURNING`)                                    | E06-P01, sobre o mesmo tipo concreto (§3)          |
| `ColumnTypes`                                                                        | E07                                                |

As expressões e `Where` decididas aqui servem também aos construtores de escrita de E06, que reutilizarão os mesmos predicados.

## 3. Seleção e contrato do dialeto

- Os dialetos vivem em `internal/dialect` (pacote interno; nenhuma extensão pública de dialetos nesta versão, coerente com [E01-P01_decisions.md](E01-P01_decisions.md)).
- Cada banco tem um tipo concreto (`sqlite`, `mysql`, `postgres`) que implementa a interface `Dialect` **e** o `MigrationDialect` de [E08-P01_decisions.md](E08-P01_decisions.md). São interfaces separadas (segregação); migrações consomem a capacidade transacional pela interface já contratada, e a consulta consome a geração de SQL pela `Dialect`.
- Seleção por `Config.Type`, com uma função de pacote (`dialect.For`) que devolve o dialeto do tipo. `Open` grava o dialeto no `DB`; `Begin` o repassa à `Tx` (`transaction.New` passa a aceitá-lo). O `DB`/`Tx` retêm apenas o dialeto — não a configuração completa, preservando o contrato de [E02-P01_decisions.md](E02-P01_decisions.md).
- Interface da versão inicial:

```go
type Dialect interface {
    Name() string          // "sqlite" | "mysql" | "postgres" — mensagens e testes
    Placeholder(n int) string // n-ésimo parâmetro, 1-based: "?" ou "$n"
    QuoteIdent(name string) string // aspas de identificador; "a.b" particionado em partes aspadas
}
```

- A estrutura do SQL é gerada por **um único renderizador** compartilhado; o dialeto abstrai apenas os pontos de diferença. "Gerar a equivalência nos três dialetos" (E05-P02) significa exercitar o renderizador com os três dialetos e cobrir cada um com testes unitários de geração, sem banco.
- O renderizador emite literais booleanos `TRUE`/`FALSE`, válidos em SQLite (≥ 3.23; o incorporado pelo driver de teste é 3.53.4), MySQL e PostgreSQL.
- `QuoteIdent` usa crase no MySQL e aspas duplas em SQLite/PostgreSQL. Nomes qualificados `schema.tabela` (possíveis via `TableName()`) são particionados por `.` e cada parte é aspada.
- Extensões previstas, sem superfície nesta versão: capacidade de retorno de campos (E06) e flags de suporte de junção quando `RIGHT`/`FULL` forem consideradas.

## 4. Modelo de expressões: coluna × valor

A distinção coluna × valor é resolvida por **construtores ancorados na entidade**:

- Novo método `(*Entity).Col(name string) Column` valida o nome contra `Columns()` no momento da renderização (não em `Col`, para não poluir o caminho comum com tratamento de erro). Nome desconhecido falha em `Build()` e antes de qualquer execução, com `ErrInvalidQuery`.
- `Column` recebe comparadores como métodos: `Eq`, `Ne`, `Lt`, `Lte`, `Gt`, `Gte`, `IsNull`, `IsNotNull`, `In`, `Like`. Cada um devolve um `Condition` (tipo opaco exportado; o consumidor não inspeciona o AST).
- Operandos: o lado direito aceita `any`. Se for `Column` (ou expressão futura, ex.: agregação), é renderizado como identificador; qualquer outro valor Go vira **parâmetro** separado. Isso resolve as comparações coluna × coluna das junções e coluna × valor dos filtros com a mesma forma.
- `nil` como operando é rejeitado (`ErrInvalidQuery`); `IS NULL`/`IS NOT NULL` são os únicos caminhos para nulidade.
- `In()` sem argumentos produz condição sempre falsa (literal `FALSE`, suportado nos três bancos); com argumentos, cada valor vira parâmetro. `nil` dentro da lista é rejeitado.
- O AST interno tem nós de coluna, parâmetro, comparação, grupo lógico (`AND`/`OR`), `LIKE`, `IN`, nulidade e **chamada de função** (nó reservado para as agregações de E05-P03).
- Ordem dos parâmetros: travessia em ordem textual do SQL final (`FROM` → `JOIN`/`ON` → `WHERE` → demais cláusulas). Os argumentos sempre acompanham o texto, nunca o interlecalam.

## 5. Operadores e busca textual

- Operadores da versão inicial: `=`, `<>`, `<`, `<=`, `>`, `>=`, `IS NULL`, `IS NOT NULL`, `IN`, `LIKE`.
- `Like(mode, value)` distingue os modos exigidos pelo RF-06: `krios.Contains` (`%valor%`), `krios.StartsWith` (`valor%`), `krios.EndsWith` (`%valor`). O valor do consumidor tem `!`, `%` e `_` escapados com `!`, e a cláusula `ESCAPE '!'` é sempre emitida — o valor é tratado literalmente, sem coringa acidental. _Revisão da implementação (E05-P02, 2026-10-19): a decisão original previa `ESCAPE '\'`, mas o literal `'\'` é inválido no MySQL com o escape padrão; o caractere neutro `!` funciona nos três dialetos._
- Sensibilidade a maiúsculas/minúsculas **não é adaptada**: segue a collation do banco. MySQL é insensível por padrão; PostgreSQL é sensível; SQLite é insensível para ASCII por padrão e sensível fora dele. Diferenças documentadas como limitação, sem tradução de sintaxe (decisão de consumidor em 2026-10-19). `ILIKE` e `NOT LIKE` ficam fora do escopo inicial.

## 6. `Where` e grupos lógicos

- `Where(c ...Condition)` combina as condições com `AND`; encadear várias chamadas de `Where` acumula com `AND`.
- Grupos aninhados por `krios.And(c ...Condition)` e `krios.Or(c ...Condition)`, aceitando qualquer condição, inclusive outros grupos. `And`/`Or` sem argumentos é erro em `Build()` (`ErrInvalidQuery`).
- `Where` sem condições não produz cláusula `WHERE` (consulta sem filtro). A validade de `Where` vazio para autorizar `UPDATE`/`DELETE` permanece em E06-P01.
- Parâmetros de `ON` (junções) antecede os de `WHERE` na lista de argumentos, seguindo a ordem textual.

## 7. Junções

- `Join(ent *Entity, kind JoinKind, on Condition)` no construtor, com `krios.InnerJoin` e `krios.LeftJoin` na versão inicial (RF-06: junções). `RIGHT` e `FULL` são extensão futura registrada, com as diferenças por banco (MySQL não tem `FULL OUTER`; SQLite passou a aceitar `RIGHT`/`FULL OUTER` na 3.39).
- O critério é uma `Condition` construída com os mesmos comparadores, tipicamente coluna × coluna entre entidades (`user.Col("id").Eq(task.Col("owner_id"))`); critérios não equitativos usam o mesmo AST.
- Toda coluna referenciada em `Only`, `Where` ou `Join` deve pertencer a uma entidade da consulta (base ou junção); referência a entidade ausente falha em `Build()` com `ErrInvalidQuery`.
- Cada entidade aparece no máximo uma vez (base + junções); repetição é erro. **Alias e esquema não têm superfície pública nesta versão** — junção a mesma tabela/self-join entra na extensão futura junto com alias.
- A ordem de declaração das junções é preservada no SQL gerado.

## 8. Fronteira construção × execução

Go não permite parâmetros de tipo em métodos, então o vínculo escolhido (`db.Select[T]()`) é materializado como receptor **não genérico**; a tipagem fica nos destinos, no mesmo contrato de `Rows.All(*[]T)` e `Row.Scan(*T)` (decisão de consumidor em 2026-10-19):

```go
func (db *DB) Select(ent *Entity) *Select   // também em *Tx
```

- `Select` guarda o receptor (contrato interno `executor` de [E03-P01_decisions.md](E03-P01_decisions.md), satisfeito por ambos) e o dialeto retido em `DB`/`Tx` (§3). Execução pelos métodos do builder preserva guards (`ErrClosed`/`ErrTxDone`), serialização da `Tx` e bloqueio de `Commit` (§3.4.5–§3.4.6) sem mecanismo novo. O pacote do builder declara interface interna equivalente ao `executor`; `*DB` e `*Tx` a satisfazem estruturalmente.
- **Construção** (encadeável, pura, não executa): `Only(names ...string)`, `Where(...Condition)`, `Join(...)`. Builder mutável em lugar, padrão Go; não é seguro para reuso concorrente — cada consulta monta a sua. `Only` valida nomes contra as colunas da entidade base em `Build()`; vazio mantém a seleção padrão (todas as colunas mapeadas, na ordem de declaração dos campos, conforme [E04-P01_decisions.md](E04-P01_decisions.md)); nomes duplicados são erro.
- **Renderização**: método público `Build() (query string, args []any, err error)` — renderiza sem executar, para testes de geração, inspeção e log. Toda falha de construção (coluna desconhecida, `nil`, entidade repetida, junção sem entidade consultada, `And`/`Or` vazio) aparece aqui **e** é interceptada antes da execução.
- **Execução**: `Rows(ctx) (*Rows, error)` e `Row(ctx) Row` replicam o contrato de E03 (`Rows.Close` obrigatório, `sql.ErrNoRows` preservado, erro tardio registrado na `Tx`); as conveniências `All(ctx, dest any) error` e `One(ctx, dest any) error` delegam a `Rows.All` e `Row.Scan`. Erros de `Build` são retornados antes de tocar o executor. `Count`/`Sum`/`Max`/`Min` e paginação entram em E05-P03 como novos métodos do mesmo construtor.
- Esboço de consumo:

```go
user := must(krios.EntityOf[User]())
task := must(krios.EntityOf[Task]())

var active []User
err := db.Select(user).
    Only("id", "name", "status").
    Join(task, krios.LeftJoin, user.Col("id").Eq(task.Col("owner_id"))).
    Where(
        user.Col("status").Eq("active"),
        user.Col("name").Like(krios.Contains, "ana"),
    ).
    All(ctx, &active)

sql, args, err := db.Select(user).
    Where(user.Col("id").In(1, 2, 3)).
    Build()
// sql:  SELECT "user"."id", ... FROM "user" WHERE "user"."id" IN (?, ?, ?)
// args: []any{1, 2, 3}   (SQLite/MySQL; PostgreSQL: IN ($1, $2, $3))
```

## 9. Erros

Novo sentinela em `internal/errs`, exportado pela fachada, no padrão dos existentes:

- `ErrInvalidQuery` — construção inválida: coluna desconhecida, operando `nil`, entidade ausente ou repetida na consulta, `And`/`Or`/`In` vazio quando inválido, `Only` com nome duplicado ou desconhecido, `Select(nil)`. A mensagem contextualiza tabela/coluna; causa embrulhada com `%w`.

Erros de execução continuam sendo os de E02/E03 (`ErrClosed`, `ErrTxDone`, contexto, erros do driver), repassados pelo executor sem conversão.

## 10. Diferenças por banco na versão inicial

| Ponto                   | SQLite                       | MySQL                         | PostgreSQL                   |
| ----------------------- | ---------------------------- | ----------------------------- | ---------------------------- |
| Marcador de parâmetro   | `?`                          | `?`                           | `$1..$n`                     |
| Aspas de identificador  | `"`                          | `` ` ``                       | `"`                          |
| `TRUE`/`FALSE`          | ≥ 3.23                       | sim                           | sim                          |
| Sensibilidade do `LIKE` | insensível p/ ASCII (padrão) | insensível (collation padrão) | sensível                     |
| `FULL OUTER JOIN`       | ≥ 3.39                       | não                           | sim (fora do escopo inicial) |

## 11. Itens resolvidos e permanentes

Resolvidos nesta entrega:

- [x] Seleção e interface dos dialetos (§6.2): `internal/dialect`, `Dialect` + `MigrationDialect` no mesmo tipo concreto, seleção por `Config.Type`, retenção em `DB`/`Tx`.
- [x] Combinação de condições em `Where` e grupos aninhados (§6.5).
- [x] Distinção entre coluna e valor, inclusive em junções (§6.5).
- [x] Tipos de junção, operadores e modos de busca textual (§6.5; agregações em E05-P03).
- [x] Fronteira construção × execução e vínculo do construtor com conexão/transação (§6.3 e pendência de [E04-P01_decisions.md](E04-P01_decisions.md) §7): receptor `db.Select(ent)`/`tx.Select(ent)`.

Permanecem em outras entregas: `krios.Paginate` e estatísticas, agregações e demais cláusulas (E05-P03); `SetValues` e escritas (E06-P01); retorno de campos e `RETURNING` (E06-P01); alias/esquema públicos, `RIGHT`/`FULL` join e self-join (extensão futura); SQL incorporado (E07); implementação das migrações sobre o dialeto (E08-P02/P03).

## 12. Referências

- RF-06 de [0001_planning.md](0001_planning.md); §6.2 (dialetos), §6.3 (construtores × transação) e §6.5 (consultas).
- [E03-P01_decisions.md](E03-P01_decisions.md): contrato `executor`, `Row`/`Rows` e política de `Tx`.
- [E04-P01_decisions.md](E04-P01_decisions.md) e [E04-P02_decisions.md](E04-P02_decisions.md): entidade por reflexão, `Columns()`, ordem de declaração.
- [E08-P01_decisions.md](E08-P01_decisions.md): `MigrationDialect` e seleção por `Config.Type`.
- Passos E05-P01–E05-P04 em [0002_execution_plan.md](0002_execution_plan.md).
