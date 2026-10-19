# Exemplos de uso — KriosTasks

Este documento mostra como fica o código de um projeto consumidor do Krios,
com tudo o que já foi implementado nas entregas **E01–E04**:

| Entrega | Escopo                       | O que os exemplos cobrem                                                                                       |
| ------- | ---------------------------- | -------------------------------------------------------------------------------------------------------------- |
| E02     | Configuração e ciclo de vida | `krios.Config` dos 3 bancos, `Open`, `Close`, erros de abertura, timeout                                       |
| E03     | Execução e transações        | SQL puro com parâmetros, `Result`, `Begin`/`Commit`/`Rollback`, commit bloqueado, `sql.ErrNoRows`, `ErrTxDone` |
| E04     | Entidades e mapeamento       | tags `krios`, `EntityOf`, `Row.Scan`, `Rows.All`, NULL, `Validator`/`Formatter`                                |

As entregas **E05–E08** (builders de consulta e escrita, SQL embutido e
migrações) ainda não existem; por isso o projeto exemplo é dono do seu DDL e
escreve SQL à mão. Quando forem implementadas, os pontos de extensão aparecem
marcados ao longo do texto.

O código completo e compilável vive no pacote **`examples/`** deste módulo —
os trechos abaixo são extraídos dele:

```
examples/
├── doc.go      // visão geral do pacote
├── config.go   // Config dos 3 bancos, OpenStore, classificação de erros
├── models.go   // Status (Validator/Formatter), User, Task
├── store.go    // Store: esquema, escritas, transações, leituras mapeadas
└── run.go      // Run: narrativa completa passo a passo
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

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

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

// User maps the users table. Tags name the columns; Bio is a pointer so a
// missing value maps to SQL NULL.
type User struct {
	ID     int64  `krios:"id,pk"`
	Name   string `krios:"name"`
	Email  string `krios:"email"`
	Bio    *string `krios:"bio"`
	Status Status `krios:"status"`
}

func (User) TableName() string { return "users" }

// Task maps the tasks table. OwnerID and Done carry no tag: their columns are
// derived from the field names as snake_case (owner_id, done).
type Task struct {
	ID      int64 `krios:"id,pk"`
	Title   string `krios:"title"`
	OwnerID int64
	Done    bool
}

func (Task) TableName() string { return "tasks" }
```

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

A configuração é **código Go** (`krios.Config`): nenhum arquivo de configuração
externo, nenhuma variável de ambiente obrigatória — tudo compila no
executável.

## 3. Configuração e conexão (E02)

```go
func SQLiteConfig(dbPath string) krios.Config {
	return krios.Config{
		Type:    krios.DBTypeSQLite,
		Driver:  "sqlite",
		SQLite:  &krios.SQLiteConfig{DBPath: dbPath},
		Timeout: 5 * time.Second,
	}
}

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

Cancelamento e deadline do `ctx` não viram `ErrConnectionFailed`: o erro
original de `context` é preservado. `Close` é idempotente e, depois dele,
qualquer operação no `*DB` retorna `krios.ErrClosed`:

```go
type Store struct {
	db *krios.DB
}

func (s *Store) Close() error { return s.db.Close() }
```

## 4. Esquema com SQL puro (E03)

Sem migrações (E08), o consumidor é dono do DDL; o Krios executa o SQL sem
retraduzir entre dialetos — o que está no arquivo/na string é o que vai ao
banco:

```go
func (s *Store) CreateSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			bio TEXT,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			owner_id INTEGER NOT NULL,
			done BOOLEAN NOT NULL DEFAULT FALSE
		)`,
	}
	for _, ddl := range statements {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	return nil
}
```

> **Dialeto:** os DDLs acima são SQLite. MySQL pede `ENGINE=InnoDB` e tipos
> diferentes; PostgreSQL também. É SQL do consumidor, portanto escreva-o por
> dialeto (ou espere E08, em que cada migração também será responsabilidade
> sua).

## 5. Escrita básica e `Result` (E03)

Placeholders seguem o dialeto do driver: `?` em SQLite e MySQL, `$1..$n` em
PostgreSQL. `Result` expõe `RowsAffected` e `LastInsertId` com **flag de
disponibilidade** — o driver pode não suportar a informação, e isso não é
erro:

```go
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	if err := userEntity.Validate(user); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		user.Name, user.Email, user.Bio, user.Status)
	if err != nil {
		return err
	}
	if id, ok := res.LastInsertId(); ok {
		user.ID = id
	}
	return nil
}
```

O argumento `user.Bio` (`*string`) converte sozinho: ponteiro não nulo vira o
valor, `nil` vira SQL NULL.

## 6. Transações (E03)

### 6.1 Grupo atômico com rollback automático de limpeza

`Begin(ctx)` — o `ctx` governa a vida da transação. O padrão é
`defer tx.Rollback(ctx)` logo após o `Begin`: cobre todos os returns
antecipados e vira no-op após um `Commit` bem-sucedido:

```go
func (s *Store) CreateUserWithFirstTasks(ctx context.Context, user *User, titles ...string) error {
	if err := userEntity.Validate(user); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	// Covers every early return; Rollback after a successful Commit is a no-op.
	defer tx.Rollback(ctx)

	res, err := tx.ExecContext(ctx,
		"INSERT INTO users (name, email, bio, status) VALUES (?, ?, ?, ?)",
		user.Name, user.Email, user.Bio, user.Status)
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
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO tasks (title, owner_id, done) VALUES (?, ?, ?)",
			title, user.ID, false); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
```

Usuário e tarefas persistem juntos ou não persistem nada. Operações fora do
grupo são independentes: um `INSERT` feito direto no `*DB` durante a transação
não faz parte dela.

### 6.2 Commit com retorno de linhas afetadas

```go
func (s *Store) TransferTasks(ctx context.Context, from, to int64) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", to).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("target user %d does not exist", to)
	}
	if err != nil {
		return 0, err
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE tasks SET owner_id = ? WHERE owner_id = ? AND done = ?",
		to, from, false)
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

### 6.5 Estado terminal

Depois do `Commit` ou do `Rollback` — inclusive do automático — a transação é
terminal: qualquer operação posterior devolve `krios.ErrTxDone`
(`errors.Is` funcional). Chamadas concorrentes no mesmo `*Tx` são serializadas
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

### 7.1 Multi-linha: `Rows.All` casa colunas por **nome**

```go
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, email, bio, status FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	var users []User
	if err := rows.All(&users); err != nil {
		return nil, err
	}
	return users, nil
}
```

`All` exige `*[]T` com `T` struct: casa colunas com campos por nome (exato,
depois case-insensitive), descarta colunas extras, deixa zero os campos sem
coluna, **sempre fecha** o iterador e só substitui o slice em caso de sucesso.
Aliases e subconjuntos de colunas funcionam por esse motivo.

### 7.2 Linha única: `Row.Scan` é **posicional**

```go
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, email, bio, status FROM users WHERE id = ?", id,
	).Scan(&user)
	return user, err
}
```

Com destino `*T` struct (que não implementa `sql.Scanner`), o resultset deve
seguir a ordem de declaração dos campos. Tipos que implementam `sql.Scanner`
(`time.Time`, `sql.Null*`) mantêm a semântica posicional e nunca são tratados
como entidades. Falhas de conversão são contextualizadas preservando a causa:

```
krios: map users: sql: Scan error on column index 4, name "status": ...
```

### 7.3 NULL

`*string` em `Bio` distingue valor ausente de vazio; `sql.NullString` é a
alternativa para quem prefere o par `(Valid, String)`:

```go
bio := "leitora assídua"
carol := &User{Name: "carol", Email: "carol@exemplo.dev", Bio: &bio, Status: StatusInactive}
_ = s.CreateUser(ctx, carol) // bio presente

// alice.Bio fica nil → NULL no banco; ao ler, Bio volta nil.
// NULL em campo não anulável ou conversão impossível: erro com tabela e coluna.
```

## 8. Validação e formatação (E04)

As interfaces ficam no **tipo do campo** (receptor de valor ou ponteiro) e são
detectadas automaticamente pelo descritor. A execução é **explícita** — o Krios
não intercepta escritas:

```go
func (s *Store) ValidateUsers(users []User) error {
	return userEntity.ValidateAll(users) // []User ou *[]User; para no primeiro inválido
}

func (s *Store) PresentUser(user User) (map[string]string, error) {
	return userEntity.Format(user) // texto por coluna, para TODAS as colunas
}
```

Uso no fluxo:

```go
err = s.ValidateUsers([]User{{Name: "dave", Email: "dave@x.dev", Status: "unknown"}})
errors.Is(err, krios.ErrValidation) // true
// err: krios: field validation failed: users: column "status": unsupported status "unknown"

text, _ := s.PresentUser(user)
text["status"] // "ACTIVE"  (Status.Format; o valor original no banco continua "active")
text["bio"]    // ""        (ponteiro nil formata texto vazio)
text["id"]     // "1"       (campos sem Formatter usam fmt.Sprint)
```

Contratos (E04-P03): fail-fast; campos nil são pulados na validação e formatam
vazio; o valor formatado é **só apresentação** — operações de banco sempre
usam o campo original; falha de `Format` casa com `krios.ErrFormat`; destino
errado em `Validate`/`Format` casa com `krios.ErrInvalidEntity`.

## 9. Erros sentinela — resumo

Todos casam com `errors.Is` e preservam a causa original:

| Erro                                    | Quando aparece no projeto exemplo                                                             |
| --------------------------------------- | --------------------------------------------------------------------------------------------- |
| `krios.ErrInvalidConfig`                | `Config` inconsistente em `OpenStore` (tipo sem bloco, porta fora da faixa, timeout negativo) |
| `krios.ErrDriverNotLoaded`              | driver não importado em branco ou nome divergente de `Config.Driver`                          |
| `krios.ErrConnectionFailed`             | banco inacessível ao ping (causa do driver preservada)                                        |
| `krios.ErrClosed`                       | operação após `store.Close()`                                                                 |
| `krios.ErrTxDone`                       | operação em `*Tx` já finalizada                                                               |
| `krios.ErrInvalidEntity`                | struct inválida em `EntityOf` ou destino errado em `All`/`Validate`/`Format`                  |
| `krios.ErrValidation`                   | `ValidateUsers` com campo que rejeita o valor                                                 |
| `krios.ErrFormat`                       | `Format()` de um campo devolvendo erro                                                        |
| `sql.ErrNoRows`                         | `UserByID`/`UserByEmail` sem registro — preservado, não embrulhado                            |
| `context.Canceled` / `DeadlineExceeded` | prazo estourado em `Open`/operações — preservados                                             |

## 10. Fluxo completo — `Run`

`Run` costura tudo: abre o banco, cria o esquema, demonstra validação,
escrita em grupo com rollback, escrita simples, leituras mapeadas,
transferência em transação e commit bloqueado — imprimindo cada passo
(`examples/run.go`):

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
UserByID: alice active
  id 999 ausente: true
transferência: movidas 2 tarefas; alice ainda tem 0
commit bloqueado: true
  ghost ausente após o commit bloqueado
execução concluída
```

Observe o contraste didático: `PresentUser` mostra `status=ACTIVE`
(formatação), `UserByID` imprime `active` (valor original), e `bio=""` de
alice versus `bio="leitora assídua"` de carol evidencia o NULL via ponteiro.

## 11. Como executar e verificar

O pacote `examples` é biblioteca: o consumidor chama `Run` do próprio
`main` (substituindo o caminho do arquivo):

```go
func main() {
	if err := examples.Run(context.Background(), "kriostasks.db"); err != nil {
		log.Fatal(err)
	}
}
```

Com o repositório na raiz do módulo:

```sh
make check                       # fmt + build + vet + test (inclui ./examples)
go test -run '^Example' -v ./tests   # exemplos executáveis das garantias de E03/E04
```

## 12. Próximos passos (E05–E08)

Quando as entregas seguintes chegarem, os pontos de extensão do projeto
exemplo serão:

- **E05** — `ListUsers`/`CountOpenTasks` ganham builder encadeado com filtros,
  joins e paginação (`krios.Paginate` com total).
- **E06** — `CreateUser`/`CreateTask` passam ao builder de escrita, com
  `Where` obrigatório em UPDATE/DELETE e integração automática com o motor de
  validação já usado em `ValidateUsers`.
- **E07** — `CreateSchema` deixa de ser strings em Go e vira arquivos `.sql`
  embutidos via `go:embed`.
- **E08** — `CreateSchema` dá lugar a migrações versionadas up/down com
  histórico na tabela `migrations` (design em `E08-P01_decisions.md`).
