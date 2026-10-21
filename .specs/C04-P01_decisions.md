# C04-P01 — Reprodução de recursos retidos (D21)

Data: 2026-10-20. Resolve C04-P01 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §6 e o achado A05. Registra as regressões que comprovam, com contadores do driver simulado, que as falhas de leitura/mapeamento retêm recursos nativos e não são registradas na transação. A correção é C04-P02; esta entrega é **exclusivamente reprodução** — os testes introduzidos falham por design até C04-P02. Complementa [E03-P01_decisions.md](E03-P01_decisions.md) §3.3/§3.4.6 e [C01-P01_decisions.md](C01-P01_decisions.md) D04 §5.

## 1. Contexto e objetivo

O achado A05 aponta que [Rows.All](../internal/scan/mapping.go) retorna antes de fechar em falhas de destino (`allDest`), de derivação (`entity.OfType`) e de metadados (`Columns`); que [Row.Scan](../internal/scan/rows.go) rejeita a entidade antes de consumir a linha nativa; e que os atalhos públicos nem sempre devolvem o recurso.

A reprodução usa o `testutil.Driver` e os contadores de fechamento (`testutil.Rows.Closes`, `tableRows.closes`). O observável é o `driver.Rows.Close`, que ocorre somente quando `(*sql.Rows).Close`/`(*sql.Row).Scan` é chamado — conforme a fonte de `database/sql` de Go 1.26 (`Row.Scan` só libera a conexão pelo `defer r.rows.Close()`, `sql.go:3504`). O teste observa o contador **antes** de qualquer `defer` do próprio teste que pudesse mascarar o defeito.

Lacunas comprovadas (todas retêm o recurso; `closes = 0` quando o esperado é `1`):

| Caminho                                  | Local                                                                       | Observável no teste                                     |
| ---------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------- |
| `Rows.All` com destino inválido          | [mapping.go:140](../internal/scan/mapping.go) `allDest` early return        | `TestRowsAllClosesOnInvalidDestination` (5 variantes)   |
| `Rows.All` com entidade inválida         | [mapping.go:146](../internal/scan/mapping.go) `entity.OfType` early return  | `TestRowsAllClosesOnInvalidEntity`                      |
| `Row.Scan` com entidade nil rejeitada    | [rows.go:44](../internal/scan/rows.go) retorna sem `rw.r.Scan`              | `TestRowScanReleasesNativeRowOnRejectedEntity`          |
| `File.All` (atalho)                      | [sqlfile.go](../internal/sqlfile/sqlfile.go) delega a `rows.All`            | `TestFileOneAndAllRejectInvalidDest` (ampliado)         |
| `Select.All/One/Paginate` (atalho)       | [query.go](../internal/query/query.go), [pagination.go](../internal/query/pagination.go) | `TestReadShortcutsCloseOnInvalidDestination`  |
| `Update.All`/`Delete.All` (atalho)       | [update.go](../internal/write/update.go), [delete.go](../internal/write/delete.go) | `TestReadShortcutsCloseOnInvalidDestination`      |
| `Tx.Select.All` não registra a falha     | [mapping.go:140](../internal/scan/mapping.go) sem `rw.record`/`rw.Close`    | `TestReadShortcutsCloseOnInvalidDestination/tx_...`     |

## 2. D21 — O observável de fechamento é o `driver.Rows`, e a reprodução fica vermelha até C04-P02

**Comportamento escolhido.** A regressão observa o fechamento do `driver.Rows` nativo (contadores `testutil.Rows.Closes` / `tableRows.closes`) e **não** o fechamento de conexão do `Driver.Closes`: o pool padrão de `sql.Open` (sem `SetMaxOpenConns`) não torna o fechamento de conexão determinístico. Para `Rows.All`, o wrapper é o único responsável pelo `Close`; para `Row.Scan`, `database/sql` só libera o `*sql.Row` (e o `driver.Rows` subjacente) quando o wrapper chama `rw.r.Scan`, de modo que rejeitar a entidade sem consumir a linha aparece como `closes = 0`.

Os testes desta entrega **falham de propósito** enquanto A05 não é corrigido. `make check`/`make race` só voltam a verde em C04-P02; a evidência de C04-P01 é o conjunto de falhas exatas acima. Cada asserção verifica o fechamento logo após a chamada, antes de qualquer `defer` do teste.

**Alternativas rejeitadas.**

- _Observar `Driver.Closes` (conexões)._ Rejeitada: o pool padrão reutiliza conexões de forma não determinística; um vazamento de `driver.Rows` não se traduz necessariamente em fechamento de conexão no momento do teste.
- _Cobrir o "erro de metadados" (`rw.Columns()` falhando) com o driver simulado._ Rejeitada nesta entrega: `database/sql` preenche as colunas de `*sql.Rows` internamente a partir de `driver.Rows.Columns()` (que não retorna erro) e só expõe erro de `Columns()` em estado inválido/fechado, não injetável de forma limpa por um `driver.Rows`. O early return correspondente em [mapping.go:152](../internal/scan/mapping.go) compartilha a mesma causa-raiz dos demais e é coberto pela correção de C04-P02; a reprodução direta do erro de metadados fica para a evidência real de E10 se necessário.
- _Deixar os testes de P01 verdes já nesta entrega._ Rejeitada: contraria o plano ("demonstrar recursos retidos"); a correção pertence a C04-P02.

**Impacto de compatibilidade.** Nenhum código de produção muda em C04-P01; apenas testes e fixtures são adicionados. Nenhuma assinatura pública muda.

## 3. Escopo entregue

- [internal/scan/mapping_test.go](../internal/scan/mapping_test.go): `TestRowsAllClosesOnInvalidDestination`, `TestRowsAllClosesOnInvalidEntity` e o tipo `embeddedEntity`.
- [internal/scan/rows_test.go](../internal/scan/rows_test.go): `TestRowScanReleasesNativeRowOnRejectedEntity`.
- [internal/sqlfile/sqlfile_test.go](../internal/sqlfile/sqlfile_test.go): `TestFileOneAndAllRejectInvalidDest` ampliado para observar `Closes` em `One` e `All`.
- [internal/access/read_shortcut_test.go](../internal/access/read_shortcut_test.go): `TestReadShortcutsCloseOnInvalidDestination` cobrindo `Select.All/One/Paginate`, `Update.All`, `Delete.All` e o caminho de `Tx` (falha de mapeamento ainda não registrada, `Commit` não bloqueado).

## 4. Verificação

- `go build ./...` e `go vet` limpos nos pacotes afetados.
- Os testes de regressão falham com `closes = 0, want 1` (e, no caso de `Tx`, `Commit() = nil` confirmando que a falha não é registrada). Verde esperado somente após C04-P02.
