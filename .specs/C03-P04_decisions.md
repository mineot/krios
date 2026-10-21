# C03-P04 — Decisões de progresso parcial (D18–D20)

Data: 2026-10-20. Resolve C03-P04 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §5 e o achado A03. Registra o que a reprodução simulada provou sobre o estado parcial de versões com múltiplos arquivos, em `up` (MySQL transacional por arquivo e sem transação) e em `down` não contíguo, e sobre a atomicidade por versão no SQLite/PostgreSQL. Complementa [E08-P01_decisions.md](E08-P01_decisions.md) §7/§9, [C03-P03_decisions.md](C03-P03_decisions.md) (D14–D17), [C03-P02_decisions.md](C03-P02_decisions.md) (D10–D13) e [C01-P01_decisions.md](C01-P01_decisions.md) D03.

## 1. Contexto e reprodução

O achado A03 suspeitava que `LastVersion`/`LastFile` e a conclusão de versões com múltiplos arquivos não correspondiam ao contrato de E08 §7/§8/§9 ("versão só é efetiva quando **todos** os arquivos da versão estão `applied`"). Até esta entrega nenhum teste exercitava versões com mais de um arquivo por direção — a lacuna que manteve o defeito invisível.

A reprodução usa um driver simulado com uma tabela de histórico em memória fiel ([progress_apply_test.go](../internal/migrate/progress_apply_test.go)): as escritas de histórico da biblioteca (DELETE/INSERT/UPDATE) refletem em um `historyStore`, as transações são fiéis (snapshot no `Begin`, restauração no `Rollback`), e os passos puros são exercitados em [progress_test.go](../internal/migrate/progress_test.go). Evidência real de servidor permanece para E10/E09.

Duas lacunas foram comprovadas antes da correção:

| Lacuna                                                                                  | Comportamento anterior                                                                                          | Problema                                                                                                   |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `insertInProgress` apagava a **versão inteira** (`WHERE version = ? AND direction = ?`) antes de cada arquivo `up` | Só o último arquivo da versão permanecia na tabela; irmãos já aplicados eram apagados a cada arquivo          | Versão com múltiplos arquivos **nunca** conclui: `completeUp`/`effectiveVersion` retornam 0 para sempre.     |
| `applySteps` elevava `lastApplied` a cada **arquivo**                                  | Falhar no 2º arquivo da versão N reportava `LastVersion = N`, embora só o 1º arquivo tivesse sido aplicado    | `LastVersion` do `*PartialMigrationError` inflava a versão em relação ao estado real e ao `detectPartial`. |

A correção das duas lacunas faz o caminho de execução concordar com o de preparação (`detectPartial`/`effectiveVersion`), que sempre usou a conclusão por versão.

## 2. D18 — O histórico registra um arquivo por linha; nunca apagar a versão inteira

**Comportamento escolhido.** `insertInProgress` limpa, no avanço (`up`), apenas a linha do **próprio arquivo** antes de inserir: `DELETE FROM migrations WHERE version = ? AND filename = ? AND direction = ?`. A chave única `(version, filename, direction)` de E08 §7 passa a ser respeitada de fato: cada arquivo da versão mantém sua linha, e a versão só é considerada completa quando **todos** os arquivos estão `applied`.

A limpeza por arquivo preserva o objetivo original (descartar o registro `failed`/`in_progress` de uma tentativa anterior do mesmo arquivo, permitindo retomada), sem afetar os arquivos irmãos já aplicados. O caminho `down` permanece sem DELETE: a reversão atualiza as linhas existentes por `markApplied`/`markFailed`.

**Alternativas rejeitadas.**

- _Manter o DELETE por versão e concluir a versão pelo último arquivo._ Rejeitada: contradiz E08 §7 (chave `(version, filename, direction)`) e §8/§9 (versão efetiva exige todos os arquivos); tornaria `detectPartial` inalcançável para `up` com múltiplos arquivos.
- _DELETE por versão apenas no primeiro arquivo._ Rejeitada: exige distinguir "primeiro arquivo do grupo" dentro de `insertInProgress`, acoplando o registro ao laço de execução e reintroduzindo o risco de apagar irmãos quando o grupo é reexecutado parcialmente.
- _Não limpar nenhuma linha no `up`._ Rejeitada: uma tentativa anterior `failed`/`in_progress` do mesmo arquivo violaria a chave única no INSERT seguinte e não permitiria a retomada do arquivo específico.

**Impacto de compatibilidade.** Corrige comportamento incorreto (A03) para versões com múltiplos arquivos; versões de um único arquivo têm comportamento idêntico (a linha apagada é a mesma). Nenhuma assinatura pública muda. Um consumidor que já contava com a linha única por versão passa a ver uma linha por arquivo, como o contrato de E08 §7 sempre descreveu.

## 3. D19 — `LastVersion` do estado parcial de execução é a última versão **completa**

**Comportamento escolhido.** Em `applySteps`, `lastApplied` só avança quando o **grupo (versão)** termina por inteiro; durante os arquivos de um grupo, o valor repassado a `runNoTxScope` (e assim a `*PartialMigrationError`) é a última versão **completamente** aplicada. O valor alcançado pelo grupo (`appliedAfter`) é atribuído somente após o sucesso de todo o grupo.

Com isso, falhar no 2º arquivo de uma versão com 2 arquivos reporta `LastVersion` = versão anterior completa, igual ao que `detectPartial` calcula na preparação. Nenhum caminho infla mais a versão reportada.

**Alternativas rejeitadas.**

- _Recomputar `lastAppliedComplete` a partir do histórico dentro do erro._ Rejeitada nesta entrega: o executor sem transação não tem leitura do histórico no ponto da falha sem consulta adicional; manter o valor em memória alinhado ao cálculo de preparação é mais simples e suficiente.
- _Reportar `LastVersion` como a versão em execução._ Rejeitada: foi exatamente o defeito (A03); indicaria conclusão que não ocorreu.

**Impacto de compatibilidade.** Corrige A03 na composição do erro; o tipo e a sentinela não mudam. `LastVersion` passa a ser sempre a última versão integralmente aplicada, coerente entre execução e preparação.

## 4. D20 — Atomicidade por versão em SQLite/PostgreSQL; `down` nomeia o último arquivo aplicado

**Comportamento escolhido.**

- **SQLite/PostgreSQL (`SupportsTransactionalDDL`):** um grupo onde **todos** os passos são transacionais roda em **escopo por versão** ([`runVersionScope`](../internal/migrate/apply.go)). Como `fileTransactional` descarta o booleano nesses dialetos, os arquivos de DDL também são tratados como transacionais, e a versão inteira é atômica: falha no meio reverte a versão por completo e produz erro comum (via `rollbackVersion`), **não** estado parcial. Versões anteriores confirmadas permanecem. Isso corresponde a C01 D03 ("rollback da versão em execução, versões já confirmadas permanecem") e é verificado em [progress_apply_test.go](../internal/migrate/progress_apply_test.go) (`TestSQLiteVersionScopeRollsBackWholeVersion`).
- **MySQL sem transação:** a falha no meio de uma versão deixa os arquivos já commitados e bloqueia a retomada com `ErrPartialMigration` (`TestMySQLPartialNoTxRestartBlocks`).
- **MySQL transacional por arquivo:** cada arquivo tem sua própria transação e histórico; falha em um arquivo faz rollback só daquele arquivo e deixa os anteriores aplicados, bloqueando depois por estado parcial (`TestMySQLPartialFileScopeRestartBlocks`).
- **`down` não contíguo:** `detectPartial` nomeia em `LastFile` o **último arquivo `applied` do prefixo** reaplicado (ex.: `m/001_b.down.sql`), não o próximo pendente; `LastVersion` é a última versão completa (`TestDetectionDownNonContiguous` em [progress_test.go](../internal/migrate/progress_test.go), além de `TestDetectPartialDownReportsLastFile`).

**Alternativas rejeitadas.**

- _Forçar caminho sem transação no SQLite para produzir estado parcial observável._ Rejeitada: contraria a promessa transacional de C01 D03 e a capacidade real do motor; o SQLite não deve apresentar estado parcial onde consegue ser atômico.
- _Nomear em `down` o próximo arquivo pendente._ Rejeitada: `LastFile` descreve o ponto alcançado, não o alvo; o prefixo aplicado é a informação útil para a resolução manual, coerente com o caminho `up`.

**Impacto de compatibilidade.** Nenhuma mudança de assinatura; consolida comportamento já prometido por C01 D03 e E08 §9. A reprodução é simulada; a confirmação em servidores reais segue para E09 (roteiro M01–M04 de [E09_manual_validation.md](E09_manual_validation.md)).

## 5. Rastreabilidade

Evidência simulada: [progress_test.go](../internal/migrate/progress_test.go) (funções puras `detectPartial`/`effectiveVersion`/`lastAppliedComplete`/`appliedAfter`, `up` e `down`, versões não contíguas) e [progress_apply_test.go](../internal/migrate/progress_apply_test.go) (`TestMySQLPartialNoTxRestartBlocks`, `TestMySQLPartialFileScopeRestartBlocks`, `TestSQLiteVersionScopeRollsBackWholeVersion`, `TestGuidedRecoveryAfterPartial` — fluxo M01–M04 reconstruído). As correções ficam em [history.go](../internal/migrate/history.go) (`insertInProgress`) e [apply.go](../internal/migrate/apply.go) (`applySteps`). `make check` e `make race` passaram em Go 1.26.8 e 1.27.2.
