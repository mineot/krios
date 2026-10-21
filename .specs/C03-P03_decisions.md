# C03-P03 — Decisões de diagnóstico e limpeza (D14–D17)

Data: 2026-10-20. Resolve C03-P03 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §5 e o achado A07. Registra o comportamento dos caminhos de falha da aplicação antes da correção: registro inicial, corpo do arquivo, marcação de sucesso/falha, cancelamento entre passos, commit e rollback. As escolhas foram apresentadas e aprovadas pelo consumidor em 2026-10-20. Complementa [E08-P01_decisions.md](E08-P01_decisions.md) §9/§11, [C01-P01_decisions.md](C01-P01_decisions.md) D03 §4 (comportamento da tabela de infraestrutura) e [C03-P02_decisions.md](C03-P02_decisions.md) (D10–D13).

## 1. Contexto e reprodução

O achado A07 apontou que `apply.go` nem sempre preservava `ErrPartialMigration` quando o SQL sem transação executou e a atualização do histórico também falhou, e que os escopos transacionais não registravam limpeza logo após `Begin`. A reprodução com o driver simulado confirmou quatro lacunas, antes da correção:

| Caminho                                                       | Comportamento anterior                                                                 | Problema                                                                 |
| ------------------------------------------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `runNoTxScope`: corpo falha e `markFailed` também falha       | `fmt.Errorf("record ... failure: %w (cause: %w)")`                                      | Perde a identidade de `ErrPartialMigration`; retomada insegura não é bloqueável por `errors.Is`. |
| `runNoTxScope`: `markApplied` falha após corpo commitado      | `fmt.Errorf("mark ... applied: %w")`                                                    | Erro comum: o SQL commitou sem registro `applied` (estado incerto) mas o consumidor recebe erro genérico. |
| `runVersionScope`/`runFileScope`: `Commit` falha              | `fmt.Errorf("commit ...: %w")`                                                          | Nenhuma tentativa de limpeza da transação possivelmente ativa; nenhuma indicação de desfecho incerto.    |
| `runNoTxScope`: `insertInProgress` falha                      | `fmt.Errorf("record ...: %w")`                                                          | Aceitável (nada executou); mantido, com causa preservada.                 |

Evidência de regressão: [diagnose_test.go](../internal/migrate/diagnose_test.go) (falhava nos quatro caminhos antes da correção; passa depois). Injeção via fixtures de [scope_test.go](../internal/migrate/scope_test.go) — evidência real de servidor pertence a E10/E09.

## 2. D14 — Falha do registro de falha preserva o bloqueio

**Comportamento escolhido.** Quando o corpo do arquivo sem transação falha e a atualização de `failed` também falha, `partialFailure` continua retornando um `*PartialMigrationError` (`errors.Is(err, ErrPartialMigration)` verdadeiro), com:

- **status `failed`**, pois o corpo já executou e falhou;
- **mensagem explícita de estado incerto** ("the state is uncertain, resolve it manually before retrying");
- **ambas as causas preservadas** via `%w`: a causa do corpo e a causa da falha de registro (`combined`), encadeadas de modo que `errors.Is` alcança as duas.

A identidade de `ErrPartialMigration` é o que garante o bloqueio da retomada automática: sem ela, o consumidor trataria a falha como erro comum e poderia reexecutar o arquivo sobre um esquema já alterado.

**Alternativas rejeitadas.**

- _Manter `fmt.Errorf` simples com as duas causas._ Rejeitada: preserva texto mas não a sentinela; quebra o contrato de bloqueio e a inspeção por `errors.As` do tipo estruturado.
- _Retornar só a causa do corpo, descartando a falha de registro._ Rejeitada: perde diagnóstico da escrita de histórico que também falhou; o consumidor precisa saber que o registro pode estar inconsistente.
- _Criar status novo (ex.: `"unknown"`)._ Rejeitada: o contrato de status é `in_progress`|`failed` (E08 §9); ampliar valores é mudança pública desnecessária — a mensagem já comunica a incerteza.

**Impacto de compatibilidade.** Corrige comportamento incorreto (A07); consumidores que hoje recebem erro comum passam a receber o estado parcial bloqueável, como o contrato já prometia. Nenhuma assinatura muda.

## 3. D15 — Falha ao marcar `applied` é estado incerto, não erro comum

**Comportamento escolhido.** Quando o corpo do arquivo sem transação **commitou** e a marcação `applied` falha, `runNoTxScope` retorna um `*PartialMigrationError` com **status `in_progress`** e mensagem de estado incerto ("committed without a confirmed history record; the state is uncertain, resolve it manually before retrying"), preservando a causa da falha de marcação.

O status `in_progress` reflete a realidade observável: o corpo rodou, mas o histórico não confirmou `applied`. A retomada automática é bloqueada porque o registro `in_progress` já é, por si, condição de bloqueio em `detectPartial` (E08 §9).

**Alternativas rejeitadas.**

- _Erro comum, deixando a tabela em `in_progress` para bloquear depois._ Rejeitada: o bloqueio até ocorreria em uma chamada futura, mas o chamador atual recebe erro enganoso (parece falha de metadados, não estado parcial) e não tem o contexto estruturado para agir.
- _Marcar `failed` em vez de `in_progress`._ Rejeitada: o corpo commitou; afirmar `failed` sugeriria que nada persistiu, contrariando o estado real. `in_progress` é o estado verdadeiro entre execução e confirmação.

**Impacto de compatibilidade.** Corrige A07 no caminho de sucesso parcial; nenhuma assinatura muda.

## 4. D16 — Falha de commit: limpeza defensiva e desfecho incerto

**Comportamento escolhido.** Falha de `Commit` em `runVersionScope`/`runFileScope` passa por `commitScope`, que:

- tenta `Rollback` **defensivamente**, para não deixar uma transação possivelmente ativa no recurso/defer;
- preserva o erro de commit como causa (`%w`);
- preserva o erro de rollback, se houver (`%w`);
- acrescenta **texto de desfecho incerto** ("state uncertain"), porque um commit que falhou não estabelece se o servidor confirmou ou não.

A falha de commit **não** vira `ErrPartialMigration`: não é estado parcial de arquivo já commitado (que o consumidor reverte por procedimento), é incerteza sobre o resultado da finalização; converter em estado parcial bloquearia retomada legítima sem base observável.

**Alternativas rejeitadas.**

- _Retornar o erro de commit sem rollback._ Rejeitada (A07): deixa transação possivelmente ativa sem tentativa de limpeza; contraria o objetivo de C03-P03.
- _Emitir `ErrPartialMigration` na falha de commit._ Rejeitada: mudaria o contrato de bloqueio para um caso sem registro de estado parcial correspondente; a D03 §4 reserva o estado parcial ao caminho sem transação do MySQL.
- _Engolir a falha de rollback._ Rejeitada: o consumidor precisa saber que a limpeza também falhou para dimensionar a incerteza.

**Impacto de compatibilidade.** Nenhuma assinatura muda; adiciona tentativa de limpeza e contexto. Testes que apenas verificavam presença de "commit" na mensagem continuam válidos (`commit` permanece no texto).

## 5. D17 — Cancelamento e registro inicial

**Comportamento escolhido.**

- **Cancelamento entre passos:** `applySteps` já consulta `ctx.Err()` antes de cada grupo e de cada passo e devolve `fmt.Errorf("krios: migrations apply: %w", err)`, preservando `context.Canceled`/`DeadlineExceeded` por `errors.Is`. Nenhuma transação é iniciada depois do cancelamento. Mantido sem mudança de contrato; coberto por teste.
- **Falha de `insertInProgress`:** erro comum com contexto do arquivo e causa preservada (`%w`). Nada executou, não há estado parcial a reportar; iniciar transação/limpeza seria desnecessário. Mantido.
- **Falha de `Begin`:** erro com contexto de escopo e causa preservada; nenhum recurso fica ativo. Mantido.

**Alternativas rejeitadas.**

- _Converter cancelamento em `ErrPartialMigration`._ Rejeitada: cancelamento antes de qualquer passo não cria estado parcial; forçar a sentinela confundiria o consumidor.
- _Tentar rollback em falha de `insertInProgress` no caminho sem transação._ Rejeitada: esse caminho não tem transação própria; não há o que limpar.

**Impacto de compatibilidade.** Nenhum; documenta e cobre o comportamento já existente.

## 6. Verificação e evidência

- Regressão antes da correção: [diagnose_test.go](../internal/migrate/diagnose_test.go) falhava em `TestNoTxBodyFailureMarkFailedAlsoFails`, `TestNoTxMarkAppliedFailure`, `TestFileScopeCommitFailureCleansUp` e `TestVersionScopeCommitFailureCleansUp`; passa após as correções de `apply.go`.
- Cobertura adicional: `TestNoTxInsertInProgressFailure`, `TestVersionScopeRollbackOnFileFailure` (limpeza + causa dupla), `TestBeginFailure`, `TestCancelBetweenSteps`.
- Fixtures: `scopeExecutor`/`scopeTx`/`scopeBeginner` ampliados em [scope_test.go](../internal/migrate/scope_test.go) (hooks de injeção, contadores de commit/rollback), `hookedBeginner`/`hookedTx` e `callbackExecutor` em diagnose_test.go.
- `make check` e `make race` passaram (Go 1.26.8 e 1.27.2).

**Código afetado.** `internal/migrate/apply.go` (`runNoTxScope`, `partialFailure`, `partial`, `uncertainPartial`, `commitScope`, `runVersionScope`, `runFileScope`), [diagnose_test.go](../internal/migrate/diagnose_test.go) (novo) e [scope_test.go](../internal/migrate/scope_test.go) (fixtures ampliadas), [E08-P01_decisions.md](E08-P01_decisions.md) §9/§11 e [C01-P01_decisions.md](C01-P01_decisions.md) D03 §4.
