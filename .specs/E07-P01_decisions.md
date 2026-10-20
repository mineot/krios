# E07-P01 — Decisões: Recebimento e execução de arquivos SQL incorporados

Data: 2026-10-20. Resolve as pendências de E07-P01 em [0002_execution_plan.md](0002_execution_plan.md) e a seção §6.8 de [0001_planning.md](0001_planning.md), sob o RF-08. Contratos de consumidor decididos pelo consumidor em 2026-10-20.

## 1. Contexto

E03 fixou os contratos de execução (`executor` interno, `Result`/`Row`/`Rows`, guards e política transacional); E04 fixou o mapeamento por reflexão com destinos `*T`/`*[]T`; E05/E06 ancoraram construtores no acesso (`db.Select(ent)`, `db.Insert(ent)`) com fronteira construção × execução. Restava especificar o RF-08: o objeto que identifica um arquivo SQL incorporado, a forma de receber os recursos do consumidor, a passagem de parâmetros, a combinação de resultados nativos e mapeados e a associação a DB/Tx (§6.8). Este documento fixa esses contratos antes da implementação em E07-P02.

O consumidor continua responsável pela incorporação via `go:embed` durante a compilação (RF-08 e RF-04); a biblioteca apenas lê recursos já incorporados. Nenhuma tradução de sintaxe ou de marcadores entre bancos acontece.

## 2. Escopo e adiamentos

Decidido aqui: objeto público e recebimento dos recursos (§3); superfície de execução ancorada em DB/Tx (§4); parâmetros (§5); resultados nativos e mapeados (§6); transações (§7); erros (§8); limitações por banco (§9).

| Item                                                                                                      | Destino                                           |
| --------------------------------------------------------------------------------------------------------- | ------------------------------------------------- |
| Implementação (`internal/`, fachada em `krios.go`, testes)                                                | E07-P02                                           |
| Exemplo compilável com `go:embed` e conversão de `CreateSchema` em arquivos `.sql`                        | E07-P03                                           |
| Harness de SQL incorporado no [roteiro manual E09](E09_manual_validation.md) e evidência MySQL/PostgreSQL | E09 (a nota atual do roteiro já sinaliza E07–E08) |
| Como `krios.Config` receberá os recursos de migração (§6.4)                                               | E08 — esta entrega é independente de `Config`     |
| Divisão de um arquivo em múltiplos statements (splitting)                                                 | Extensão futura registrada (§9)                   |
| Reescrita de marcadores (`?` ↔ `$n`) para o dialeto configurado                                           | Nunca — proibido pelo RF-08                       |
| Carregamento lazy (leitura do FS a cada execução)                                                         | Não adotado; eager na construção (§3)             |

## 3. Objeto público: `SQLFile`

```go
// krios.NewSQLFile lê path em fsys e devolve o objeto imutável.
func NewSQLFile(fsys fs.FS, path string) (*SQLFile, error)

func (f *SQLFile) Name() string  // caminho, para mensagens de erro e log
func (f *SQLFile) Text() string  // conteúdo SQL carregado, para inspeção
```

- **Recebimento dos recursos:** `fs.FS` da biblioteca padrão; `embed.FS` satisfaz a interface. O caminho segue a semântica de `io/fs` (`fs.ValidPath`): separado por `/`, sem prefixo `./` nem absoluto.
- **Leitura eager:** o conteúdo é carregado uma vez na construção. O objeto é imutável depois disso, seguro para reuso concorrente e independente de `DB`/`Tx`/`Config` — o mesmo arquivo pode ser executado em várias conexões e em vários bancos que aceitem o mesmo SQL.
- **Falhas acontecem na construção**, antes de qualquer toque no banco: FS `nil`, caminho inválido, arquivo inexistente, diretório e conteúdo vazio são erros de `NewSQLFile` (§8). Construção bem-sucedida garante que os métodos de execução só enfrentam guards e erros de driver.
- O nome apenas identifica um recurso já incorporado; a distinção entre compilação e configuração do RF-04 permanece: nada aqui adiciona arquivos ao executável em tempo de execução.
- Relação com E08: este contrato é a base de carregamento reutilizável pelas migrações, mas a associação de arquivos a versões por `krios.Config` continua pendente em §6.4.

## 4. Superfície de execução: runner ancorado

Mesmo molde de `db.Select(ent)`/`db.Insert(ent)` ([E05-P01_decisions.md](E05-P01_decisions.md) §8, [E06-P01_decisions.md](E06-P01_decisions.md) §3): receptor **não genérico**, ancorado no acesso, guardando o contrato interno `executor` de [E03-P01_decisions.md](E03-P01_decisions.md) (satisfeito por `*DB` e `*Tx`) e o dialeto retido em `DB`/`Tx`:

```go
func (db *DB) File(f *SQLFile) *File   // também em *Tx

type File struct { /* executor + f + primeiro erro */ }

func (b *File) Exec(ctx context.Context, args ...any) (Result, error)
func (b *File) Query(ctx context.Context, args ...any) (*Rows, error)
func (b *File) QueryRow(ctx context.Context, args ...any) Row
func (b *File) One(ctx context.Context, dest any, args ...any) error  // destino *T
func (b *File) All(ctx context.Context, dest any, args ...any) error  // destino *[]T
```

- **Sem etapa de renderização:** não há `Build()` — o texto vem do arquivo e não é gerado pela biblioteca. `File.Text()`/`Name()` cobrem inspeção e log.
- **Parâmetros em todos os métodos:** `Exec`/`Query`/`QueryRow` recebem `args ...any`; `One`/`All` recebem `dest` primeiro e depois `args ...any`, para manter a ordem uniforme `ctx → dest → args` e permitir consultas parametrizadas mapeadas. O runner não armazena argumentos entre chamadas.
- **Guards de E03 preservados sem mecanismo novo:** os métodos passam pelo mesmo `executor`, mantendo `ErrClosed`, `ErrTxDone`, validação de contexto, serialização de chamadas na mesma `Tx`, bloqueio de `Commit` por erro imediato e por erro de leitura tardia (§3.4.5–§3.4.6 de E03-P01). `Rows`/`Row` retornados mantêm a back-reference à `Tx`.
- **Primeiro erro memorizado (first-wins):** `db.File(nil)` não faz panic nem retorna erro na cadeia; o erro é memorizado no runner e devolvido no primeiro método, antes de tocar o executor — mesma filosofia de E05/E06 (§3 de E06-P01).
- **Ordem das verificações:** erro do runner (arquivo `nil`) → guards do executor → chamada ao driver. Arquivo válido com `DB` fechado produz `ErrClosed`; arquivo `nil` produz o erro memorizado mesmo com o `DB` fechado.
- O runner é mutável em lugar (armazena o primeiro erro), não seguro para reuso concorrente do mesmo valor — cada operação monta a sua, como nos demais builders.

Esboço de consumo:

```go
//go:embed queries/*.sql
var queries embed.FS

usersByEmail, err := krios.NewSQLFile(queries, "queries/users_by_email.sql")
if err != nil {
    return err
}

// leitura com parâmetros e resultado mapeado (E04)
var users []User
if err := db.File(usersByEmail).All(ctx, &users, active); err != nil {
    return err
}

// escrita sem retorno
res, err := db.File(createUser).Exec(ctx, name, email)

// na transação selecionada
tx, err := db.Begin(ctx)
// ...
if err := tx.File(createUser).Exec(ctx, name, email); err != nil {
    return tx.Rollback(ctx)
}
return tx.Commit(ctx)
```

## 5. Parâmetros

- Assinatura variádica `args ...any`, sempre **separada do texto SQL**, conforme RF-08 e o contrato de E03 (§3.2).
- Os marcadores são os aceitos pelo **driver configurado**: `?` em SQLite e MySQL, `$1..$n` em PostgreSQL. A biblioteca não conta, não renumera e não reescreve parâmetros (diferente dos construtores de E05/E06, que geram marcadores pelo dialeto).
- O consumidor mantém conjuntos distintos de arquivos por banco quando os marcadores diferirem (RF-08); o nome do arquivo não carrega informação de dialeto.
- Quantidade ou tipo incompatível de argumentos é erro do driver, repassado sem conversão.

## 6. Resultados nativos e mapeados

A superfície cobre os dois modos pedidos por §6.8, sem estado mutuamente exclusivo (diferente do `Returning()` de E06 §7 — aqui a escolha é por método):

| Método     | Retorno           | Contrato                                                                                 |
| ---------- | ----------------- | ---------------------------------------------------------------------------------------- |
| `Exec`     | `krios.Result`    | E03 §3.1: `RowsAffected`/`LastInsertId` com flags de disponibilidade                     |
| `Query`    | `*krios.Rows`     | E03 §3.3: consumidor faz `defer Close()` e consulta `Err`                                |
| `QueryRow` | `krios.Row`       | E03: erro adiado até `Scan`; `sql.ErrNoRows` preservado                                  |
| `One`      | `error` (destino) | destino `*T`, preenchido como `Row.Scan` (E04)                                           |
| `All`      | `error` (destino) | destino `*[]T`, preenchido como `Rows.All` (E04): match por nome, subconjunto inalterado |

- `One`/`All` reaproveitam o mapeamento de [E04-P02_decisions.md](E04-P02_decisions.md): correspondência por nome de coluna, `NULL` por ponteiro/`sql.Null*`, conversões com causa preservada e `sql.ErrNoRows` em `One` sem linha.
- `One`/`All` não exigem entidade: a coluna selecionada vem do arquivo; o struct de destino é apenas preenchido — não há validação (`Entity.Validate`) nem formatação no caminho de leitura, exatamente como em `Row.Scan`/`Rows.All`.
- Falhas de leitura tardia em `One`/`All` (conversão, iteração) são registradas na `Tx` e bloqueiam `Commit` (E03 §3.4.6); a back-reference vem do executor, sem código novo.

## 7. Transações

- A associação ao arquivo é escolhida no receptor: `db.File(f)` executa fora de transação, `tx.File(f)` executa na transação — mesmo contrato de §6.3 resolvido para os construtores em [E05-P01_decisions.md](E05-P01_decisions.md).
- O objeto `SQLFile` não retém conexão nem dialeto; é dado puro, como a entidade de E04.
- Todas as regras de concorrência, finalização e rollback seguem inalteradas ([E03-P01_decisions.md](E03-P01_decisions.md) §3.4): nada desta entrega cria caminho de execução alternativo ao `executor`.

## 8. Erros

| Sentinela                | Quando                                                                        | Observação                                                             |
| ------------------------ | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `ErrFileNotFound` (nova) | `NewSQLFile` com FS `nil`, caminho inválido, recurso inexistente ou diretório | `%w` sobre `fs.ErrNotExist` quando couber; nome do caminho na mensagem |
| `ErrInvalidQuery`        | `NewSQLFile` com conteúdo vazio; `File(nil)` no primeiro método               | mesmas regras de construção de E05/E06                                 |
| (sem sentinela)          | argumentos incompatíveis, falha de driver, contextos cancelados/expirados     | repassados como em E02/E03, `%w` preservado                            |

- `ErrClosed`/`ErrTxDone` continuam indicando estado do acesso, nunca problema do arquivo.
- Não há sentinela própria para falha de execução do driver: o SQL do consumidor falha como falharia um comando escrito em código.

## 9. Diferenças por banco e limitações

| Ponto                         | SQLite                              | MySQL                       | PostgreSQL                                  |
| ----------------------------- | ----------------------------------- | --------------------------- | ------------------------------------------- |
| Marcador de parâmetro         | `?` (arquivo do consumidor)         | `?` (arquivo do consumidor) | `$1..$n` (arquivo do consumidor)            |
| Multi-statement               | comportamento do driver             | comportamento do driver     | comportamento do driver                     |
| `RowsAffected`/`LastInsertId` | contrato de E03/E06 (§6 de E06-P01) | idem                        | `LastInsertId` indisponível (`ok == false`) |

- **Sem splitting:** o conteúdo vai como está, em uma única chamada ao driver. Arquivos com vários statements dependem do suporte do driver/banco e não são separados nem validados pela biblioteca — limitação documentada; splitting é extensão futura (§2).
- **Sem tradução:** nenhum ajuste de sintaxe, marcação ou codificação; conteúdo é enviado em texto como lido do FS.
- Evidência real de integração nos três bancos segue em E07-P03 (SQLite automatizado) e E09 (MySQL/PostgreSQL manual).

## 10. Itens resolvidos e permanentes

Resolvidos nesta entrega:

- [x] Tipo público que referencia o arquivo e recebimento dos recursos (§6.8, RF-08): `krios.NewSQLFile(fs.FS, path)` com leitura eager e objeto imutável.
- [x] Passagem de parâmetros (§6.8, RF-08): variádica separada do texto, marcadores do driver, sem tradução.
- [x] Combinação de resultados nativos e mapeados (§6.8): `Exec`/`Query`/`QueryRow` de E03 + `One`/`All` de E04, escolha por método.
- [x] Associação à transação selecionada (§6.3/§6.8): `db.File(f)`/`tx.File(f)` sobre o mesmo `executor`.
- [x] Erros de recurso inexistente: sentinela `ErrFileNotFound` na construção (§8).

Permanecem em outras entregas: implementação e guards (E07-P02); exemplo `go:embed`, validação de resultados/erros/transações e conversão de `CreateSchema` (E07-P03); recursos de migração via `Config` (E08, §6.4); splitting de arquivos e evidência manual MySQL/PostgreSQL (E09/extensão futura).

## 11. Referências

- RF-08 de [0001_planning.md](0001_planning.md); §6.8 (SQL incorporado) e §6.3 (associação a transações).
- [E03-P01_decisions.md](E03-P01_decisions.md): contrato `executor`, `Result`/`Row`/`Rows`, guards e política transacional (§3.4.5–§3.4.6).
- [E04-P02_decisions.md](E04-P02_decisions.md): mapeamento por nome, destinos `*T`/`*[]T` reutilizados em `One`/`All`.
- [E05-P01_decisions.md](E05-P01_decisions.md): molde do construtor ancorado no acesso e fronteira construção × execução.
- [E06-P01_decisions.md](E06-P01_decisions.md): primeiro erro memorizado no builder, reuso do `executor`, `Returning()` como contraponto de escolha por método.
- [E08-P01_decisions.md](E08-P01_decisions.md): migrações; associação de arquivos por `Config` continua em §6.4.
- Passos E07-P01–E07-P03 em [0002_execution_plan.md](0002_execution_plan.md).
