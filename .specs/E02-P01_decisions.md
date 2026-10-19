# E02-P01 — Decisões

Data original: 2026-10-18. Revisão vigente: 2026-10-19, M01–M06.

## 1. Escopo e organização

O pacote público é `krios`, na raiz de `github.com/mineot/krios`. A fachada pública está em `krios.go`. A implementação está sob `internal/`: configuração em `internal/config`, erros em `internal/errs`, ciclo de vida em `internal/access` e DSN em `internal/config`. Os testes internos acompanham a implementação; a fachada utiliza aliases e delegação, mantendo o mesmo import. Ver [E01-P01](E01-P01_decisions.md).

Dialetos permanecem em E05-P01; migrações em E08. `MigrationsConfig` é uma reserva vazia e não implementa execução de migrações.

## 2. Configuração vigente

| Campo                                 | Contrato                                                                        |
| ------------------------------------- | ------------------------------------------------------------------------------- |
| `Type DBType`                         | Obrigatório: `DBTypeSQLite`, `DBTypeMySQL` ou `DBTypePostgres`                  |
| `Driver string`                       | Nome registrado pelo driver importado pelo consumidor em `database/sql`         |
| `SQLite *SQLiteConfig`                | Obrigatório somente para SQLite; `DBPath` não vazio                             |
| `MySQL *MySQLConfig`                  | Obrigatório somente para MySQL; `Host`, `User`, `Password`, `DBName` não vazios |
| `Postgres *PostgresConfig`            | Obrigatório somente para PostgreSQL; mesmos campos obrigatórios do MySQL        |
| `Port int`, nas configurações de rede | Zero aplica 3306/5432; valores explícitos entre 1 e 65535                       |
| `Timeout time.Duration`               | Zero aplica 10 segundos; negativo é inválido                                    |
| `Migrations *MigrationsConfig`        | Opcional; sem validação de conteúdo nesta entrega                               |

A ordem de validação é: tipo, driver vazio, configuração do banco ausente, configuração de outro banco preenchida, campos obrigatórios, porta e timeout. Valores inválidos preservam `ErrInvalidConfig` via `errors.Is`. Os campos são recebidos literalmente, sem trim de senha ou identificador. Senha vazia continua fora do escopo aprovado. Não existe opção de desativar o timeout.

`DB` e `Tx` não armazenam uma cópia completa de `Config` após a abertura. A API não expõe o pool nativo. Configurações de pool, TLS e opções adicionais de drivers não fazem parte desta entrega.

## 3. Drivers e DSN

O consumidor importa o driver; a biblioteca não o importa. `Open` confere `sql.Drivers()`, mas não tenta deduzir compatibilidade pelo nome. O consumidor deve escolher um driver que aceite o formato gerado.

| Banco      | Import de referência             | Nome de registro | DSN                                             |
| ---------- | -------------------------------- | ---------------- | ----------------------------------------------- |
| SQLite     | `modernc.org/sqlite`             | `sqlite`         | O próprio `DBPath`                              |
| MySQL      | `github.com/go-sql-driver/mysql` | `mysql`          | `user:password@tcp(host:port)/dbname`           |
| PostgreSQL | `github.com/jackc/pgx/v5/stdlib` | `pgx`            | URL `postgres://user:password@host:port/dbname` |

MySQL mantém a senha literal e aplica `url.PathEscape` a `DBName`. A instrução original de percent-encoding da senha estava incorreta e foi substituída pelo [contrato do driver](https://github.com/go-sql-driver/mysql#dsn-data-source-name). PostgreSQL usa `net/url`; ambos usam `net.JoinHostPort` para preservar IPv6. Os testes de DSN não abrem servidores reais nem validam todos os drivers possíveis.

## 4. Abertura, fechamento e erros

```go
func Open(ctx context.Context, cfg Config) (*DB, error)
func (db *DB) Close() error
```

Após validar configuração e registro do driver, `Open` inicia o orçamento temporal antes de `sql.Open`. O prazo efetivo é o menor entre o contexto do consumidor e `Timeout`. A expiração é verificada antes/depois da preparação e depois do ping. Falha após criação do acesso provoca seu fechamento.

Operações bloqueantes exigem cooperação do driver: o Krios não interrompe preparação, conexão ou fechamento que ignorem cancelamento. Não são criadas goroutines para abandonar essas operações. O sucesso do ping confirma conectividade naquele momento, sem garantir esquema ou disponibilidade futura.

| Condição                                                    | Identificação com `errors.Is`                                              |
| ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| Configuração inválida                                       | `ErrInvalidConfig`                                                         |
| Driver não registrado                                       | `ErrDriverNotLoaded`                                                       |
| Cancelamento/prazo observado                                | `context.Canceled` / `context.DeadlineExceeded`, sem `ErrConnectionFailed` |
| Falha de preparação/conexão/ping sem cancelamento observado | `ErrConnectionFailed` e a causa original                                   |
| Operação iniciada após `Close`                              | `ErrClosed`; em `QueryRowContext`, comunicado por `Scan`                   |

`Close` é idempotente: chamadas seguintes retornam `nil`. O estado é protegido por mutex. Operações que concorrem com o encerramento podem concluir ou retornar falha nativa; não há promessa de ordem, mas não devem causar corrida de dados no wrapper. Os resultados já entregues continuam sujeitos às regras de propriedade de E03.

## 5. Exemplos e validação

Os nomes de testes internos abaixo são relativos a `internal/`; os caminhos de integração começam com `tests/`.

Exemplos executáveis com import externo e banco temporário estão em [tests/examples_test.go](../tests/examples_test.go). O consumidor preenche a configuração em código; não há exigência de arquivo externo ou variável de ambiente.

| Evidência                                              | Localização                                                | Alcance                                                               |
| ------------------------------------------------------ | ---------------------------------------------------------- | --------------------------------------------------------------------- |
| Campos, padrões e portas                               | `krios_test.go`, `dsn_test.go`                             | Regras locais e formato do DSN                                        |
| Falha/cancelamento no ping e liberação                 | `db_test.go`, `driver_test.go`                             | Driver simulado com contadores; inclui preparação dentro do orçamento |
| Abertura/fechamento reais                              | `tests/lifecycle_test.go`, `tests/lifecycle_three_test.go` | SQLite temporário                                                     |
| Falhas controladas para configurações MySQL/PostgreSQL | `tests/lifecycle_three_test.go`                            | Propagação de falhas, sem conexão real                                |
| Integração MySQL/PostgreSQL                            | [Roteiro E09](E09_manual_validation.md)                    | Pendente de execução pelo consumidor                                  |

## 6. Histórico das decisões

| Marco                         | Decisão e revisão                                                                                                                                                         |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| E02-P01, 18/10                | Configuração por banco, driver explícito, timeout de 10 s e erros distinguíveis                                                                                           |
| Revisão de organização, 18/10 | Arquivo único adotado provisoriamente; substituído pela divisão por responsabilidade em M05                                                                               |
| E02-P03, 18/10                | `Open`, `Close` idempotente e `ErrClosed` implementados                                                                                                                   |
| M01–M04, 19/10                | Correção do DSN MySQL, portas até 65535, orçamento anterior à preparação e testes de limpeza                                                                              |
| M05, 19/10                    | Remoção de configuração retida sem uso e organização por responsabilidade; posteriormente movida para `internal/`, com fachada pública na raiz a pedido do consumidor |

### Adendo — E02-P03 (2026-10-18)

Este adendo é histórico. O contrato vigente está nas seções 2–5: conserva as assinaturas e a idempotência originais, corrige o escape do DSN, amplia a validação de portas e torna verificáveis os limites de abertura/limpeza. A integração real dos três bancos nunca deve ser inferida do driver que sempre falha.
