# Manual de Uso do Krios

Este manual explica, símbolo por símbolo, toda a API pública do Krios (`github.com/mineot/krios`): cada função, tipo, constante e erro sentinela tem uma seção com a descrição do seu propósito e um exemplo de código Go mostrando o uso na prática.

O Krios é uma biblioteca de acesso a banco de dados para SQLite3, MySQL e PostgreSQL construída sobre `database/sql`. A configuração é feita em código, sem arquivo externo; o consumidor escolhe e importa o driver — a biblioteca nunca importa drivers.

---

## Convenções deste manual

Todo exemplo do manual assume que:

- `ctx` é um `context.Context` já criado;
- `db` é um `*krios.DB` aberto com `krios.Open` (a criação aparece no [Capítulo 1](#capítulo-1-configuração-e-abertura));
- `ent`, `taskEnt` são descritores `*krios.Entity` derivados com `krios.EntityOf`;
- o driver já foi importado pelo consumidor.

Erros são conferidos sempre com `errors.Is` / `errors.As`, nunca pela comparação de mensagens. Os erros do Krios são sentinela e podem ser embrulhados com contexto.

O SQL do consumidor (métodos `ExecContext`, `QueryContext`, `QueryRowContext` e os arquivos de `NewSQLFile`) é enviado ao driver sem tradução: os marcadores de parâmetro são os que o driver aceita (`?` em SQLite e MySQL, `$1, $2, ...` em PostgreSQL) e um arquivo SQL contém um único statement, pois o Krios não divide múltiplos statements.

---

## Modelos de exemplo

Todos os exemplos usam estes dois modelos, os mesmos do pacote `examples` do repositório:

```go
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// Status implementa krios.Validator e krios.Formatter.
func (s Status) Validate() error {
	switch s {
	case StatusActive, StatusInactive:
		return nil
	default:
		return fmt.Errorf("unsupported status %q", s)
	}
}

func (s Status) Format() (string, error) {
	return strings.ToUpper(string(s)), nil
}

type User struct {
	ID     int64   `krios:"id,pk"`
	Name   string  `krios:"name"`
	Email  string  `krios:"email"`
	Bio    *string `krios:"bio"`
	Status Status  `krios:"status"`
}

func (User) TableName() string { return "users" }

type Task struct {
	ID      int64  `krios:"id,pk"`
	Title   string `krios:"title"`
	OwnerID int64
	Done    bool
}

func (Task) TableName() string { return "tasks" }
```

As tags `krios` batizam as colunas (`krios:"id,pk"` marca a chave primária); campo sem tag deriva a coluna em snake_case (`OwnerID` → `owner_id`); `krios:"-"` ignora o campo; `*string` representa SQL NULL; `TableName()` sobrescreve o nome de tabela derivado do tipo.

---

## Instalação e drivers

```sh
go get github.com/mineot/krios
```

O consumidor importa o driver que vai usar. O nome dado em `Config.Driver` deve ser o nome com que o driver se registra no `database/sql`:

```go
import (
	_ "modernc.org/sqlite"            // registra "sqlite"
	_ "github.com/go-sql-driver/mysql" // registra "mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registra "pgx"
)
```

Importe apenas o driver que a aplicação usa.

---

## Exemplo completo

Um programa que abre o banco, cria a tabela, insere um usuário e consulta:

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()

	db, err := krios.Open(ctx, krios.Config{
		Type:    krios.DBTypeSQLite,
		Driver:  "sqlite",
		SQLite:  &krios.SQLiteConfig{DBPath: "app.db"},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err) // errors.Is com krios.ErrInvalidConfig, ErrDriverNotLoaded, ErrConnectionFailed
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		bio TEXT,
		status TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatal(err)
	}

	ent, err := krios.EntityOf[User]()
	if err != nil {
		log.Fatal(err)
	}

	res, err := db.Insert(ent).
		SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
		Omit("id").
		Exec(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if id, ok := res.LastInsertId(); ok {
		fmt.Println("novo id:", id)
	}

	var users []User
	err = db.Select(ent).
		Where(ent.Col("status").Eq(StatusActive)).
		OrderBy(ent.Col("name").Asc()).
		All(ctx, &users)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Fatal(err)
	}
	fmt.Println(users)
}
```

---

## Sumário

- [Capítulo 1: Configuração e abertura](#capítulo-1-configuração-e-abertura)
  - [Config](#config), [Config.Validate](#configvalidate), [Config.EffectiveTimeout](#configeffectivetimeout), [SQLiteConfig](#sqliteconfig), [MySQLConfig](#mysqlconfig), [PostgresConfig](#postgresconfig), [DBType](#dbtype), [DBTypeSQLite](#dbtypesqlite), [DBTypeMySQL](#dbtypemysql), [DBTypePostgres](#dbtypepostgres), [Open](#open), [DB](#db), [DB.Close](#dbclose), [DB.ExecContext](#dbexeccontext), [DB.QueryContext](#dbquerycontext), [DB.QueryRowContext](#dbqueryrowcontext), [DB.Begin](#dbbegin), [DB.Select](#dbselect), [DB.Insert](#dbinsert), [DB.Update](#dbupdate), [DB.Delete](#dbdelete)
- [Capítulo 2: Entidades e mapeamento](#capítulo-2-entidades-e-mapeamento)
  - [EntityOf](#entityof), [Entity](#entity), [Entity.Table](#entitytable), [Entity.Type](#entitytype), [Entity.Columns](#entitycolumns), [Entity.PrimaryKeys](#entityprimarykeys), [Entity.FieldIndexes](#entityfieldindexes), [Entity.FieldIndex](#entityfieldindex), [Entity.Col](#entitycol), [Entity.Validate](#entityvalidate), [Entity.ValidateAll](#entityvalidateall), [Entity.Format](#entityformat), [Entity.WriteValues](#entitywritevalues), [Validator](#validator), [Formatter](#formatter)
- [Capítulo 3: Condições, expressões e agregados](#capítulo-3-condições-expressões-e-agregados)
  - [Condition](#condition), [Condition.Entities](#conditionentities), [Column](#column), [Column.Eq](#columneq), [Column.Ne](#columnne), [Column.Lt](#columnlt), [Column.Lte](#columnlte), [Column.Gt](#columngt), [Column.Gte](#columngte), [Column.IsNull](#columnisnull), [Column.IsNotNull](#columnisnotnull), [Column.In](#columnin), [Column.Like](#columnlike), [Column.Name](#columnname), [Column.Alias](#columnalias), [Column.Entities](#columnentities), [LikeMode](#likemode), [Contains](#contains), [StartsWith](#startswith), [EndsWith](#endswith), [And](#and), [Or](#or), [Expr](#expr), [Aggregate](#aggregate), [Count](#count), [Column.Count](#columncount), [Column.Sum](#columnsum), [Column.Max](#columnmax), [Column.Min](#columnmin), [Aggregate.As](#aggregateas), [Aggregate.Alias](#aggregatealias), [Aggregate.Entities](#aggregateentities), [Sort](#sort), [Column.Asc](#columnasc), [Column.Desc](#columndesc), [Aggregate.Asc](#aggregateasc), [Aggregate.Desc](#aggregatedesc), [Sort.Entities](#sortentities)
- [Capítulo 4: Consultas com Select](#capítulo-4-consultas-com-select)
  - [Select](#select), [Select.Only](#selectonly), [Select.Fields](#selectfields), [Select.Distinct](#selectdistinct), [Select.Where](#selectwhere), [Select.Join](#selectjoin), [JoinKind](#joinkind), [InnerJoin](#innerjoin), [LeftJoin](#leftjoin), [Select.GroupBy](#selectgroupby), [Select.OrderBy](#selectorderby), [Select.Limit](#selectlimit), [Select.Offset](#selectoffset), [Select.Build](#selectbuild), [Select.Rows](#selectrows), [Select.Row](#selectrow), [Select.All](#selectall), [Select.One](#selectone), [Select.Count](#selectcount), [Select.Paginate](#selectpaginate), [Paginate](#paginate)
- [Capítulo 5: Escritas com Insert, Update e Delete](#capítulo-5-escritas-com-insert-update-e-delete)
  - [Insert](#insert), [Insert.SetValues](#insertsetvalues), [Insert.Omit](#insertomit), [Insert.Returning](#insertreturning), [Insert.Build](#insertbuild), [Insert.Exec](#insertexec), [Insert.One](#insertone), [Update](#update), [Update.SetValues](#updatesetvalues), [Update.Where](#updatewhere), [Update.Omit](#updateomit), [Update.Returning](#updatereturning), [Update.Build](#updatebuild), [Update.Exec](#updateexec), [Update.All](#updateall), [Delete](#delete), [Delete.Where](#deletewhere), [Delete.Returning](#deletereturning), [Delete.Build](#deletebuild), [Delete.Exec](#deleteexec), [Delete.All](#deleteall)
- [Capítulo 6: Transações](#capítulo-6-transações)
  - [Tx](#tx), [Tx.ExecContext](#txexeccontext), [Tx.QueryContext](#txquerycontext), [Tx.QueryRowContext](#txqueryrowcontext), [Tx.Commit](#txcommit), [Tx.Rollback](#txrollback), [Tx.Select](#txselect), [Tx.Insert](#txinsert), [Tx.Update](#txupdate), [Tx.Delete](#txdelete)
- [Capítulo 7: Resultados e leitura de linhas](#capítulo-7-resultados-e-leitura-de-linhas)
  - [Result](#result), [Result.RowsAffected](#resultrowsaffected), [Result.LastInsertId](#resultlastinsertid), [Row](#row), [Row.Scan](#rowscan), [Rows](#rows), [Rows.Next](#rowsnext), [Rows.Scan](#rowsscan), [Rows.Err](#rowserr), [Rows.Columns](#rowscolumns), [Rows.Close](#rowsclose), [Rows.All](#rowsall)
- [Capítulo 8: SQL embutido](#capítulo-8-sql-embutido)
  - [NewSQLFile](#newsqlfile), [SQLFile](#sqlfile), [SQLFile.Name](#sqlfilename), [SQLFile.Text](#sqlfiletext), [DB.File](#dbfile), [Tx.File](#txfile), [File](#file), [File.Exec](#fileexec), [File.Query](#filequery), [File.QueryRow](#filequeryrow), [File.One](#fileone), [File.All](#fileall)
- [Capítulo 9: Migrações](#capítulo-9-migrações)
  - [MigrationVersion](#migrationversion), [MigrationsConfig](#migrationsconfig), [MigrationsConfig.Validate](#migrationsconfigvalidate), [DB.Migrations](#dbmigrations), [DB.RunMigrations](#dbrunmigrations), [MigrationPlan](#migrationplan), [MigrationStep](#migrationstep), [PartialMigrationError](#partialmigrationerror), [PartialMigrationError.Error](#partialmigrationerrorerror), [PartialMigrationError.Unwrap](#partialmigrationerrorunwrap)
- [Capítulo 10: Erros sentinela](#capítulo-10-erros-sentinela)
  - [ErrInvalidConfig](#errinvalidconfig), [ErrDriverNotLoaded](#errdrivernotloaded), [ErrConnectionFailed](#errconnectionfailed), [ErrClosed](#errclosed), [ErrTxDone](#errtxdone), [ErrInvalidEntity](#errinvalidentity), [ErrValidation](#errvalidation), [ErrFormat](#errformat), [ErrInvalidQuery](#errinvalidquery), [ErrFileNotFound](#errfilenotfound), [ErrPartialMigration](#errpartialmigration)

---

# Capítulo 1: Configuração e abertura

---

## Config

Descrição de todo o acesso ao banco: motor, driver, configuração do motor, orçamento de abertura e, opcionalmente, as migrações. É preenchida em código pelo consumidor e passada para `Open` — não há arquivo de configuração externo.

```go
cfg := krios.Config{
	Type:   krios.DBTypeSQLite,
	Driver: "sqlite",
	SQLite: &krios.SQLiteConfig{DBPath: "app.db"},
	Timeout: 5 * time.Second,
}
```

Campos: `Type` (obrigatório), `Driver` (obrigatório, nome do driver registrado), `SQLite`/`MySQL`/`Postgres` (obrigatório conforme o `Type` e proibido se de outro motor), `Timeout` (zero aplica 10s; negativo é inválido) e `Migrations` (opcional, ver [Capítulo 9](#capítulo-9-migrações)). Use `Validate` antes de `Open` se quiser checar a configuração isoladamente.

---

## Config.Validate

Verifica se a configuração está completa e consistente: motor suportado, driver presente, configuração do motor presente (e ausente para os demais), campos de conexão obrigatórios, porta entre 0 e 65535, `Timeout` não negativo e, quando houver, a validação de `Migrations`. Falhas casam com `ErrInvalidConfig`. `Open` chama este método automaticamente.

```go
cfg := krios.Config{
	Type:   krios.DBTypeMySQL,
	Driver: "mysql",
	// MySQL ausente
}

if err := cfg.Validate(); err != nil {
	fmt.Println(errors.Is(err, krios.ErrInvalidConfig)) // true
}
```

---

## Config.EffectiveTimeout

Devolve o orçamento de tempo efetivo da abertura: o valor configurado, ou 10 segundos quando `Timeout` é zero. `Open` usa o menor prazo entre o deadline do `context` e este valor.

```go
cfg := krios.Config{}

fmt.Println(cfg.EffectiveTimeout()) // 10s (default)

cfg.Timeout = 3 * time.Second
fmt.Println(cfg.EffectiveTimeout()) // 3s
```

---

## SQLiteConfig

Ajustes de conexão do SQLite. `DBPath` é o caminho do arquivo `.db` (relativo ou absoluto) e é obrigatório.

```go
cfg := krios.Config{
	Type:   krios.DBTypeSQLite,
	Driver: "sqlite",
	SQLite: &krios.SQLiteConfig{DBPath: "data/app.db"},
}
```

---

## MySQLConfig

Ajustes de conexão do MySQL. `Host`, `User`, `Password` e `DBName` são obrigatórios. `Port` com valor zero aplica a porta padrão 3306; valores explícitos devem estar em 1..65535.

```go
cfg := krios.Config{
	Type:   krios.DBTypeMySQL,
	Driver: "mysql",
	MySQL: &krios.MySQLConfig{
		Host:     "db.local",
		User:     "app",
		Password: "secret",
		DBName:   "appdb",
	},
}
```

---

## PostgresConfig

Ajustes de conexão do PostgreSQL. `Host`, `User`, `Password` e `DBName` são obrigatórios. `Port` com valor zero aplica a porta padrão 5432; valores explícitos devem estar em 1..65535.

```go
cfg := krios.Config{
	Type:   krios.DBTypePostgres,
	Driver: "pgx",
	Postgres: &krios.PostgresConfig{
		Host:     "db.local",
		User:     "app",
		Password: "secret",
		DBName:   "appdb",
	},
}
```

---

## DBType

Tipo `string` que identifica o motor suportado, usado em `Config.Type`. Os valores estão nas constantes abaixo.

```go
var t krios.DBType = krios.DBTypeSQLite
fmt.Println(t == "sqlite") // true
```

---

## DBTypeSQLite

Valor de `Config.Type` para SQLite3 (`"sqlite"`). Exige `Config.SQLite`.

```go
cfg.Type = krios.DBTypeSQLite
```

---

## DBTypeMySQL

Valor de `Config.Type` para MySQL (`"mysql"`). Exige `Config.MySQL`.

```go
cfg.Type = krios.DBTypeMySQL
```

---

## DBTypePostgres

Valor de `Config.Type` para PostgreSQL (`"postgres"`). Exige `Config.Postgres`.

```go
cfg.Type = krios.DBTypePostgres
```

---

## Open

Valida a configuração, confere se o driver nomeado está registrado em `database/sql`, abre o acesso e verifica a conectividade com um ping. O prazo efetivo é o menor entre o deadline do `ctx` e `cfg.Timeout` (10s quando zero). Recursos criados em uma tentativa falha são liberados; falhas casam com `ErrInvalidConfig`, `ErrDriverNotLoaded` ou `ErrConnectionFailed`, e cancelamento/deadline do contexto são reportados como `context.Canceled`/`context.DeadlineExceeded` embrulhados.

```go
db, err := krios.Open(ctx, krios.Config{
	Type:    krios.DBTypeSQLite,
	Driver:  "sqlite",
	SQLite:  &krios.SQLiteConfig{DBPath: "app.db"},
	Timeout: 5 * time.Second,
})
if err != nil {
	switch {
	case errors.Is(err, krios.ErrInvalidConfig):
		// configuração inconsistente
	case errors.Is(err, krios.ErrDriverNotLoaded):
		// driver não importado ou nome errado
	case errors.Is(err, krios.ErrConnectionFailed):
		// rede, arquivo, permissão, recusa
	}
	return err
}
defer db.Close()
```

---

## DB

Acesso ao banco retornado por `Open`. É seguro para concorrência: várias goroutines podem usá-lo ao mesmo tempo. Após `Close`, toda operação falha com `ErrClosed`. Cria os builders `Select`, `Insert`, `Update`, `Delete` e `File`, inicia transações com `Begin` e executa migrações com `RunMigrations`.

```go
db, err := krios.Open(ctx, cfg)
if err != nil {
	return err
}
defer db.Close() // idempotente

fmt.Printf("%T\n", db) // *krios.DB
```

`db` é a porta de entrada da biblioteca; os capítulos seguintes documentam cada método.

---

## DB.Close

Libera os recursos associados ao acesso. É idempotente: após a primeira chamada, as demais retornam `nil`. Operações posteriores falham com `ErrClosed`.

```go
if err := db.Close(); err != nil {
	return err
}

_, err := db.ExecContext(ctx, "SELECT 1")
fmt.Println(errors.Is(err, krios.ErrClosed)) // true
```

---

## DB.ExecContext

Executa um statement sem retorno de linhas (DDL, DML). Os parâmetros são vinculados separadamente, com os marcadores que o driver aceita. Depois de `Close`, falha com `ErrClosed`.

```go
res, err := db.ExecContext(ctx,
	"UPDATE users SET status = ? WHERE id = ?",
	StatusInactive, int64(1),
)
if err != nil {
	return err
}
affected, _ := res.RowsAffected()
```

No PostgreSQL os marcadores seriam `$1` e `$2`. O retorno é um `krios.Result` (ver [Capítulo 7](#capítulo-7-resultados-e-leitura-de-linhas)).

---

## DB.QueryContext

Executa uma consulta que retorna linhas. O consumidor deve fechar o `*krios.Rows` e checar `Err`, ou usar `All`.

```go
rows, err := db.QueryContext(ctx,
	"SELECT id, name FROM users WHERE status = ?",
	StatusActive,
)
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
}
return rows.Err()
```

---

## DB.QueryRowContext

Executa uma consulta esperada em no máximo uma linha. O resultado ou o erro chegam no `Scan`, que preserva `sql.ErrNoRows`.

```go
var name string
err := db.QueryRowContext(ctx,
	"SELECT name FROM users WHERE id = ?", int64(1),
).Scan(&name)
if errors.Is(err, sql.ErrNoRows) {
	// usuário inexistente
}
```

---

## DB.Begin

Inicia uma transação. O contexto permanece em vigor durante toda a vida da transação; seu cancelamento faz o `database/sql` reverter a transação. Use `defer tx.Rollback(ctx)` — o rollback após um `Commit` bem-sucedido é um no-op. Ver [Capítulo 6](#capítulo-6-transações).

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

// ... operações em tx ...

return tx.Commit(ctx)
```

---

## DB.Select

Cria um builder de `Select` renderizado com o dialeto do `DB` e executado por ele. A construção é validada no `Build` ou na primeira chamada de execução; as guardas do `DB` (ex.: `ErrClosed` após `Close`) valem inalteradas. Ver [Capítulo 4](#capítulo-4-consultas-com-select).

```go
ent, _ := krios.EntityOf[User]()

var users []User
err := db.Select(ent).
	Where(ent.Col("status").Eq(StatusActive)).
	All(ctx, &users)
if errors.Is(err, krios.ErrClosed) {
	// DB.Close já foi chamado
}
```

---

## DB.Insert

Cria um builder de `Insert` para a entidade, com o dialeto e o executor do `DB`. Falhas de construção são reportadas antes do driver; guardas do `DB` valem inalteradas. Ver [Capítulo 5](#capítulo-5-escritas-com-insert-update-e-delete).

```go
res, err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Exec(ctx)
if err != nil {
	return err
}
```

---

## DB.Update

Cria um builder de `Update` para a entidade. Pelo menos uma condição em `Where` é exigida antes da execução; sem ela, `Build` falha com `ErrInvalidQuery`. Ver [Capítulo 5](#capítulo-5-escritas-com-insert-update-e-delete).

```go
res, err := db.Update(ent).
	SetValues(&User{Status: StatusInactive}).
	Omit("id", "name", "email", "bio").
	Where(ent.Col("id").Eq(int64(1))).
	Exec(ctx)
if err != nil {
	return err
}
```

---

## DB.Delete

Cria um builder de `Delete` para a entidade. Pelo menos uma condição em `Where` é exigida antes da execução. Ver [Capítulo 5](#capítulo-5-escritas-com-insert-update-e-delete).

```go
res, err := db.Delete(taskEnt).
	Where(taskEnt.Col("done").Eq(true)).
	Exec(ctx)
if err != nil {
	return err
}
```

---

# Capítulo 2: Entidades e mapeamento

---

## EntityOf

Deriva (e cacheia por tipo) o descritor de entidade a partir do tipo `T`, que pode ser struct ou ponteiro para struct. O nome da tabela vem de `TableName() string` ou do snake_case do tipo; as colunas vêm das tags `krios:"col[,pk]"` (sem tag, snake_case do campo; `krios:"-"` ignora). Inválido retorna `ErrInvalidEntity`. O cache torna a chamada barata depois da primeira derivação.

```go
ent, err := krios.EntityOf[User]()
if err != nil {
	// erros de programa: struct inválida, tag inválida, coluna duplicada...
	return err
}

taskEnt, err := krios.EntityOf[Task]()
if err != nil {
	return err
}
```

---

## Entity

Descritor cacheado de uma tabela: nome da tabela, colunas mapeadas em ordem de declaração, chaves primárias e índices dos campos. Produzido por `EntityOf`; trate o resultado como somente leitura. É comum guardar os descritores em variáveis de pacote para derivar uma única vez.

```go
var (
	userEntity = mustEntity[User]()
	taskEntity = mustEntity[Task]()
)

func mustEntity[T any]() *krios.Entity {
	ent, err := krios.EntityOf[T]()
	if err != nil {
		panic(err) // modelo inválido é erro de programa
	}
	return ent
}
```

---

## Entity.Table

Devolve o nome da tabela mapeado: o retorno de `TableName()` quando declarado, senão o snake_case do nome do tipo.

```go
ent, _ := krios.EntityOf[User]()
fmt.Println(ent.Table()) // users
```

---

## Entity.Type

Devolve o tipo `reflect.Type` da struct de onde a entidade foi derivada.

```go
ent, _ := krios.EntityOf[User]()
fmt.Println(ent.Type().String()) // examples.User
```

---

## Entity.Columns

Devolve os nomes das colunas mapeadas, na ordem de declaração dos campos. A fatia retornada é uma cópia.

```go
ent, _ := krios.EntityOf[User]()
fmt.Println(ent.Columns()) // [id name email bio status]

taskEnt, _ := krios.EntityOf[Task]()
fmt.Println(taskEnt.Columns()) // [id title owner_id done]
```

---

## Entity.PrimaryKeys

Devolve as colunas marcadas com `,pk` na tag, em ordem de declaração. Fatia copiada; vazia quando a entidade não declara chave.

```go
ent, _ := krios.EntityOf[User]()
fmt.Println(ent.PrimaryKeys()) // [id]
```

---

## Entity.FieldIndexes

Devolve o índice do campo da struct para cada coluna mapeada, em ordem de declaração. É o que o caminho de mapeamento usa para preencher structs. Fatia copiada.

```go
ent, _ := krios.EntityOf[User]()
fmt.Println(ent.FieldIndexes()) // [0 1 2 3 4] — id, name, email, bio, status
```

---

## Entity.FieldIndex

Resolve o nome de uma coluna do resultado para a posição do campo: correspondência exata primeiro, depois ignorando maiúsculas/minúsculas. Devolve `(0, false)` quando não há campo.

```go
ent, _ := krios.EntityOf[User]()

idx, ok := ent.FieldIndex("email") // 2, true
idx, ok = ent.FieldIndex("EMAIL")  // 2, true (case-insensitive)
idx, ok = ent.FieldIndex("outro")  // 0, false
```

---

## Entity.Col

Devolve uma referência (`krios.Column`) à coluna informada. O nome é validado no render/execução: coluna desconhecida falha com `ErrInvalidQuery` antes de chegar ao driver. É a porta de entrada para todos os comparadores e agregados.

```go
ent, _ := krios.EntityOf[User]()

id := ent.Col("id")
name := ent.Col("name")

err := db.Select(ent).Where(id.Eq(int64(1))).One(ctx, &user)
```

---

## Entity.Validate

Valida, em ordem de declaração, todos os campos mapeados de `record` (T ou `*T`) que implementam `krios.Validator`. Retorna a primeira falha embrulhada com `ErrValidation` (com tabela e coluna). Campos nil são pulados. Os escritores (`SetValues`) executam esta validação automaticamente.

```go
user := &User{Name: "Ana", Email: "ana@example.com", Status: "bogus"}

err := ent.Validate(user)
if errors.Is(err, krios.ErrValidation) {
	fmt.Println(err) // krios: field validation failed: users: column "status": unsupported status "bogus"
}
```

---

## Entity.ValidateAll

Valida cada elemento de um lote (`[]T` ou `*[]T`), parando no primeiro registro inválido com `ErrValidation`.

```go
users := []User{
	{Name: "Ana", Email: "ana@example.com", Status: StatusActive},
	{Name: "Bia", Email: "bia@example.com", Status: "bogus"},
}

if err := ent.ValidateAll(users); err != nil {
	fmt.Println(errors.Is(err, krios.ErrValidation)) // true (Bia)
}
```

---

## Entity.Format

Devolve o texto de apresentação de cada coluna mapeada de `record`, em um mapa coluna → texto. Campos com `krios.Formatter` usam o texto dele; os demais usam `fmt.Sprint`; campo ponteiro nil vira `""`. Os valores originais nunca são alterados e o texto nunca é usado em operações de banco.

```go
user := User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}

text, err := ent.Format(user)
if err != nil {
	return err
}
fmt.Println(text["status"]) // ACTIVE (Format de Status)
fmt.Println(text["bio"])    // "" (nil)
```

---

## Entity.WriteValues

Devolve a disposição de escrita de cada coluna mapeada de `record` (`[]WriteValue` com `Column`, `Value` e `Absent`), na ordem de declaração. É o motor dos escritores: campo ponteiro nil é `Absent` (INSERT grava NULL, UPDATE não escreve a coluna); um `driver.Valuer` inválido (ex.: `sql.Null*` com `Valid false`) produz `Value nil` (NULL); campos não ponteiro levam o valor, inclusive zeros. Rejeição por `Valuer` ou destino inválido falha com `ErrInvalidEntity`.

```go
values, err := ent.WriteValues(&User{Name: "Ana", Bio: nil})
if err != nil {
	return err
}

for _, v := range values {
	fmt.Printf("%s = %v (absent=%v)\n", v.Column, v.Value, v.Absent)
}
// id = 0 (absent=false)
// name = Ana (absent=false)
// bio = <nil> (absent=true)
// ...
```

---

## Validator

Interface implementada por tipos de campo que validam o próprio valor: `Validate() error` devolve `nil` quando o valor é válido. É detectada por reflexão (no valor ou no ponteiro) na derivação da entidade e chamada por `Entity.Validate`, `Entity.ValidateAll` e automaticamente pelos escritores via `SetValues`.

```go
type Status string

func (s Status) Validate() error {
	switch s {
	case StatusActive, StatusInactive:
		return nil
	default:
		return fmt.Errorf("unsupported status %q", s)
	}
}
```

A falha vira `ErrValidation` contextualizada com tabela e coluna.

---

## Formatter

Interface implementada por tipos de campo que fornecem texto de apresentação: `Format() (string, error)`. Detectada por reflexão; usada apenas por `Entity.Format`. O texto retornado **nunca** vai ao banco — as operações de escrita usam sempre o valor original do campo.

```go
type Status string

func (s Status) Format() (string, error) {
	return strings.ToUpper(string(s)), nil
}

// ent.Format(user)["status"] == "ACTIVE"
```

Falha na formatação vira `ErrFormat`.

---

# Capítulo 3: Condições, expressões e agregados

---

## Condition

Predicado booleano opaco usado em `Select.Where`, `Update.Where`, `Delete.Where` e no `ON` de `Select.Join`. Construa com os comparadores de `Column` e com `And`/`Or`. O valor zero (`Condition{}`) é inválido e falha no render com `ErrInvalidQuery`.

```go
var c krios.Condition = ent.Col("status").Eq(StatusActive)

err := db.Select(ent).Where(c).All(ctx, &users)
```

Os métodos `Render`/`RenderInto` existem para o uso interno dos builders (recebem o dialeto interno da biblioteca e não são chamáveis fora do módulo); no uso normal a condição só circula pelos métodos encadeados. Para introspecção, use [`Condition.Entities`](#conditionentities).

---

## Condition.Entities

Devolve as entidades distintas citadas pelas colunas da condição (ordem não garantida). Os builders usam esta lista para validar que toda entidade referenciada participa da consulta; `Update`/`Delete`, por exemplo, exigem que apenas a tabela base apareça. Útil para introspecção de predicados compostos.

```go
cond := krios.And(
	taskEnt.Col("owner_id").Eq(int64(1)),
	taskEnt.Col("done").Eq(false),
)

for _, e := range cond.Entities() {
	fmt.Println(e.Table()) // tasks
}
```

Condição zero devolve uma lista vazia; o render dela ainda falha com `ErrInvalidQuery`.

---

## Column

Referência a uma coluna mapeada, produzida por `Entity.Col`. Carrega os comparadores que constroem `Condition` (`Eq`, `Ne`, `Lt`, `Lte`, `Gt`, `Gte`, `IsNull`, `IsNotNull`, `In`, `Like`), os agregados (`Count`, `Sum`, `Max`, `Min`), a ordenação (`Asc`, `Desc`) e `Name`. Em comparações, uma coluna do mesmo lado vira comparação coluna × coluna; qualquer outro valor não nil vira parâmetro vinculado.

```go
ent, _ := krios.EntityOf[User]()

name := ent.Col("name")      // referencia users.name
email := ent.Col("email")

// coluna × coluna
cond := ent.Col("id").Eq(ent.Col("id"))
```

Coluna desconhecida é detectada no render/execução com `ErrInvalidQuery`, antes de chegar ao driver.

---

## Column.Eq

Constrói uma condição `=`. Um operando `Column` é renderizado como comparação entre colunas; qualquer outro valor não nil vira parâmetro. `nil` é rejeitado — use `IsNull`/`IsNotNull`.

```go
byID := ent.Col("id").Eq(int64(1))
byStatus := ent.Col("status").Eq(StatusActive)

// coluna × coluna:
byOwner := taskEnt.Col("owner_id").Eq(userEnt.Col("id"))
```

---

## Column.Ne

Constrói uma condição `<>`. Mesmas regras de operandos de `Eq`; `nil` é rejeitado.

```go
cond := ent.Col("status").Ne(StatusInactive)
```

---

## Column.Lt

Constrói uma condição `<`. `nil` é rejeitado.

```go
cond := ent.Col("id").Lt(int64(100))
```

---

## Column.Lte

Constrói uma condição `<=`. `nil` é rejeitado.

```go
cond := ent.Col("id").Lte(int64(100))
```

---

## Column.Gt

Constrói uma condição `>`. `nil` é rejeitado.

```go
cond := ent.Col("id").Gt(int64(100))
```

---

## Column.Gte

Constrói uma condição `>=`. `nil` é rejeitado.

```go
cond := ent.Col("id").Gte(int64(100))
```

---

## Column.IsNull

Constrói uma condição `IS NULL`. É a forma corrigida de comparar colunas com `nil`.

```go
cond := ent.Col("bio").IsNull()

err := db.Select(ent).Where(cond).All(ctx, &users)
```

---

## Column.IsNotNull

Constrói uma condição `IS NOT NULL`.

```go
cond := ent.Col("bio").IsNotNull()
```

---

## Column.In

Constrói uma condição `IN` com um parâmetro por valor. Lista vazia renderiza a constante `FALSE`; um `nil` dentro da lista é rejeitado com `ErrInvalidQuery`.

```go
cond := ent.Col("status").In(StatusActive, StatusInactive)
cond = ent.Col("id").In(int64(1), int64(2), int64(3))
```

---

## Column.Like

Constrói uma condição `LIKE` com o modo informado (`LikeMode`). O valor é tratado literalmente: `!`, `%` e `_` dentro dele são escapados e uma cláusula `ESCAPE '!'` é emitida sempre. A sensibilidade a maiúsculas segue a collation do banco e não é adaptada.

```go
cond := ent.Col("name").Like(krios.StartsWith, "Ana")
// users.name LIKE 'Ana%' ESCAPE '!'

cond = ent.Col("name").Like(krios.Contains, "50%_off")
// o valor vira literal: %50!%!_off%
```

---

## Column.Name

Devolve o nome mapeado da coluna.

```go
ent, _ := krios.EntityOf[User]()
fmt.Println(ent.Col("owner_id").Name()) // em Task: owner_id
fmt.Println(ent.Col("name").Name())     // name
```

---

## Column.Alias

Devolve `""` — colunas são sempre selecionadas pelo próprio nome; alias de saída é exclusivo dos agregados (`Aggregate.As`).

```go
fmt.Println(ent.Col("name").Alias()) // ""
fmt.Println(taskEnt.Col("id").Count().As("total").Alias()) // total
```

---

## Column.Entities

Devolve a entidade dona da coluna em uma lista de um elemento (`nil` para coluna sem entidade). É o que `Expr.Entities` expõe no uso comum.

```go
for _, e := range ent.Col("name").Entities() {
	fmt.Println(e.Table()) // users
}
```

---

## LikeMode

Tipo que seleciona como `Column.Like` rodeia o valor com curingas. Os valores são `Contains`, `StartsWith` e `EndsWith`.

```go
var mode krios.LikeMode = krios.Contains
cond := ent.Col("name").Like(mode, "ana")
```

---

## Contains

Modo `LikeMode` que casa valores que contêm o texto (`%valor%`).

```go
cond := ent.Col("name").Like(krios.Contains, "an")
// users.name LIKE '%an%' ESCAPE '!'
```

---

## StartsWith

Modo `LikeMode` que casa valores que começam com o texto (`valor%`).

```go
cond := ent.Col("name").Like(krios.StartsWith, "Ana")
// users.name LIKE 'Ana%' ESCAPE '!'
```

---

## EndsWith

Modo `LikeMode` que casa valores que terminam com o texto (`%valor`).

```go
cond := ent.Col("email").Like(krios.EndsWith, "@example.com")
// users.email LIKE '%@example.com%' ESCAPE '!'  — curinga no final
```

---

## And

Combina condições em conjunção (E). Aninhável com `Or`. Grupo vazio falha no render com `ErrInvalidQuery`.

```go
cond := krios.And(
	taskEnt.Col("done").Eq(false),
	taskEnt.Col("owner_id").Eq(int64(1)),
)

err := db.Select(taskEnt).Where(cond).All(ctx, &tasks)
```

---

## Or

Combina condições em disjunção (OU), aninhável com `And` e `Or`. Grupo vazio falha no render com `ErrInvalidQuery`.

```go
cond := krios.Or(
	ent.Col("status").Eq(StatusActive),
	ent.Col("status").Eq(StatusInactive),
)

cond = krios.And(
	taskEnt.Col("done").Eq(false),
	krios.Or(
		taskEnt.Col("owner_id").Eq(int64(1)),
		taskEnt.Col("owner_id").Eq(int64(2)),
	),
)
// (done = false AND (owner_id = 1 OR owner_id = 2))
```

---

## Expr

Expressão selecionável — uma `Column` ou um `Aggregate` — usada em `Select.Fields`. Pode carregar alias de saída (via `Aggregate.As`).

```go
var e krios.Expr = ent.Col("name")
var a krios.Expr = krios.Count()

err := db.Select(ent).Fields(e, a.As("total")).All(ctx, &rows)
```

A interface declara `ExprInto` (renderização interna, parametrizada por um dialeto não exportado — não chamável fora do módulo), `Entities` e `Alias`, os dois últimos disponíveis nas implementações ([`Column.Entities`](#columnentities), [`Aggregate.Alias`](#aggregatealias), [`Column.Alias`](#columnalias)).

---

## Aggregate

Expressão de agregação (`COUNT`, `SUM`, `MAX`, `MIN`) sobre uma coluna, ou `COUNT(*)` quando criada pelo `Count()` de pacote. Use `As` para dar um alias de saída, permitindo mapear o resultado por nome. Implementa `Expr`.

```go
var a krios.Aggregate = krios.Count()                 // COUNT(*)
var b krios.Aggregate = taskEnt.Col("id").Count()      // COUNT(tasks.id)
var c krios.Aggregate = taskEnt.Col("id").Sum().As("total")
```

---

## Count

Função de pacote que devolve o agregado `COUNT(*)` — o número de linhas (do grupo, quando há `GROUP BY`). Use em `Select.Fields`; para contar linhas de uma consulta sem projetar, `Select.Count(ctx)` é mais direto. `Column.Count` conta valores não nulos de uma coluna.

```go
type Total struct {
	Total int64 `krios:"total"`
}

var t Total
err := db.Select(taskEnt).
	Fields(krios.Count().As("total")).
	Where(taskEnt.Col("owner_id").Eq(int64(1))).
	One(ctx, &t)
fmt.Println(t.Total)
```

---

## Column.Count

Agregado `COUNT(coluna)`: conta apenas os valores não nulos da coluna.

```go
agg := userEnt.Col("bio").Count().As("com_bio")
// COUNT(users.bio) AS com_bio
```

---

## Column.Sum

Agregado `SUM(coluna)`.

```go
agg := taskEnt.Col("id").Sum().As("total")
// SUM(tasks.id) AS total
```

---

## Column.Max

Agregado `MAX(coluna)`.

```go
agg := taskEnt.Col("id").Max().As("maior")
// MAX(tasks.id) AS maior
```

---

## Column.Min

Agregado `MIN(coluna)`.

```go
agg := taskEnt.Col("id").Min().As("menor")
// MIN(tasks.id) AS menor
```

---

## Aggregate.As

Define o alias de saída do agregado, usado para mapear a coluna do resultado por nome. Alias só de espaços é rejeitado no render com `ErrInvalidQuery`. Devolve o agregado, permitindo encadear.

```go
fields := []krios.Expr{
	krios.Count().As("total"),
	taskEnt.Col("id").Sum().As("soma"),
}

err := db.Select(taskEnt).Fields(fields...).All(ctx, &stats)
```

---

## Aggregate.Alias

Devolve o alias de saída definido com `As`, ou `""` quando não há alias (o agregado é selecionado sob o nome computado, ex.: `count`).

```go
fmt.Println(krios.Count().As("total").Alias()) // total
fmt.Println(krios.Count().Alias())             // ""
```

---

## Aggregate.Entities

Devolve a entidade da coluna agregada, ou `COUNT(*)` não referencia nenhuma entidade (`krios.Count()`).

```go
for _, e := range taskEnt.Col("owner_id").Count().Entities() {
	fmt.Println(e.Table()) // tasks
}

fmt.Println(len(krios.Count().Entities())) // 0 — COUNT(*)
```

---

## Sort

Critério de `ORDER BY`: uma expressão e uma direção. Construa com `Asc`/`Desc` em uma `Column` ou `Aggregate` e passe para `Select.OrderBy`.

```go
s1 := ent.Col("name").Asc()
s2 := ent.Col("id").Desc()
s3 := krios.Count().As("total").Desc() // ordena por agregado

err := db.Select(ent).OrderBy(s1, s2).All(ctx, &users)
```

---

## Column.Asc

Ordenação ascendente pela coluna.

```go
err := db.Select(ent).OrderBy(ent.Col("name").Asc()).All(ctx, &users)
// ORDER BY users.name ASC
```

---

## Column.Desc

Ordenação descendente pela coluna.

```go
err := db.Select(ent).OrderBy(ent.Col("id").Desc()).All(ctx, &users)
// ORDER BY users.id DESC
```

---

## Aggregate.Asc

Ordenação ascendente pelo agregado — comum em consultas agrupadas que ordenam pelo valor calculado.

```go
err := db.Select(taskEnt).
	Fields(taskEnt.Col("owner_id"), krios.Count().As("total")).
	GroupBy(taskEnt.Col("owner_id")).
	OrderBy(krios.Count().Asc()).
	All(ctx, &stats)
// ORDER BY COUNT(*) ASC
```

---

## Aggregate.Desc

Ordenação descendente pelo agregado.

```go
err := db.Select(taskEnt).
	Fields(taskEnt.Col("owner_id"), krios.Count().As("total")).
	GroupBy(taskEnt.Col("owner_id")).
	OrderBy(krios.Count().Desc()).
	All(ctx, &stats)
// ORDER BY COUNT(*) DESC
```

---

## Sort.Entities

Devolve as entidades referenciadas pela expressão do critério de ordenação (sem ordem garantida); vale para ordenações por coluna ou por agregado.

```go
s := taskEnt.Col("id").Desc()

for _, e := range s.Entities() {
	fmt.Println(e.Table()) // tasks
}
```

---

# Capítulo 4: Consultas com Select

---

## Select

Builder encadeável de SELECT para uma entidade base. Crie com `db.Select(ent)` ou `tx.Select(ent)`; é mutável no lugar (um builder por consulta, não seguro para reuso concorrente) e executa através do `DB` ou `Tx` que o criou. A construção é pura: falhas de construção aparecem no `Build` ou na primeira execução, com `ErrInvalidQuery`, antes de chegar ao driver.

```go
ent, _ := krios.EntityOf[User]()

var users []User
err := db.Select(ent).                       // ou tx.Select(ent)
	Where(ent.Col("status").Eq(StatusActive)).
	OrderBy(ent.Col("name").Asc()).
	All(ctx, &users)
```

Sem `Only`/`Fields`, a seleção padrão é todas as colunas mapeadas, na ordem de declaração. Os métodos desta seção e do restante do capítulo documentam cada encadeamento e cada forma de execução.

---

## Select.Only

Restringe a seleção às colunas nomeadas, validadas contra a entidade base no render. Chamadas repetidas acumulam; seleção vazia mantém o padrão (todas as colunas). Coluna desconhecida ou duplicada falha com `ErrInvalidQuery`. Com `All`, colunas ausentes na projeção ficam com o valor zero na struct.

```go
var users []User
err := db.Select(ent).
	Only("id", "name").
	All(ctx, &users)
// email, bio e status ficam zerados em cada User
```

---

## Select.Fields

Acrescenta expressões (`Column` ou `Aggregate`) à lista de seleção, na ordem das chamadas. Sem `Only`, a seleção contém somente essas expressões; com `Only`, elas seguem as colunas selecionadas. Aliases são validados no render (não podem ser brancos nem duplicados).

```go
type NameCount struct {
	Name  string `krios:"name"`
	Total int64  `krios:"total"`
}

var counts []NameCount
err := db.Select(ent).
	Fields(ent.Col("name"), krios.Count().As("total")).
	GroupBy(ent.Col("name")).
	All(ctx, &counts)
```

---

## Select.Distinct

Remove linhas duplicadas do resultado (`SELECT DISTINCT`).

```go
var ownerIDs []Task
err := db.Select(taskEnt).
	Only("owner_id").
	Distinct().
	All(ctx, &ownerIDs)
```

---

## Select.Where

Acrescenta condições de filtro, combinadas com AND. Chamadas repetidas acumulam com AND; sem condições não há cláusula WHERE. Condições podem citar entidades participantes do JOIN.

```go
err := db.Select(taskEnt).
	Where(
		taskEnt.Col("done").Eq(false),
		taskEnt.Col("owner_id").Eq(int64(1)),
	).
	All(ctx, &tasks)
// WHERE tasks.done = false AND tasks.owner_id = 1
```

---

## Select.Join

Acrescenta um JOIN da entidade informada, com o tipo (`JoinKind`) e a condição `ON`. A entidade juntada deve ser diferente da base e das já juntadas; a ordem das declarações é preservada. Entidade referenciada mas fora da query falha com `ErrInvalidQuery`.

```go
var tasks []Task
err := db.Select(taskEnt).
	Join(userEnt, krios.InnerJoin,
		userEnt.Col("id").Eq(taskEnt.Col("owner_id"))).
	Where(userEnt.Col("email").Eq("ana@example.com")).
	All(ctx, &tasks)
```

---

## JoinKind

Tipo que seleciona a operação de `Select.Join`. Valores: `InnerJoin` e `LeftJoin`.

```go
var kind krios.JoinKind = krios.LeftJoin
err := db.Select(userEnt).Join(taskEnt, kind, on).All(ctx, &users)
```

---

## InnerJoin

`JoinKind` que mantém apenas as linhas com correspondência dos dois lados.

```go
err := db.Select(taskEnt).
	Join(userEnt, krios.InnerJoin,
		userEnt.Col("id").Eq(taskEnt.Col("owner_id"))).
	All(ctx, &tasks)
```

---

## LeftJoin

`JoinKind` que mantém todas as linhas da tabela base, preenchendo com NULL as colunas juntadas quando não há correspondência.

```go
err := db.Select(userEnt).
	Join(taskEnt, krios.LeftJoin,
		taskEnt.Col("owner_id").Eq(userEnt.Col("id"))).
	All(ctx, &users)
// usuários sem tarefas aparecem com os campos de tasks zerados
```

---

## Select.GroupBy

Acrescenta colunas de agrupamento, na ordem das chamadas; repetições acumulam.

```go
err := db.Select(taskEnt).
	Fields(taskEnt.Col("owner_id"), krios.Count().As("total")).
	GroupBy(taskEnt.Col("owner_id")).
	All(ctx, &stats)
// GROUP BY tasks.owner_id
```

---

## Select.OrderBy

Acrescenta critérios de ORDER BY, na ordem das chamadas; repetições acumulam. Critérios são construídos com `Asc`/`Desc`.

```go
err := db.Select(ent).
	OrderBy(ent.Col("status").Asc(), ent.Col("name").Desc()).
	All(ctx, &users)
// ORDER BY users.status ASC, users.name DESC
```

---

## Select.Limit

Define o número máximo de linhas retornadas. A última chamada vence; `Limit(0)` retorna nenhuma linha. Valor negativo é reportado pelo `Build` com `ErrInvalidQuery`.

```go
err := db.Select(ent).Limit(10).All(ctx, &users)
// ... LIMIT ? com arg 10
```

---

## Select.Offset

Define quantas linhas pular. A última chamada vence. Um offset positivo exige `Limit` para portabilidade entre os bancos suportados (`ErrInvalidQuery` caso contrário).

```go
err := db.Select(ent).
	OrderBy(ent.Col("id").Asc()).
	Limit(10).
	Offset(20).
	All(ctx, &users)
// ... LIMIT ? OFFSET ? com args 10, 20
```

---

## Select.Build

Renderiza a consulta e seus parâmetros sem executar. Os parâmetros seguem a ordem textual do SQL. Falhas de construção (coluna/entidade desconhecida, operando nil, entidade ou seleção ou alias duplicado, tipo de JOIN inválido, limit/offset inválido) são reportadas com `ErrInvalidQuery`.

```go
query, args, err := db.Select(ent).
	Where(ent.Col("status").Eq(StatusActive)).
	OrderBy(ent.Col("name").Asc()).
	Limit(10).
	Build()
if errors.Is(err, krios.ErrInvalidQuery) {
	return err
}
fmt.Println(query)
// SELECT "users"."id", "users"."name", ... FROM "users"
//   WHERE "users"."status" = ? ORDER BY "users"."name" ASC LIMIT ?
fmt.Println(args) // [active 10]
```

No PostgreSQL os placeholders seriam `$1`, `$2`, `$3`.

---

## Select.Rows

Executa a consulta e devolve o iterador. O consumidor deve fechar o iterador e checar `Err`, ou usar `All`. Falhas de construção são devolvidas antes de chegar ao executor; guardas de `DB`/`Tx` e a política transacional valem inalteradas.

```go
rows, err := db.Select(ent).
	Where(ent.Col("status").Eq(StatusActive)).
	Rows(ctx)
if err != nil {
	return err
}
defer rows.Close()

for rows.Next() {
	var u User
	if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Bio, &u.Status); err != nil {
		return err
	}
}
return rows.Err()
```

`Rows.All` (ver [Capítulo 7](#capítulo-7-resultados-e-leitura-de-linhas)) mapeia por nome de coluna e é mais simples quando o destino é `*[]Struct`.

---

## Select.Row

Executa a consulta esperada em no máximo uma linha. O consumo e o erro chegam no `Scan`, que preserva `sql.ErrNoRows`; falhas de construção são adiadas para o `Scan`.

```go
row := db.Select(ent).
	Where(ent.Col("id").Eq(int64(1))).
	Row(ctx)

var user User
if err := row.Scan(&user); errors.Is(err, sql.ErrNoRows) {
	// inexistente
}
```

---

## Select.All

Executa a consulta e mapeia cada linha em `dest`, que deve ser ponteiro para slice de structs (`*[]T`). O iterador é fechado em todo caminho e o slice só é substituído no sucesso. As colunas do resultado são casadas com os campos por nome (exato, depois case-insensitive).

```go
var users []User
err := db.Select(ent).
	Where(ent.Col("status").Eq(StatusActive)).
	All(ctx, &users)
if err != nil {
	return err
}
fmt.Println(len(users))
```

---

## Select.One

Executa a consulta e faz o scan da única linha esperada em `dest`. Devolve `sql.ErrNoRows` quando nenhuma linha casa. O destino struct é preenchido **posicionalmente**: a lista de seleção deve seguir a ordem de declaração dos campos mapeados (com a seleção padrão, essa ordem é a do struct).

```go
var user User
err := db.Select(ent).
	Where(ent.Col("id").Eq(int64(1))).
	One(ctx, &user)
if errors.Is(err, sql.ErrNoRows) {
	// inexistente
}
```

---

## Select.Count

Devolve o número de linhas que a consulta casa, ignorando `ORDER BY`, `LIMIT` e `OFFSET` do builder. Conta via `SELECT COUNT(*)` sobre a consulta como tabela derivada, preservando `DISTINCT` e `GROUP BY`.

```go
total, err := db.Select(taskEnt).
	Where(
		taskEnt.Col("owner_id").Eq(int64(1)),
		taskEnt.Col("done").Eq(false),
	).
	Count(ctx)
if err != nil {
	return err
}
fmt.Println(total)
```

---

## Select.Paginate

Conta a consulta, executa a página a partir do offset, mapeia as linhas em `dest` (como `Rows.All`) e devolve os totais e estatísticas de navegação. `Limit` deve ser maior que zero e `Offset` não negativo; em erro nenhum resultado parcial é devolvido. `Paginate` não muta o builder. Em `*DB` a contagem e a página são consultas independentes (escritas concorrentes podem divergi-las — use uma transação para um snapshot consistente); em `*Tx` ambas compartilham a transação.

```go
var users []User
page, err := db.Select(ent).
	OrderBy(ent.Col("id").Asc()).
	Paginate(ctx, 10, 20, &users) // página 3 de tamanho 10
if err != nil {
	return err
}

fmt.Println(page.Total, page.Page, page.Pages, page.HasNext, page.HasPrev)
```

---

## Paginate

Estrutura devolvida por `Select.Paginate` com os totais e a navegação da página. Campos: `Total` (linhas que casam com a consulta, ignorando ordenação/limit/offset), `Limit` e `Offset` (valores usados na consulta de dados), `Page` (número da página, base 1), `Pages` (total de páginas, zero quando `Total` é zero), `HasNext` e `HasPrev`.

```go
var users []User
page, err := db.Select(ent).Paginate(ctx, 10, 0, &users)
if err != nil {
	return err
}

if page.HasNext {
	fmt.Println("próxima página em offset", page.Limit)
}
fmt.Println(page.Page, "de", page.Pages, "— total", page.Total)
```

---

# Capítulo 5: Escritas com Insert, Update e Delete

Todos os escritores são criados com `db.Insert(ent)`/`tx.Insert(ent)` (idem `Update`/`Delete`), são mutáveis no lugar (um builder por statement, sem reuso concorrente) e reportam falhas de construção com `ErrInvalidQuery`, `ErrInvalidEntity` ou `ErrValidation` antes de chegar ao driver. `Update` e `Delete` exigem ao menos uma condição em `Where` — não existe atalho para afetar todas as linhas.

---

## Insert

Builder encadeável de INSERT de **uma** linha (inserções múltiplas são extensão futura registrada). Crie com `db.Insert(ent)` ou `tx.Insert(ent)`. O caminho típico é `SetValues` → `Omit` (opcional) → `Exec` (ou `Returning` + `One`).

```go
ent, _ := krios.EntityOf[User]()

user := &User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}

res, err := db.Insert(ent).
	SetValues(user).
	Omit("id").
	Exec(ctx)
if err != nil {
	return err
}
if id, ok := res.LastInsertId(); ok {
	user.ID = id
}
```

---

## Insert.SetValues

Atribui a linha a inserir. `record` deve ser `T` ou `*T` da entidade. Os valores são extraídos e os `Validator` dos campos rodam **na chamada**; falhas são lembradas e reportadas por `Build` e pelos métodos de execução antes do driver (a primeira falha vence). O registro vira snapshot: mudanças posteriores em `record` não afetam o statement. Política de escrita: ponteiro nil vincula NULL; colunas omitidas (ver `Omit`) usam o default do banco.

```go
res, err := db.Insert(ent).
	SetValues(&User{
		Name:   "Ana",
		Email:  "ana@example.com",
		Bio:    nil,           // vira NULL
		Status: StatusActive,  // validado por Status.Validate
	}).
	Omit("id").
	Exec(ctx)
```

Sem `SetValues`, `Build` falha com `ErrInvalidQuery`.

---

## Insert.Omit

Exclui as colunas nomeadas do INSERT para que o default do banco se aplique. Nomes desconhecidos ou duplicados são reportados por `Build` com `ErrInvalidQuery`. Chamadas repetidas acumulam.

```go
res, err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id"). // id é gerado pelo banco
	Exec(ctx)
```

---

## Insert.Returning

Solicita a linha inserida de volta via cláusula `RETURNING`. Sem ele, `Exec` devolve o resultado da execução; com ele, `One` preenche o destino. O dialeto precisa suportar `RETURNING` (SQLite e PostgreSQL; MySQL falha no `Build` antes de executar com `ErrInvalidQuery`).

```go
var created User
err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Returning().
	One(ctx, &created)
// created.ID preenchido pelo RETURNING (indisponível em MySQL)
```

Com `Returning`, `Exec` é rejeitado com `ErrInvalidQuery` ("insert with Returning requires One").

---

## Insert.Build

Renderiza o statement e seus parâmetros sem executar, na ordem textual: lista de colunas, valores e então `RETURNING`. Falhas de construção: entidade ou `SetValues` ausente, nenhuma coluna para escrever (após `Omit`), `Omit` inválido, `RETURNING` sem suporte no dialeto ou falha lembrada de `SetValues` — com `ErrInvalidQuery`, `ErrInvalidEntity` ou `ErrValidation`.

```go
query, args, err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Build()
fmt.Println(query)
// INSERT INTO "users" ("name", "email", "bio", "status") VALUES (?, ?, ?, ?)
fmt.Println(args) // [Ana ana@example.com <nil> active]
```

---

## Insert.Exec

Executa o INSERT e devolve o `krios.Result` (linhas afetadas e último id, com flags de disponibilidade). É rejeitado após `Returning`, onde `One` deve ser usado. Falhas de construção e guardas de `DB`/`Tx` são aplicadas antes da execução.

```go
res, err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Exec(ctx)
if err != nil {
	return err
}
if id, ok := res.LastInsertId(); ok {
	user.ID = id // indisponível em PostgreSQL (ok == false)
}
```

---

## Insert.One

Executa o INSERT com `Returning` e faz o scan da linha devolvida em `dest` (ponteiro para o struct da entidade). `sql.ErrNoRows` é preservado quando nenhuma linha volta. É rejeitado sem `Returning`.

```go
var created User
err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Returning().
	One(ctx, &created)
if err != nil {
	return err
}
fmt.Println(created.ID)
```

O `RETURNING` devolve todas as colunas mapeadas na ordem de declaração — a mesma do preenchimento posicional do struct.

---

## Update

Builder encadeável de UPDATE. Crie com `db.Update(ent)` ou `tx.Update(ent)`. Exige `SetValues` e ao menos uma condição em `Where` (sempre combinadas com AND); pode afetar várias linhas.

```go
res, err := db.Update(taskEnt).
	SetValues(&Task{Done: true}).
	Where(taskEnt.Col("id").Eq(int64(1))).
	Exec(ctx)
```

---

## Update.SetValues

Atribui os valores a gravar. `record` deve ser `T` ou `*T` da entidade; extração e `Validator` rodam na chamada (primeira falha vence, lembrada para `Build`/execução) e o registro vira snapshot. Política de escrita: campo ponteiro nil **não** atualiza a coluna; `driver.Valuer` inválido (ex.: `sql.Null*` com `Valid false`) grava NULL; campo não ponteiro é gravado **inclusive com valor zero** — nesse caso use `Omit` para preservar a coluna.

```go
// altera apenas owner_id: title/done/id ficam de fora pelo Omit
res, err := db.Update(taskEnt).
	SetValues(&Task{OwnerID: 7}).
	Omit("id", "title", "done").
	Where(taskEnt.Col("owner_id").Eq(int64(1))).
	Exec(ctx)
```

---

## Update.Where

Acrescenta condições de filtro, combinadas com AND; chamadas repetidas acumulam. Ao menos uma condição é exigida antes da execução — não existe atalho para atualizar tudo. As condições devem citar apenas a tabela base do statement.

```go
res, err := db.Update(taskEnt).
	SetValues(&Task{Done: true}).
	Where(
		taskEnt.Col("owner_id").Eq(int64(1)),
		taskEnt.Col("done").Eq(false),
	).
	Exec(ctx)
// UPDATE tasks SET ... WHERE owner_id = 1 AND done = false
```

Sem `Where`, `Build` falha com `ErrInvalidQuery`.

---

## Update.Omit

Exclui as colunas nomeadas da atribuição, deixando-as inalteradas. Nomes desconhecidos ou duplicados são reportados por `Build` com `ErrInvalidQuery`. Chamadas repetidas acumulam.

```go
res, err := db.Update(ent).
	SetValues(&User{Name: "Ana novo", Email: "novo@example.com"}).
	Omit("id", "bio", "status"). // preserva id, bio e status
	Where(ent.Col("id").Eq(int64(1))).
	Exec(ctx)
```

---

## Update.Returning

Solicita as linhas atualizadas de volta via `RETURNING`. Sem ele, `Exec` devolve o resultado; com ele, `All` preenche o destino. SQLite e PostgreSQL suportam; MySQL falha no `Build` com `ErrInvalidQuery`.

```go
var updated []User
err := db.Update(ent).
	SetValues(&User{Status: StatusInactive}).
	Omit("id", "name", "email", "bio").
	Where(ent.Col("status").Eq(StatusActive)).
	Returning().
	All(ctx, &updated)
```

Com `Returning`, `Exec` é rejeitado com `ErrInvalidQuery`.

---

## Update.Build

Renderiza o statement sem executar, na ordem textual: atribuições primeiro e depois o `WHERE`, para que a numeração dos placeholders (inclusive `$n` do PostgreSQL) siga o texto do SQL. Falhas: entidade ou `SetValues` ausente, nenhuma coluna para gravar (após a política e `Omit`), `Where` ausente, condições de outra tabela, `Omit` inválido, `RETURNING` sem suporte ou falha lembrada de `SetValues`.

```go
query, args, err := db.Update(taskEnt).
	SetValues(&Task{Done: true}).
	Where(taskEnt.Col("id").Eq(int64(1))).
	Build()
fmt.Println(query)
// UPDATE "tasks" SET "id" = ?, "title" = ?, "owner_id" = ?, "done" = ? WHERE "tasks"."id" = ?
```

Como `Task` não tem ponteiros, todos os campos entram na atribuição — é o efeito da política de escrita; use `Omit` para preservar colunas.

---

## Update.Exec

Executa o UPDATE e devolve o `krios.Result` (com `RowsAffected`). É rejeitado após `Returning`, onde `All` deve ser usado.

```go
res, err := db.Update(ent).
	SetValues(&User{Status: StatusInactive}).
	Omit("id", "name", "email", "bio").
	Where(ent.Col("status").Eq(StatusActive)).
	Exec(ctx)
if err != nil {
	return err
}
affected, ok := res.RowsAffected()
fmt.Println(affected, ok)
```

---

## Update.All

Executa o UPDATE com `Returning` e mapeia todas as linhas devolvidas em `dest` (ponteiro para slice do struct da entidade). O iterador é fechado em todo caminho. É rejeitado sem `Returning`.

```go
var updated []User
err := db.Update(ent).
	SetValues(&User{Status: StatusInactive}).
	Omit("id", "name", "email", "bio").
	Where(ent.Col("status").Eq(StatusActive)).
	Returning().
	All(ctx, &updated)
```

---

## Delete

Builder encadeável de DELETE. Crie com `db.Delete(ent)` ou `tx.Delete(ent)`. Exige ao menos uma condição em `Where`; pode afetar várias linhas.

```go
res, err := db.Delete(taskEnt).
	Where(taskEnt.Col("done").Eq(true)).
	Exec(ctx)
```

---

## Delete.Where

Acrescenta condições de filtro, combinadas com AND; chamadas repetidas acumulam. Ao menos uma condição é exigida antes da execução. As condições devem citar apenas a tabela base.

```go
res, err := db.Delete(taskEnt).
	Where(taskEnt.Col("owner_id").Eq(int64(1))).
	Exec(ctx)
// DELETE FROM tasks WHERE owner_id = 1
```

Sem `Where`, `Build` falha com `ErrInvalidQuery`.

---

## Delete.Returning

Solicita as linhas removidas de volta via `RETURNING`. Sem ele, `Exec` devolve o resultado; com ele, `All` preenche o destino. SQLite e PostgreSQL suportam; MySQL falha no `Build` com `ErrInvalidQuery`.

```go
var removed []User
err := db.Delete(ent).
	Where(ent.Col("status").Eq(StatusInactive)).
	Returning().
	All(ctx, &removed)
```

---

## Delete.Build

Renderiza o statement sem executar. Falhas: entidade ausente, `Where` ausente, condições de outra tabela, `RETURNING` sem suporte — todas com `ErrInvalidQuery`.

```go
query, args, err := db.Delete(taskEnt).
	Where(taskEnt.Col("done").Eq(true)).
	Build()
fmt.Println(query)
// DELETE FROM "tasks" WHERE "tasks"."done" = ?
fmt.Println(args) // [true]
```

---

## Delete.Exec

Executa o DELETE e devolve o `krios.Result` (com `RowsAffected`). É rejeitado após `Returning`, onde `All` deve ser usado.

```go
res, err := db.Delete(taskEnt).
	Where(taskEnt.Col("done").Eq(true)).
	Exec(ctx)
if err != nil {
	return err
}
removed, _ := res.RowsAffected()
```

---

## Delete.All

Executa o DELETE com `Returning` e mapeia as linhas devolvidas em `dest` (ponteiro para slice do struct da entidade). É rejeitado sem `Returning`.

```go
var removed []Task
err := db.Delete(taskEnt).
	Where(taskEnt.Col("done").Eq(true)).
	Returning().
	All(ctx, &removed)
```

---

# Capítulo 6: Transações

---

## Tx

Transação explícita retornada por `DB.Begin`. Execução e finalização são serializadas por um mutex (sem garantia de ordem entre goroutines); o consumo dos `Row`/`Rows` devolvidos **não** é serializado — consuma ou feche resultados antes da próxima operação e aguarde os consumidores antes de finalizar. Depois de finalizada, toda chamada retorna `ErrTxDone`. A **primeira falha observada** (erro imediato de `ExecContext`/`QueryContext` **ou** falha tardia de leitura em `Scan`/iteração/`Close`) é lembrada e bloqueia o `Commit`.

`Tx` oferece os mesmos construtores de builder que `DB` — `Select`, `Insert`, `Update`, `Delete`, `File` — além de `ExecContext`, `QueryContext` e `QueryRowContext`, todos executando na transação:

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx) // no-op após um Commit bem-sucedido

var user User
if err := tx.Select(ent).
	Where(ent.Col("id").Eq(int64(1))).
	One(ctx, &user); err != nil {
	return err
}

if _, err := tx.Update(ent).
	SetValues(&User{Status: StatusInactive}).
	Omit("id", "name", "email", "bio").
	Where(ent.Col("id").Eq(user.ID)).
	Exec(ctx); err != nil {
	return err
}

return tx.Commit(ctx)
```

O contexto passado ao `Begin` vigora durante toda a transação; seu cancelamento faz o `database/sql` reverter tudo. O padrão recomendado é `defer tx.Rollback(ctx)` + `Commit` explícito no fim do caminho de sucesso.

---

## Tx.ExecContext

Executa um statement sem retorno de linhas na transação. O primeiro erro é lembrado e bloqueia o `Commit`. Depois de finalizada, retorna `ErrTxDone`.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

if _, err := tx.ExecContext(ctx,
	"UPDATE users SET status = ? WHERE id = ?",
	StatusInactive, int64(1),
); err != nil {
	return err // erro registrado: Commit seria bloqueado
}

return tx.Commit(ctx)
```

---

## Tx.QueryContext

Executa uma consulta com linhas na transação. O primeiro erro é lembrado e bloqueia o `Commit`; falhas tardias observadas no consumo do iterador também são registradas. Depois de finalizada, retorna `ErrTxDone`.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

rows, err := tx.QueryContext(ctx,
	"SELECT id, name FROM users WHERE status = ?", StatusActive,
)
if err != nil {
	return err
}
defer rows.Close()

for rows.Next() {
	var id int64
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		return err // falha de leitura registrada no tx
	}
}
if err := rows.Err(); err != nil {
	return err
}

return tx.Commit(ctx)
```

---

## Tx.QueryRowContext

Executa uma consulta esperada em no máximo uma linha na transação. O erro chega no `Scan`; falhas tardias (que não `sql.ErrNoRows`) são registradas e bloqueiam o `Commit`. Depois de finalizada, devolve uma linha que falha com `ErrTxDone`.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

var name string
err = tx.QueryRowContext(ctx,
	"SELECT name FROM users WHERE id = ?", int64(1),
).Scan(&name)
if errors.Is(err, sql.ErrNoRows) {
	return nil
}
if err != nil {
	return err
}

return tx.Commit(ctx)
```

---

## Tx.Commit

Tenta confirmar, **a menos que** uma falha já tenha sido registrada. Falhas registradas: o primeiro erro imediato de `ExecContext`/`QueryContext` e a primeira falha tardia de leitura (`Row.Scan`, `Rows.Scan`, iteração, `Close`). Nesses casos o `Commit` tenta o rollback mesmo com `ctx` cancelado e devolve o erro original (embrulhando também um erro de rollback, se houver). Sem falha registrada, um `ctx` cancelado rejeita o `Commit` antes da chamada nativa e permite nova tentativa; após qualquer tentativa nativa a transação é terminal, mesmo em falha. `sql.ErrNoRows` e observações de estado terminal **não** são registradas. O consumidor ainda deve reverter explicitamente no primeiro erro — o bloqueio do `Commit` é a rede de proteção.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

_, err = tx.ExecContext(ctx,
	"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
	"Ana", "ana@example.com", nil, StatusActive)
if err == nil {
	// segunda escrita viola o UNIQUE de email e falha:
	_, _ = tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		"Clone", "ana@example.com", nil, StatusActive)
}

if err := tx.Commit(ctx); err != nil {
	// Commit bloqueado pelo erro registrado: rollback automático
	// e o erro original devolvido (embutido na mensagem)
	return err
}
return errors.New("commit não foi bloqueado")
```

Após qualquer tentativa nativa (inclusive falha), as chamadas seguintes retornam `ErrTxDone`.

---

## Tx.Rollback

Tenta a limpeza **mesmo** com `ctx` cancelado ou expirado; o contexto é mantido por compatibilidade de API e não interrompe o rollback nativo. Depois da tentativa nativa, todas as chamadas seguintes retornam `ErrTxDone`. Um rollback após `Commit` bem-sucedido é controlado pelo estado interno — por isso o padrão `defer tx.Rollback(ctx)` é seguro.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx) // cobre todos os retornos antecipados

if _, err := tx.ExecContext(ctx, "DELETE FROM tasks WHERE done = ?", true); err != nil {
	return err // o defer reverte o grupo inteiro
}

if err := tx.Commit(ctx); err != nil {
	return err
}
return nil
// defer: Rollback após Commit é inofensivo
```

---

## Tx.Select

Cria um builder de `Select` renderizado com o dialeto da transação e executado por ela. Falhas de construção são reportadas antes da execução com `ErrInvalidQuery`; falhas de statement na execução são registradas no `Tx` e bloqueiam o `Commit`. Depois de terminal, a execução devolve `ErrTxDone`.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

var user User
if err := tx.Select(ent).
	Where(ent.Col("id").Eq(int64(1))).
	One(ctx, &user); err != nil {
	return err
}

return tx.Commit(ctx)
```

---

## Tx.Insert

Cria um builder de `Insert` executado na transação. Erros de statement registrados bloqueiam o `Commit`; falhas de construção (`ErrInvalidQuery`, `ErrInvalidEntity`, `ErrValidation`) são adiadas para `Build`/execução, antes do driver.

```go
if _, err := tx.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Exec(ctx); err != nil {
	return err
}
```

---

## Tx.Update

Cria um builder de `Update` executado na transação. Exige pelo menos uma condição em `Where`; falhas de execução são registradas e bloqueiam o `Commit`.

```go
if _, err := tx.Update(taskEnt).
	SetValues(&Task{Done: true}).
	Where(taskEnt.Col("id").Eq(int64(1))).
	Exec(ctx); err != nil {
	return err
}
```

---

## Tx.Delete

Cria um builder de `Delete` executado na transação. Exige pelo menos uma condição em `Where`; falhas de execução são registradas e bloqueiam o `Commit`.

```go
if _, err := tx.Delete(taskEnt).
	Where(taskEnt.Col("done").Eq(true)).
	Exec(ctx); err != nil {
	return err
}
```

---

# Capítulo 7: Resultados e leitura de linhas

---

## Result

Interface que expõe as informações do driver de forma agnóstica ao banco: `RowsAffected()` e `LastInsertId()`, cada um devolvendo `(int64, bool)`. A flag `false` significa que a informação não está disponível — o erro original do driver **não** é exposto. Valor zero com `true` é um resultado válido. Retornado pelos métodos `Exec` de `DB`/`Tx`, pelos builders de escrita e por `File.Exec`.

```go
res, err := db.ExecContext(ctx, "DELETE FROM tasks WHERE done = ?", true)
if err != nil {
	return err
}

if affected, ok := res.RowsAffected(); ok {
	fmt.Println("linhas removidas:", affected)
}
```

---

## Result.RowsAffected

Número de linhas afetadas pelo statement, com flag de disponibilidade. Disponibilidade depende do driver e do statement.

```go
res, err := db.Update(taskEnt).
	SetValues(&Task{Done: true}).
	Where(taskEnt.Col("done").Eq(false)).
	Exec(ctx)
if err != nil {
	return err
}
if n, ok := res.RowsAffected(); ok {
	fmt.Println("concluídas:", n)
}
```

---

## Result.LastInsertId

Identificador gerado pelo banco na última inserção, com flag de disponibilidade. Disponível em SQLite e MySQL; **indisponível no PostgreSQL** (`ok == false`) — nesse caso use `Returning` + `One` para obter o id.

```go
res, err := db.Insert(ent).
	SetValues(&User{Name: "Ana", Email: "ana@example.com", Status: StatusActive}).
	Omit("id").
	Exec(ctx)
if err != nil {
	return err
}

if id, ok := res.LastInsertId(); ok {
	user.ID = id
} else {
	// PostgreSQL: use Returning().One(ctx, &user)
}
```

---

## Row

Resultado adiado de uma consulta de linha única (criado por `DB.QueryRowContext`, `Tx.QueryRowContext`, `Select.Row` e `File.QueryRow`). O resultado ou o erro chegam no `Scan`. `sql.ErrNoRows` é preservado. Dentro de uma transação, qualquer falha de `Scan` que não seja `ErrNoRows` é registrada e bloqueia o `Commit`.

```go
row := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = ?", int64(1))

var name string
if err := row.Scan(&name); errors.Is(err, sql.ErrNoRows) {
	// inexistente
}
```

---

## Row.Scan

Copia os valores da linha correspondente para os destinos. Devolve `sql.ErrNoRows` quando não há linha. Um único destino struct é preenchido a partir das colunas mapeadas da entidade **na ordem de declaração** (a lista de seleção deve seguir essa ordem); qualquer outro conjunto de destinos é varrido posicionalmente. Falhas são registradas no dono (a transação) e bloqueiam o `Commit`.

```go
// struct: a seleção padrão segue a ordem dos campos
var user User
err := db.Select(ent).Where(ent.Col("id").Eq(int64(1))).One(ctx, &user)

// destinos posicionais: a ordem da lista de seleção deve coincidir
row := db.Select(ent).
	Only("id", "name").
	Where(ent.Col("id").Eq(int64(1))).
	Row(ctx)
var id int64
var name string
if err := row.Scan(&id, &name); errors.Is(err, sql.ErrNoRows) {
	// inexistente
}
```

Tipos que implementam `sql.Scanner` (ex.: `time.Time`, `sql.NullString`) mantêm semântica posicional mesmo como destino único.

---

## Rows

Iterador sobre o resultado de uma consulta (criado por `DB.QueryContext`, `Tx.QueryContext`, `Select.Rows` e `File.Query`). O consumidor é responsável por chamar `Close()` — inclusive em retornos antecipados — e por checar `Err()` depois da iteração. `Close` é idempotente e preserva `Err`. Use um único consumidor para `Next` e `Scan`; consumo concorrente do mesmo iterador não é suportado. Dentro de uma transação, falhas de scan/iteração são registradas e bloqueiam o `Commit`.

```go
rows, err := db.Select(ent).Rows(ctx)
if err != nil {
	return err
}
defer rows.Close()

for rows.Next() {
	var u User
	if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Bio, &u.Status); err != nil {
		return err
	}
}
return rows.Err()
```

---

## Rows.Next

Prepara a próxima leitura; devolve `false` no fim dos resultados. Uma iteração terminada com falha é reportada ao dono (transação) e bloqueia o `Commit`.

```go
for rows.Next() {
	// ...
}
if err := rows.Err(); err != nil {
	return err
}
```

---

## Rows.Scan

Copia os valores da linha atual para os destinos, posicionalmente. Uma falha é reportada ao dono e bloqueia o `Commit`.

```go
for rows.Next() {
	var id int64
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		return err
	}
	fmt.Println(id, name)
}
```

Para structs, `Rows.All` (mapeamento por nome de coluna) é mais simples e menos sensível à ordem.

---

## Rows.Err

Devolve o erro, se houver, encontrado durante a iteração. Deve ser chamado após o loop de `Next`. Uma falha não nil é reportada ao dono e bloqueia o `Commit`.

```go
for rows.Next() {
	// ...
}
if err := rows.Err(); err != nil {
	return err
}
```

---

## Rows.Columns

Devolve os nomes das colunas do conjunto de resultados, na ordem do driver. Falhas são reportadas ao dono e bloqueiam o `Commit`.

```go
cols, err := rows.Columns()
if err != nil {
	return err
}
fmt.Println(cols) // [id name email bio status]
```

---

## Rows.Close

Fecha o iterador subjacente. É idempotente. Falhas de fechamento ou de iteração pendente são reportadas ao dono e bloqueiam o `Commit`. Chame sempre, inclusive via `defer`.

```go
rows, err := db.Select(ent).Rows(ctx)
if err != nil {
	return err
}
defer rows.Close()
```

---

## Rows.All

Consome as linhas restantes e as armazena no slice apontado por `dest` (`*[]T`, `T` struct). As colunas do resultado são casadas com os campos **por nome** (exato primeiro, depois case-insensitive); colunas sem campo são descartadas e campos sem coluna ficam zero. O iterador é sempre fechado; no sucesso o slice é substituído, em falha permanece inalterado. Destino inválido falha com `ErrInvalidEntity`.

```go
var users []User
err := db.Select(ent).
	Only("id", "name").
	Where(ent.Col("status").Eq(StatusActive)).
	All(ctx, &users)
if err != nil {
	return err
}
// bio, email e status zerados; slice intacto em caso de erro
```

O mesmo mapeamento é usado por `Select.All`, `Update.All`, `Delete.All` e `File.All`.

---

# Capítulo 8: SQL embutido

---

## NewSQLFile

Carrega `path` de um `fs.FS` do consumidor (por exemplo um `embed.FS`) e devolve o recurso SQL imutável. O carregamento é imediato: um FS nil, caminho inválido ou ausente, diretório ou arquivo em branco falham com `ErrFileNotFound` **antes** de tocar no banco. O texto é lido uma única vez; o recurso é seguro para reuso concorrente.

```go
import (
	"embed"

	"github.com/mineot/krios"
)

//go:embed schema/*.sql
var schemaFS embed.FS

schema, err := krios.NewSQLFile(schemaFS, "schema/users.sql")
if err != nil {
	if errors.Is(err, krios.ErrFileNotFound) {
		// recurso ausente, caminho inválido ou arquivo em branco
	}
	return err
}
```

O recurso é executado fora de transação com `DB.File` ou dentro de uma com `Tx.File`; os parâmetros vão separados, com os marcadores que o driver aceita, e não há tradução de sintaxe.

---

## SQLFile

Recurso SQL imutável criado por `NewSQLFile`: guarda o caminho e o texto carregado, sem conexão, dialeto ou estado mutável — seguro para reuso. `Name` e `Text` expõem o conteúdo.

```go
schema, err := krios.NewSQLFile(schemaFS, "schema/users.sql")
if err != nil {
	return err
}

fmt.Println(schema.Name()) // schema/users.sql
fmt.Println(schema.Text()) // CREATE TABLE users (...)
```

---

## SQLFile.Name

Devolve o caminho de onde o recurso foi carregado (usado em mensagens e logs). Recurso nil devolve `""`.

```go
fmt.Println(schema.Name()) // schema/users.sql
```

---

## SQLFile.Text

Devolve o SQL carregado. É uma cópia do conteúdo do arquivo — exatamente o texto enviado ao driver. Recurso nil devolve `""`.

```go
fmt.Println(schema.Text())
// CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, ...)
```

---

## DB.File

Cria um runner que executa o `SQLFile` por este `DB`, **fora** de transação. O texto vai ao driver inalterado, com os parâmetros vinculados separadamente; a guarda do `DB` (`ErrClosed` após `Close`) vale inalterada.

```go
schema, err := krios.NewSQLFile(schemaFS, "schema/users.sql")
if err != nil {
	return err
}

if _, err := db.File(schema).Exec(ctx); err != nil {
	return err
}
```

---

## Tx.File

Cria um runner que executa o `SQLFile` dentro da transação. Falhas de statement registradas na execução bloqueiam o `Commit`, como nas demais operações de `Tx`.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}
defer tx.Rollback(ctx)

if _, err := tx.File(schema).Exec(ctx); err != nil {
	return err // registrado no tx: Commit bloqueado
}

return tx.Commit(ctx)
```

---

## File

Runner que executa um `SQLFile` através do `DB` ou `Tx` que o criou: `db.File(f)` ou `tx.File(f)`. É mutável no lugar (um runner por execução, sem reuso concorrente) e lembra a primeira falha de construção (primeiro erro vence), reportada antes do executor. O texto vai ao driver **sem** tradução de dialeto ou placeholders e **sem** divisão de statements — um arquivo, um statement.

```go
schema, err := krios.NewSQLFile(schemaFS, "schema/users.sql")
if err != nil {
	return err
}

if _, err := db.File(schema).Exec(ctx); err != nil {
	return err
}
```

---

## File.Exec

Executa o recurso sem ler linhas e devolve o `krios.Result`. Parâmetros vão separados; guardas de `DB`/`Tx` valem inalteradas (com `tx.File`, o erro registrado bloqueia o `Commit`).

```go
if _, err := db.File(usersSchema).Exec(ctx); err != nil {
	return err
}
```

---

## File.Query

Executa o recurso e devolve o iterador. O consumidor deve fechar e checar `Err`, ou usar `All`.

```go
rows, err := db.File(findActiveUsers).Query(ctx, StatusActive)
if err != nil {
	return err
}
defer rows.Close()

for rows.Next() {
	// ...
}
return rows.Err()
```

O arquivo `findActiveUsers` conteria, por exemplo:
`SELECT id, name FROM users WHERE status = ?` — com `?` apenas se o driver for SQLite/MySQL.

---

## File.QueryRow

Executa o recurso esperado em no máximo uma linha. `Scan` consome a linha e preserva `sql.ErrNoRows`; falhas de construção são adiadas para o `Scan`.

```go
var name string
err := db.File(findUserByID).QueryRow(ctx, int64(1)).Scan(&name)
if errors.Is(err, sql.ErrNoRows) {
	// inexistente
}
```

---

## File.One

Executa o recurso e faz o scan da única linha esperada em `dest` (ponteiro para struct; mapeado como `Row.Scan`). Devolve `sql.ErrNoRows` quando não há linha. Parâmetros vão separados.

```go
var user User
err := db.File(findUserByID).One(ctx, &user, int64(1))
if errors.Is(err, sql.ErrNoRows) {
	// inexistente
}
```

---

## File.All

Executa o recurso e mapeia todas as linhas em `dest` (`*[]T`), casando colunas com campos por nome como `Rows.All`. O iterador é fechado em todo caminho. Parâmetros vão separados.

```go
var users []User
if err := db.File(listUsersByStatus).All(ctx, &users, StatusActive); err != nil {
	return err
}
```

Com `tx.File(...)`, tudo roda dentro da transação e falhas registradas bloqueiam o `Commit`.

---

# Capítulo 9: Migrações

As migrações são configuradas em `Config.Migrations` **antes** do `Open` (a configuração é validada na abertura e retida) e executadas com `DB.RunMigrations`. Os arquivos são embutidos pelo consumidor (`go:embed`) e cada versão associa os arquivos `up` e `down` na ordem de execução.

---

## MigrationVersion

Associa um número de versão aos arquivos embutidos que a implementam em cada direção. `Version` é obrigatório, único e maior que zero; `Up` e `Down` listam os arquivos na ordem de execução. O prefixo numérico do nome do arquivo é apenas convenção — a versão e a ordem vêm desta declaração.

```go
version := krios.MigrationVersion{
	Version: 1,
	Up:      []string{"migrations/001_init.up.sql"},
	Down:    []string{"migrations/001_init.down.sql"},
}
```

Versões devem ser declaradas em ordem estritamente crescente (validado por `MigrationsConfig.Validate`).

---

## MigrationsConfig

Configuração das migrações versionadas, opcional em `Config` (nil = a biblioteca nunca carrega nem roda migrações). Campos: `FS` (recursos embutidos do consumidor), `TargetVersion` (versão desejada, sempre explícita; `0` pede reverter tudo), `Versions` (associações versão → arquivos, na ordem das declarações) e `ManagedObjects` (objetos sob gestão, ex.: tabelas, usados para checar um banco sem histórico).

```go
//go:embed migrations/*.sql
var migrationFS embed.FS

cfg := krios.Config{
	Type:   krios.DBTypeSQLite,
	Driver: "sqlite",
	SQLite: &krios.SQLiteConfig{DBPath: "app.db"},
	Migrations: &krios.MigrationsConfig{
		FS:             migrationFS,
		TargetVersion:  2,
		ManagedObjects: []string{"users", "tasks"},
		Versions: []krios.MigrationVersion{
			{Version: 1, Up: []string{"migrations/001_init.up.sql"}, Down: []string{"migrations/001_init.down.sql"}},
			{Version: 2, Up: []string{"migrations/002_tasks.up.sql"}, Down: []string{"migrations/002_tasks.down.sql"}},
		},
	},
}
```

---

## MigrationsConfig.Validate

Valida a configuração de migrações de forma estrutural (a camada de migrações carrega os arquivos depois): `FS` presente, ao menos um objeto gerenciado, ao menos uma versão, versões `> 0`, únicas e em ordem estritamente crescente, e arquivos não vazios nem duplicados dentro de cada direção. Falhas casam com `ErrInvalidConfig`. `Config.Validate`/`Open` chamam este método automaticamente.

```go
m := krios.MigrationsConfig{
	FS:             migrationFS,
	ManagedObjects: []string{"users"},
	Versions: []krios.MigrationVersion{
		{Version: 1, Up: []string{"migrations/001_init.up.sql"}},
	},
}

if err := m.Validate(); err != nil {
	fmt.Println(errors.Is(err, krios.ErrInvalidConfig)) // true
}
```

---

## DB.Migrations

Devolve a configuração de migrações retida do `Open`, ou `nil` quando migrações não foram configuradas. Permite ao chamador inspecionar a mesma configuração com que o acesso foi aberto.

```go
if m := db.Migrations(); m != nil {
	fmt.Println("alvo:", m.TargetVersion)
} else {
	fmt.Println("migrações não configuradas")
}
```

---

## DB.RunMigrations

Prepara as migrações configuradas no `Open` (carrega recursos, inspeciona o histórico, monta o plano) e aplica o plano, devolvendo o plano para inspeção — **mesmo em falha**, para que o chamador veja o que foi tentado. A preparação é pura e reporta todo problema de configuração, associação, histórico ou atomicidade **antes** de qualquer statement. Plano sem passos = esquema já no alvo, nada executa. Guardas de `DB` (`ErrClosed`) valem. Um registro `in_progress`/`failed` ou objetos gerenciados em banco sem histórico bloqueiam com `ErrPartialMigration` (use `errors.As` com `*PartialMigrationError`).

```go
plan, err := db.RunMigrations(ctx)
if err != nil {
	var pme *krios.PartialMigrationError
	if errors.As(err, &pme) {
		fmt.Println("bloqueada:", pme.Status, pme.LastVersion, pme.LastFile)
	}
	return err
}

fmt.Println(plan.CurrentVersion, "→", plan.TargetVersion)
for _, step := range plan.Steps {
	fmt.Println(step.Version, step.Direction, step.File.Name(), step.Transactional)
}
```

Escopo transacional: PostgreSQL/SQLite usam uma transação por versão; MySQL usa transação por arquivo somente quando todos os statements são transacionais (DDL não transacional roda sem tx, com registro `in_progress` → `applied`/`failed` e `ErrPartialMigration` em falha — nada é revertido).

---

## MigrationPlan

Plano ordenado de passos que leva o esquema da versão atual à desejada, devolvido por `RunMigrations`. Campos: `CurrentVersion` (versão encontrada na inspeção), `TargetVersion` (versão desejada) e `Steps` (arquivos a executar, em ordem, agrupados por versão e direção; down percorre da maior versão até o alvo). Um plano vazio significa que nenhuma migração é necessária.

```go
plan, err := db.RunMigrations(ctx)
if err != nil {
	return err
}

if len(plan.Steps) == 0 {
	fmt.Println("esquema já está na versão", plan.TargetVersion)
	return nil
}

fmt.Printf("v%d → v%d em %d passos\n",
	plan.CurrentVersion, plan.TargetVersion, len(plan.Steps))
```

---

## MigrationStep

Um arquivo a executar dentro de uma versão e direção, com o escopo transacional decidido para ele. Campos: `Version` (versão do arquivo), `Direction` (`"up"` ou `"down"`), `File` (recurso `*SQLFile` carregado) e `Transactional` (se o arquivo pode rodar em transação: verdadeiro em PostgreSQL/SQLite; em MySQL, apenas quando todos os seus statements são transacionais).

```go
plan, err := db.RunMigrations(ctx)
if err != nil {
	return err
}

for _, step := range plan.Steps {
	fmt.Printf("v%d %s (%s) transacional=%v\n",
		step.Version, step.Direction, step.File.Name(), step.Transactional)
}
```

---

## PartialMigrationError

Contexto estruturado de uma migração bloqueada, recuperável com `errors.As` de um `ErrPartialMigration`. Campos: `Direction` (`"up"`/`"down"`), `TargetVersion`, `LastVersion` (última versão aplicada), `LastFile` (arquivo do registro parcial, quando conhecido), `Status` (`"in_progress"` ou `"failed"`) e `Message`. Satisfaiz `errors.Is(err, krios.ErrPartialMigration)` via `Unwrap`.

```go
plan, err := db.RunMigrations(ctx)
if err == nil {
	_ = plan
	return nil
}

var pme *krios.PartialMigrationError
if errors.As(err, &pme) {
	fmt.Println("direção:", pme.Direction)
	fmt.Println("alvo:", pme.TargetVersion, "última:", pme.LastVersion)
	fmt.Println("arquivo:", pme.LastFile, "status:", pme.Status)
	fmt.Println("motivo:", pme.Message)
	// resolva o registro parcial manualmente antes de nova tentativa
}
```

O bloqueio nunca é contornado automaticamente; o consumidor deve resolver o estado explicitamente.

---

## PartialMigrationError.Error

Implementa a interface `error` com uma mensagem única que reúne a sentinela, o motivo e o contexto (direção, alvo, última versão, arquivo e status). Não compare mensagens — use `errors.Is`/`errors.As`.

```go
var pme *krios.PartialMigrationError
if errors.As(err, &pme) {
	fmt.Println(pme.Error())
	// krios: migration blocked: <motivo> (direction up, target 2,
	// last version 1, file "002_tasks.up.sql", status "failed")
}
```

---

## PartialMigrationError.Unwrap

Devolve a sentinela `ErrPartialMigration` e, quando houver, a causa original (retornos múltiplos de `Unwrap`). É o que faz `errors.Is(err, krios.ErrPartialMigration)` funcionar sobre o erro embrulhado.

```go
var pme *krios.PartialMigrationError
if errors.As(err, &pme) {
	fmt.Println(errors.Is(pme, krios.ErrPartialMigration)) // true
}
```

---

# Capítulo 10: Erros sentinela

Todos os erros do Krios são sentinela: conferidos com `errors.Is` (ou `errors.As` para o contexto estruturado), frequentemente embrulhados com detalhe e causa. Nenhum código deve comparar mensagens.

```go
if _, err := krios.Open(ctx, cfg); err != nil {
	switch {
	case errors.Is(err, krios.ErrInvalidConfig):
		// ...
	case errors.Is(err, krios.ErrDriverNotLoaded):
		// ...
	case errors.Is(err, krios.ErrConnectionFailed):
		// ...
	}
}
```

---

## ErrInvalidConfig

Configuração ausente ou inconsistente: motor não suportado, driver vazio, configuração do motor ausente (ou presente para outro motor), campos de conexão obrigatórios faltando, porta fora de 0..65535, `Timeout` negativo ou migrações estruturalmente inválidas. Vem de `Config.Validate` (e portanto de `Open`) e de `MigrationsConfig.Validate`.

```go
cfg := krios.Config{
	Type:   krios.DBTypeSQLite,
	Driver: "sqlite",
	// SQLite ausente
}

if err := cfg.Validate(); err != nil {
	fmt.Println(errors.Is(err, krios.ErrInvalidConfig)) // true
}
```

---

## ErrDriverNotLoaded

O nome em `Config.Driver` não está registrado em `database/sql`: o consumidor não importou o driver ou registrou sob outro nome. Detectado em `Open`, antes de qualquer conexão.

```go
cfg := krios.Config{
	Type:   krios.DBTypeSQLite,
	Driver: "sqlite",
	SQLite: &krios.SQLiteConfig{DBPath: "app.db"},
}
// sem o import _ "modernc.org/sqlite"

if _, err := krios.Open(ctx, cfg); err != nil {
	fmt.Println(errors.Is(err, krios.ErrDriverNotLoaded)) // true
}
```

---

## ErrConnectionFailed

Falha de conexão (rede, arquivo, permissão, recusa do banco). Embrulha o erro original do driver — recupere a causa com `errors.Is`/`errors.As` nos erros do driver. Cancelamento/deadline do contexto **não** usam este erro: são reportados como `context.Canceled`/`context.DeadlineExceeded` embrulhados.

```go
cfg := krios.Config{
	Type:   krios.DBTypeMySQL,
	Driver: "mysql",
	MySQL: &krios.MySQLConfig{
		Host: "inexistente.invalid", User: "app",
		Password: "x", DBName: "appdb",
	},
}

if _, err := krios.Open(ctx, cfg); err != nil {
	fmt.Println(errors.Is(err, krios.ErrConnectionFailed)) // true
}
```

---

## ErrClosed

Operação tentada depois de `DB.Close`. Toda chamada de execução, consulta, `Begin`, builders e `RunMigrations` passa pela guarda. `Close` em si é idempotente.

```go
_ = db.Close()

_, err := db.ExecContext(ctx, "SELECT 1")
fmt.Println(errors.Is(err, krios.ErrClosed)) // true
```

---

## ErrTxDone

Operação tentada em transação terminal — depois de `Commit`/`Rollback` ou de uma falha nativa. `sql.ErrTxDone` nativo é normalizado para este erro preservando a causa. Depois de qualquer tentativa nativa de finalização, o wrapper é terminal mesmo em falha.

```go
tx, err := db.Begin(ctx)
if err != nil {
	return err
}

if err := tx.Commit(ctx); err != nil {
	return err
}

_, err = tx.ExecContext(ctx, "SELECT 1")
fmt.Println(errors.Is(err, krios.ErrTxDone)) // true
```

---

## ErrInvalidEntity

A struct não pôde ser mapeada em entidade: não é struct, sem campos mapeados, campo embutido, tag ou campo inválido, `TableName` com assinatura errada ou nome vazio, coluna duplicada (inclusive ignorando maiúsculas). Também cobre destinos inválidos de `Validate`, `ValidateAll` e `Format` e rejeição de `driver.Valuer` em `WriteValues`.

```go
type SemCampos struct {
	oculto int // sem tag: ignorado → nenhuma coluna mapeada
}

if _, err := krios.EntityOf[SemCampos](); err != nil {
	fmt.Println(errors.Is(err, krios.ErrInvalidEntity)) // true
}
```

---

## ErrValidation

Um campo rejeitou seu próprio valor pelo `krios.Validator`. A mensagem contextualiza tabela, coluna e embrulha a causa.

```go
user := &User{Status: "bogus"}

if err := ent.Validate(user); err != nil {
	fmt.Println(errors.Is(err, krios.ErrValidation)) // true
	// krios: field validation failed: users: column "status": unsupported status "bogus"
}
```

Os escritores (`SetValues`) reportam este mesmo erro por `Build`/`Exec` antes de chegar ao driver.

---

## ErrFormat

Um campo falhou ao produzir seu texto de apresentação pelo `krios.Formatter`. Contextualiza tabela e coluna e embrulha a causa. Só ocorre por `Entity.Format`.

```go
text, err := ent.Format(user)
if errors.Is(err, krios.ErrFormat) {
	// Formatter devolveu erro; text é nil
}
```

---

## ErrInvalidQuery

Construção inválida detectada **antes** da execução: coluna desconhecida, operando nil, grupo lógico vazio, seleção/alias/entidade duplicados, `Only`/`Omit` com nome inválido, `Update`/`Delete` sem `Where`, `Offset` sem `Limit`, limit/offset negativo, `RETURNING` em MySQL, `Exec` com `Returning` (exige `One`/`All`) ou builders sem `SetValues`.

```go
_, _, err := db.Select(ent).
	Where(ent.Col("inexistente").Eq(1)).
	Build()
fmt.Println(errors.Is(err, krios.ErrInvalidQuery)) // true

_, _, err = db.Delete(ent).Build() // sem Where
fmt.Println(errors.Is(err, krios.ErrInvalidQuery)) // true
```

---

## ErrFileNotFound

Recurso SQL não pôde ser carregado: FS nil, caminho inválido ou ausente, diretório no lugar de arquivo ou arquivo em branco. Embrulha `fs.ErrNotExist` quando o caminho não existe. Ocorre em `NewSQLFile`, antes de qualquer acesso ao banco.

```go
_, err := krios.NewSQLFile(schemaFS, "schema/inexistente.sql")
if errors.Is(err, krios.ErrFileNotFound) {
	// embrulha fs.ErrNotExist
}

_, err = krios.NewSQLFile(nil, "schema/users.sql")
fmt.Println(errors.Is(err, krios.ErrFileNotFound)) // true (FS nil)
```

---

## ErrPartialMigration

A execução das migrações está bloqueada: registro `in_progress`/`failed` no histórico, ou objetos gerenciados presentes em um banco sem histórico. Nunca é contornada automaticamente — o consumidor deve resolver o estado explicitamente. Use `errors.As` com `*PartialMigrationError` para o contexto estruturado (direção, alvo, última versão, arquivo, status).

```go
_, err := db.RunMigrations(ctx)
if errors.Is(err, krios.ErrPartialMigration) {
	var pme *krios.PartialMigrationError
	if errors.As(err, &pme) {
		fmt.Println(pme.Status, pme.LastVersion, pme.LastFile)
	}
}
```

---

Fim do manual. Exemplos executáveis desta API estão em [`examples/`](examples/) e nos `Example_*` de [`tests/examples_test.go`](tests/examples_test.go); uma narrativa completa de uso está em [`.specs/0001_use_examples.md`](.specs/0001_use_examples.md).
