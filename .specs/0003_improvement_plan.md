# Krios — Plano de melhorias da base E01–E03

Data: 2026-10-19

## 1. Objetivo e escopo

Corrigir os achados da revisão parcial, alinhar os contratos documentados ao comportamento observável e preparar a base para E04. Este plano complementa [0002_execution_plan.md](0002_execution_plan.md), preservando os identificadores e as entregas originais.

O escopo inclui configuração, conexão, execução, resultados, transações, testes, exemplos e documentação já existentes. Entidades, construtores, SQL incorporado e migrações continuam nas entregas E04–E08.

**Status (2026-10-19): M01–M06 implementados e validados localmente.** As decisões técnicas abaixo foram aplicadas e registradas nas especificações vigentes. A execução remota do workflow atualizado e a integração real MySQL/PostgreSQL permanecem pendentes, conforme o escopo. As assinaturas públicas foram preservadas.

## 2. Base da revisão e critérios de trabalho

Na revisão de 2026-10-19, `make check` e `go test -race -count=1 ./...` passaram em Go 1.26.8. Uma reprodução isolada com driver simulado demonstrou perda de erros em `QueryRowContext`, perda do erro de iteração após `Rows.Close` e divergência do sentinela transacional após falhas de finalização. O teste existente de `QueryRowContext` após encerramento exige o resultado incorreto e deverá ser corrigido.

O DSN MySQL foi avaliado por inspeção e comparação com o contrato do driver. Não houve integração real com MySQL/PostgreSQL nessa revisão nem nova execução remota da matriz Go 1.26/1.27. Os resultados anteriores do CI permanecem como evidência histórica, não como validação deste plano.

Regras de execução:

- Cada passo tem identificador estável, caixa de seleção, resultado esperado e verificação. Marcar somente após obter a evidência correspondente.
- Corrigir comportamento com testes de regressão que falhem na implementação anterior. Fazer a reorganização física em alteração separada, depois das correções.
- Preservar o módulo, o pacote público e as assinaturas existentes. Registrar mudanças comportamentais e seus exemplos de migração.
- Usar SQLite temporário para integração automatizada e drivers simulados para falhas determinísticas. Não acessar `dev.db` nos testes normais.
- Manter drivers fora dos imports de produção. Não adicionar dependências de produção para resolver os achados.
- Preservar a condição de execução gratuita do CI em repositório público. A integração real dos outros bancos continua manual, com evidências pendentes até E09.
- Usar a biblioteca padrão como referência de interoperabilidade; políticas próprias do Krios devem ser identificadas como tais, com limites explícitos.

## 3. Direção técnica aplicada

### 3.1. Erros e recursos de leitura

`QueryRowContext` deve transportar erros até `Scan`, preservando `errors.Is`/`errors.As`. Banco fechado, transação encerrada, contexto cancelado e prazo excedido não representam ausência de registro. `sql.ErrNoRows` deve indicar uma consulta sem resultado. `Rows.Err` deve continuar informando a falha de iteração após `Close`, conforme o [contrato de database/sql](https://pkg.go.dev/database/sql#Rows.Err).

Preferir um erro pendente em `Row` e manter a referência ao iterador nativo após fechar seus recursos. Evitar estado e sincronização duplicados em `Rows` quando a delegação resolve o contrato. Continuar recomendando `defer rows.Close()` e checagem de `rows.Err()`; o fechamento automático ao esgotar um resultado não substitui o cuidado com retornos antecipados.

### 3.2. Transações e limpeza

Manter `Begin(ctx)`, `Commit(ctx)` e `Rollback(ctx)`. `Rollback(ctx)` foi revisado para **sempre tentar a limpeza de uma transação ativa**, mesmo com contexto cancelado ou expirado. Se a limpeza tiver sucesso, retornar `nil`; se falhar, preservar a falha. Chamadas posteriores ao encerramento continuam retornando `ErrTxDone`.

Essa escolha é uma política do Krios para preservar a assinatura existente. `database/sql` possui `Commit()` e `Rollback()` sem contexto. Não prometer interrupção de uma finalização já iniciada. O padrão de registrar rollback logo após `Begin` e tratar erro de confirmação segue as [orientações de transações do Go](https://go.dev/doc/database/execute-transactions).

Separar explicitamente os estados e resultados:

| Situação                                                                               | Comportamento esperado do Krios                                                                |
| -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Operação após encerramento                                                             | Retornar `ErrTxDone`, inclusive via `Row.Scan`                                                 |
| Contexto fornecido a `Commit` já cancelado, antes da chamada nativa, sem erro anterior | Retornar erro de contexto; não marcar encerramento apenas por essa rejeição                    |
| Primeiro erro de `ExecContext`/`QueryContext` já registrado                            | `Commit` tenta rollback, inclusive com contexto cancelado; preserva o erro original            |
| Falha de `Commit` ou `Rollback` depois de finalização nativa                           | Encerrar o uso do wrapper; preservar a causa da falha; chamadas seguintes retornam `ErrTxDone` |
| Cancelamento do contexto de `Begin`                                                    | Considerar a reversão automática nativa e normalizar o estado terminal quando observado        |
| Erro original e erro de rollback                                                       | Preservar ambos para inspeção por `errors.Is`/`errors.As`, usando encadeamento múltiplo        |
| Falha de confirmação com resultado remoto incerto                                      | Não afirmar sucesso, rollback garantido ou segurança de repetir a operação                     |

Manter o bloqueio de commit após erro imediato como proteção adicional já adotada. Não ampliar nesta correção a observação automática de falhas em `Scan`/iteração: o consumidor deve tratá-las e solicitar rollback. Ausência de linha e erro de conversão não receberão novas políticas implícitas. Documentar essa fronteira sem afirmar que todo grupo com qualquer falha será automaticamente revertido.

### 3.3. Concorrência

Manter `DB` utilizável por goroutines distintas e a serialização das chamadas de execução/finalização de uma mesma `Tx`. Uma corrida com encerramento não pode causar panic ou corrupção; uma operação iniciada antes de `Close` pode concluir ou falhar conforme o estado nativo. Não prometer ordem entre chamadas simultâneas.

Um resultado tem um consumidor responsável por `Next`/`Scan`; não prometer consumo concorrente do mesmo iterador. O mutex de `Tx` não cobre toda a vida útil de `Row`/`Rows`. O fluxo portátil deve consumir ou fechar resultados antes de iniciar outra operação na mesma transação e aguardar os consumidores antes de confirmar. Finalização com resultados ainda abertos deve ser testada, mas não anunciada como coordenação automática entre goroutines.

### 3.4. Configuração e compatibilidade

No MySQL, preservar a senha literal e aplicar `url.PathEscape` ao nome do banco. Essa distinção segue o [formato oficial do go-sql-driver/mysql](https://github.com/go-sql-driver/mysql#dsn-data-source-name). Testar os campos separadamente, incluindo `%`, espaços e endereços IPv6. Manter o formato URL do PostgreSQL com testes dos escapes e do host.

Manter `Config.Driver` explícito e documentar que o driver deve aceitar o formato gerado para `Type`; registro em `sql.Drivers()` não comprova compatibilidade. Validar também portas acima de 65535, ampliando de forma registrada a lista atual de configurações inválidas. Não aplicar trim ou normalização silenciosa a senhas e identificadores.

Documentar que o prazo de abertura e o cancelamento dependem da cooperação do driver durante operações bloqueantes. Aplicar o orçamento temporal antes da preparação do acesso e checar sua expiração nas fronteiras; não criar goroutines que abandonem conexões para simular cancelamento de drivers que não o suportam.

### 3.5. Contratos internos e organização

Foi criada uma interface interna `executor` satisfeita por `*DB` e `*Tx`, com retornos `Result`, `*Rows` e `Row`. Os futuros construtores devem passar por esses métodos para preservar as verificações e a política transacional. Se os helpers de delegação ainda precisarem do contrato nativo, nomeá-lo `sqlExecutor`; os dois contratos representam camadas distintas.

Manter `Result` com as assinaturas atuais `(int64, bool)`. Documentar que `false` significa informação indisponível e que o erro nativo desse acesso não é exposto. Testar zero válido, valor disponível e erro do driver. Uma mudança pública para `(int64, error)` fica fora desta correção por não ser necessária aos achados.

Organização revisada a pedido do consumidor após M06: `krios.go` na raiz mantém documentação e fachada pública (aliases e `Open`); implementação e testes internos ficam sob `internal/`, distribuídos em pacotes por responsabilidade (`internal/config`, `internal/errs`, `internal/entity`, `internal/access`, `internal/transaction`, `internal/execute`, `internal/scan` e `internal/result`). Arquivos pequenos podem ser combinados quando não houver benefício de leitura. Não criar subpacotes vazios.

## 4. Entregas de melhoria

| Entrega | Resultado                                       | Prioridade                   | Depende de    |
| ------- | ----------------------------------------------- | ---------------------------- | ------------- |
| M01     | Contratos vigentes e evidências reconciliados   | Alta                         | —             |
| M02     | Erros e resultados de leitura preservados       | Alta                         | M01           |
| M03     | Finalização e limpeza transacional consistentes | Alta                         | M02           |
| M04     | DSNs, validações e abertura verificados         | Média                        | M01           |
| M05     | Base interna e organização simplificadas        | Média                        | M02, M03, M04 |
| M06     | Exemplos, verificações e passagem para E04      | Alta para concluir a revisão | M01–M05       |

Sequência recomendada: M01 → M02 → M03 → M04 → M05 → M06. M04 é tecnicamente independente das correções de leitura/transação após M01.

### M01 — Reconciliar documentação e contratos

- [x] **M01-P01 — Registrar o contrato vigente e as revisões planejadas.** Atualizar E02-P01 e E03-P01 com as decisões da seção 3, identificando a mudança de rollback e a prioridade da limpeza após erro anterior. Corrigir `Rows` para `*Rows` nas assinaturas. Identificar trechos superados sem apagar a rastreabilidade.
- [x] **M01-P02 — Reconciliar o progresso.** Em `0001_planning.md`, vincular perguntas já resolvidas às decisões. Em `0002_execution_plan.md`, distinguir implementação, evidência automatizada e evidência manual pendente. Registrar E02/E03 como implementadas com correções em andamento; não declarar integração dos três bancos concluída. Remover instruções obsoletas de organização e promessas amplas de atomicidade.

**Critério de conclusão:** há uma única descrição vigente por contrato; implementação ainda pendente está identificada como tal; cada pendência aponta à entrega responsável. O sucesso histórico de testes não é apresentado como prova de correções futuras.

### M02 — Corrigir erros e resultados de leitura

- [x] **M02-P01 — Preservar erros em `Row`.** Implementar o erro pendente e a propagação de contexto, `ErrClosed`, `ErrTxDone` e causas do driver. Corrigir o teste que espera `sql.ErrNoRows` depois do encerramento.
- [x] **M02-P02 — Preservar o diagnóstico de `Rows`.** Tornar `Err` estável após fechamento, manter a idempotência e remover mutações/sincronização redundantes que não sejam necessárias ao contrato. Verificar erros na leitura, fechamento e conversão sem confundi-los.
- [x] **M02-P03 — Demonstrar os contratos públicos.** Testar DB e Tx: consulta vazia, erro de SQL/driver, contexto previamente cancelado/expirado, encerramento e erro adiado até `Scan`. Para `Rows`, testar consumo parcial, erro depois de uma linha, fechamento explícito e automático, `Err` após `Close` e fechamento repetido. Verificar também os três estados de disponibilidade de `Result` definidos em §3.5.

**Critério de conclusão:** os testes distinguem ausência de dados, erro operacional e estado inválido; não há sucesso aparente causado pela perda de um erro. Os testes de regressão demonstram o comportamento incorreto anterior.

### M03 — Corrigir finalização, limpeza e limites transacionais

- [x] **M03-P01 — Sincronizar o estado terminal.** Tratar falhas de confirmação/reversão e término automático pelo contexto de `Begin`. Normalizar `sql.ErrTxDone` nas fronteiras públicas, inclusive erros adiados. Preservar a causa da primeira falha de finalização e rejeitar reutilização do objeto.
- [x] **M03-P02 — Garantir tentativa de limpeza.** Aplicar o comportamento recomendado de `Rollback(ctx)` e do commit bloqueado, sem impedir rollback por cancelamento. Preservar simultaneamente o erro original e eventual falha de limpeza. Manter o registro do primeiro erro quando houver erros subsequentes.
- [x] **M03-P03 — Validar estados e concorrência.** Usar driver simulado com contadores e pontos de sincronização para provar quais chamadas chegaram ao driver. Cobrir finalizações que falham, contexto cancelado antes de `Commit`, rollback com contexto cancelado, cancelamento de `Begin`, erro de statement seguido de commit com contexto cancelado, falha dupla, chamadas repetidas e resultados abertos. Manter os cenários SQLite de persistência, reversão e independência do DB. Testar explicitamente que erro de leitura exige rollback do consumidor.

**Critério de conclusão:** todas as transições da tabela de §3.2 têm evidência; nenhuma tentativa de limpeza é descartada apenas pelo cancelamento fornecido; o wrapper não trata uma transação terminal como reutilizável. Testes concorrentes usam sincronização determinística e passam com `-race`, sem depender de sleeps para acertar uma corrida.

### M04 — Verificar configuração, DSN e abertura

- [x] **M04-P01 — Corrigir DSN e limites.** Escapar `DBName` do MySQL, corrigir a orientação sobre senha e validar o intervalo de portas. Exercitar portas padrão/limites, IPv4/IPv6 e caracteres especiais nas configurações MySQL/PostgreSQL. Evitar imprimir credenciais nos diagnósticos dos testes.
- [x] **M04-P02 — Validar o orçamento de abertura e a liberação.** Verificar o posicionamento do timeout em `Open`, cancelamento durante `Ping`, prevalência do menor prazo e fechamento dos recursos criados quando o ping falha. Usar driver que respeite contexto e conte aberturas/fechamentos; o driver atual que falha antes de criar conexão não comprova essa limpeza. Documentar o limite de drivers não cooperativos.
- [x] **M04-P03 — Separar evidências de compatibilidade.** Distinguir teste do DSN, teste de propagação de falha simulada e conexão real. Documentar formatos suportados e manter integração real MySQL/PostgreSQL pendente. Incluir no roteiro manual credenciais com caracteres especiais, banco com identificador especial e IPv6 quando o ambiente o disponibilizar.

**Critério de conclusão:** a configuração produz DSNs corretos para os formatos declarados, rejeita portas fora do intervalo e tem testes observáveis de prazo e limpeza. Não se declara compatibilidade real com base apenas em driver simulado.

### M05 — Simplificar a estrutura sem mudar os contratos corrigidos

- [x] **M05-P01 — Separar os executores.** Criar o contrato interno de Krios e, se necessário, renomear o contrato nativo para `sqlExecutor`. Adicionar verificações de implementação em compilação para `*DB` e `*Tx`. Demonstrar em teste que a execução pelo contrato de Krios preserva estados e bloqueio de commit.
- [x] **M05-P02 — Remover estado e auxiliares sem uso.** Remover `Tx.cfg` e revisar a necessidade de `DB.cfg`; armazenar somente informação utilizada. Remover `sqliteValidNoMigrations`, `tTempDir` e helpers redundantes. Consolidar casos duplicados sem perder cenários distintos. Registrar limpeza de transações nos testes para também liberar recursos quando uma asserção falhar.
- [x] **M05-P03 — Organizar por responsabilidade.** Distribuir o código conforme §3.5 e agrupar testes por comportamento. Atualizar a revisão de organização em E01-P01 e o mapa de arquivos no README. Manter os testes de detalhes internos junto ao pacote e os drivers nos arquivos de teste.

**Critério de conclusão:** os métodos públicos continuam compatíveis, a refatoração passa pela mesma suíte de regressão e a inspeção das dependências compiladas da biblioteca confirma ausência de drivers. A alteração estrutural é revisável separadamente das correções funcionais.

### M06 — Consolidar validação e preparar E04

- [x] **M06-P01 — Publicar exemplos verificáveis no repositório.** Criar exemplos executados por `go test` para abertura, consulta, tratamento de `ErrNoRows`, checagem de `Rows.Err`, rollback registrado após `Begin`, commit bloqueado e erro de leitura tratado pelo consumidor. Usar import externo de Krios e SQLite temporário nos exemplos de integração. Identificar `cmd/krios` como placeholder enquanto apenas imprimir uma saudação, em vez de apresentá-lo como exemplo funcional da biblioteca.
- [x] **M06-P02 — Tornar as verificações reproduzíveis.** Executar `make check` e `go test -race -count=1 ./...`. Criar alvo explícito para race e execução dedicada no CI, mantendo o job normal sem CGO e habilitando CGO/toolchain C no job do detector, conforme seus [requisitos oficiais](https://go.dev/doc/articles/race_detector#Requirements). Preservar a restrição a repositórios públicos. Verificar as linhas Go 1.26 e 1.27; registrar separadamente resultado local e remoto, sem atribuir sucesso remoto antes da execução.
- [x] **M06-P03 — Fechar a rastreabilidade.** Relacionar cada achado à correção, teste e revisão correspondente. Atualizar progresso, README e decisões com os resultados obtidos. Preparar roteiro manual para MySQL/PostgreSQL contendo versões, configuração sem segredos, passos, resultado esperado/observado e limitações. Transferir explicitamente a execução e coleta dessas evidências para E09.

**Critério de conclusão:** os exemplos exercitam a API corrigida, as verificações disponíveis passaram e as demais têm estado explícito. A aprovação da revisão depende das regressões e dos contratos, não de uma meta arbitrária de cobertura percentual.

## 5. Passagem para E04 e validações posteriores

E04 pode iniciar sua implementação quando M01–M05 e os exemplos/regressões de M06 estiverem concluídos e validados localmente nas linhas previstas. Uma execução remota ainda não realizada deve continuar pendente; não equivale a falha nem a sucesso. Falhas remotas conhecidas precisam ser corrigidas antes da passagem.

A integração real com MySQL/PostgreSQL permanece obrigatória antes de concluir E09. O roteiro deve cobrir conexão, parâmetros especiais, confirmação/reversão, falha de statement, resultados abertos, contexto e independência das operações externas. Registrar resultado incerto de confirmação como tal; não adotar repetição automática de escrita.

Na especificação de E04 foi adotado o mapeamento por reflexão sobre structs ([E04-P01_decisions.md](E04-P01_decisions.md)). Ainda assim, permanece em avaliação a necessidade de metadados de colunas (`Columns`/`ColumnTypes`) para SQL arbitrário e aliases. Essa decisão deve partir dos casos de uso de E04/E07; não adicionar métodos públicos antecipadamente nesta revisão.

## 6. Registro de progresso

| Entrega | Estado               | Evidência                                                                                                                                                                                                          |
| ------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| M01     | Concluída localmente | Contratos vigentes em E02/E03, organização em E01, progresso e pendências reconciliados em 0001/0002.                                                                                                              |
| M02     | Concluída localmente | [rows_test.go](../internal/scan/rows_test.go): regressões falharam antes da correção; erros, recursos e Result validados após a correção.                                                                          |
| M03     | Concluída localmente | [tx_test.go](../internal/transaction/tx_test.go), [execution_test.go](../internal/access/execution_test.go) e integração SQLite: limpeza, estado terminal, erro duplo, cancelamento e concorrência.                |
| M04     | Concluída localmente | [dsn_test.go](../internal/config/dsn_test.go), [db_test.go](../internal/access/db_test.go): escapes, portas, orçamento incluindo preparação e liberação no ping.                                                   |
| M05     | Concluída localmente | Correções testadas antes da divisão em arquivos; suíte preservada depois. `go list -deps ./...` confirmou drivers ausentes da compilação normal.                                                                   |
| M06     | Concluída localmente | [Exemplos](../tests/examples_test.go), check/race locais em Go 1.26.8 e 1.27.1, [workflow](../.github/workflows/check.yml) e [roteiro E09](E09_manual_validation.md). Execução remota e integração real pendentes. |

Ao concluir cada entrega, registrar a revisão/commit, os testes executados, a versão de Go e qualquer limitação remanescente. A revisão parcial original é a motivação do plano, não evidência de que suas melhorias foram implementadas.

## 7. Evidências da execução de M01–M06

Revisão local de 2026-10-19, antes de commit/publicação. Após M06, a pedido do consumidor, implementação e testes internos foram movidos para `internal/`; os links abaixo apontam para os arquivos nos novos pacotes. A fachada pública e o import do consumidor foram preservados. Os caminhos abaixo identificam a implementação e seus testes; não foi criada uma versão publicada.

| Verificação                                 | Go 1.26.8 | Go 1.27.1                                    |
| ------------------------------------------- | --------- | -------------------------------------------- |
| `make check CGO_ENABLED=0`                  | Passou    | Passou, selecionado com `GO=/caminho/bin/go` |
| `make race`                                 | Passou    | Passou, selecionado com `GO=/caminho/bin/go` |
| Exemplos compilados e executados pela suíte | Passou    | Passou                                       |

Antes das correções, a suíte nova falhou nos erros de `QueryRowContext`, preservação de `Rows.Err`, finalização transacional, rollback cancelado, preservação do erro duplo, porta 65536, DSN MySQL e orçamento de preparação. Depois das correções, a suíte passou; a divisão física em arquivos ocorreu posteriormente e foi validada novamente nas duas linhas Go.

| Achado ou melhoria                                | Implementação                                        | Evidência principal                                                                                                                                                                                                        |
| ------------------------------------------------- | ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| QueryRow mascarava falhas                         | `rows.go`, `execution.go`, `db.go`, `tx.go`          | `TestQueryRowErrors`, correção de `TestTxRejectsAllMethodsAfterDone`                                                                                                                                                       |
| Close apagava erro de leitura                     | `rows.go`                                            | `TestRowsLifecycle`                                                                                                                                                                                                        |
| Estado divergente após falha de finalização       | `tx.go`, `errors.go`                                 | `TestTxFinalizationFailureIsTerminal`, `TestBeginCancellationNormalizesNativeTerminalState`                                                                                                                                |
| Cancelamento impedia limpeza                      | `tx.go`                                              | `TestRollbackIgnoresCanceledContext`, `TestBlockedCommitPreservesBothErrors`                                                                                                                                               |
| Política de erros tardios e concorrência          | Contrato E03 e `tx.go`                               | `TestLateReadFailureBlocksCommit` (renomeado de `...RequiresConsumerRollback` em E03-P04), `TestErrNoRowsDoesNotBlockCommit`, `TestFinalizationClosesOpenResults`, `TestConcurrentFinalizationWaitsForExecution`, exemplos |
| DSN e limite de portas                            | `dsn.go`, `config.go`                                | `TestMySQLDSN`, `TestPostgresDSN`, `TestConnectionPortRange`                                                                                                                                                               |
| Timeout só começava após preparação               | `db.go`                                              | `TestOpenBudgetIncludesDriverPreparation`, `TestOpenUsesShortestDeadline`                                                                                                                                                  |
| Falta de evidência de liberação/cancelamento      | `db.go`, contrato executor                           | `TestOpenReleasesConnectionOnPingFailure`, `TestOpenCancellationDuringPing`, `TestExecutionCancellationInFlight`                                                                                                           |
| Executor inadequado para consumidores internos    | `execution.go`                                       | Asserções em compilação e `TestExecutorPreservesPublicGuards`                                                                                                                                                              |
| Estado, helpers e organização redundantes         | `db.go`, `tx.go`, arquivos separados e testes limpos | Suíte após refatoração e dependências compiladas somente da biblioteca padrão                                                                                                                                              |
| Documentação e progresso excessivamente otimistas | E01/E02/E03, 0001/0002, README                       | E02-P04 reaberto como parcial; formatos, histórico e limitações explícitos                                                                                                                                                 |
| Exemplos e automação                              | `tests/examples_test.go`, Makefile, workflow         | Exemplos reais SQLite e check/race locais; novo job remoto ainda não executado                                                                                                                                             |

A verificação de dependências foi feita com `go list -deps ./...`: os pacotes de produção usam somente a biblioteca padrão além dos próprios pacotes Krios. Os drivers reais continuam nos testes. Links locais da documentação e sintaxe dos exemplos completos de Go em Markdown foram verificados; o consumidor manual não foi executado contra servidores.

**Pendências externas explícitas:** executar o workflow atualizado após envio ao GitHub; executar e registrar o roteiro E09 em MySQL/PostgreSQL. Elas não impedem o avanço local para E04, mas integração real permanece obrigatória para encerrar E09. Recursos de E04–E08 não foram antecipados nesta revisão.
