# Exemplos de uso — KriosTasks

Este documento mostra como fica o código de um projeto consumidor do Krios,
com tudo o que já foi implementado nas entregas **E01–E08**:

| Entrega | Escopo                       | O que os exemplos cobrem                                                                                       |
| ------- | ---------------------------- | -------------------------------------------------------------------------------------------------------------- |
| E02     | Configuração e ciclo de vida | `krios.Config` dos 3 bancos, `Open`, `Close`, erros de abertura, timeout                                       |
| E03     | Execução e transações        | SQL puro com parâmetros, `Result`, `Begin`/`Commit`/`Rollback`, commit bloqueado, `sql.ErrNoRows`, `ErrTxDone` |
| E04     | Entidades e mapeamento       | tags `krios`, `EntityOf`, `Row.Scan`, `Rows.All`, NULL, `Validator`/`Formatter`                                |
| E05     | Consultas encadeadas         | `Select`/`Only`/`Where`/`Join`, agregações, `Count`, `Paginate`, leituras mapeadas                             |
| E06     | Escrita encadeada            | `Insert`/`Update`/`Delete`, `SetValues`, `Where` obrigatório, `Returning`                                      |
| E07     | SQL embutido                 | `NewSQLFile` com `go:embed`, `db.File`/`tx.File`, parâmetros do dialeto                                        |
| E08     | Migrações versionadas        | `MigrationsConfig`, `RunMigrations`, tabela `migrations`, avanço/reversão com histórico                        |

As entregas E01–E08 estão implementadas. As consultas de leitura e as
escritas usam os builders de E05/E06 e o esquema é SQL embutido de E07.

## Convenção de extração

Cada bloco de código deste documento é classificado:

- **Extraído** — o código existe no pacote `examples/` deste módulo, com o
  mesmo comportamento. A citação indica o arquivo de origem.
- **Ilustrativo** — o código **não** está em `examples/`; existe para mostrar
  uma variante (SQL direto, outra forma de API). Não é garantia de execução.

Nenhum bloco é declarado "compilado" só por ser sintaticamente válido: o que
está marcado como extraído é conferido contra o código, e o que está marcado
como ilustrativo é identificado como tal.

```
examples/
├── doc.go             // visão geral do pacote
├── config.go          // Config dos 3 bancos, OpenStore, classificação de erros
├── models.go          // Status (Validator/Formatter), User, Task
├── store.go           // Store: esquema, escritas, transações, leituras encadeadas
├── store_test.go      // verificação SQLite das leituras de E05
├── migrations.go      // MigrationsConfig e a narrativa Migrate (E08)
├── migrations_test.go // avanço → reversão → reaplicação contra SQLite
├── run.go             // Run: narrativa completa passo a passo
├── schema/            // DDL embutido de users e tasks (E07)
│   ├── users.sql
│   └── tasks.sql
└── migrations/        // arquivos up/down versionados (E08)
    ├── 001_projects.up.sql
    ├── 001_projects.down.sql
    ├── 002_project_members.up.sql
    └── 002_project_members.down.sql
```

## 1. O projeto exemplo

**KriosTasks** é um gerenciador de tarefas mínimo: usuários com estado de
conta e tarefas pendentes. O banco tem duas tabelas:

```
users                          tasks
─────                          ─────
id INTEGER PK                  id INTEGER PK
name TEXT                      title TEXT
email TEXT UNIQUE              owner_id INTEGER  → users.id
bio TEXT (nullable)            done BOOLEAN
status TEXT
```

O modelo Go espelha as tabelas via reflection (E04) — sem DSL e sem
registro manual de entidades:

```go
// Status is the account lifecycle state stored in users.status. It implements
// the field-level krios.Validator and krios.Formatter interfaces detected by
// the entity descriptor.
type Status string

// Supported account states.
const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// Validate implements krios.Validator and rejects unknown states.
func (s Status) Validate() error {
	switch s {
	case StatusActive, StatusInactive:
		return nil
	default:
		return fmt.Errorf("unsupported status %q", s)
	}
}

// Format implements krios.Formatter and returns the presentation text used by
// Entity.Format. Database operations always use the original value.
func (s Status) Format() (string, error) {
	return strings.ToUpper(string(s)), nil
}

// User maps the users table. Tags name the columns; Bio is a pointer so a
// missing value maps to SQL NULL.
type User struct {
	ID     int64   `krios:"id,pk"`
	Name   string  `krios:"name"`
	Email  string  `krios:"email"`
	Bio    *string `krios:"bio"`
	Status Status  `krios:"status"`
}

// TableName overrides the snake_case derivation of the type name.
func (User) TableName() string { return "users" }

// Task maps the tasks table. OwnerID and Done carry no tag: their columns are
// derived from the field names as snake_case (owner_id, done).
type Task struct {
	ID      int64  `krios:"id,pk"`
	Title   string `krios:"title"`
	OwnerID int64
	Done    bool
}

// TableName overrides the snake_case derivation of the type name.
func (Task) TableName() string { return "tasks" }
```

> **Extraído de** [`examples/models.go`](../examples/models.go).

Regras em uso (E04-P01/P02):

- `krios:"coluna"` nomeia a coluna; `krios:"coluna,pk"` marca a chave primária
  (compostas são permitidas); `krios:"-"` ignora o campo.
- Sem tag, a coluna vem do nome do campo em snake_case (`OwnerID` →
  `owner_id`, `Done` → `done`).
- A tabela vem de `TableName() string`; sem o método, usa-se o snake_case do
  tipo (`TaskUser` → `task_user`).
- NULL em coluna anulável: ponteiro (`*string`) ou `sql.Null*`.
- Campos sem exportação não podem ter tag; structs embutidas são rejeitadas.

## 2. Dependências e drivers

O Krios **não importa drivers**; o consumidor escolhe um driver compatível com
`database/sql`, importa-o em branco e informa o nome do registro em
`Config.Driver`. Se o nome não estiver registrado, `Open` falha com
`krios.ErrDriverNotLoaded` — ele nunca infere compatibilidade pelo nome.

```go
import (
	"github.com/mineot/krios"
	_ "modernc.org/sqlite" // registrado sob o nome "sqlite"
)
```

| Banco      | Import em branco                                                        | `Config.Driver` |
| ---------- | ----------------------------------------------------------------------- | --------------- |
| SQLite3    | `_ "modernc.org/sqlite"` (puro Go; já é dependência de teste do módulo) | `"sqlite"`      |
| MySQL      | `_ "github.com/go-sql-driver/mysql"`                                    | `"mysql"`       |
| PostgreSQL | `_ "github.com/jackc/pgx/v5/stdlib"`                                    | `"pgx"`         |

> **Ilustrativo** — o bloco acima mostra o formato do import; em `examples/`
> o driver entra em `config.go`, e o exemplo roda com `modernc.org/sqlite`.

A configuração é **código Go** (`krios.Config`): nenhum arquivo de configuração
externo, nenhuma variável de ambiente obrigatória — tudo compila no
executável.

## 3. Configuração e conexão (E02)

```go
// SQLiteConfig returns the configuration used by Run: one local SQLite file
// with a five second budget for opening the access.
func SQLiteConfig(dbPath string) krios.Config {
	return krios.Config{
		Type:    krios.DBTypeSQLite,
		Driver:  "sqlite",
		SQLite:  &krios.SQLiteConfig{DBPath: dbPath},
		Timeout: 5 * time.Second,
	}
}

// MySQLConfig returns the equivalent configuration for MySQL. The consumer
// registers the driver with _ "github.com/go-sql-driver/mysql"; Port zero
// applies the MySQL default of 3306.
func MySQLConfig(host, user, password, dbName string) krios.Config {
	return krios.Config{
		Type:   krios.DBTypeMySQL,
		Driver: "mysql",
		MySQL: &krios.MySQLConfig{
			Host:     host,
			User:     user,
			Password: password,
			DBName:   dbName,
		},
		Timeout: 5 * time.Second,
	}
}

// PostgresConfig returns the equivalent configuration for PostgreSQL. The
// consumer registers the driver with _ "github.com/jackc/pgx/v5/stdlib";
// Port zero applies the PostgreSQL default of 5432.
func PostgresConfig(host, user, password, dbName string) krios.Config {
	return krios.Config{
		Type:   krios.DBTypePostgres,
		Driver: "pgx",
		Postgres: &krios.PostgresConfig{
			Host:     host,
			User:     user,
			Password: password,
			DBName:   dbName,
		},
		Timeout: 5 * time.Second,
	}
}
```

> **Extraído de** [`examples/config.go`](../examples/config.go).

Pontos do contrato (E02-P01) visíveis acima: só o bloco do tipo escolhido pode
ser preenchido (exclusão mútua validada); `Port == 0` aplica 3306/5432;
`Timeout == 0` aplica 10 s e negativo é inválido; a validação é uma lista
fechada de checagens e todo erro casa com `krios.ErrInvalidConfig` via
`errors.Is`.

A abertura valida a configuração, confere o driver, monta o DSN e faz ping
dentro do menor prazo entre o deadline do `context` e o `Timeout`; recursos
criados numa tentativa falha são liberados:

```go
func OpenStore(ctx context.Context, cfg krios.Config) (*Store, error) {
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// OpenErrorKind classifies a failed open for the caller's message. Consumers
// branch with errors.Is on the krios sentinel errors instead of parsing text.
func OpenErrorKind(err error) string {
	switch {
	case errors.Is(err, krios.ErrInvalidConfig):
		return "configuração inválida"
	case errors.Is(err, krios.ErrDriverNotLoaded):
		return "driver não importado"
	case errors.Is(err, krios.ErrConnectionFailed):
		return "falha de conexão"
	default:
		return "erro inesperado"
	}
}
```

> **Extraído de** [`examples/config.go`](../examples/config.go).

Cancelamento e deadline do `ctx` não viram `ErrConnectionFailed`: o erro
original de `context` é preservado. `Close` é idempotente e, depois dele,
qualquer operação no `*DB` retorna `krios.ErrClosed`:

```go
// Store owns the database access for KriosTasks. A Store is safe for
// concurrent use and typically serves the whole application.
type Store struct {
	db *krios.DB
}

// Close releases the database access. It is idempotent.
func (s *Store) Close() error { return s.db.Close() }
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

## 4. Esquema com SQL puro (E03; embutido em E07)

O Krios executa o SQL sem retraduzir entre dialetos — o que está no arquivo é o
que vai ao banco. A partir de E07 o DDL deixa de ser strings em Go e passa a
arquivos `.sql` embutidos com `go:embed`, um statement por arquivo:

```go
// schemaFS embeds the DDL files into the executable, so the consumer does not
// ship or read them at run time. Each file holds a single statement, since
// Krios does not split multiple statements.
//
//go:embed schema/*.sql
var schemaFS embed.FS

// Schema resources loaded once at startup. Eager loading surfaces a missing or
// unreadable resource immediately, not on first use.
var (
	usersSchema = mustSQLFile(schemaFS, "schema/users.sql")
	tasksSchema = mustSQLFile(schemaFS, "schema/tasks.sql")
)
```

> **Extraído de** [`examples/store.go`](../examples/store.go) — o DDL vive em
> [`examples/schema/users.sql`](../examples/schema/users.sql) e
> [`examples/schema/tasks.sql`](../examples/schema/tasks.sql).

```go
// CreateSchema creates both tables from the embedded DDL files. This example
// keeps owning its DDL directly instead of using versioned migrations
// (DB.RunMigrations); Krios executes the file text without translation between
// dialects.
//
// The migration example in migrations.go owns a separate schema (projects and
// project_members) in the same database file, so the two owners never share a
// table. A real application would pick one owner for its schema.
func (s *Store) CreateSchema(ctx context.Context) error {
	for _, schema := range []*krios.SQLFile{usersSchema, tasksSchema} {
		if _, err := s.db.File(schema).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

> **Dialeto:** os DDLs acima são SQLite. MySQL pede `ENGINE=InnoDB` e tipos
> diferentes; PostgreSQL também. É SQL do consumidor, portanto escreva-o por
> dialeto. O mesmo vale para as migrações de E08 (§10): cada arquivo up/down é
> responsabilidade sua.

**Dois donos de esquema.** O exemplo de migrações (§10) administra um segundo
conjunto de tabelas (`projects`, `project_members`) no mesmo arquivo SQLite.
Os dois conjuntos são disjuntos, e cada um tem um dono: as migrações nunca
tocam em `users`/`tasks`, e o DDL direto nunca toca em `projects`/
`project_members`. Um projeto real escolhe um dono para o seu esquema; o
exemplo usa os dois para exercitar E07 e E08 sem misturar responsabilidades.

## 5. Escrita, `Omit` e `Result` (E03/E06)

As escritas do projeto exemplo usam o builder de escrita de E06, ancorado na
entidade:

```go
// CreateUser validates user, inserts it with the entity write builder and
// stores the generated id.
//
// The validation is not repeated here: SetValues runs the field validators of
// the entity and fails fast with krios.ErrValidation before the statement
// reaches the driver.
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	res, err := s.db.Insert(userEntity).SetValues(user).Omit("id").Exec(ctx)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		user.ID = id
	}
	return nil
}

// CreateTask inserts one task owned by task.OwnerID with the entity write
// builder. As in CreateUser, the field validators run inside SetValues.
func (s *Store) CreateTask(ctx context.Context, task *Task) error {
	res, err := s.db.Insert(taskEntity).SetValues(task).Omit("id").Exec(ctx)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		task.ID = id
	}
	return nil
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

### 5.1 `Omit` e a política de valores

`Omit` remove a coluna do `INSERT` (o banco aplica o default) ou do `SET` do
`UPDATE` (a coluna mantém o valor anterior). Em `CreateUser` o `id` é omitido
para o SQLite atribuir o `rowid`; em `TransferTasks` (§6.2) o `Omit("id",
"title", "done")` restringe o `UPDATE` a `owner_id`.

`SetValues` aplica a política de valores da entidade:

| Valor do campo | INSERT | UPDATE |
| --- | --- | --- |
| Zero de tipo primitivo (`0`, `false`, `""`) | Grava o zero | Grava o zero |
| Ponteiro `nil` | Grava `NULL` | Omite a coluna, preservando o valor atual |
| Ponteiro não nulo | Grava o valor apontado, inclusive zero | Grava o valor apontado, inclusive zero |
| `sql.Null*` com `Valid: false` | Grava `NULL` | Grava `NULL` |
| Coluna em `Omit` | Usa o default do banco | Preserva o valor atual |

Para gravar `false`, atribua esse valor ao campo e use `SetValues`; para
preservar uma coluna primitiva no UPDATE, inclua-a em `Omit`. Não existe
método `Set` nos builders de escrita.

A validação dos campos roda **dentro de `SetValues`**, antes do statement
chegar ao driver: um valor inválido falha com `krios.ErrValidation` e nada é
gravado. Ver §8.

### 5.2 `Result` e a disponibilidade das informações

`Result` expõe `RowsAffected` e `LastInsertId` com **flag de
disponibilidade** — o driver pode não suportar a informação, e isso não é
erro:

```go
res, err := s.db.Insert(userEntity).SetValues(user).Omit("id").Exec(ctx)
if err != nil {
	return err
}
if id, ok := res.LastInsertId(); ok {
	user.ID = id
}
```

`ok == false` significa "o driver não informou o valor" — não "valor zero". O
mesmo vale para `RowsAffected`.

### 5.3 Variante em SQL direto

O mesmo registro pode ser gravado sem o builder, com SQL do consumidor:

```go
res, err := s.db.ExecContext(ctx,
	"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
	user.Name, user.Email, user.Bio, user.Status)
if err != nil {
	return err
}
if id, ok := res.LastInsertId(); ok {
	user.ID = id
}
```

> **Ilustrativo** — variante equivalente à de §5, sem o builder de E06. O
> argumento `user.Bio` (`*string`) converte sozinho: ponteiro não nulo vira o
> valor, `nil` vira SQL NULL. Os placeholders seguem o dialeto do driver: `?`
> em SQLite e MySQL, `$1..$n` em PostgreSQL — o Krios **não** traduz
> marcadores no SQL do consumidor.

## 6. Transações (E03)

### 6.1 Grupo atômico com rollback automático de limpeza

`Begin(ctx)` — o `ctx` governa a vida da transação. O padrão é
`defer tx.Rollback(ctx)` logo após o `Begin`: cobre todos os returns
antecipados. **Depois de um `Commit` bem-sucedido esse `Rollback` devolve
`krios.ErrTxDone`** — não é um no-op silencioso; o `defer` apenas descarta o
erro. O exemplo usa esse padrão:

```go
func (s *Store) CreateUserWithFirstTasks(ctx context.Context, user *User, titles ...string) error {
	// The record is validated before the transaction starts, so an invalid user
	// never opens one. SetValues validates again, since the write builder
	// always runs the field validators.
	if err := userEntity.Validate(user); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	// Covers every early return. After a successful Commit this deferred
	// Rollback returns krios.ErrTxDone, which the deferred call discards; a
	// consumer that needs to observe it must check it with errors.Is.
	defer tx.Rollback(ctx)

	res, err := tx.Insert(userEntity).SetValues(user).Omit("id").Exec(ctx)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		user.ID = id
	}
	for _, title := range titles {
		if title == "" {
			return errors.New("task title must not be empty")
		}
		task := &Task{Title: title, OwnerID: user.ID, Done: false}
		// SetValues validates the task; a rejection aborts the group.
		if _, err := tx.Insert(taskEntity).SetValues(task).Omit("id").Exec(ctx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

Usuário e tarefas persistem juntos ou não persistem nada. Operações fora do
grupo são independentes: um `INSERT` feito direto no `*DB` durante a transação
não faz parte dela.

Para quem quiser o `Rollback` explícito, o mesmo grupo fica assim:

```go
tx, err := s.db.Begin(ctx)
if err != nil {
	return err
}
defer func() {
	if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, krios.ErrTxDone) {
		// Registre rbErr como falha de limpeza.
	}
}()
```

> **Ilustrativo** — variante com o erro de limpeza observado. `ErrTxDone`
> significa "a transação já foi finalizada" e é o resultado esperado depois de
> um `Commit` bem-sucedido.

### 6.2 Commit com retorno de linhas afetadas

```go
// TransferTasks reassigns every open task of one owner to another inside a
// transaction using the UPDATE builder and reports how many rows moved.
func (s *Store) TransferTasks(ctx context.Context, from, to int64) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var target User
	err = tx.Select(userEntity).Where(userEntity.Col("id").Eq(to)).One(ctx, &target)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("target user %d does not exist", to)
	}
	if err != nil {
		return 0, err
	}

	res, err := tx.Update(taskEntity).
		SetValues(&Task{OwnerID: to}).
		Omit("id", "title", "done").
		Where(taskEntity.Col("owner_id").Eq(from), taskEntity.Col("done").Eq(false)).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	moved, _ := res.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return moved, nil
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

O `Omit("id", "title", "done")` limita o `UPDATE` à coluna `owner_id` — as
outras mantêm o valor anterior, e `owner_id` zero seria omitido pela política
de valores (§5.1). `RowsAffected` devolve o número de linhas movidas, com a
mesma flag de disponibilidade de `Result`.

Note o teste de existência: `sql.ErrNoRows` é **preservado** (não é embrulhado
com camadas do Krios), então o idiomático `errors.Is(err, sql.ErrNoRows)`
continua funcionando — inclusive dentro de transação.

### 6.3 Commit bloqueado com auto-rollback (rede de proteção)

A transação memoriza o **primeiro erro** — falha imediata de `ExecContext` ou
`QueryContext` **e** falha tardia de leitura (`Scan`, iteração, `Close`). Com
erro memorizado, `Commit` faz rollback sozinho e devolve esse mesmo erro
(`errors.Is` casando com o erro original). Não há escape para limpar o erro.

```go
func (s *Store) RegisterAndCloneEmail(ctx context.Context, first, clone *User) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		first.Name, first.Email, first.Bio, first.Status); err != nil {
		return err
	}

	_, statementErr := tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		clone.Name, clone.Email, clone.Bio, clone.Status)
	if statementErr == nil {
		return errors.New("expected a duplicate email error")
	}

	// Explicit rollback remains the recommended response; this Commit shows the
	// extra protection when the consumer attempts it anyway.
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return errors.New("commit was not blocked by the statement error")
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go). As instruções
> usam marcadores `?` (SQLite/MySQL); PostgreSQL usaria `$1..$n`. O Krios
> nunca traduz marcadores no SQL do consumidor.

O `INSERT` do `first` também é descartado: ele pertence ao mesmo grupo. A
recomendação continua sendo rollback explícito no primeiro erro — o commit
bloqueado é a rede de proteção para o consumidor esquecer o caminho de falha.

### 6.4 Erro tardio de leitura e rollback com contexto cancelado

Um `Scan` que falha por conversão **envenena** a transação (o erro é
memorizado); `Commit` seria bloqueado. `Rollback` ainda funciona — mesmo com
o contexto já cancelado, a limpeza é tentada. Verificável com
`go test -run Example_rollbackAfterReadError -v ./tests`:

```go
tx, _ := db.Begin(context.Background())
defer tx.Rollback(ctx)

var number int
err := tx.QueryRowContext(ctx, "SELECT 'not a number'").Scan(&number)
// err != nil: erro tardio memorizado pela transação.

cancel() // ctx cancelado
_ = tx.Rollback(ctx)            // nil: a limpeza é tentada mesmo assim
// tx.Commit(ctx) também devolveria o erro de conversão, com rollback automático.
```

> **Ilustrativo** — o exemplo executável correspondente está em
> [`tests/examples_test.go`](../tests/examples_test.go).

### 6.5 Estado terminal

Depois do `Commit` ou do `Rollback` — inclusive do automático — a transação é
terminal: qualquer operação posterior devolve `krios.ErrTxDone`
(`errors.Is` funcional). Isso vale para o `Rollback` deferred de §6.1, que é
por isso ignorado. Chamadas concorrentes no mesmo `*Tx` são serializadas
por um mutex; falha nativa de finalização também é terminal (sem retry
automático — o estado remoto pode ser incerto).

## 7. Entidades e mapeamento (E04)

O descritor é derivado por reflection e **cacheado por tipo**; os métodos
servem para inspeção (nomes de colunas, chaves, ordem de declaração):

```go
var (
	userEntity = mustEntity[User]()
	taskEntity = mustEntity[Task]()
)

func mustEntity[T any]() *krios.Entity {
	ent, err := krios.EntityOf[T]()
	if err != nil {
		// An invalid model is a programming error caught at development time.
		panic(err)
	}
	return ent
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

### 7.1 Multi-linha: `Rows.All` casa colunas por **nome**

A leitura usa o builder de E05: `Select(ent)` cria a consulta, `OrderBy`
ordena e `All` mapeia o resultado por nome.

```go
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	err := s.db.Select(userEntity).
		OrderBy(userEntity.Col("id").Asc()).
		All(ctx, &users)
	return users, err
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

`All` exige `*[]T` com `T` struct: casa colunas com campos por nome (exato,
depois case-insensitive), descarta colunas extras, deixa zero os campos sem
coluna, **sempre fecha** o iterador e só substitui o slice em caso de sucesso.
Aliases e subconjuntos de colunas funcionam por esse motivo.

### 7.2 Linha única: `One` — mapeamento **posicional**

```go
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.Select(userEntity).
		Where(userEntity.Col("id").Eq(id)).
		One(ctx, &user)
	return user, err
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

`One` executa e faz `Scan` de um único registro esperado. Com destino `*T`
struct (que não implementa `sql.Scanner`), o resultset deve seguir a ordem de
declaração dos campos — por isso a seleção padrão do builder usa todas as
colunas mapeadas nessa ordem. `All`, ao contrário, casa **por nome**. Tipos que
implementam `sql.Scanner` (como `sql.Null*`) e `time.Time` mantêm a semântica
posicional e nunca são tratados como entidades. `sql.ErrNoRows` é preservado;
falhas de conversão são contextualizadas preservando a causa:

```
krios: map tasks: sql: Scan error on column index 2, name "owner_id": converting driver.Value type string ("abc") to a int64: invalid syntax
```

> Mensagem ilustrativa de conversão ao ler `owner_id` com texto inválido. O prefixo `krios: map <tabela>` vem do mapeamento; o resto
> é o erro do `database/sql`.

### 7.3 NULL

`*string` em `Bio` distingue valor ausente de vazio; `sql.NullString` é a
alternativa para quem prefere o par `(Valid, String)`:

```go
	// E04: Bio receives a value here and stays nil for alice (SQL NULL).
	bio := "leitora assídua"
	carol := &User{Name: "carol", Email: "carol@exemplo.dev", Bio: &bio, Status: StatusInactive}
	if err := store.CreateUser(ctx, carol); err != nil {
		return fmt.Errorf("carol: %w", err)
	}
```

> **Extraído de** [`examples/run.go`](../examples/run.go). O `Bio` de alice fica
> `nil` → `NULL` no banco; ao ler, volta `nil`. NULL em campo não anulável ou
> conversão impossível vira erro com tabela e coluna (§7.2).

## 8. Validação e formatação (E04/E06)

As interfaces ficam no **tipo do campo** (receptor de valor ou ponteiro) e são
detectadas automaticamente pelo descritor:

```go
// ValidateUsers reports whether a batch of users satisfies every field-level
// Validator, stopping at the first invalid record with krios.ErrValidation.
func (s *Store) ValidateUsers(users []User) error {
	return userEntity.ValidateAll(users)
}

// PresentUser returns the display text of every mapped column of user. The
// formatted values are presentation-only; writes always use the original
// fields.
func (s *Store) PresentUser(user User) (map[string]string, error) {
	return userEntity.Format(user)
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go). `ValidateAll`
> aceita `[]User` ou `*[]User` e para no primeiro inválido; `Format` devolve o
> texto de **todas** as colunas.

Uso no fluxo:

```go
	err = store.ValidateUsers([]User{{Name: "dave", Email: "dave@exemplo.dev", Status: "unknown"}})
	fmt.Println("validação rejeitada:", errors.Is(err, krios.ErrValidation))
	fmt.Println("  causa:", err)
```

> **Extraído de** [`examples/run.go`](../examples/run.go). O erro devolvido é
> `krios: field validation failed: users: column "status": unsupported status
> "unknown"` e casa com `krios.ErrValidation`.

```go
		text, err := store.PresentUser(u)
		if err != nil {
			return err
		}
		fmt.Printf("  usuário %s: %s <%s> status=%s bio=%q\n",
			text["id"], text["name"], text["email"], text["status"], text["bio"])
```

> **Extraído de** [`examples/run.go`](../examples/run.go). `text["status"]` vale
> `"ACTIVE"` (`Status.Format`; o valor original no banco continua `"active"`),
> `text["bio"]` vale `""` para ponteiro `nil` e `text["id"]` vale `"1"` —
> campos sem `Formatter` usam `fmt.Sprint`.

**Onde a validação roda.** `ValidateAll` é a chamada explícita para validar um
lote. Mas a validação **não é exclusiva dela**: o builder de escrita de E06
roda os validadores dentro de `SetValues`, antes do statement chegar ao
driver, e falha com `krios.ErrValidation` sem gravar nada. Por isso
`CreateUser` (§5) não repete a chamada — ela é redundante — e
`CreateUserWithFirstTasks` (§6.1) mantém uma validação *antes* do `Begin`
para que um registro inválido nem abra transação.

Contratos (E04-P03): fail-fast; campos nil são pulados na validação e formatam
vazio; o valor formatado é **só apresentação** — operações de banco sempre
usam o campo original; falha de `Format` casa com `krios.ErrFormat`; destino
errado em `Validate`/`Format` casa com `krios.ErrInvalidEntity`.

## 9. Consultas encadeadas (E05)

`DB.Select(ent)` e `Tx.Select(ent)` criam o construtor; a construção é pura e a
execução usa o mesmo contrato e as mesmas proteções de E03. O projeto exemplo
migrou as leituras de E04 para o builder e adicionou três consultas que
exercitam junções, agregações e paginação.

### 9.1 Contagem com filtros

```go
func (s *Store) CountOpenTasks(ctx context.Context, ownerID int64) (int64, error) {
	return s.db.Select(taskEntity).
		Where(
			taskEntity.Col("owner_id").Eq(ownerID),
			taskEntity.Col("done").Eq(false),
		).
		Count(ctx)
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

`Count` conta por subconsulta (`SELECT COUNT(*) FROM (...) AS "krios_total"`),
sem ordenação nem limite, então continua exato quando a consulta tem `DISTINCT`
ou agrupamento.

### 9.2 Junção `INNER` + filtros

```go
func (s *Store) ListOpenTasksByOwnerEmail(ctx context.Context, email string) ([]Task, error) {
	var tasks []Task
	err := s.db.Select(taskEntity).
		Join(userEntity, krios.InnerJoin, userEntity.Col("id").Eq(taskEntity.Col("owner_id"))).
		Where(
			userEntity.Col("email").Eq(email),
			taskEntity.Col("done").Eq(false),
		).
		OrderBy(taskEntity.Col("id").Asc()).
		All(ctx, &tasks)
	return tasks, err
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

A entidade base é `tasks`; `users` participa só na junção e no filtro. Toda
entidade citada precisa participar da consulta e a tabela não pode se repetir.

### 9.3 Agregação com `LEFT JOIN` e modelo de leitura

```go
// UserTaskCount is a read model mapped by output alias. It demonstrates a
// grouped projection built with Fields.
type UserTaskCount struct {
	ID    int64  `krios:"id"`
	Name  string `krios:"name"`
	Tasks int64  `krios:"tasks"`
}

// ListUserTaskCounts returns every user with their task count. The LEFT JOIN
// keeps users without tasks, whose count is zero.
func (s *Store) ListUserTaskCounts(ctx context.Context) ([]UserTaskCount, error) {
	var counts []UserTaskCount
	err := s.db.Select(userEntity).
		Join(taskEntity, krios.LeftJoin, taskEntity.Col("owner_id").Eq(userEntity.Col("id"))).
		Fields(
			userEntity.Col("id"),
			userEntity.Col("name"),
			taskEntity.Col("id").Count().As("tasks"),
		).
		GroupBy(userEntity.Col("id"), userEntity.Col("name")).
		OrderBy(userEntity.Col("id").Asc()).
		All(ctx, &counts)
	return counts, err
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

`Fields` usa expressões; o alias `"tasks"` casa com o campo pelo nome. O
`LEFT JOIN` mantém usuários sem tarefas, cuja contagem é `0`.

### 9.4 Paginação com estatísticas

```go
func (s *Store) ListUsersPage(ctx context.Context, limit, offset int) ([]User, krios.Paginate, error) {
	var users []User
	page, err := s.db.Select(userEntity).
		OrderBy(userEntity.Col("id").Asc()).
		Paginate(ctx, limit, offset, &users)
	if err != nil {
		return nil, krios.Paginate{}, err
	}
	return users, page, nil
}
```

> **Extraído de** [`examples/store.go`](../examples/store.go).

`Paginate` devolve `krios.Paginate{Total, Limit, Offset, Page, Pages, HasNext,
HasPrev}`. `limit` deve ser maior que zero e `offset` não pode ser negativo
(`ErrInvalidQuery`). Na `*DB` a contagem e a página são consultas
independentes; dentro de uma `*Tx` compartilham a transação. `Paginate` não
muta o construtor.

As leituras são verificadas por `examples/store_test.go` (SQLite temporário),
cobrindo filtros, contagem, junção, agrupamento e paginação. As decisões da API
estão em [E05-P01](E05-P01_decisions.md) e [E05-P03](E05-P03_decisions.md).

## 10. Migrações versionadas (E08)

O exemplo de migrações administra um **segundo esquema** no mesmo arquivo
SQLite: `projects` e `project_members` (§4). Os arquivos up/down são embutidos
com `go:embed` e declarados versão a versão:

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationVersions declares the schema versions in execution order. The
// version number and the order come from this declaration; the numeric prefix
// of the file name is only a convention. Every version declares both
// directions, so the same configuration can advance and revert the schema.
func MigrationVersions() []krios.MigrationVersion {
	return []krios.MigrationVersion{
		{
			Version: 1,
			Up:      []string{"migrations/001_projects.up.sql"},
			Down:    []string{"migrations/001_projects.down.sql"},
		},
		{
			Version: 2,
			Up:      []string{"migrations/002_project_members.up.sql"},
			Down:    []string{"migrations/002_project_members.down.sql"},
		},
	}
}

// MigrationsConfig returns the migration configuration for the given target
// version. The target is always explicit: 2 advances the schema to the second
// version and 0 reverts every applied version.
//
// The managed objects are the tables Krios owns in this database. They let
// the library refuse to adopt a database that already has the schema without
// a migration history, instead of silently assuming it was created by Krios.
func MigrationsConfig(target int) *krios.MigrationsConfig {
	return &krios.MigrationsConfig{
		FS:             migrationsFS,
		TargetVersion:  &target,
		Versions:       MigrationVersions(),
		ManagedObjects: []string{"projects", "project_members"},
	}
}
```

> **Extraído de** [`examples/migrations.go`](../examples/migrations.go).

A configuração entra no `krios.Config`:

```go
cfg := SQLiteConfig(dbPath)
cfg.Migrations = MigrationsConfig(2)

db, err := krios.Open(ctx, cfg)
if err != nil {
	return err
}
defer db.Close()

plan, err := db.RunMigrations(ctx)
```

> **Ilustrativo** — o corpo de `migratePhase` em
> [`examples/migrations.go`](../examples/migrations.go) faz exatamente isto.

Pontos do contrato (E08-P01):

- `TargetVersion` é a versão desejada e é **sempre explícito** (ponteiro): `nil`
  é inválido (`ErrInvalidConfig`) e o ponteiro para `0` **reverte** todas as
  versões aplicadas.
- `Versions` é a lista ordenada; as versões precisam ser crescentes, únicas e
  maiores que zero, e os nomes de arquivo de uma direção não podem se repetir
  nem ser vazios. A direção `down` só é exigida quando o consumidor pretende
  reverter — a preparação falha com `ErrInvalidQuery` se faltar o arquivo
  necessário.
- `ManagedObjects` é **obrigatório** quando `Migrations != nil` (lista vazia é
  `ErrInvalidConfig`): sem ela a biblioteca não distingue "banco vazio" de
  "esquema preexistente".
- Sem a tabela `migrations` e sem nenhum objeto gerido presente → versão atual
  `0` e percursos normais. Sem a tabela `migrations` mas com algum objeto
  gerido presente → erro **antes** de executar (`ErrInvalidQuery`): adotar
  esquema preexistente sem histórico está fora do escopo.
- Um registro `in_progress`/`failed` no histórico bloqueia a execução com
  `krios.ErrPartialMigration` e contexto de recuperação.
- A tabela `migrations` é criada pela própria biblioteca, com
  `version, filename, direction, status, checksum, applied_at, error` e
  unicidade em `(version, filename, direction)`.

O escopo transacional é o máximo viável por dialeto (E08-P01 §11): por versão
em SQLite/PostgreSQL (falha no meio reverte a versão inteira), por arquivo em
MySQL transacional e sem transação quando o arquivo contém DDL não
transacional. No MySQL, uma versão também pode ficar parcial quando um
arquivo transacional termina e um arquivo seguinte falha. O histórico
incompleto bloqueia a retomada automática. Em SQLite/PostgreSQL, versões já
confirmadas permanecem se uma versão posterior falhar; não há rollback do
percurso inteiro.

A preparação consulta o banco. O plano retornado descreve os passos
preparados, inclusive quando a aplicação falha; a presença de um passo no
plano não comprova sua execução. `DB.Migrations()` retorna a configuração
retida e mutável; alterá-la exige sincronização pelo consumidor. Este exemplo
reabre o banco com um alvo explícito a cada fase.

### 10.1 Avanço, reversão e reaplicação

`Migrate` percorre as três fases, cada uma abrindo o acesso com o alvo
próprio — o alvo faz parte da configuração, e a configuração é retida pelo
`Open`:

```go
func Migrate(ctx context.Context, dbPath string) error {
	advance, err := migratePhase(ctx, dbPath, 2)
	if err != nil {
		return fmt.Errorf("avanço: %w", err)
	}

	fmt.Printf("avanço: %s\n", describePlan(advance.plan))
	fmt.Printf("  objetos: %s\n", describeTables(advance.tables))
	fmt.Printf("  histórico: %d registro(s)\n", len(advance.history))

	revert, err := migratePhase(ctx, dbPath, 0)
	if err != nil {
		return fmt.Errorf("reversão: %w", err)
	}

	fmt.Printf("reversão: %s\n", describePlan(revert.plan))
	fmt.Printf("  objetos: %s\n", describeTables(revert.tables))
	fmt.Printf("  histórico: %d registro(s)\n", len(revert.history))

	reapply, err := migratePhase(ctx, dbPath, 2)
	if err != nil {
		return fmt.Errorf("reaplicação: %w", err)
	}

	fmt.Printf("reaplicação: %s\n", describePlan(reapply.plan))
	fmt.Printf("  objetos: %s\n", describeTables(reapply.tables))
	fmt.Printf("  histórico: %d registro(s)\n", len(reapply.history))

	fmt.Println("execução de migrações concluída")
	return nil
}
```

> **Extraído de** [`examples/migrations.go`](../examples/migrations.go).

Cada fase observa o resultado real — o plano aplicado, a existência das
tabelas geridas e as linhas do histórico — em vez de confiar no retorno do
método:

```go
func tableExists(ctx context.Context, db *krios.DB, name string) (bool, error) {
	var count int

	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}
```

> **Extraído de** [`examples/migrations.go`](../examples/migrations.go). A
> consulta é do SQLite; em outro dialeto a introspecção do catálogo é
> diferente — a biblioteca só usa `sqlite_master` para a verificação pontual
> dos objetos declarados em `ManagedObjects`.

Saída real (arquivo SQLite novo):

```
avanço: 0->2 (2 passo(s))
  objetos: projects=true project_members=true
  histórico: 2 registro(s)
reversão: 2->0 (2 passo(s))
  objetos: projects=false project_members=false
  histórico: 4 registro(s)
reaplicação: 0->2 (2 passo(s))
  objetos: projects=true project_members=true
  histórico: 4 registro(s)
execução de migrações concluída
```

O histórico cresce de 2 para 4 registros na reversão (uma linha `down` por
arquivo) e permanece em 4 na reaplicação: a linha `up` do próprio arquivo é
substituída, não duplicada. As três fases são verificadas por
[`examples/migrations_test.go`](../examples/migrations_test.go), que confere o
plano, o esquema real e o histórico em cada fase.

## 11. Erros sentinela — resumo

Todos casam com `errors.Is` e preservam a causa original:

| Erro                                    | Quando aparece no projeto exemplo                                                             |
| --------------------------------------- | --------------------------------------------------------------------------------------------- |
| `krios.ErrInvalidConfig`                | `Config` inconsistente em `OpenStore` (tipo sem bloco, porta fora da faixa, timeout negativo) |
| `krios.ErrDriverNotLoaded`              | driver não importado em branco ou nome divergente de `Config.Driver`                          |
| `krios.ErrConnectionFailed`             | banco inacessível ao ping (causa do driver preservada)                                        |
| `krios.ErrClosed`                       | operação após `store.Close()`                                                                 |
| `krios.ErrTxDone`                       | operação em `*Tx` já finalizada — inclusive o `Rollback` deferred depois do `Commit`          |
| `krios.ErrInvalidEntity`                | struct inválida em `EntityOf` ou destino errado em `All`/`Validate`/`Format`                  |
| `krios.ErrValidation`                   | campo que rejeita o valor, em `ValidateUsers` **ou** em `SetValues`                            |
| `krios.ErrFormat`                       | `Format()` de um campo devolvendo erro                                                        |
| `krios.ErrInvalidQuery`                 | `Paginate` com `limit <= 0` ou `offset < 0`; esquema preexistente sem histórico (§10)          |
| `krios.ErrFileNotFound`                 | `NewSQLFile` com arquivo em branco ou ausente no `fs.FS`                                      |
| `krios.ErrPartialMigration`             | histórico em `in_progress`/`failed` bloqueia a execução das migrações                          |
| `sql.ErrNoRows`                         | `UserByID`/`UserByEmail` sem registro — preservado, não embrulhado                            |
| `context.Canceled` / `DeadlineExceeded` | prazo estourado em `Open`/operações — preservados                                             |

## 12. Fluxo completo — `Run`

`Run` costura tudo: abre o banco, cria o esquema, demonstra validação,
escrita em grupo com rollback, escrita simples, leituras mapeadas com o builder,
paginação, junções, contagem agrupada, transferência em transação e commit
bloqueado — imprimindo cada passo (`examples/run.go`):

```go
func Run(ctx context.Context, dbPath string) error {
	store, err := OpenStore(ctx, SQLiteConfig(dbPath))
	if err != nil {
		return fmt.Errorf("abertura (%s): %w", OpenErrorKind(err), err)
	}
	defer store.Close()

	if err := store.CreateSchema(ctx); err != nil {
		return fmt.Errorf("esquema: %w", err)
	}
	fmt.Println("esquema criado (users, tasks)")

	// E04: validation is explicit and stops at the first invalid record.
	err = store.ValidateUsers([]User{{Name: "dave", Email: "dave@exemplo.dev", Status: "unknown"}})
	fmt.Println("validação rejeitada:", errors.Is(err, krios.ErrValidation))
	fmt.Println("  causa:", err)

	// E03: one transaction creates the user and its first tasks together.
	alice := &User{Name: "alice", Email: "alice@exemplo.dev", Status: StatusActive}
	if err := store.CreateUserWithFirstTasks(ctx, alice, "escrever a spec", "revisar o PR"); err != nil {
		return fmt.Errorf("grupo de alice: %w", err)
	}
	fmt.Printf("alice criada (id %d) com duas tarefas\n", alice.ID)

	// E03: a failure inside the group rolls the whole group back.
	bob := &User{Name: "bob", Email: "bob@exemplo.dev", Status: StatusActive}
	err = store.CreateUserWithFirstTasks(ctx, bob, "tarefa válida", "")
	fmt.Println("grupo de bob revertido:", err != nil)
	if _, err := store.UserByEmail(ctx, bob.Email); !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("bob não deveria ter sido persistido: %v", err)
	}
	fmt.Println("  bob ausente após a falha")

	// E04: Bio receives a value here and stays nil for alice (SQL NULL).
	bio := "leitora assídua"
	carol := &User{Name: "carol", Email: "carol@exemplo.dev", Bio: &bio, Status: StatusInactive}
	if err := store.CreateUser(ctx, carol); err != nil {
		return fmt.Errorf("carol: %w", err)
	}
	fmt.Printf("carol criada (id %d) com bio\n", carol.ID)

	// E04: Rows.All maps by column name; Status.Format supplies the display text.
	users, err := store.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		text, err := store.PresentUser(u)
		if err != nil {
			return err
		}
		fmt.Printf("  usuário %s: %s <%s> status=%s bio=%q\n",
			text["id"], text["name"], text["email"], text["status"], text["bio"])
	}

	// E05: Paginate executes the count and one page, returning navigation data.
	pageUsers, pageInfo, err := store.ListUsersPage(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("paginação: %w", err)
	}
	fmt.Printf("página %d/%d de %d usuários:", pageInfo.Page, pageInfo.Pages, pageInfo.Total)
	for _, u := range pageUsers {
		fmt.Printf(" %s", u.Name)
	}
	fmt.Println()

	// E04: single-row mapping; sql.ErrNoRows stays available for checks.
	one, err := store.UserByID(ctx, alice.ID)
	if err != nil {
		return err
	}
	fmt.Println("UserByID:", one.Name, one.Status)
	if _, err := store.UserByID(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("esperava sql.ErrNoRows para o id 999: %v", err)
	}
	fmt.Println("  id 999 ausente: true")

	// E03: the transfer commits as one unit and reports the affected rows.
	moved, err := store.TransferTasks(ctx, alice.ID, carol.ID)
	if err != nil {
		return fmt.Errorf("transferência: %w", err)
	}
	remaining, err := store.CountOpenTasks(ctx, alice.ID)
	if err != nil {
		return err
	}
	fmt.Println("transferência: movidas", moved, "tarefas; alice ainda tem", remaining)

	// E05: an INNER JOIN finds open tasks by owner email; the LEFT JOIN count
	// keeps users without tasks, so alice appears with zero after the transfer.
	carolOpen, err := store.ListOpenTasksByOwnerEmail(ctx, carol.Email)
	if err != nil {
		return fmt.Errorf("join de tarefas: %w", err)
	}
	fmt.Println("tarefas abertas de carol via join:", len(carolOpen))

	counts, err := store.ListUserTaskCounts(ctx)
	if err != nil {
		return fmt.Errorf("contagem agrupada: %w", err)
	}
	for _, row := range counts {
		fmt.Printf("  %s: %d tarefa(s)\n", row.Name, row.Tasks)
	}

	// E03: the failed duplicate insert poisons the commit; the first insert of
	// the same group is discarded together with it.
	ghost := &User{Name: "ghost", Email: "ghost@exemplo.dev", Status: StatusActive}
	dupe := &User{Name: "ghost clone", Email: ghost.Email, Status: StatusActive}
	err = store.RegisterAndCloneEmail(ctx, ghost, dupe)
	fmt.Println("commit bloqueado:", err != nil)
	if _, err := store.UserByEmail(ctx, ghost.Email); !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("ghost não deveria ter sido persistido: %v", err)
	}
	fmt.Println("  ghost ausente após o commit bloqueado")

	fmt.Println("execução concluída")
	return nil
}
```

> **Extraído de** [`examples/run.go`](../examples/run.go).

Saída real (arquivo SQLite novo):

```
esquema criado (users, tasks)
validação rejeitada: true
  causa: krios: field validation failed: users: column "status": unsupported status "unknown"
alice criada (id 1) com duas tarefas
grupo de bob revertido: true
  bob ausente após a falha
carol criada (id 2) com bio
  usuário 1: alice <alice@exemplo.dev> status=ACTIVE bio=""
  usuário 2: carol <carol@exemplo.dev> status=INACTIVE bio="leitora assídua"
página 1/2 de 2 usuários: alice
UserByID: alice active
  id 999 ausente: true
transferência: movidas 2 tarefas; alice ainda tem 0
tarefas abertas de carol via join: 2
  alice: 0 tarefa(s)
  carol: 2 tarefa(s)
commit bloqueado: true
  ghost ausente após o commit bloqueado
execução concluída
```

Observe o contraste didático: `PresentUser` mostra `status=ACTIVE`
(formatação), `UserByID` imprime `active` (valor original), e `bio=""` de
alice versus `bio="leitora assídua"` de carol evidencia o NULL via ponteiro.

## 13. Fluxo de migrações — `Migrate`

`Migrate` (§10.1) é a narrativa de E08: abre o acesso com o alvo 2, avança o
esquema, reabre com o alvo 0 e reverte, reabre com o alvo 2 e reaplica —
imprimindo o plano, o estado dos objetos geridos e o tamanho do histórico em
cada fase. A saída real está em §10.1.

## 14. Como executar e verificar

O pacote `examples` é biblioteca: o consumidor chama `Run` e `Migrate` do
próprio `main` (substituindo o caminho do arquivo):

```go
func main() {
	if err := examples.Run(context.Background(), "kriostasks.db"); err != nil {
		log.Fatal(err)
	}
	if err := examples.Migrate(context.Background(), "kriostasks.db"); err != nil {
		log.Fatal(err)
	}
}
```

> **Ilustrativo** — corpo do `main` do consumidor, com imports de
> `context`, `log` e `github.com/mineot/krios/examples`. Use um arquivo SQLite
> novo para reproduzir as saídas; `Run` insere dados a cada execução.

Com o repositório na raiz do módulo:

```sh
make check                              # fmt + build + vet + test (inclui ./examples)
go test -run '^Example' -v ./tests     # exemplos executáveis das garantias de E03/E04
go test -run 'TestMigrat' -v ./examples # avanço, reversão e reaplicação de migrações
```

## 15. Recursos disponíveis (E01–E08)

- **E02 — usado pelo projeto.** `Config` dos três bancos, `Open`/`Close`,
  classificação de erros de abertura.
- **E03 — usado pelo projeto.** SQL puro com parâmetros, `Result` com flag de
  disponibilidade, transações explícitas com commit bloqueado.
- **E04 — usado pelo projeto.** Entidades por reflection com tags `krios`,
  validação e formatação por campo, mapeamento posicional (`One`) e por nome
  (`All`).
- **E05 — usado pelo projeto.** As leituras usam o builder encadeado com
  filtros, joins, agregações e paginação (`krios.Paginate` com total).
- **E06 — usado pelo projeto.** `CreateUser`/`CreateTask`/`TransferTasks` usam
  o builder de escrita, com `Where` obrigatório em UPDATE/DELETE, política de
  valores e `Omit`, e integração automática com o motor de validação.
- **E07 — usado pelo projeto.** `CreateSchema` deixou de ser strings em Go e
  passou a arquivos `.sql` embutidos via `go:embed` (um statement por arquivo
  em `examples/schema/`), carregados com `krios.NewSQLFile` e executados por
  `db.File(f).Exec`. Ver `examples/store.go` e `examples/schema/`.
- **E08 — usado pelo projeto.** Migrações versionadas up/down com histórico na
  tabela `migrations`, configuradas por `krios.MigrationsConfig` e executadas
  por `DB.RunMigrations`, com o exemplo de avanço, reversão e reaplicação em
  `examples/migrations.go`. Ver `E08-P01_decisions.md`.
