# E02-P01 — Decisões

Data: 2026-10-18

## Organização do código

Seguir a organização definida em [E01-P01_decisions.md](E01-P01_decisions.md): pacote público `krios` na raiz do módulo (`github.com/mineot/krios`). Não criar pasta `core` nem antecipar subpacotes; subpacotes internos serão criados somente quando uma separação concreta da implementação justificar, conforme já registrado em E01-P01.

**Revisão (2026-10-18):** o código desta fase foi consolidado em um **único arquivo `krios.go`** (documentação do pacote, configuração e erros), em vez de `errs.go` e `config.go` separados. A mudança é intencional e está registrada em [E01-P01_decisions.md](E01-P01_decisions.md#revisão-2026-10-18); o caminho de import permanece `github.com/mineot/krios`. Em E02-P03, `Open` e `Close` também serão adicionados a `krios.go`.

A definição da interface de dialetos e sua seleção **não** fazem parte desta entrega: permanecem adiadas para E05-P01, conforme [0002_execution_plan.md](0002_execution_plan.md).

Os testes do pacote ficam concentrados em `krios_test.go` (`package krios`), conforme a revisão registrada em [E01-P01_decisions.md](E01-P01_decisions.md#revisão-2026-10-18--testes); o teste de ambiente SQLite permanece isolado em `tests/`.

## Tabela de decisões

| Item | Decisão | Justificativa |
|-----|---------|---------------|
| Organização | Pacote `krios` na raiz; código desta fase em `krios.go` (revisão de 2026-10-18); sem pasta `core` | Consistente com E01-P01 revisado e com os exemplos de importação |
| Identificação do driver | Campo obrigatório `Config.Driver string` com o nome sob o qual o driver se registrou em `database/sql` | O RF-01/§4.1 exige informar a seleção do driver na configuração; nenhum mapeamento `DBType`→nome é universal (`sqlite`/`sqlite3`, `postgres`/`pgx`) |
| Detecção de driver ausente | Em `Open`, comparar `Config.Driver` com `sql.Drivers()` | Permite distinguir "driver não importado" de falha de conexão |
| Relação driver × banco | Não validar nome contra `DBType`; o dialeto segue `Type` e incompatibilidade se manifesta como falha de conexão | Evita regras frágeis contra a diversidade de drivers |
| DSN | O Krios monta o DSN a partir da configuração do banco, em formato documentado por banco | O consumidor preenche campos de conexão (RF-01); drivers com sintaxe divergente não são suportados |
| `Timeout` default | `0` aplica `10 * time.Second`; negativo é `ErrInvalidConfig` | Valor zero distingue "omitido" de "explícito" com campo simples; não há como desativar (usar valor muito grande) |
| Precedência ctx × Timeout | `Open` aplica `context.WithTimeout(ctx, cfg.Timeout)`; prazo efetivo é o menor entre os dois | Cancelamento e prazo são do consumidor ou do Krios, e ambos resultam em erro de contexto verificável |
| Contrato de erros | Todo erro de `Open` satisfaz `errors.Is` contra o sentinela da condição, preservando o erro original com `%w` | Exigido pelo RF-02 (cancelamento e prazo comunicados como erro) |
| Validações | Lista fechada abaixo, sempre com `ErrInvalidConfig` | Determinística e testável em E02-P02 |
| `Close` idempotente | Chamadas repetidas retornam `nil` | Resolve a pendência de chamadas repetidas de E02-P03 |

## Mensagens de erro

Criar no arquivo `errs.go`:

```go
package krios

import "errors"

var (
	// Configuração inválida: campos obrigatórios ausentes, valores
	// inconsistentes ou Timeout negativo. O texto detalha o campo.
	ErrInvalidConfig = errors.New("krios: invalid configuration")

	// Driver não registrado em database/sql no momento de Open:
	// o consumidor não importou o driver ou registrou outro nome.
	ErrDriverNotLoaded = errors.New("krios: database driver not loaded for the configured type")

	// Falha de conexão (rede, arquivo, permissão, recusa do banco).
	// Encadeia o erro original do driver.
	ErrConnectionFailed = errors.New("krios: failed to connect to database")
)
```

## Contrato de erros de `Open`

| Condição | Retorno de `Open` | Verificável com `errors.Is` |
| -------- | ----------------- | --------------------------- |
| Configuração inválida (lista abaixo) | `ErrInvalidConfig` com detalhe no texto | `krios.ErrInvalidConfig` |
| Driver vazio | `ErrInvalidConfig` com detalhe no texto | `krios.ErrInvalidConfig` |
| Driver não registrado em `sql.Drivers()` | `ErrDriverNotLoaded` com o nome do driver no texto | `krios.ErrDriverNotLoaded` |
| `ctx` cancelado pelo consumidor | Erro de contexto embrulhado, **sem** encadear `ErrConnectionFailed` | `context.Canceled` |
| Prazo esgotado (menor entre `ctx` e `Timeout`) | Erro de contexto embrulhado, **sem** encadear `ErrConnectionFailed` | `context.DeadlineExceeded` |
| Falha de conexão, ping ou montagem do DSN | `ErrConnectionFailed` encadeando o erro original com `%w` | `krios.ErrConnectionFailed` e o erro do driver |

Regras:

- Cancelamento e prazo esgotado **não** são tratados como falha de conexão; assim, o consumidor pode distinguir "desistiu de esperar" de "banco indisponível".
- O erro original nunca é descartado: sempre há `%w` para inspeção.
- `ErrInvalidConfig` e `ErrDriverNotLoaded` são detectados antes de abrir recursos; `ErrConnectionFailed` só é retornado depois que a tentativa começou, e `Open` libera qualquer recurso criado (RF-02).

## Validações (`ErrInvalidConfig`)

Lista fechada, executada na ordem:

1. `Type` vazio ou diferente de `DBTypeSQLite`, `DBTypeMySQL` e `DBTypePostgres`.
2. `Driver` vazio.
3. Struct do tipo selecionado ausente: `SQLite == nil` com `Type == DBTypeSQLite`, `MySQL == nil` com `Type == DBTypeMySQL` ou `Postgres == nil` com `Type == DBTypePostgres`.
4. Struct de **outro** banco preenchida junto com o tipo selecionado (valor inconsistente), inclusive as três preenchidas ao mesmo tempo.
5. SQLite: `DBPath` vazio.
6. MySQL/PostgreSQL: `Host`, `User`, `Password` ou `DBName` vazios. A senha é obrigatória conforme a tabela do RF-01; montagens locais com senha vazia não são suportadas no escopo inicial.
7. `Port < 0` (`0` aplica a porta padrão: 3306 no MySQL, 5432 no PostgreSQL).
8. `Timeout < 0`.

`Migrations` não recebe validação nesta entrega; `nil` é válido. A validação do conteúdo fica para E08-P01.

`Timeout == 0` **não** é erro: aplica o default de 10 segundos.

## Identificação do driver e DSN

O consumidor importa o driver (import em branco) e informa em `Config.Driver` o nome sob o qual ele se registrou em `database/sql`. Em `Open`, o Krios verifica a presença desse nome em `sql.Drivers()`; ausente, retorna `ErrDriverNotLoaded`. Essa é a distinção entre "não importado/registrado" e "registrado mas não conecta" (`ErrConnectionFailed`).

Nomes usados nos exemplos (conforme documentação dos respectivos projetos):

| Banco | Exemplo de import | `Config.Driver` |
| ----- | ----------------- | --------------- |
| SQLite3 | `_ "modernc.org/sqlite"` | `"sqlite"` |
| MySQL | `_ "github.com/go-sql-driver/mysql"` | `"mysql"` |
| PostgreSQL | `_ "github.com/jackc/pgx/v5/stdlib"` | `"pgx"` |

O nome correto é o que o driver registrado exporta; outros drivers do mesmo banco podem usar outro nome. Cabe ao consumidor conferir a documentação do driver escolhido.

### DSN gerado pelo Krios

O Krios monta a string de conexão a partir da configuração do banco, em formato fixo por banco:

| Banco | Formato | Observações |
| ----- | ------- | ----------- |
| SQLite3 | o próprio `DBPath` | Caminho do arquivo, relativo ou absoluto |
| MySQL | `user:password@tcp(host:port)/dbname` | Formato `go-sql-driver/mysql`; caracteres especiais na senha exigem percent-encoding |
| PostgreSQL | `postgres://user:password@host:port/dbname` | URL aceita por `lib/pq` e `jackc/pgx/v5/stdlib` |

Drivers cuja sintaxe de DSN diverja do formato acima não são suportados. A compatibilidade é responsabilidade do consumidor e será verificada na validação de E02-P04 (SQLite automatizado; MySQL e PostgreSQL manualmente).

## Configuração

Criar no arquivo `config.go`:

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
	// Obrigatório (não pode ser vazio).
	DBPath string
}

type MySQLConfig struct {
	// Obrigatórios (não podem ser vazios): Host, User, Password e DBName.
	Host     string
	User     string
	Password string
	DBName   string
	Port     int // Se 0, usar porta padrão 3306; negativo é inválido
}

type PostgresConfig struct {
	// Obrigatórios (não podem ser vazios): Host, User, Password e DBName.
	Host     string
	User     string
	Password string
	DBName   string
	Port     int // Se 0, usar porta padrão 5432; negativo é inválido
	// Espaço opcional para SSLMode no futuro, não necessário para escopo inicial
}

type MigrationsConfig struct {
	// Reservado: versão desejada e associação de arquivos incorporados.
	// Definição completa fica para E08-P01.
}

type Config struct {
	// Tipo do banco. Obrigatório.
	Type DBType

	// Nome do driver informado pelo consumidor em database/sql.
	// Obrigatório. Ex.: "sqlite", "mysql", "pgx".
	// Deve corresponder ao nome do driver importado (em branco) pelo consumidor.
	Driver string

	// Configuração específica para SQLite3. Obrigatória quando Type == DBTypeSQLite.
	SQLite *SQLiteConfig

	// Configuração específica para MySQL. Obrigatória quando Type == DBTypeMySQL.
	MySQL *MySQLConfig

	// Configuração específica para PostgreSQL. Obrigatória quando Type == DBTypePostgres.
	Postgres *PostgresConfig

	// Tempo máximo para aguardar a abertura da conexão.
	// Se zero, aplicar o padrão de 10 * time.Second.
	// Se negativo, configuração inválida.
	Timeout time.Duration

	// Configuração opcional de migrações (reservada para E08).
	// Pode ser nil quando não houver intenção de executar migrações.
	Migrations *MigrationsConfig
}
```

Valores omitidos e explícitos: com campos simples (`string`, `int`, `time.Duration`), o valor zero representa a omissão — `Port == 0` aplica a porta padrão e `Timeout == 0` aplica 10 s. Valores negativos de `Port` e `Timeout` são explícitos e inválidos. Não há como desativar o `Timeout`; para prazos maiores, o consumidor define o valor desejado.

## Database

Criar no arquivo `database.go`:

```go
package krios

import "context"

type DB struct {
	// Detalhes internos não exportados nesta etapa
}

// Open valida a configuração, verifica o registro do driver, inicializa os
// recursos de acesso e verifica a conectividade, respeitando cancelamento
// e prazo limite.
//
// Prazo efetivo: o menor entre o deadline de ctx e cfg.Timeout
// (default 10 * time.Second quando cfg.Timeout é zero).
//
// Retorna os erros descritos em "Contrato de erros de Open".
// Se qualquer etapa falhar, os recursos criados são liberados.
func Open(ctx context.Context, cfg Config) (*DB, error)

// Close encerra o acesso e libera os recursos associados.
// Deve ser idempotente: chamadas subsequentes retornam nil (ou erro sem efeito).
func (db *DB) Close() error
```

A interação entre `ctx` e `Timeout` é resolvida por `context.WithTimeout(ctx, cfg.Timeout)` internamente: se o `ctx` do consumidor tiver prazo mais curto, ele prevalece; se `Timeout` for mais curto, o Krios prevalece. Cancelamento do consumidor resulta em `context.Canceled`; esgotamento de qualquer dos prazos resulta em `context.DeadlineExceeded`.

Comportamento de operações após `Close` (além da idempotência da própria chamada) permanece para E02-P03.

## Dialect (adiado)

A interface dos dialetos e sua seleção ficam para **E05-P01**, conforme o plano de execução. Nenhum tipo de dialeto é definido nesta entrega.

## Exemplos

### Exemplo 1 — SQLite3

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/mineot/krios"
	_ "modernc.org/sqlite" // driver importado pelo consumidor (o postgresql e mysql devem vir assim também)
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg := krios.Config{
		Type: krios.DBTypeSQLite,
		Driver: "sqlite", // nome de registro do driver importado
		SQLite: &krios.SQLiteConfig{
			DBPath: "dev.db",
		},
		Timeout: 5 * time.Second,
		// Migrations: nil (não configurado)
	}

	// Prazo efetivo: 3s (ctx), pois é menor que Timeout de 5s.
	db, err := krios.Open(ctx, cfg)
	if err != nil {
		// errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded),
		// errors.Is(err, krios.ErrInvalidConfig), etc.
		log.Fatal(err)
	}
	defer db.Close()
}
```

### Exemplo 2 — MySQL

```go
import (
	_ "github.com/go-sql-driver/mysql"
)

cfg := krios.Config{
	Type:   krios.DBTypeMySQL,
	Driver: "mysql",
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

### Exemplo 3 — PostgreSQL

```go
import (
	_ "github.com/jackc/pgx/v5/stdlib"
)

cfg := krios.Config{
	Type:   krios.DBTypePostgres,
	Driver: "pgx",
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

### Exemplo 4 — Idempotência e tratamento de erros

```go
db, err := krios.Open(context.Background(), cfg)

if err != nil {
	// erros.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded),
	// errors.Is(err, krios.ErrInvalidConfig),
	// errors.Is(err, krios.ErrDriverNotLoaded),
	// errors.Is(err, krios.ErrConnectionFailed)
	return
}

// Close pode ser chamado múltiplas vezes com segurança
_ = db.Close()
_ = db.Close() // idempotente
```
