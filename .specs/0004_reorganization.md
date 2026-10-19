# Krios — Reorganização dos pacotes internos

Data: 2026-10-19

## 1. Objetivo e escopo

O pacote `internal/core` cresceu para um diretório plano com 11 arquivos de
produção e 10 arquivos de teste. Esta alteração distribui o código em pacotes
coesos por responsabilidade, mantendo **integralmente** a API pública exposta
por `github.com/mineot/krios`. É uma mudança estrutural: nenhum comportamento
observável, assinatura pública ou sentinela muda. Ao final, o nível `core` é
removido e os pacotes ficam diretamente sob `internal/`.

Esta nota complementa a seção §3.5 de [0003_improvement_plan.md](0003_improvement_plan.md),
que já previa a organização por responsabilidade. Nenhum construtor, mapeamento
ou política transacional é redesenhado aqui; a suíte de regressão é a mesma.

## 2. Estrutura adotada

```
internal/
├── errs/         package errs         — erros sentinela + normalização nativa
├── config/       package config       — configuração, validação e DSNs
├── entity/       package entity       — entidades por reflexão + validação/formatação
├── result/       package result       — resultado de escrita
├── scan/         package scan         — Row/Rows, All e mapeamento para *T/*[]T
├── execute/      package execute      — delegação nativa compartilhada
├── transaction/  package transaction  — Tx, Commit/Rollback e erro tardio
├── access/       package access       — DB, Open/Close e contrato executor
└── testutil/     package testutil     — driver simulador dos testes
```

| Pacote | Arquivos | Responsabilidade |
| --- | --- | --- |
| `internal/errs` | `errors.go` | Os oito sentinelas `Err*` e `NormalizeTxError` |
| `internal/config` | `config.go`, `dsn.go` | `DBType`, `*Config`, `Config`, `Validate`, `EffectiveTimeout`, `BuildDSN` |
| `internal/entity` | `entity.go`, `validation.go` | `Entity`, `EntityOf`/`OfType`, derivação por reflexão, `Validator`/`Formatter` e os métodos `Validate`/`ValidateAll`/`Format` |
| `internal/result` | `result.go` | `Result`, `Wrap` |
| `internal/scan` | `rows.go`, `mapping.go` | `Row`, `Rows`, `All`, mapeamento para `*T`/`*[]T`, `WrapRow`/`WrapRows`/`BindRow`/`BindRows`/`ErrorRow`/`NativeRowError` |
| `internal/execute` | `execution.go` | `sqlExecutor` e `ExecWith`/`QueryWith`/`QueryRowWith` |
| `internal/transaction` | `tx.go` | `Tx`, `New`, `Commit`/`Rollback`, `recordError`/`recordLate` |
| `internal/access` | `db.go` | `DB`, `Open`, `Close`, `Begin` e o contrato interno `executor` |
| `internal/testutil` | `driver.go` | `Driver`, `Config`, `Rows`, `ReadFailure` |

A fachada `krios.go` permanece a única superfície pública e passa a importar os
pacotes internos (`access`, `config`, `entity`, `errs`, `result`, `scan`,
`transaction`).

## 3. Fronteiras e restrições da linguagem

Os cortes respeitam três restrições de Go, já observadas na exploração:

- **Métodos não atravessam pacotes.** `Row`/`Rows` são definidos em `scan` e
  recebem métodos de mapeamento (`scanEntity`, `All`); por isso `mapping.go`
  fica junto de `rows.go` em `scan`.
- **`entity` e `validation` formam um ciclo de arquivo.** `entity.go` consulta
  `validatorType`/`formatterType` e `validation.go` opera sobre `Entity`; os
  métodos `Validate`/`ValidateAll`/`Format` são métodos de `Entity`. Os dois
  arquivos ficam no mesmo pacote.
- **`Row`/`Rows` e `Tx` formariam um ciclo.** `Tx` retorna `Row`/`Rows`, que
  antes retinham `*Tx` para registrar erro tardio. `scan` não pode importar
  `transaction`. A dependência é invertida por injeção: `Row`/`Rows` guardam um
  recorder `func(error)`; `transaction` o injeta em `QueryContext`/
  `QueryRowContext` via `scan.BindRows`/`scan.BindRow`, e lê o erro terminal
  nativo com `scan.NativeRowError` (que substitui o acesso ao `*sql.Row`). O
  contexto de cancelamento pré-driver é repassado com `scan.ErrorRow`.

Grafo de dependências resultante, sem ciclos:

```
errs  ←  config  ←  access  ←  transaction
errs  ←  entity ─┘         ↗
errs  ←  result  ←  execute ─┐
errs  ←  entity  ←  scan    ─┴─→  access/transaction
```

## 4. Superfície exportada adicional (apenas interna)

Como `internal/` não é importável por consumidores, exportar aqui não altera a
API pública. Símbolos que antes eram não exportados e passaram a ser acessíveis
entre os pacotes internos:

- `errs.NormalizeTxError` (era `normalizeTxError`).
- `config.Validate`, `config.EffectiveTimeout`, `config.BuildDSN` (usados por
  `access`).
- `entity.OfType`, `entity.TypeName`, `(*Entity).FieldIndex` e
  `(*Entity).FieldIndexes` (usados por `scan`).
- `result.Wrap` (era `wrapResult`).
- `scan.WrapRow`, `scan.WrapRows`, `scan.BindRow`, `scan.BindRows`,
  `scan.ErrorRow`, `scan.NativeRowError`.
- `execute.ExecWith`, `execute.QueryWith`, `execute.QueryRowWith`.
- `transaction.New` (constrói o wrapper a partir do `*sql.Tx` de `access.Begin`).

O nome `EffectiveTimeout` evita a colisão entre o campo `Config.Timeout` e um
método homônimo. A interface não exportada `executor` migrou para `access`, com
as asserções `_ executor = (*access.DB)(nil)` e
`_ executor = (*transaction.Tx)(nil)`.

## 5. Mapa de arquivos (antes → depois)

| Antes | Depois |
| --- | --- |
| `internal/core/errors.go` | `internal/errs/errors.go` |
| `internal/core/config.go` | `internal/config/config.go` |
| `internal/core/dsn.go` | `internal/config/dsn.go` |
| `internal/core/entity.go` | `internal/entity/entity.go` |
| `internal/core/validation.go` | `internal/entity/validation.go` |
| `internal/core/result.go` | `internal/result/result.go` |
| `internal/core/rows.go`, `mapping.go` | `internal/scan/rows.go`, `mapping.go` |
| `internal/core/execution.go` | `internal/execute/execution.go` |
| `internal/core/tx.go` | `internal/transaction/tx.go` |
| `internal/core/db.go` | `internal/access/db.go` |
| `internal/core/driver_test.go` | `internal/testutil/driver.go` (fixtures com campos/hooks exportados) |

Além da repartição em pacotes, o nível intermediário `core` foi removido: os
pacotes ficam diretamente sob `internal/` (`internal/access`, `internal/scan`,
...). A privacidade é preservada — `internal/` continua sendo o que impede o
consumidor de importá-los — e a hierarquia `internal/core/...` deixa de existir.

## 6. Testes

Os testes foram repartidos no pacote que passaram a exercitar; os que usam
`access.Open`/`Begin` viraram pacotes externos (`scan_test`,
`transaction_test`) para evitar o ciclo `access → scan/transaction`:

- `config`, `entity`, `result`: caixa branca no próprio pacote.
- `access` (`package access`): `db_test.go` e `execution_test.go`, usando o
  contrato interno `executor` e helpers locais `openDriver`/`beginDriver`.
- `transaction` (`package transaction_test`): `tx_test.go`, com helpers locais,
  `assertTxDone` e a verificação comportamental de que resultados de `DB` não
  registram erro tardio (sem inspecionar campos internos).
- `scan` (`package scan_test`): `rows_test.go` e `mapping_test.go`, com
  helpers locais (`openDriver`/`beginDriver`/`driverConfig`/`assertTxDone`),
  `equalStrings` e a fixture local `tableRows`.
- `testutil`: depende apenas de `config` e da biblioteca padrão, então cada
  camada monta suas fixtures sem ciclo.

A movimentação preserva cada cenário; `TestResultAvailability` e a fixture
`testResult` passaram para `result`.

## 7. Evidências

- `go build ./...`, `go vet ./...` sem avisos.
- `gofmt -l .` sem arquivos fora do padrão.
- `make check` (CGO desabilitado por padrão no alvo) e `make race`
  (`CGO_ENABLED=1 go test -race -count=1 ./...`) passaram em Go 1.26.8.
- `go list -deps ./...` confirma que a produção continua usando somente a
  biblioteca padrão e os pacotes do próprio módulo; drivers permanecem nos
  testes.

A execução remota do workflow e a integração real MySQL/PostgreSQL continuam
pendentes, conforme o escopo herdado de [E09_manual_validation.md](E09_manual_validation.md).
