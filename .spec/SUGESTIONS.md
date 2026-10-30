# Proposta técnica

Contratos propostos; sujeitos à aprovação. Assinaturas abaixo são esboços, não um arquivo Go compilável.

## 1. Instâncias, conexão e contexto

```go
type Dialect string
const (
    Postgres Dialect = "pg"
    MySQL    Dialect = "mysql"
    SQLite   Dialect = "sqlite"
)

type Config struct {
    Dialect Dialect
    Driver  string
    DSN     string
    Timeout time.Duration
}

func Open(ctx context.Context, cfg Config) (*DB, error)
func Wrap(db *sql.DB, dialect Dialect) (*DB, error)
func (db *DB) Close() error
func (db *DB) Begin(ctx context.Context, opts *sql.TxOptions) (*Tx, error)
func (tx *Tx) Commit() error
func (tx *Tx) Rollback() error
```

- Utilizador registra o driver; Krios depende de `database/sql` e da biblioteca padrão.
- `Dialect`: geração de SQL; `Driver`: nome registrado em `sql`; `DSN`: conexão, incluindo caminho SQLite.
- `Open`: validar → `sql.Open` → `PingContext`; fechar pool se o ping falhar.
- `Timeout < 0`: erro; `== 0`: prazo do contexto; `> 0`: limitar abertura/ping com `context.WithTimeout`.
- `Wrap`: pool emprestado; `DB.Close` invalida o wrapper, mas não fecha o pool externo.
- `Open`: pool próprio; `DB.Close` fecha o pool. Fechamento idempotente.
- `DB` compartilhável; `Tx` por unidade de trabalho, sem uso concorrente na API inicial.
- Builders e SQL direto executam exclusivamente por `Tx`; `DB` fornece conexão e início da transação.
- Contexto de `Begin` define a vida da transação; cada execução recebe seu próprio `ctx` para cancelamento/prazo local.

```go
tx, err := db.Begin(ctx, nil)
if err != nil { return err }
defer tx.Rollback() // sql.ErrTxDone após Commit é esperado

if _, err := tx.Exec(ctx, "...", args...); err != nil { return err }
return tx.Commit()
```

## 2. Builders e assinaturas

```go
type TableRef struct { Name, Alias, Schema string }
func Table(name string) TableRef
func (t TableRef) As(alias string) TableRef
func (t TableRef) InSchema(schema string) TableRef

func (tx *Tx) Select(table TableRef) SelectBuilder
func (tx *Tx) Insert(table TableRef) InsertBuilder
func (tx *Tx) Update(table TableRef) UpdateBuilder
func (tx *Tx) Delete(table TableRef) DeleteBuilder

func (q SelectBuilder) Exec(ctx context.Context) (*sql.Rows, error)
func (q SelectBuilder) First(ctx context.Context) (*sql.Row, error)
func (q SelectBuilder) Last(ctx context.Context) (*sql.Row, error)
func (q SelectBuilder) Count(ctx context.Context) (int64, error)
func (q InsertBuilder) Exec(ctx context.Context) (sql.Result, error)
func (q UpdateBuilder) Exec(ctx context.Context) (sql.Result, error)
func (q DeleteBuilder) Exec(ctx context.Context) (sql.Result, error)
```

- Builders imutáveis: métodos retornam cópias independentes; sempre reatribuir/encadear.
- Copiar slices internos ao modificar; não executar builders após finalizar `Tx`.
- `Build() (string, []any, error)`: compilar sem executar; validações antes do acesso ao banco.
- Erro de construção em `First/Last`: retorno `error`; erro de consulta/ausência: `Row.Scan`, incluindo `sql.ErrNoRows`.
- Sem argumentos posicionais opcionais; usar `Table("users").As("u").InSchema("public")`.

## 3. Retorno de escritas

- INSERT/UPDATE/DELETE: `sql.Result`; quantidade via `RowsAffected()`.
- `LastInsertId()`: capacidade do driver; propagar erro quando indisponível.
- UPDATE/DELETE não possuem conceito de “último ID”.
- Retorno explícito de colunas: `.Returning("id", ...).Query(ctx) (*sql.Rows, error)`.
- Sem suporte a `RETURNING` no banco/versão: `ErrUnsupported`; nunca inferir chave primária.
- `.Exec()` com `Returning` configurado: erro; `.Query()` sem `Returning`: erro.

## 4. Paginação, extremos e contagem

```go
func (q SelectBuilder) Paginate(offset, limit int64) SelectBuilder
type Page struct { Offset, Limit, Total int64 }
func (q SelectBuilder) ExecPage(ctx context.Context) (*sql.Rows, Page, error)
```

- `offset >= 0`, `limit > 0`; sem paginação: consulta completa.
- `Exec`: apenas linhas; `ExecPage`: exige paginação e executa contagem antes da consulta.
- `Count`: `SELECT COUNT(*) FROM (<consulta sem ORDER BY/LIMIT/OFFSET>) AS krios_count`.
- Preservar `WHERE`, `JOIN`, `DISTINCT`, `GROUP BY` e `HAVING` na contagem.
- `First/Last`: exigir `OrderBy`, rejeitar paginação prévia, aplicar `LIMIT 1`.
- `Last`: inverter todas as direções e a ordenação explícita de NULLs.
- Ordem estável exige desempate único informado pelo utilizador.
- Consistência entre total e linhas depende do isolamento da transação; Krios não altera o isolamento implicitamente.

## 5. Expressões e condições

```go
type Expr interface { /* AST selada pelo pacote */ }
type Predicate interface { /* AST selada pelo pacote */ }

func Column(name string) Expr
func Value(value any) Expr
func Eq(column string, value any) Predicate
func EqExpr(left, right Expr) Predicate
func Exists(query SelectBuilder) Predicate
func NotExists(query SelectBuilder) Predicate

// JOIN: identificador à direita, não parâmetro textual.
EqExpr(Column("u.id"), Column("p.user_id"))
```

- `Eq/Ne` com `nil`: `IS NULL`/`IS NOT NULL`; demais comparações com `nil`: erro.
- `Eq` sempre recebe valor; comparação entre expressões exige `EqExpr`.
- `In()` vazio: falso; `NotIn()` vazio: verdadeiro; ambos representados na AST.
- `And/Or` vazios: erro; WHERE ausente permitido somente em SELECT.
- Chamadas sucessivas de `Where` combinadas por AND; agrupamentos preservados com parênteses.
- `Between`: exatamente dois valores; `Like/NotLike`: padrão textual parametrizado.
- `time.Time`, `[]byte`, `sql.Null*` e `driver.Valuer`: encaminhar a `database/sql`, sem interpolação/conversão textual.
- Agregações: `Count`, `Avg`, `Sum`, `Min`, `Max`, com `.As(alias)`; filtro agregado via `Having`.

## 6. Segurança e dialetos

- AST separa identificadores, parâmetros, operadores e subconsultas.
- Valores somente em argumentos; PostgreSQL: `$1...$n`; MySQL/SQLite: `?`.
- Numeração PostgreSQL compartilhada por toda a compilação, incluindo subconsultas.
- Identificadores: componentes qualificados separados, escape por componente; `*` somente em posições permitidas.
- PostgreSQL/SQLite: aspas duplas; MySQL: crases. Nunca aceitar SQL como nome de coluna.
- UPDATE/DELETE: rejeitar WHERE ausente, vazio ou constante verdadeira após simplificação da AST.
- Sem operação destrutiva global nos builders iniciais; SQL direto permanece explícito e parametrizado.
- `RightJoin` corrige `RigthJoin`; substituir `OuterJoin` ambíguo por `FullJoin`.
- Capacidades por dialeto/versão: schemas, JOINs, `RETURNING`, NULLs, DDL transacional.
- Operação não suportada: `ErrUnsupported` antes de executar; sem emulação silenciosa.
- SQLite: `Schema` identifica banco anexado; MySQL: database; PostgreSQL: schema.

## 7. SQL direto e arquivos incorporados

```go
func (tx *Tx) Query(ctx context.Context, sqlText string, args ...any) (*sql.Rows, error)
func (tx *Tx) Exec(ctx context.Context, sqlText string, args ...any) (sql.Result, error)
func (tx *Tx) QueryFile(ctx context.Context, files fs.FS, path string, args ...any) (*sql.Rows, error)
func (tx *Tx) ExecFile(ctx context.Context, files fs.FS, path string, args ...any) (sql.Result, error)
```

- Aplicação declara `//go:embed` e fornece `embed.FS`; leitura via `fs.ReadFile`.
- SQL direto usa placeholders nativos do dialeto; sem reescrita ou detecção do tipo de comando.
- Sem inferência de paginação, IDs ou esquema de resultado; SELECT paginado recebe contagem explícita da aplicação.
- Arquivos não são divididos por `;`; execução de múltiplas instruções depende do driver.

## 8. Migrações

```go
type Migration struct {
    Version int64
    Up      []string // caminhos; uma instrução SQL por arquivo
    Down    []string // ordem explícita de execução
}
type MigrationConfig struct {
    TargetVersion  int64
    Files          fs.FS
    Migrations     []Migration
    AllowNonAtomic bool // padrão false
}
func Migrate(ctx context.Context, db Config, cfg MigrationConfig) error
```

- `Migrate`: abrir pool próprio → adquirir exclusividade → validar histórico → executar → liberar → fechar.
- Versões positivas e únicas; `TargetVersion == 0`: remover todas; outro destino deve existir no manifesto.
- Histórico aplicado deve ser prefixo do manifesto ordenado; rejeitar divergência.
- Up: versões não aplicadas até o destino, crescente; Down: versões acima do destino, decrescente.
- Tabela `migrations`: `version` PK, `checksum`, `applied_at`; tipos SQL por dialeto.
- Checksum SHA-256 sobre versão, caminhos e conteúdos Up/Down, com campos delimitados por comprimento.
- Mudança de migração aplicada, versão desconhecida ou Down ausente quando necessário: erro antes da execução.
- Exclusividade durante toda a operação: lock de sessão PostgreSQL/MySQL; transação de escrita SQLite. Fixar conexão com `sql.Conn` quando necessário.
- Modo atômico: todas as etapas e registros na mesma transação; qualquer falha → rollback.
- DDL com commit implícito, como no MySQL, impede garantia geral de rollback: rejeitar no modo atômico conservador.
- Modo não atômico exige `AllowNonAtomic`; registrar estado persistente em `migration_state` antes de executar cada etapa: versão, direção, índice, `dirty`.
- Estado `dirty` bloqueia novas migrações; recuperação manual após inspeção, sem repetir automaticamente instrução de resultado incerto.
- Up concluído: inserir registro; Down concluído: excluir registro; limpar estado somente após confirmar sucesso.
- Scripts por dialeto quando necessário; sem tradução automática de SQL de migração.

## 9. Erros e recursos

- Erros sentinela: `ErrInvalidConfig`, `ErrInvalidQuery`, `ErrMissingWhere`, `ErrUnsupported`, `ErrClosed`, `ErrMigrationDirty`.
- Preservar causas com `%w`; permitir `errors.Is/As`, incluindo erros de contexto e `sql.ErrTxDone`.
- `Rows`: utilizador chama `Close`, percorre `Next/Scan` e verifica `Err` antes de `Commit`.
- `Row`: verificar `Scan`; ausência de registro é `sql.ErrNoRows`.
- Falha durante operação transacional: chamador executa rollback; não continuar assumindo transação utilizável.
- Erros e logs não incluem DSN nem valores de parâmetros.

## 10. Validação

- Testes de compilação SQL por dialeto: placeholders, identificadores, agrupamentos e argumentos.
- Casos críticos: WHERE vazio/verdadeiro, `nil`, IN vazio, colunas em JOIN e operação não suportada.
- Integração nos três bancos: commit/rollback, cancelamento, paginação, retorno de escritas e migrações concorrentes/com falha.
- Drivers somente nos testes de integração; executar também `go test -race`.
- Implementação: conexão/Tx → AST/dialetos → CRUD → SQL/FS → migrações.
