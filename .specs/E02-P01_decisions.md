# E02-P01 - Decisions

Vamos criar uma pasta chamada core e tudo abaixo vai ficar dentro dele, se possivel
Aceito sugestões da IA caso essa arquitetura esteja errada

## Mensagens de erro

Criar no arquivos `errs.go`

```go
package krios

import "errors"

var (
	// Configuração inválida (campos obrigatórios ausentes, valores inconsistentes, timeout negativo)
	ErrInvalidConfig = errors.New("krios: invalid configuration")

	// Driver não carregado/registro ausente detectado ao abrir
	ErrDriverNotLoaded = errors.New("krios: database driver not loaded for the configured type")

	// Falha genérica de conexão (pode envolver rede/arquivo/permissão)
	ErrConnectionFailed = errors.New("krios: failed to connect to database")
)
```

## Configuração

Criar no arquivo config.go

```go
package krios

import "time"

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypeMySQL    DBType = "mysql"
	DBTypePostgres DBType = "postgres"
)

type SQLiteConfig struct {
	// Caminho absoluto ou relativo do arquivo .db
	// Ex.: "dev.db", "/data/app.db"
	DBPath string
}

type MySQLConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Port     int // Se 0, usar porta padrão 3306
}

type PostgresConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Port     int // Se 0, usar porta padrão 5432
	// Espaço opcional para SSLMode no futuro, não necessário para escopo inicial
}

type MigrationsConfig struct {
	// Reservado: versão desejada e associação de arquivos incorporados.
	// Definição completa fica para E08-P01.
}

type Config struct {
	// Tipo do banco. Obrigatório.
	Type DBType

	// Configuração específica para SQLite3. Obrigatória quando Type == DBTypeSQLite.
	SQLite *SQLiteConfig

	// Configuração específica para MySQL. Obrigatória quando Type == DBTypeMySQL.
	MySQL *MySQLConfig

	// Configuração específica para PostgreSQL. Obrigatória quando Type == DBTypePostgres.
	Postgres *PostgresConfig

	// Tempo máximo para aguardar a abertura da conexão.
	// Se zero, aplicar valor padrão (ver item 3).
	// Se negativo, deve ser tratado como configuração inválida.
	Timeout time.Duration

	// Configuração opcional de migrações (reservada para E08).
	// Pode ser nil quando não houver intenção de executar migrações.
	Migrations *MigrationsConfig
}
```

## Database

Criar no arquivo database.go

```go
package krios

import "context"

type DB struct {
	// Detalhes internos não exportados nesta etapa
}

// Open inicializa o acesso ao banco, valida a configuração,
// verifica conectividade e respeita cancelamento/prazo limite.
func Open(ctx context.Context, cfg Config) (*DB, error)

// Close encerra o acesso e libera os recursos associados.
// Deve ser idempotente: chamadas subsequentes retornam nil (ou erro sem efeito).
func (db *DB) Close() error

```

## Dialect

Criar no arquivo dialect.go

```go
package krios

// internal/dialect/dialect.go (conceitual)
type Dialect interface {
	Name() string // "sqlite"|"mysql"|"postgres"
	// Outros métodos entram em E05-P01: placeholder, quote, etc.
}

func FromType(t krios.DBType) (Dialect, error) { ... }
```

## Exemplos

### Exemplo 1 SQLite3

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite" // driver importado pelo consumidor (o postgresql e mysql devem vir assim tbem)
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	defer cancel()

	cfg := krios.Config{
		Type: krios.DBTypeSQLite,
		SQLite: &krios.SQLiteConfig{
			DBPath: "dev.db",
		},
		Timeout: 5 * time.Second,
		// Migrations: nil (não configurado)
	}

	db, err := krios.Open(ctx, cfg)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()
}
```

### Exemplo 2 MySql

```go
import (
	_ "github.com/go-sql-driver/mysql"
)

cfg := krios.Config{
	Type: krios.DBTypeMySQL,
	MySQL: &krios.MySQLConfig{
		Host:     "127.0.0.1",
		User:     "app",
		Password: "secret",
		DBName:   "app_db",
		// Port: 0 -> 3306
	},
	Timeout: 5 * time.Second,
}
```

### Exemplo 3 PostgreSQL

```go
import (
	_ "github.com/jackc/pgx/v5/stdlib"
)

cfg := krios.Config{
	Type: krios.DBTypePostgres,
	Postgres: &krios.PostgresConfig{
		Host:     "127.0.0.1",
		User:     "app",
		Password: "secret",
		DBName:   "app_db",
		Port:     5432,
	},
	Timeout: 5 * time.Second,
}
```

### Exemplo 4 Demonstração de idempotência e tratamento

```go
db, err := krios.Open(context.Background(), cfg)

if err != nil {
	// erros.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, krios.ErrInvalidConfig), etc.
	return
}

// Close pode ser chamado múltiplas vezes com segurança
_ = db.Close()
_ = db.Close() // idempotente
```
