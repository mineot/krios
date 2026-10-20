# Manual técnico do Krios

Biblioteca Go sobre `database/sql` para SQLite3, MySQL e PostgreSQL. Importação: `github.com/mineot/krios`.

Os exemplos são trechos de funções que retornam `error`, salvo declarações de tipos e recursos. `ctx` é um `context.Context`; `db` é um `*krios.DB` aberto; `userEnt` e `taskEnt` são os descritores definidos na seção de entidades. Nas assinaturas, os tipos do pacote aparecem sem o prefixo `krios.`.

## Sumário

1. [Assuntos transversais](#1-assuntos-transversais)
2. [Configuração](#2-configuração)
3. [Migrações](#3-migrações)
4. [Entidades](#4-entidades)
5. [Consultas — Select](#5-consultas--select)
6. [Inserções — Insert](#6-inserções--insert)
7. [Alterações — Update](#7-alterações--update)
8. [Exclusões — Delete](#8-exclusões--delete)
9. [Condições e expressões](#9-condições-e-expressões)
10. [Queries embutidas](#10-queries-embutidas)
11. [Queries diretas](#11-queries-diretas)

## 1. Assuntos transversais

### Execução, contexto e concorrência

- `DB` aceita uso concorrente. Após `DB.Close`, novas execuções e `Begin` retornam `ErrClosed`.
- `Select`, `Insert`, `Update`, `Delete` e `File` são mutáveis: use uma instância por operação, sem compartilhamento concorrente. `Entity` deve ser tratado como somente leitura; `SQLFile` é imutável.
- `Build` gera SQL e argumentos, sem executar. Os erros de construção precedem o acesso ao executor; construir um builder não verifica se o banco está aberto.
- Os métodos de execução recebem `context.Context`. Cancelamento e deadline são preservados por wrapping; a interrupção de operações bloqueantes depende do driver.
- `DB` e `Tx` oferecem os mesmos builders e métodos de SQL direto. O receptor determina se a execução ocorre fora ou dentro da transação.

### Transações

| Método                                       | Contrato                                                                                                                                          |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DB.Begin(ctx context.Context) (*Tx, error)` | Inicia uma transação. O contexto vigora por toda a sua duração; seu cancelamento provoca rollback pelo `database/sql`.                            |
| `Tx.Commit(ctx context.Context) error`       | Confirma se não houver falha registrada. Caso contrário, tenta rollback e retorna a primeira falha, preservando também eventual erro de rollback. |
| `Tx.Rollback(ctx context.Context) error`     | Tenta rollback mesmo com contexto cancelado. Após finalização retorna `ErrTxDone`.                                                                |

```go
tx, err := db.Begin(ctx)

if err != nil {
    return err
}

defer tx.Rollback(ctx) // o erro ErrTxDone após Commit é ignorado pelo defer

if _, err := tx.Delete(taskEnt).
    Where(taskEnt.Col("done").Eq(true)).
    Exec(ctx); err != nil {
    return err
}

return tx.Commit(ctx)
```

A primeira falha de execução ou de leitura observada em `Row`/`Rows` bloqueia `Commit`. `sql.ErrNoRows` e observações de transação já encerrada não são registradas como novas falhas. Erros de construção anteriores ao executor não entram nesse registro.

Sem falha registrada, um contexto cancelado passado a `Commit` rejeita a chamada antes da finalização nativa; outra tentativa é possível se a transação continuar ativa. Após qualquer tentativa nativa de commit/rollback, mesmo com erro, o wrapper fica terminal. Uma falha de commit não determina o resultado remoto. O contexto não interrompe a finalização nativa.

`Tx` serializa execução e finalização, sem garantir ordem entre goroutines. O consumo de `Row`/`Rows` não é serializado: consuma ou feche os resultados antes da próxima operação e antes de finalizar.

### Resultados e leitura

| Tipo / método                         | Retorno e comportamento                                                                                                     |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `Result`                              | Interface retornada por execuções sem linhas.                                                                               |
| `Result.RowsAffected() (int64, bool)` | Quantidade afetada e disponibilidade da informação.                                                                         |
| `Result.LastInsertId() (int64, bool)` | ID gerado e disponibilidade da informação.                                                                                  |
| `Row.Scan(dest ...any) error`         | Consome a primeira linha; ausência retorna `sql.ErrNoRows`. Não verifica unicidade. Erros adiados chegam aqui.              |
| `Rows.Next() bool`                    | Avança o iterador; `false` indica término ou erro.                                                                          |
| `Rows.Scan(dest ...any) error`        | Lê a linha atual por posição.                                                                                               |
| `Rows.Columns() ([]string, error)`    | Nomes das colunas na ordem do resultado.                                                                                    |
| `Rows.Err() error`                    | Erro de iteração; conferir após o loop.                                                                                     |
| `Rows.Close() error`                  | Fecha o iterador; idempotente.                                                                                              |
| `Rows.All(dest any) error`            | Consome as linhas restantes em `*[]T`, com `T` struct; substitui o slice apenas no sucesso. Sem linhas, produz slice vazio. |

Em `Result`, `false` significa informação indisponível; `0, true` é válido. O erro original ao obter a informação do driver não é exposto.

| Forma de leitura                         | Mapeamento                                                                                                                                 |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `Row.Scan(&struct)` e métodos `One`      | Posicional: quantidade e ordem das colunas devem coincidir com os campos mapeados.                                                         |
| `Row.Scan(&a, &b)` / `Rows.Scan(&a, &b)` | Posicional, conforme `database/sql`.                                                                                                       |
| `Rows.All` e métodos `All`               | Por nome: correspondência exata, depois sem distinção de maiúsculas. Colunas desconhecidas são descartadas; campos ausentes ficam zerados. |

Tipos que implementam `sql.Scanner`, como `sql.NullString`, mantêm leitura escalar em `Row.Scan`. Structs sem campos exportados, como `time.Time`, também não são tratadas como entidades.

Ao obter `*Rows`, use `defer rows.Close()`. `Rows.All` fecha ao concluir o consumo, inclusive em falhas de scan/iteração, mas a implementação atual pode retornar antes do fechamento se o destino ou os metadados forem inválidos. Forneça destinos válidos aos atalhos `All` e `Paginate`.

### Erros

Use `errors.Is` para sentinelas e `errors.As` para erros estruturados; não compare mensagens. Erros do driver também podem ser retornados com contexto.

| Sentinela             | Situação                                                                                                   |
| --------------------- | ---------------------------------------------------------------------------------------------------------- |
| `ErrInvalidConfig`    | Configuração ausente ou inconsistente.                                                                     |
| `ErrDriverNotLoaded`  | Nome do driver não registrado em `database/sql`.                                                           |
| `ErrConnectionFailed` | Falha de abertura/conectividade; preserva a causa. Cancelamento/deadline usa os erros de `context`.        |
| `ErrClosed`           | Execução após fechamento do `DB`.                                                                          |
| `ErrTxDone`           | Transação terminal; preserva `sql.ErrTxDone` quando essa é a causa.                                        |
| `ErrInvalidEntity`    | Mapeamento, registro ou destino inválido; falha de `driver.Valuer`.                                        |
| `ErrValidation`       | Campo rejeitado por `Validator`.                                                                           |
| `ErrFormat`           | Falha de `Formatter`.                                                                                      |
| `ErrInvalidQuery`     | Construção inválida, operação incompatível ou preparação inválida de migrações.                            |
| `ErrFileNotFound`     | Recurso SQL ausente ou inválido; preserva `fs.ErrNotExist` quando aplicável.                               |
| `ErrPartialMigration` | Histórico parcial bloqueando migração ou falha de aplicação parcial. Contexto em `*PartialMigrationError`. |

```go
if errors.Is(err, krios.ErrInvalidQuery) {
    return fmt.Errorf("consulta inválida: %w", err)
}
```

### Dialetos

| Recurso                  | SQLite                 | MySQL                    | PostgreSQL                             |
| ------------------------ | ---------------------- | ------------------------ | -------------------------------------- |
| Placeholder dos builders | `?`                    | `?`                      | `$1`, `$2`, …                          |
| `Returning()`            | Suportado pelo dialeto | `ErrInvalidQuery`        | Suportado pelo dialeto                 |
| `LastInsertId`           | Conforme driver        | Conforme driver          | Indisponível; usar `Returning().One()` |
| Transação de migração    | Por versão             | Por arquivo transacional | Por versão                             |

SQL embutido e SQL direto são enviados sem tradução de sintaxe ou placeholders. O suporte efetivo também depende da versão do banco e do driver.

## 2. Configuração

```sh
go get github.com/mineot/krios
```

O consumidor instala e importa o driver escolhido; Krios não importa drivers.

| Banco      | Import do driver                     | `Config.Driver` | `Config.Type` (`DBType`, baseado em `string`) |
| ---------- | ------------------------------------ | --------------- | --------------------------------------------- |
| SQLite     | `_ "modernc.org/sqlite"`             | `"sqlite"`      | `DBTypeSQLite` (`"sqlite"`)                   |
| MySQL      | `_ "github.com/go-sql-driver/mysql"` | `"mysql"`       | `DBTypeMySQL` (`"mysql"`)                     |
| PostgreSQL | `_ "github.com/jackc/pgx/v5/stdlib"` | `"pgx"`         | `DBTypePostgres` (`"postgres"`)               |

```go
cfg := krios.Config{
    Type:    krios.DBTypeSQLite,
    Driver:  "sqlite",
    SQLite:  &krios.SQLiteConfig{DBPath: "app.db"},
    Timeout: 5 * time.Second,
}

db, err := krios.Open(ctx, cfg)

if err != nil {
    return err
}

defer db.Close()
```

| Campo de `Config` | Tipo                | Regra                                                        |
| ----------------- | ------------------- | ------------------------------------------------------------ |
| `Type`            | `DBType`            | Um dos três bancos suportados.                               |
| `Driver`          | `string`            | Nome registrado pelo driver; obrigatório.                    |
| `SQLite`          | `*SQLiteConfig`     | Obrigatório somente para SQLite.                             |
| `MySQL`           | `*MySQLConfig`      | Obrigatório somente para MySQL.                              |
| `Postgres`        | `*PostgresConfig`   | Obrigatório somente para PostgreSQL.                         |
| `Timeout`         | `time.Duration`     | Orçamento de abertura; zero aplica 10s; negativo é inválido. |
| `Migrations`      | `*MigrationsConfig` | Opcional; configurado antes de `Open`.                       |

Somente a configuração do banco selecionado pode estar preenchida.

| Tipo             | Campos e validação                                                                                                  |
| ---------------- | ------------------------------------------------------------------------------------------------------------------- |
| `SQLiteConfig`   | `DBPath string`: caminho relativo ou absoluto, obrigatório.                                                         |
| `MySQLConfig`    | `Host`, `User`, `Password`, `DBName`: `string`, obrigatórios. `Port int`: zero usa 3306; explícito entre 1 e 65535. |
| `PostgresConfig` | Mesmos campos obrigatórios do MySQL; `Port` zero usa 5432.                                                          |

| Função / método                                      | Comportamento                                                                                                              |
| ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `Config.Validate() error`                            | Valida banco, driver, campos, porta, timeout e configuração de migrações. Falha com `ErrInvalidConfig`.                    |
| `Config.EffectiveTimeout() time.Duration`            | Retorna `Timeout`, ou 10s quando zero. Não substitui a validação.                                                          |
| `Open(ctx context.Context, cfg Config) (*DB, error)` | Valida, verifica driver registrado, abre e faz ping. Usa o menor prazo entre contexto e timeout; libera recursos em falha. |
| `DB.Close() error`                                   | Libera o acesso; chamadas posteriores de `Close` retornam `nil`.                                                           |

## 3. Migrações

Configuração em `Config.Migrations`; execução explícita após `Open`. A abertura não aplica migrações.

```go
// Declaração no pacote; requer import "embed" e arquivos existentes.
//go:embed migrations/*.sql
var migrationFS embed.FS
```

```go
cfg.Migrations = &krios.MigrationsConfig{
    FS:             migrationFS,
    TargetVersion:  1,
    ManagedObjects: []string{"users", "tasks"},
    Versions: []krios.MigrationVersion{
        {
            Version: 1,
            Up:      []string{"migrations/001_users.up.sql", "migrations/001_tasks.up.sql"},
            Down:    []string{"migrations/001_tasks.down.sql", "migrations/001_users.down.sql"},
        },
    },
}

db, err := krios.Open(ctx, cfg)

if err != nil {
    return err
}

defer db.Close()

plan, err := db.RunMigrations(ctx)

if err != nil {
    return err
}

fmt.Println(plan.CurrentVersion, plan.TargetVersion, len(plan.Steps))
```

| Tipo                    | Campos                                                                                                                                                                                       |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `MigrationsConfig`      | `FS fs.FS`; `TargetVersion int` (zero solicita reversão total); `Versions []MigrationVersion`; `ManagedObjects []string` (tabelas verificadas quando não há histórico).                      |
| `MigrationVersion`      | `Version int` positivo; `Up []string` e `Down []string`, na ordem de execução.                                                                                                               |
| `MigrationPlan`         | `CurrentVersion int`, `TargetVersion int`, `Steps []MigrationStep`.                                                                                                                          |
| `MigrationStep`         | `Version int`, `Direction string` (`"up"`/`"down"`), `File *SQLFile`, `Transactional bool`.                                                                                                  |
| `PartialMigrationError` | `Direction string`, `TargetVersion int`, `LastVersion int` (última versão aplicada), `LastFile string` (arquivo conhecido), `Status string` (`"in_progress"`/`"failed"`) e `Message string`. |

| Método                                                          | Comportamento                                                                                                                                                           |
| --------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `MigrationsConfig.Validate() error`                             | Exige FS, objetos e versões; versões positivas, únicas e crescentes; nomes de arquivos não vazios nem duplicados dentro de uma direção da versão. Não carrega arquivos. |
| `DB.Migrations() *MigrationsConfig`                             | Retorna a configuração retida, ou `nil`.                                                                                                                                |
| `DB.RunMigrations(ctx context.Context) (*MigrationPlan, error)` | Inspeciona histórico, carrega recursos necessários, prepara e aplica o plano. Sem configuração: `ErrInvalidConfig`.                                                     |
| `PartialMigrationError.Error() string`                          | Mensagem com contexto da falha.                                                                                                                                         |
| `PartialMigrationError.Unwrap() []error`                        | Expõe `ErrPartialMigration` e eventual causa.                                                                                                                           |

Regras de execução:

- A declaração determina versão e ordem; prefixos dos arquivos são apenas convenção. `up` percorre versões crescentes; `down`, decrescentes, preservando a ordem dos arquivos em cada lista.
- Uma direção vazia pode passar na validação estrutural, mas falha com `ErrInvalidQuery` quando necessária ao percurso.
- A preparação consulta o banco, mas não executa SQL de migração. Falha nessa etapa retorna plano `nil`; falha na aplicação retorna o plano preparado. Plano sem passos não executa alterações.
- O histórico usa a tabela `migrations`. Registros `in_progress`/`failed` bloqueiam nova execução com `ErrPartialMigration`; a resolução é explícita pelo consumidor.
- Na implementação atual, tabela gerenciada existente em banco sem histórico bloqueia com `ErrInvalidQuery`.
- SQLite/PostgreSQL usam uma transação por versão. MySQL usa uma transação por arquivo somente se todos os statements forem classificados como transacionais; arquivos não transacionais executam sem rollback, com histórico de progresso/falha.
- O texto de cada arquivo é enviado inteiro ao driver. A análise de statements para decidir atomicidade não os divide em execuções independentes; prefira um statement por arquivo.

```go
_, err := db.RunMigrations(ctx)

var partial *krios.PartialMigrationError

if errors.As(err, &partial) {
    fmt.Println(partial.Direction, partial.TargetVersion, partial.LastVersion,
        partial.LastFile, partial.Status, partial.Message)
}

return err
```

## 4. Entidades

### Modelos e mapeamento

```go
type User struct {
    ID    int64   `krios:"id,pk"`
    Name  string  `krios:"name"`
    Email string  `krios:"email"`
    Bio   *string `krios:"bio"`
}

func (User) TableName() string { return "users" }

type Task struct {
    ID      int64 `krios:"id,pk"`
    Title   string
    OwnerID int64
    Done    bool
}

func (Task) TableName() string { return "tasks" }
```

```go
userEnt, err := krios.EntityOf[User]()

if err != nil {
    return err
}

taskEnt, err := krios.EntityOf[Task]()

if err != nil {
    return err
}
```

`EntityOf[T any]() (*Entity, error)` deriva e cacheia o descritor por tipo; aceita struct ou ponteiro para struct. A entidade não cria tabelas.

| Declaração                        | Efeito                                                                                           |
| --------------------------------- | ------------------------------------------------------------------------------------------------ |
| `TableName() string`              | Sobrescreve o nome da tabela; sem método, usa snake_case do tipo.                                |
| Campo sem tag ou tag vazia        | Coluna em snake_case: `OwnerID` → `owner_id`.                                                    |
| `krios:"nome"`                    | Define o nome da coluna.                                                                         |
| `krios:"nome,pk"` / `krios:",pk"` | Marca chave primária; permite múltiplos campos. Não cria chave nem omite coluna automaticamente. |
| `krios:"-"`                       | Ignora o campo.                                                                                  |
| Campo não exportado sem tag       | Ignorado.                                                                                        |
| `*T` / `sql.Null*`                | Representação de valores anuláveis, conforme leitura e política de escrita.                      |

`ErrInvalidEntity`: tipo não struct, nenhum campo mapeado, campo embutido, campo não exportado com tag, opção de tag desconhecida, nome de coluna duplicado (inclusive sem distinção de maiúsculas) ou `TableName` inválido/vazio.

### Metadados de `Entity`

| Método                                  | Retorno                                                                                                                                                     |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Table() string`                        | Nome da tabela.                                                                                                                                             |
| `Type() reflect.Type`                   | Tipo da struct.                                                                                                                                             |
| `Columns() []string`                    | Cópia dos nomes de coluna em ordem de declaração.                                                                                                           |
| `PrimaryKeys() []string`                | Cópia das colunas `pk`; vazia se não houver.                                                                                                                |
| `FieldIndexes() []int`                  | Cópia dos índices dos campos na struct, em ordem de mapeamento.                                                                                             |
| `FieldIndex(column string) (int, bool)` | Posição no mapeamento, por nome exato ou sem distinção de maiúsculas; ausência: `(0, false)`. Para obter o índice na struct, use `FieldIndexes()[posição]`. |
| `Col(name string) Column`               | Referência à coluna; existência validada ao construir SQL.                                                                                                  |

### Validação e apresentação

| API                                                    | Contrato                                                                                          |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------------- |
| `Validator`                                            | Interface `Validate() error`, implementada pelo tipo de um campo.                                 |
| `Formatter`                                            | Interface `Format() (string, error)`, implementada pelo tipo de um campo.                         |
| `Entity.Validate(record any) error`                    | Aceita `T`/`*T`; valida campos em ordem, pula nil e para na primeira falha (`ErrValidation`).     |
| `Entity.ValidateAll(records any) error`                | Aceita `[]T`/`*[]T`; valida registros em ordem e para na primeira falha.                          |
| `Entity.Format(record any) (map[string]string, error)` | Mapa coluna → texto; usa `Formatter` ou `fmt.Sprint` do valor; nil vira `""`. Falha: `ErrFormat`. |

Os métodos de campo podem ter receptor por valor ou ponteiro. Registro incompatível retorna `ErrInvalidEntity`. Formatação é apenas apresentação: não altera os valores gravados.

```go
type Email string

func (e Email) Validate() error {
    if !strings.Contains(string(e), "@") {
        return fmt.Errorf("email inválido: %q", e)
    }

    return nil
}

func (e Email) Format() (string, error) {
    return strings.ToLower(string(e)), nil
}
// Use Email como tipo do campo para ativar essas interfaces.
```

### Valores de escrita

`Entity.WriteValues(record any)` retorna `([]WriteValue, error)` em ordem de declaração. `WriteValue` é um tipo interno exposto pelo retorno, sem alias `krios.WriteValue`; seus campos acessíveis são `Column string`, `Value any` e `Absent bool`. Use inferência de tipo.

| Valor do campo                                                     | `Insert`                      | `Update`               |
| ------------------------------------------------------------------ | ----------------------------- | ---------------------- |
| Não ponteiro, inclusive zero                                       | Grava o valor                 | Grava o valor          |
| Ponteiro não nil                                                   | Grava o valor apontado        | Grava o valor apontado |
| Ponteiro nil (`Absent=true`)                                       | Grava NULL                    | Não altera a coluna    |
| `driver.Valuer` retornando nil, como `sql.NullString{Valid:false}` | Grava NULL                    | Grava NULL             |
| Coluna em `Omit`                                                   | Deixa o banco aplicar default | Preserva a coluna      |

Falha de `driver.Valuer` retorna `ErrInvalidEntity`. `WriteValues` extrai valores; a validação é uma chamada separada, executada automaticamente por `SetValues`.

`Insert.SetValues` e `Update.SetValues` validam todos os campos mapeados e extraem os valores na chamada, antes de aplicar `Omit`. Uma chamada válida posterior substitui os valores, mas não apaga a primeira falha já registrada. Os valores extraídos são retidos pelo builder; não há cópia profunda de dados mutáveis como slices.

## 5. Consultas — Select

Construtores: `DB.Select(ent *Entity) *Select` e `Tx.Select(ent *Entity) *Select`.

```go
var users []User

err := db.Select(userEnt).
    Only("id", "name").
    Where(userEnt.Col("name").Like(krios.StartsWith, "Ana")).
    OrderBy(userEnt.Col("id").Asc()).
    Limit(20).
    All(ctx, &users)

if err != nil {
    return err
}
```

### Construção

Todos os métodos abaixo retornam `*Select`.

| Método                                           | Comportamento                                                                                         |
| ------------------------------------------------ | ----------------------------------------------------------------------------------------------------- |
| `Only(names ...string)`                          | Acrescenta colunas da entidade base. Sem projeção explícita, seleciona todas as colunas da base.      |
| `Fields(exprs ...Expr)`                          | Acrescenta colunas/agregados; sem `Only`, substitui a seleção padrão; com `Only`, segue suas colunas. |
| `Distinct()`                                     | Emite `SELECT DISTINCT`.                                                                              |
| `Where(conditions ...Condition)`                 | Acumula filtros com AND; sem filtros não emite WHERE.                                                 |
| `Join(ent *Entity, kind JoinKind, on Condition)` | Acrescenta JOIN na ordem das chamadas. Não aceita tabela repetida, inclusive a base.                  |
| `GroupBy(cols ...Column)`                        | Acumula colunas de agrupamento.                                                                       |
| `OrderBy(sorts ...Sort)`                         | Acumula critérios de ordenação.                                                                       |
| `Limit(n int)`                                   | Última chamada vence; zero retorna nenhuma linha; negativo é inválido.                                |
| `Offset(n int)`                                  | Última chamada vence; negativo é inválido; positivo exige `Limit`.                                    |

`JoinKind` aceita `InnerJoin` (somente correspondências) e `LeftJoin` (preserva a base, com NULL no lado sem correspondência). JOIN não adiciona automaticamente colunas à projeção.

Colunas desconhecidas, tabelas externas à consulta, projeções/aliases duplicados, aliases somente com espaços e tipos de JOIN inválidos produzem `ErrInvalidQuery` antes da execução. Não há alias de tabela para self-join.

### Execução

| Método                                                                         | Retorno / comportamento                                                                                                        |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| `Build() (string, []any, error)`                                               | SQL e argumentos em ordem textual; não executa.                                                                                |
| `Rows(ctx context.Context) (*Rows, error)`                                     | Iterador; fechar e conferir `Err`.                                                                                             |
| `Row(ctx context.Context) Row`                                                 | Resultado consumido com `Scan`.                                                                                                |
| `All(ctx context.Context, dest any) error`                                     | Todas as linhas em `*[]T`, por nome.                                                                                           |
| `One(ctx context.Context, dest any) error`                                     | Primeira linha via `Row.Scan`; struct por posição; ausência: `sql.ErrNoRows`.                                                  |
| `Count(ctx context.Context) (int64, error)`                                    | Conta sobre uma subconsulta, preservando `DISTINCT`/`GROUP BY` e ignorando ordenação, limite e offset.                         |
| `Paginate(ctx context.Context, limit, offset int, dest any) (Paginate, error)` | Conta e lê a página em `*[]T`; exige `limit > 0` e `offset >= 0`. Não altera o builder; ignora seus limites para essa chamada. |

```go
type TaskCount struct {
    OwnerID int64 `krios:"owner_id"`
    Total   int64 `krios:"total"`
}

var counts []TaskCount

err := db.Select(taskEnt).
    Join(userEnt, krios.InnerJoin, taskEnt.Col("owner_id").Eq(userEnt.Col("id"))).
    Fields(taskEnt.Col("owner_id"), krios.Count().As("total")).
    GroupBy(taskEnt.Col("owner_id")).
    OrderBy(krios.Count().Desc()).
    All(ctx, &counts)

if err != nil {
    return err
}
```

### Paginação

```go
var users []User

page, err := db.Select(userEnt).
    OrderBy(userEnt.Col("id").Asc()).
    Paginate(ctx, 10, 20, &users)

if err != nil {
    return err
}

fmt.Println(page.Total, page.Page, page.Pages, page.HasNext, page.HasPrev)
```

| Campo de `Paginate` | Tipo / cálculo                                              |
| ------------------- | ----------------------------------------------------------- |
| `Total`             | `int64`: total da consulta sem paginação.                   |
| `Limit`, `Offset`   | `int`: valores usados para ler a página.                    |
| `Page`              | `int`: `offset/limit + 1`.                                  |
| `Pages`             | `int`: teto de `Total/Limit`; zero se não houver registros. |
| `HasNext`           | `bool`: `offset + limit < Total`.                           |
| `HasPrev`           | `bool`: `offset > 0`.                                       |

Contagem e página são duas consultas. Em `DB`, escritas concorrentes podem gerar divergência; em `Tx`, compartilham a transação, mas a consistência do snapshot depende do isolamento do banco. Em erro, o retorno `Paginate` é zero.

## 6. Inserções — Insert

Construtores: `DB.Insert(ent *Entity) *Insert` e `Tx.Insert(ent *Entity) *Insert`. Cada builder insere uma linha.

| Método                                      | Retorno / comportamento                                                                                |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `SetValues(record any) *Insert`             | Recebe `T`/`*T`; obrigatório. Validação e extração conforme [valores de escrita](#valores-de-escrita). |
| `Omit(names ...string) *Insert`             | Acumula colunas excluídas da escrita. Nomes desconhecidos ou duplicados são inválidos.                 |
| `Returning() *Insert`                       | Solicita todas as colunas mapeadas, em ordem de declaração.                                            |
| `Build() (string, []any, error)`            | Gera SQL e argumentos; exige ao menos uma coluna após `Omit`.                                          |
| `Exec(ctx context.Context) (Result, error)` | Executa sem `Returning`; rejeita seu uso com `Returning`.                                              |
| `One(ctx context.Context, dest any) error`  | Exige `Returning`; lê a linha inserida por posição.                                                    |

```go
user := User{Name: "Ana", Email: "ana@example.com"}

res, err := db.Insert(userEnt).SetValues(&user).Omit("id").Exec(ctx)

if err != nil {
    return err
}

if id, ok := res.LastInsertId(); ok {
    user.ID = id
}
```

SQLite/PostgreSQL, com leitura da linha gerada:

```go
var created User

if err := db.Insert(userEnt).
	SetValues(&User{Name: "Bia", Email: "bia@example.com"}).
	Omit("id").Returning().One(ctx, &created); err != nil {
    return err
}
```

O ID não é preenchido automaticamente no registro passado a `SetValues`. Falhas de construção: `ErrInvalidQuery`, `ErrInvalidEntity` ou `ErrValidation`; `One` preserva `sql.ErrNoRows`.

## 7. Alterações — Update

Construtores: `DB.Update(ent *Entity) *Update` e `Tx.Update(ent *Entity) *Update`. Pode afetar várias linhas; exige valores, ao menos uma coluna efetiva e ao menos uma condição.

| Método                                      | Retorno / comportamento                                                                         |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `SetValues(record any) *Update`             | Recebe `T`/`*T`; aplica a política de escrita. Zeros são gravados; ponteiros nil são ignorados. |
| `Where(conditions ...Condition) *Update`    | Acumula filtros com AND; só pode referenciar a tabela base.                                     |
| `Omit(names ...string) *Update`             | Acumula colunas preservadas; nomes desconhecidos ou duplicados são inválidos.                   |
| `Returning() *Update`                       | Solicita todas as colunas das linhas alteradas.                                                 |
| `Build() (string, []any, error)`            | Gera SQL; argumentos das atribuições precedem os filtros.                                       |
| `Exec(ctx context.Context) (Result, error)` | Executa sem `Returning`.                                                                        |
| `All(ctx context.Context, dest any) error`  | Exige `Returning`; preenche `*[]T` por nome.                                                    |

```go
res, err := db.Update(taskEnt).
    SetValues(&Task{Done: true}).
    Omit("id", "title", "owner_id").
    Where(taskEnt.Col("id").Eq(int64(1))).
    Exec(ctx)

if err != nil {
    return err
}

affected, available := res.RowsAffected()
fmt.Println(affected, available)
```

Para obter as linhas, substitua `Exec(ctx)` por `Returning().All(ctx, &updated)`, com `var updated []Task`. MySQL não suporta esse caminho. Sem `Omit`, o exemplo também gravaria zero em `id`/`owner_id` e string vazia em `title`.

Falhas de construção: `ErrInvalidQuery`, `ErrInvalidEntity` ou `ErrValidation`. Não existe método para dispensar `Where`; uma condição válida ainda pode selecionar todas as linhas.

## 8. Exclusões — Delete

Construtores: `DB.Delete(ent *Entity) *Delete` e `Tx.Delete(ent *Entity) *Delete`.

| Método                                      | Retorno / comportamento                                               |
| ------------------------------------------- | --------------------------------------------------------------------- |
| `Where(conditions ...Condition) *Delete`    | Obrigatório; acumula AND; referências somente à tabela base.          |
| `Returning() *Delete`                       | Solicita todas as colunas das linhas removidas.                       |
| `Build() (string, []any, error)`            | Gera SQL e argumentos; construção inválida retorna `ErrInvalidQuery`. |
| `Exec(ctx context.Context) (Result, error)` | Executa sem `Returning`; quantidade em `RowsAffected`.                |
| `All(ctx context.Context, dest any) error`  | Exige `Returning`; lê as linhas removidas em `*[]T`.                  |

```go
if _, err := db.Delete(taskEnt).
    Where(taskEnt.Col("done").Eq(true)).
    Exec(ctx); err != nil {
    return err
}
```

SQLite/PostgreSQL:

```go
var removed []Task
if err := db.Delete(taskEnt).
    Where(taskEnt.Col("owner_id").Eq(int64(7))).
    Returning().All(ctx, &removed); err != nil {
    return err
}
```

## 9. Condições e expressões

### Tipos

| Tipo        | Uso                                                                                                                  |
| ----------- | -------------------------------------------------------------------------------------------------------------------- |
| `Column`    | Referência produzida por `Entity.Col`; comparação, projeção, agregado e ordenação.                                   |
| `Condition` | Predicado para `Where` e `Join`; valor zero é inválido na renderização.                                              |
| `Expr`      | Interface implementada por `Column` e `Aggregate`; usada em `Select.Fields`. Expõe `Entities`, `Alias` e `ExprInto`. |
| `Aggregate` | `COUNT`, `SUM`, `MAX` ou `MIN`; pode receber alias de saída.                                                         |
| `Sort`      | Expressão e direção para `OrderBy`.                                                                                  |
| `LikeMode`  | Modo de busca textual: `Contains`, `StartsWith`, `EndsWith`.                                                         |

### Comparadores de `Column`

Cada método retorna `Condition`. Nos exemplos, `c := taskEnt.Col("owner_id")`.

| Método                              | SQL / comportamento                               | Exemplo                                           |
| ----------------------------------- | ------------------------------------------------- | ------------------------------------------------- |
| `Eq(v any)`                         | Igualdade (`=`).                                  | `c.Eq(7)`                                         |
| `Ne(v any)`                         | Diferença (`<>`).                                 | `c.Ne(7)`                                         |
| `Lt(v any)`                         | Menor (`<`).                                      | `c.Lt(7)`                                         |
| `Lte(v any)`                        | Menor ou igual (`<=`).                            | `c.Lte(7)`                                        |
| `Gt(v any)`                         | Maior (`>`).                                      | `c.Gt(7)`                                         |
| `Gte(v any)`                        | Maior ou igual (`>=`).                            | `c.Gte(7)`                                        |
| `IsNull()`                          | `IS NULL`.                                        | `userEnt.Col("bio").IsNull()`                     |
| `IsNotNull()`                       | `IS NOT NULL`.                                    | `userEnt.Col("bio").IsNotNull()`                  |
| `In(values ...any)`                 | Um parâmetro por valor; lista vazia gera `FALSE`. | `c.In(1, 2, 3)`                                   |
| `Like(mode LikeMode, value string)` | LIKE literal com escape de `!`, `%` e `_`.        | `userEnt.Col("name").Like(krios.Contains, "Ana")` |

Nos seis comparadores binários, `Column` como operando gera comparação entre colunas; outros valores viram parâmetros. `nil` sem tipo é rejeitado; ponteiro nil tipado passa como parâmetro ao driver, mas não transforma a comparação em `IS NULL`. Para testar NULL, use os métodos próprios. `In` aplica a mesma conversão a cada elemento, inclusive referências a colunas. A validação ocorre ao renderizar/executar.

### Busca textual

| Constante    | Padrão produzido para `"ana"` |
| ------------ | ----------------------------- |
| `Contains`   | `%ana%`                       |
| `StartsWith` | `ana%`                        |
| `EndsWith`   | `%ana`                        |

`Like` sempre emite `ESCAPE '!'`. Exemplo: `Contains, "50%_off"` vincula `%50!%!_off%`. Maiúsculas/minúsculas seguem a collation do banco. Modo desconhecido retorna `ErrInvalidQuery`.

### Composição lógica

| Função                                   | Retorno / regra |
| ---------------------------------------- | --------------- |
| `And(conditions ...Condition) Condition` | Agrupa com AND. |
| `Or(conditions ...Condition) Condition`  | Agrupa com OR.  |

Ambas permitem aninhamento; grupo vazio é inválido.

```go
cond := krios.And(
    taskEnt.Col("done").Eq(false),
    krios.Or(
        taskEnt.Col("owner_id").Eq(int64(1)),
        taskEnt.Col("owner_id").Eq(int64(2)),
    ),
)
```

### Agregados, aliases e ordenação

| Função / método                        | Retorno / SQL                                                                               |
| -------------------------------------- | ------------------------------------------------------------------------------------------- |
| `Count() Aggregate`                    | `COUNT(*)`: conta linhas.                                                                   |
| `Column.Count() Aggregate`             | `COUNT(coluna)`: conta valores não NULL.                                                    |
| `Column.Sum() Aggregate`               | `SUM(coluna)`.                                                                              |
| `Column.Max() Aggregate`               | `MAX(coluna)`.                                                                              |
| `Column.Min() Aggregate`               | `MIN(coluna)`.                                                                              |
| `Aggregate.As(alias string) Aggregate` | Retorna agregado com alias de saída; vazio significa sem alias; somente espaços é inválido. |
| `Column.Asc() Sort`                    | Coluna em ordem crescente.                                                                  |
| `Column.Desc() Sort`                   | Coluna em ordem decrescente.                                                                |
| `Aggregate.Asc() Sort`                 | Expressão agregada em ordem crescente.                                                      |
| `Aggregate.Desc() Sort`                | Expressão agregada em ordem decrescente.                                                    |

Use `As` para mapear resultados agregados por nome; sem alias, o nome da coluna retornada depende do banco. Colunas não oferecem `As`. Agregados e ordenações com valor zero são inválidos.

### Introspecção e renderização

| Método                           | Retorno                                                                           |
| -------------------------------- | --------------------------------------------------------------------------------- |
| `Column.Name() string`           | Nome da coluna referenciada.                                                      |
| `Column.Alias() string`          | Sempre `""`.                                                                      |
| `Aggregate.Alias() string`       | Alias definido, ou `""`.                                                          |
| `Column.Entities() []*Entity`    | Entidade dona, ou nil para coluna sem entidade.                                   |
| `Aggregate.Entities() []*Entity` | Entidade da coluna; nenhuma para `Count()`.                                       |
| `Condition.Entities() []*Entity` | Entidades distintas referenciadas; ordem não garantida; vazio para condição zero. |
| `Sort.Entities() []*Entity`      | Entidades da expressão ordenada.                                                  |

Os aliases públicos também expõem os métodos de renderização abaixo. São usados pelos builders; `dialect.Dialect` pertence a um pacote `internal`, sem alias público. Para gerar SQL no consumidor, use `Build`.

| Receptor                      | Assinatura                                                           |
| ----------------------------- | -------------------------------------------------------------------- |
| `Condition`                   | `Render(d dialect.Dialect) (string, []any, error)`                   |
| `Condition`                   | `RenderInto(d dialect.Dialect, args *[]any) (string, error)`         |
| `Column`, `Aggregate`, `Sort` | `ExprInto(d dialect.Dialect, w *strings.Builder, args *[]any) error` |

`RenderInto` e `ExprInto` acrescentam parâmetros ao acumulador, preservando a numeração do SQL composto.

## 10. Queries embutidas

```go
// Declaração no pacote; requer import "embed".
//go:embed queries/*.sql
var queryFS embed.FS
```

Arquivo `queries/users_by_name.sql`, para SQLite/MySQL:

```sql
SELECT id, name, email, bio FROM users WHERE name = ?
```

```go
query, err := krios.NewSQLFile(queryFS, "queries/users_by_name.sql")

if err != nil {
    return err
}

var users []User

if err := db.File(query).All(ctx, &users, "Ana"); err != nil {
    return err
}
```

| API                                                     | Contrato                                                                             |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `NewSQLFile(fsys fs.FS, path string) (*SQLFile, error)` | Carrega imediatamente uma vez; caminho conforme `io/fs`, relativo, sem prefixo `./`. |
| `SQLFile.Name() string`                                 | Caminho de origem; `""` para receptor nil.                                           |
| `SQLFile.Text() string`                                 | Texto carregado, inalterado; `""` para receptor nil.                                 |
| `DB.File(f *SQLFile) *File`                             | Executor fora de transação.                                                          |
| `Tx.File(f *SQLFile) *File`                             | Executor dentro da transação.                                                        |

FS nil, caminho inválido/ausente, diretório ou arquivo em branco produzem `ErrFileNotFound`. Recurso nil passado ao executor produz `ErrInvalidQuery` na execução. `SQLFile` pode ser reutilizado concorrentemente; crie um `File` por operação.

| Método de `File`                                         | Retorno / comportamento                                   |
| -------------------------------------------------------- | --------------------------------------------------------- |
| `Exec(ctx context.Context, args ...any) (Result, error)` | Executa sem ler linhas.                                   |
| `Query(ctx context.Context, args ...any) (*Rows, error)` | Retorna iterador.                                         |
| `QueryRow(ctx context.Context, args ...any) Row`         | Consome resultado/erro no `Scan`.                         |
| `One(ctx context.Context, dest any, args ...any) error`  | Atalho de `QueryRow(...).Scan(dest)`; struct por posição. |
| `All(ctx context.Context, dest any, args ...any) error`  | Lê em `*[]T` por nome.                                    |

Parâmetros são enviados separadamente. No PostgreSQL, escreva `$1`, `$2`, … no arquivo. Krios não traduz nem divide o texto em múltiplas execuções; use um statement por recurso para portabilidade. Erros de construção são reportados antes do executor; `QueryRow` os adia para `Scan`.

## 11. Queries diretas

Os métodos abaixo existem em `DB` e `Tx`, com as mesmas assinaturas. O SQL é enviado ao driver sem alteração.

| Método                                                                        | Uso                                                |
| ----------------------------------------------------------------------------- | -------------------------------------------------- |
| `ExecContext(ctx context.Context, query string, args ...any) (Result, error)` | DDL/DML sem leitura de linhas.                     |
| `QueryContext(ctx context.Context, query string, args ...any) (*Rows, error)` | Consulta com iterador; fechar e conferir `Err`.    |
| `QueryRowContext(ctx context.Context, query string, args ...any) Row`         | Primeira linha; resultado/erro recebido no `Scan`. |

### Execução sem linhas

```go
res, err := db.ExecContext(ctx, "UPDATE tasks SET done = ? WHERE owner_id = ?", true, int64(7))

if err != nil {
    return err
}

if n, ok := res.RowsAffected(); ok {
    fmt.Println(n)
}
```

### Uma linha

```go
var name string

err := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = ?", int64(1)).Scan(&name)

if errors.Is(err, sql.ErrNoRows) {
    return nil
}

if err != nil {
    return err
}
```

### Iteração

```go
rows, err := db.QueryContext(ctx, "SELECT id, name FROM users WHERE name = ?", "Ana")

if err != nil {
    return err
}

defer rows.Close()

for rows.Next() {
    var id int64
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    fmt.Println(id, name)
}

return rows.Err()
```

Para mapear diretamente em structs, substitua o loop por `rows.All(&users)`, com `var users []User`; campos não selecionados ficam zerados. Use placeholders compatíveis com o driver (`?` nos exemplos; `$1`, `$2`, … no PostgreSQL). Valores devem ir em `args`, separados do texto SQL.

Exemplos executáveis adicionais: [`examples/`](examples/) e [`tests/examples_test.go`](tests/examples_test.go).
