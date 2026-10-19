# Krios — Plano de execução e entregas

## 1. Objetivo

Organizar o desenvolvimento em passos verificáveis, a partir dos requisitos de [0001_planning.md](0001_planning.md). Este plano define a ordem de trabalho, as entregas e seus critérios de conclusão. Não inicia a implementação nem resolve antecipadamente as decisões abertas no planejamento.

As pendências serão resolvidas conforme a entrega que depende delas for iniciada. Não é necessário fechar toda a seção 6 do planejamento para começar o projeto.

## 2. Como acompanhar a execução

- Cada passo começa com uma caixa de seleção e um identificador estável, como `E02-P01`.
- Marcar um passo como concluído somente quando seu resultado estiver implementado ou documentado e verificado, conforme a natureza do passo.
- Os comentários **Pendências** abaixo dos passos indicam decisões necessárias para executar ou concluir aquele trabalho. As referências `§6.x` apontam para a seção correspondente de `0001_planning.md`.
- Ao resolver uma pendência, registrar a decisão no planejamento ou em uma especificação vinculada e atualizar o comentário com a referência. Não apagar a rastreabilidade da decisão.
- Uma pendência deve bloquear somente o trabalho que depende dela; outros passos independentes podem avançar.
- Cada entrega inclui exemplos de uso e verificações do comportamento que introduz. A entrega final consolida essa validação, sem adiar todos os testes para o fim.
- As entregas representam marcos de desenvolvimento; não exigem publicação de uma versão a cada marco. Datas e estimativas serão definidas conforme o trabalho avançar.

### Estratégia de validação

Os testes automatizados com banco usarão inicialmente apenas SQLite3, com o driver `modernc.org/sqlite`. MySQL e PostgreSQL permanecem no escopo da biblioteca, mas seus cenários de integração serão executados manualmente pelo consumidor posteriormente. Onde uma entrega exige validação nos três bancos, a evidência será automatizada para SQLite e manual para os demais; a ausência dessa evidência deve permanecer registrada até a validação final em E09.

Também serão construídos testes unitários para as regras da biblioteca e a geração de SQL de cada dialeto, sem exigir instâncias de MySQL ou PostgreSQL. Esses testes não substituem a execução manual contra os bancos reais. Os lembretes para essa cobertura estão ao final deste documento.

### Revisão parcial da base E01–E03

**Organização vigente após M06:** fachada pública em `krios.go`; implementação e testes internos sob `internal/`; testes de integração e exemplos em `tests/`. Mudança solicitada pelo consumidor, preservando o import `github.com/mineot/krios`. Ver [revisão de organização](E01-P01_decisions.md).

A revisão de 2026-10-19 identificou correções de erros, ciclo de vida e DSN, além de ajustes de contratos, testes e documentação. O [plano de melhorias da base](0003_improvement_plan.md) organiza esse trabalho em M01–M06, com critérios para iniciar E04. M01–M06 foram implementados e validados localmente nas linhas Go 1.26/1.27. A execução remota do novo CI permanece pendente; integração real MySQL/PostgreSQL permanece em E09. Os registros datados das entregas originais abaixo são históricos; os contratos vigentes estão nas decisões revisadas e as novas evidências no plano de melhorias.

## 3. Visão geral das entregas

| Entrega | Resultado                                       | Depende de | Requisitos principais              |
| ------- | ----------------------------------------------- | ---------- | ---------------------------------- |
| E01     | Base do módulo e ambiente de desenvolvimento    | —          | Diretrizes técnicas e distribuição |
| E02     | Configuração e acesso aos três bancos           | E01        | RF-01 e RF-02                      |
| E03     | Execução básica e transações explícitas         | E02        | RF-03; base de RF-08               |
| E04     | Entidades e representação de resultados         | E03        | RF-09                              |
| E05     | Consultas de leitura encadeadas e paginação     | E04        | RF-06                              |
| E06     | Operações de escrita por entidade               | E05        | RF-07                              |
| E07     | Execução de arquivos SQL incorporados           | E06        | RF-08                              |
| E08     | Migrações versionadas de avanço e reversão      | E07        | RF-04 e RF-05                      |
| E09     | Validação completa, documentação e distribuição | E08        | Todos os critérios de atendimento  |

A ordem acima é a sequência inicial recomendada. Por exemplo, E07 pode antecipar trabalho após E04, pois precisa da execução, das transações e do mapeamento, mas não dos construtores de escrita. E08 pode avançar após E07 sem esperar funcionalidades independentes dos construtores. Qualquer ajuste deve preservar as dependências técnicas e os critérios de conclusão.

## 4. Passos de desenvolvimento

### E01 — Preparar a base do projeto

**Entrega:** módulo compilável, organização inicial documentada e rotina de verificação reproduzível.

- [x] **E01-P01 — Definir a base técnica mínima.** Registrar o caminho do módulo, a versão mínima de Go e a organização inicial das responsabilidades: configuração, conexão, execução, transações, dialetos, entidades, consultas e migrações. A divisão em pacotes deve acompanhar essas responsabilidades sem antecipar toda a API.

  > **Decisão registrada:** adotar Go 1.26 como versão mínima, declarando `go 1.26.0` no `go.mod`. Para desenvolvimento, utilizar a atualização mais recente da linha Go 1.27. Os testes automatizados devem executar nas atualizações mais recentes das linhas 1.26 e 1.27, mantendo o código compatível com a versão mínima declarada. Esta base foi escolhida em outubro de 2026 para equilibrar suporte e compatibilidade; não é uma exigência específica de `krios.Config` ou de SQL incorporado.

  > **Escopo concluído nesta etapa:** caminho do módulo (§6.9), pacote público de entrada e divisão inicial entre API pública e implementação interna (§6.1), conforme o registro de decisões abaixo. Campos, métodos e assinaturas continuam nas entregas correspondentes.

  > **Organização definida:** usar `github.com/mineot/krios` como caminho do módulo e `krios` como pacote público na raiz. Organizar as responsabilidades em arquivos separados e expor apenas os tipos e operações necessários ao consumidor: configuração, abertura e encerramento, transações, entidades, construtores, resultados e configuração/execução de migrações. Manter validação interna, montagem de SQL, adaptação de dialetos e controle de execução como detalhes internos. Manter ou criar subpacotes internos somente quando houver uma separação concreta que os justifique. Essa divisão não define antecipadamente as assinaturas nem decide se haverá extensões públicas de dialetos.

  > **Revisão (2026-10-18):** a orientação de "arquivos separados" foi revista para consolidar o código da fase inicial em um único `krios.go` na raiz, mantendo o import `github.com/mineot/krios`. Justificativa e escopo em [E01-P01_decisions.md](E01-P01_decisions.md#revisão-2026-10-18).

  > **Adiado para as entregas correspondentes:** seleção de drivers, ambientes de teste e compatibilidade com Go 1.26 em E01-P03; campos de `krios.Config`, assinaturas de `Open`/`Close`, cancelamento e erros em E02-P01; execução e transações em E03; entidades e resultados em E04; expressões e interfaces de dialetos em E05-P01; valores e retornos de escrita em E06-P01; contrato de SQL incorporado em E07-P01; representação das migrações e recuperação em E08. Cada decisão deve ser resolvida antes da implementação que depende dela.

  > **Adiado para E09-P03:** número da primeira versão, política de compatibilidade da API e procedimento de publicação. Essas decisões não bloqueiam a definição da base técnica nem a inicialização do módulo.

  > **Decisão registrada (2026-10-18):** módulo `github.com/mineot/krios` (confirmado pelo remote), pacote `krios` público na raiz e `go 1.26.0` no `go.mod`. A pasta `internal/` com pacotes apenas documentais foi removida; subpacotes serão criados conforme a necessidade concreta da implementação. O mecanismo de mapeamento, incluindo eventual uso de reflexão, permanece para E04. Detalhes em [E01-P01_decisions.md](E01-P01_decisions.md).

- [x] **E01-P02 — Inicializar o módulo e as rotinas locais.** Criar `go.mod`, estrutura inicial, instruções de desenvolvimento e comandos para formatação, compilação, análise estática e testes. Preservar a licença e os documentos existentes.

  > **Base utilizada:** reutilizados o `go.mod` e o pacote `krios` existentes, conforme E01-P01. Não foram criados pacotes adicionais nem implementadas funcionalidades previstas para outras entregas.

  > **Concluído (2026-10-18):** [Makefile](../Makefile) com `fmt`, `fmt-check`, `build`, `vet`, `test` e `check`; [README.md](../README.md) com requisitos, comandos equivalentes sem Make, organização e estado atual da biblioteca. Licença preservada. `make check` passou com Go 1.26.8; ainda não há testes de funcionalidades. Em diretório temporário, foi verificado que a checagem rejeita formatação inválida sem modificar arquivos, que `make fmt` corrige o código e que o alvo padrão executa as verificações. A matriz Go 1.26/1.27 e os ambientes de banco permanecem em E01-P03.

- [x] **E01-P03 — Preparar a validação com SQLite.** Preparar o driver dos testes, documentar o banco local e configurar a execução automatizada das verificações básicas do projeto com SQLite. Os ambientes de MySQL e PostgreSQL ficam para a validação manual posterior.

  > **Decisões registradas:** usar `modernc.org/sqlite` nos testes automatizados com banco. Selecionar a versão estável mais recente disponível na preparação do ambiente e registrar a versão exata do driver e do SQLite incorporado. O banco de desenvolvimento será `dev.db`, na raiz, ignorado pelo Git junto a seus arquivos auxiliares. O arquivo será criado ao abrir o banco pelo driver. Drivers de testes e exemplos não devem se tornar imports automáticos da biblioteca.

  > **Reprodutibilidade:** o SQLite utilizado é o incorporado pelo driver, não uma instalação independente do sistema. Usar a versão de SQLite incorporada pela versão estável selecionada de `modernc.org/sqlite`, mesmo que seja diferente da versão upstream mais recente, conforme aceitação do consumidor em 18/10/2026. Conferir a compatibilidade das atualizações com Go 1.26. Fixar a versão selecionada no módulo e atualizar deliberadamente, em vez de baixar uma versão diferente em cada execução.

  > **Isolamento:** reservar `dev.db` para desenvolvimento local. Os testes automatizados devem usar bancos temporários próprios, sem depender dos dados existentes nem apagar o banco de desenvolvimento. A integração contínua usará SQLite e a matriz Go 1.26/1.27, sem servidores MySQL ou PostgreSQL nesta etapa.

  > **Implementação local (2026-10-18):** `modernc.org/sqlite v1.60.1`, resolvido por `@latest`, fixado em `go.mod` e importado exclusivamente em [tests/sqlite_test.go](../tests/sqlite_test.go). SQLite incorporado: 3.53.4, confirmado em execução. `make dev-db` prepara o banco local sem reinicializar dados; testes normais usam bancos temporários e não acessam `dev.db`. `make check` passou com Go 1.26.8 e 1.27.2, sem CGO, e a inspeção das dependências confirmou ausência do driver na compilação normal. O exemplo executável foi movido para `cmd/krios`, mantendo o pacote público `krios` na raiz.

  > **Integração contínua definida:** GitHub Actions em pushes e pull requests, com matriz Go 1.26.x/1.27.x e runner padrão Ubuntu. O [workflow](../.github/workflows/check.yml) executa somente em repositórios públicos, respeitando a condição de gratuidade; em privados, os jobs são ignorados. Cache persistente e publicação de artefatos estão desabilitados. Não foram alteradas configurações de cobrança ou visibilidade.

  > **Concluído (2026-10-18):** o consumidor confirmou que realizou o push e que os checks do GitHub Actions passaram nas linhas Go 1.26 e 1.27. Também aceitou o SQLite 3.53.4 incorporado pelo driver selecionado, encerrando a pendência de versão. A evidência da execução remota é o relato do consumidor; não foi realizada nova consulta ao GitHub nesta atualização. Não restam pendências desta tarefa.

**Critério de conclusão:** o módulo compila, as verificações básicas e os testes com SQLite podem ser executados localmente e na integração contínua, e o uso de `dev.db` está documentado. A validação manual de MySQL e PostgreSQL permanece registrada como trabalho posterior.

### E02 — Configurar e abrir o acesso aos bancos

**Entrega:** consumidor configura o banco por `krios.Config`, sem arquivo externo de configuração, importa seu driver e consegue abrir e encerrar o acesso.

- [x] **E02-P01 — Especificar a configuração e o ciclo de vida.** Definir os campos e tipos da struct pública `krios.Config`, sua passagem à biblioteca, as assinaturas de `Open` e `Close`, o cancelamento e os erros relevantes. Produzir um exemplo mínimo de uso antes de implementar o contrato.

  > **Questões da entrega original, resolvidas nas decisões revisadas:** definir organização e tipos dos campos de `krios.Config`, valores omitidos e explícitos, padrões, validações, unidades de tempo, cancelamento e erros (§6.1). Definir como identificar o driver importado e sua relação com o banco (§6.2).

  > **Decisões registradas (2026-10-18) em [E02-P01_decisions.md](E02-P01_decisions.md):** organização e tipos de `krios.Config`, incluindo campo obrigatório `Driver` e structs por banco (§6.1); valores omitidos por zero (`Port == 0`, `Timeout == 0`) e valores negativos inválidos; `Timeout` default de 10 s e precedência = menor prazo entre `ctx` e `Timeout`, com `context.WithTimeout` interno (§6.1); contrato de erros com `ErrInvalidConfig`, `ErrDriverNotLoaded`, `ErrConnectionFailed`, erros de contexto e preservação do original por `%w` (§6.1); lista fechada de validações (§6.1); assinaturas de `Open(ctx, Config)` e `Close` idempotente, com exemplos mínimos para os três bancos. Identificação do driver por `sql.Drivers()` e formatos de DSN documentados por banco (§6.2). A interface dos dialetos permanece adiada para E05-P01. Nenhuma pendência restante desta tarefa.

- [x] **E02-P02 — Implementar o fluxo de configuração.** Receber `krios.Config` preenchida pelo consumidor, aplicar os padrões, validar os dados de cada banco e disponibilizar um exemplo de configuração em Go com valores ilustrativos, sem credenciais reais. Manter a configuração de migrações opcional.

  > **Questões da entrega original, resolvidas nas decisões revisadas:** aplicar E02-P01. Reservar a integração da configuração de versões e arquivos de migração para E08; ela não deve impedir o uso de conexões nesta entrega.

  > **Implementado (2026-10-18):** [krios.go](../krios.go) com `DBType`, `SQLiteConfig`, `MySQLConfig`, `PostgresConfig`, `MigrationsConfig` (reservado), `krios.Config`, os sentinelas de erro, os padrões (`Port` 3306/5432, `Timeout` 10 s) e a validação fechada de E02-P01 que retorna `ErrInvalidConfig`. Código consolidado em arquivo único conforme a revisão registrada em [E01-P01_decisions.md](E01-P01_decisions.md#revisão-2026-10-18); em E02-P03, `Open` e `Close` serão adicionados a `krios.go`. Os testes do pacote foram concentrados em [krios_test.go](../internal/config/config_test.go) (`package krios`), com o exemplo de configuração embutido; a decisão está registrada em [E01-P01_decisions.md](E01-P01_decisions.md#revisão-2026-10-18--testes). `make check` passou (Go 1.26.8). A verificação do driver e a montagem dos DSNs ficam em E02-P03.

- [x] **E02-P03 — Implementar `Open` e `Close`.** Integrar `database/sql` ao driver informado, verificar conectividade, respeitar cancelamento e tempo de espera e liberar recursos criados quando a abertura falhar.

  > **Resolvido:** `Close` idempotente e operações posteriores com `ErrClosed`, inclusive por `Row.Scan`. Ver o contrato vigente em [E02-P01_decisions.md](E02-P01_decisions.md).

  > **Implementado (2026-10-18):** `Open` e `Close` adicionados a [krios.go](../krios.go), com `DB` protegido por mutex. `Open` valida a configuração, confere o driver em `sql.Drivers()` (`ErrDriverNotLoaded`), verifica `ctx.Err()`, abre via `sql.Open` com DSN montado por banco, faz `PingContext` com `context.WithTimeout(ctx, Timeout)` e fecha a conexão em caso de falha. Cancelamento/prazo retornam `context.Canceled`/`context.DeadlineExceeded` sem `ErrConnectionFailed`. Operações após `Close` retornam `ErrClosed` (novo sentinela). Detalhes no adendo de [E02-P01_decisions.md](E02-P01_decisions.md#adendo--e02-p03-2026-10-18). Testes de caixa-preta em [tests/lifecycle_test.go](../tests/lifecycle_test.go) e guard em [krios_test.go](../internal/config/config_test.go). `make check` passou. A validação dos três bancos (E02-P04) permanece.

- [ ] **E02-P04 — Validar o ciclo de vida nos três bancos.** Cobrir configuração válida e inválida, driver indisponível, falha de conexão, cancelamento, prazo esgotado e encerramento. Demonstrar que abrir o banco não exige configurar nem executar migrações e que a configuração em código funciona sem arquivos externos ou variáveis de ambiente obrigatórias.

  > **Evidência parcial (2026-10-18; escopo corrigido em M01):** validado SQLite com sucesso (abertura/encerramento idempotente), e exercitados MySQL e PostgreSQL por meio de configuração inválida e falha de conexão com driver `krios_fail`. Cobertura adicional em [tests/lifecycle_three_test.go](../tests/lifecycle_three_test.go): casos inválidos por banco, `ErrConnectionFailed` para MySQL/Postgres, `Open` sem migrações (`Migrations` nil). As verificações automatizadas usam apenas SQLite para integração positiva; MySQL/Postgres são validados por falhas controladas. `make check` passou (Go 1.26.8).

**Critério de conclusão:** exemplos de consumidores com `krios.Config` definida em código e drivers explicitamente importados funcionam em SQLite3, MySQL e PostgreSQL, sem arquivos externos de configuração; falhas de abertura são retornadas e não deixam recursos associados à tentativa abertos.

### E03 — Criar a execução básica e as transações

**Entrega:** operações podem ser executadas no acesso principal ou em uma transação explicitamente escolhida.

- [x] **E03-P01 — Definir os contratos de execução.** Distinguir comandos de escrita e consultas com linhas, especificando parâmetros, resultados, erros e encerramento dos recursos de leitura. Criar a base compartilhada que será utilizada por construtores e arquivos SQL.

  > **Questões da entrega original, resolvidas nas decisões revisadas:** definir tipos expostos de `database/sql`, assinaturas de execução, propriedade dos recursos de leitura (§6.8) e a associação dos executores à conexão ou à transação (§6.3). A exposição pública de SQL em texto não é um requisito adicional desta etapa.

  > **Decisões (2026-10-19) em [E03-P01_decisions.md](E03-P01_decisions.md):** `Executor` mantido **interno** (§4, pergunta 1), `krios.Result` definido como **interface** com `RowsAffected()` e `LastInsertId()` (§4, pergunta 2), `Rows` com `Close()` obrigatório (§4, pergunta 3), serialização de `*Tx` com `sync.Mutex` (§4, pergunta 4), `Commit`/`Rollback` aceitam `context.Context` validando `ctx.Err()` (§4, pergunta 5) e `sql.ErrNoRows` preservado em `Row.Scan` (§4, pergunta 6). Todos os contratos especificados nas §3.1–§3.4.

- [x] **E03-P02 — Implementar `Begin`, `Commit` e `Rollback`.** Criar o objeto de transação e a execução vinculada a ele, mantendo operações do acesso principal independentes. Rejeitar novas operações em uma transação encerrada.

  > **Questões da entrega original, resolvidas nas decisões revisadas:** definir concorrência no mesmo objeto, responsabilidade pelo rollback após erro e tratamento de falhas em confirmação ou reversão (§6.3).

  > **Histórico E03-P02 (superado nos pontos de erro/finalização por M02–M03):** em `krios.go` (implementação posteriormente distribuída por responsabilidade) — sentinela `ErrTxDone`; tipos `Result` (interface), `Row` e `Rows` com `Close()` idempotente; interface interna `executor`; método `Begin(ctx) (*Tx, error)` via `BeginTx`; `Commit(ctx)`/`Rollback(ctx)` validando `done` e `ctx.Err()` antes da chamada nativa e marcando `done` apenas após sucesso; métodos `ExecContext`/`QueryContext`/`QueryRowContext` em `*DB` e `*Tx`, com serialização por `sync.Mutex` no mesmo `*Tx` (perguntas 1–6 de E03-P01 aplicadas). Não há rollback automático após falha de execução: `Commit`/`Rollback` são explícitos (§3.3 de E03-P01) e a responsabilidade pelo rollback do grupo permanece como pendência de §6.3 para validação em E03-P03. Falhas em confirmação/reversão repassam o erro nativo do driver. Testes em [tests/tx_test.go](../tests/tx_test.go): commit persiste, rollback descarta, `ErrTxDone` após encerrar, contexto cancelado em `Begin`/`Commit`, `ErrClosed` em DB fechado, `sql.ErrNoRows` em `QueryRow` e idempotência de `Rows.Close()`. `make check` passou (Go 1.26.8).

- [x] **E03-P03 — Validar os limites da transação.** Demonstrar três inserts relacionados confirmados em conjunto e a reversão do grupo quando um deles falha. Cobrir operações fora do grupo, transações encerradas e o comportamento concorrente definido no contrato.

  > **Decisão registrada (2026-10-19) em [E03-P01_decisions.md](E03-P01_decisions.md#345-falha-de-statement-e-commit-decidido-em-e03-p03):** **Commit bloqueado com auto-rollback** (§3.4.5, respondendo a pendência de §6.3 sobre quem inicia o rollback após falha): `*Tx` memoriza o primeiro erro de `ExecContext`/`QueryContext`; `Commit` com erro memorizado executa rollback, encerra a transação e retorna o erro original por `%w` (`errors.Is`). ~~Erros que só aparecem em `Row.Scan` ou na iteração de `Rows` não bloqueiam `Commit`~~ — a limitação foi superada em E03-P04 (§3.4.6 de [E03-P01_decisions.md](E03-P01_decisions.md)).

  > **Implementado e validado (2026-10-19):** campo `err` em `Tx` com registro do primeiro erro e novo `Commit` com auto-rollback em [krios.go](../krios.go). Testes em [tests/tx_group_test.go](../tests/tx_group_test.go): grupo de 3 inserts confirmado em conjunto; reversão do grupo por falha (rollback explícito e commit bloqueado com `errors.Is` do erro original + `ErrTxDone` após auto-rollback); operações fora do grupo independentes (antes/depois); leituras da instância principal não enxergam dados não confirmados (RF-03); rejeição de todos os métodos após `Commit` e após `Rollback`; chamadas concorrentes serializadas na mesma `*Tx`; `*DB` e `*Tx` distintos em leituras concorrentes. `make check` passou (Go 1.26.8) e `go test -race -count=2 ./...` passou como evidência local de detecção de corrida (sem alterar Makefile/CI). Escrita concorrente entre conexões distintas e escrita externa com `Tx` aberta em SQLite ficam para a validação manual em MySQL/PostgreSQL (pendente para E09). README atualizado com o estado das transações.

  > **Revisão (E03-P04):** o escopo de bloqueio de `Commit` foi estendido a erros de leitura tardios; ver [E03-P01_decisions.md](E03-P01_decisions.md#346-erros-de-leitura-tardios-e-commit-revisão-2026-10-19).

- [x] **E03-P04 — Fechar a fronteira de erro tardio em transações.** Registrar em `Row.Scan`, `Rows.Scan`, iteração (`Rows.Next`/`Rows.Err`) e fechamento de resultados pertencentes a uma `Tx` o primeiro erro encontrado, fazendo o `Commit` bloquear com auto-rollback como nos erros imediatos. Excluir `sql.ErrNoRows` e observações de estado terminal; não criar mecanismo de limpeza do erro memorizado. Inverter o teste que fixava a limitação e cobrir as exclusões e a origem no `DB`.

  > **Decisão registrada (2026-10-19) em [E03-P01_decisions.md](E03-P01_decisions.md#346-erros-de-leitura-tardios-e-commit-revisão-2026-10-19) (§3.4.6):** back-reference interna de `Row`/`Rows` à `Tx`; registro com first-wins sob `tx.mu`; `ErrNoRows` não envenena; sem escape hatch (consistente com E03-P03); conversões em `Scan` são memorizadas junto com falhas de driver por não serem distinguíveis. Responde à pendência de §6.3 sobre erros de leitura tardios.

  > **Implementado e validado (2026-10-19):** campo `tx` em `Row`/`Rows` e método `Tx.recordLate` em [rows.go](../internal/scan/rows.go) e [tx.go](../internal/transaction/tx.go); `QueryContext`/`QueryRowContext` associam o resultado à transação. Testes unitários em [tx_test.go](../internal/transaction/tx_test.go): `TestLateReadFailureBlocksCommit` (scan e iteração bloqueiam o commit com auto-rollback), `TestErrNoRowsDoesNotBlockCommit` e `TestLateReadFailureOnDBNotRecorded`. Integração em [tests/tx_test.go](../tests/tx_test.go): `TestCommitBlockedByLateReadError` (erro de conversão bloqueia commit e não persiste a escrita). Exemplo de rollback atualizado. `make check` passou (Go 1.26.8) e `go test -race -count=2 ./...` passou como evidência local. README atualizado. Evidências manuais de MySQL/PostgreSQL seguem para E09 (cenário V12).

**Critério de conclusão:** a validação demonstra confirmação e reversão do grupo, sem incluir indevidamente operações externas, por testes automatizados em SQLite e execução manual posterior nos demais bancos. As regras de erro e de concorrência estão documentadas; evidências manuais ainda ausentes permanecem pendentes para E09.

### E04 — Definir entidades e mapear os resultados

**Entrega:** tabelas e colunas podem ser descritas por structs Go mapeados por reflexão, e resultados podem ser preenchidos sem confundir valores de registros distintos.

- [x] **E04-P01 — Especificar entidades e registros.** Definido o mapeamento por reflexão sobre structs Go: a entidade é derivada do struct e das tags; a ordem das colunas é a de declaração dos campos; resultados são lidos para `*T` ou `*[]T`; `NULL` usa ponteiro/tipos `sql.Null*`. A troca da entidade explícita por reflexão está registrada em [E04-P01_decisions.md](E04-P01_decisions.md).

  > **Pendências:** detalhar a sintaxe das tags (E04-P02) e o momento de validação/formatação (E04-P03), conforme §6.7.

- [x] **E04-P02 — Implementar o mapeamento e o acesso tipado.** Derivar os metadados do struct por reflexão (tabela, colunas, ordem, chave primária), preencher `*T` e `*[]T` e tratar valores nulos e conversões incompatíveis.

  > **Pendências resolvidas (2026-10-19) em [E04-P02_decisions.md](E04-P02_decisions.md):** sintaxe das tags `krios:"coluna[,pk]"` e `krios:"-"`, nome de tabela por `snake_case` do tipo com sobrescrita por `TableName()`, correspondência de colunas por nome em `Rows.All` (posicional em `Row.Scan`), coluna não selecionada inalterada, conversões/NULL delegados ao `database/sql` com causa preservada e structs embutidos rejeitados nesta entrega.

  > **Implementado e validado (2026-10-19):** [entity.go](../internal/entity/entity.go) com parser de tags, `snake_case`, `TableName()` e `Entity`/`EntityOf` cacheados por tipo; [mapping.go](../internal/scan/mapping.go) com classificação de destino, preenchimento de `*T` e `Rows.All` para `*[]T`; `Rows.Columns()` e o ramo de struct em `Row.Scan` em [rows.go](../internal/scan/rows.go); sentinela `ErrInvalidEntity` em [errors.go](../internal/errs/errors.go) e fachada `EntityOf`/`Entity` em [krios.go](../krios.go). Testes unitários em [entity_test.go](../internal/entity/entity_test.go) e [mapping_test.go](../internal/scan/mapping_test.go) (tags, cache, correspondência por nome, aliases, subconjunto, nulos, conversões, fechamento e bloqueio de commit). Integração SQLite em [tests/mapping_test.go](../tests/mapping_test.go) e exemplo em [tests/examples_test.go](../tests/examples_test.go) (`Example_structMapping`). `make check` e `go test -race -count=1 ./...` passaram (Go 1.26.8). Validação manual de MySQL/PostgreSQL permanece para E09. Junções, esquema/alias e `ColumnTypes` ficam para E05/E07; validação/formatação fica em E04-P03.

- [x] **E04-P03 — Implementar validação e formatação por campo.** Receber as funções fornecidas pelo consumidor, associadas a campos do struct, e integrar seus resultados e erros ao fluxo definido para atribuição, leitura ou escrita.

  > **Pendências resolvidas (2026-10-19) em [E04-P03_decisions.md](E04-P03_decisions.md):** interfaces `Validator`/`Formatter` implementadas pelo tipo do campo (valor ou ponteiro), execução explícita por `Entity.Validate`/`ValidateAll`/`Format` (reutilizável pelos escritores de E06), valor formatado apenas para apresentação (original segue para o banco), fallback `fmt.Sprint` e ponteiro nulo pulado/vazio, fail-fast e erros `ErrValidation`/`ErrFormat` com tabela/coluna e causa preservada.

  > **Implementado e validado (2026-10-19):** [validation.go](../internal/entity/validation.go) com as interfaces, detecção por reflexão e os métodos `Validate`/`ValidateAll`/`Format`; flags `validator`/`formatter` em [entity.go](../internal/entity/entity.go); sentinelas `ErrValidation`/`ErrFormat` em [errors.go](../internal/errs/errors.go) e fachada de tipos/erros em [krios.go](../krios.go). Testes unitários em [validation_test.go](../internal/entity/validation_test.go) (valor/ponteiro, nulos, primeiro erro, destinos inválidos) e integração SQLite em [tests/validation_test.go](../tests/validation_test.go); exemplo `Example_validationFormatting` em [tests/examples_test.go](../tests/examples_test.go). `make check` e `go test -race -count=1 ./...` passaram (Go 1.26.8). Registro por coluna, validação estrutural e agregação de erros ficam fora desta entrega.

- [x] **E04-P04 — Validar o mapeamento.** Cobrir múltiplos registros, ordem das colunas, aliases, nulos, tipos incompatíveis, erros de validação e formatação. Usar consultas pela base de E03, sem depender do construtor de E05.

  > **Implementado e validado (2026-10-19):** sem mudança de API (E04-P04 é de verificação; decisões em [E04-P04_decisions.md](E04-P04_decisions.md)). Reforço unitário em [mapping_test.go](../internal/scan/mapping_test.go) (ordem posicional em `Row.Scan` e independência entre registros). Integração SQLite em [mapping_validation_test.go](../tests/mapping_validation_test.go): ordem posicional, múltiplos registros independentes, aliases de coluna/tabela, `sql.Null*`, `NULL` em `*T`, validação com fail-fast por registro e formatação integrada. Exemplo `Example_mappingValidation` em [examples_test.go](../tests/examples_test.go). `make check` e `go test -race -count=1 ./...` passaram (Go 1.26.8). Texto amigável para `sql.Null*` no fallback de `Format` permanece como melhoria futura; evidência manual MySQL/PostgreSQL segue em E09.

**Critério de conclusão:** um exemplo consegue ler e modificar valores de registros distintos por meio do mapeamento por reflexão; a representação dos resultados está definida e demonstrada.

### E05 — Construir consultas de leitura e paginação

**Entrega:** consultas `SELECT` encadeadas, geradas para cada dialeto e integradas ao mapeamento e às transações.

- [x] **E05-P01 — Definir expressões e contratos dos dialetos.** Especificar referências a tabelas e colunas, valores parametrizados, comparações, grupos lógicos, funções e a fronteira entre construção e execução.

  > **Decisões registradas (2026-10-19) em [E05-P01_decisions.md](E05-P01_decisions.md):** dialetos em `internal/dialect` com interfaces `Dialect` (placeholder, aspas de identificador, nome) e `MigrationDialect` no mesmo tipo concreto, selecionados por `Config.Type` e retidos por `DB`/`Tx` (§6.2); renderizador único compartilhado, com o dialeto abstrayendo apenas as diferenças. Coluna × valor resolvido por construtores ancorados na entidade (`ent.Col("status").Eq("active")`), com `nil` rejeitado e `In()` vazio como condição sempre falsa (§6.5). `Where` variádico combina com `AND`; grupos aninháveis por `krios.And`/`krios.Or`. Operadores `=`, `<>`, `<`, `<=`, `>`, `>=`, `IS [NOT] NULL`, `IN` e `LIKE` com modos contém/começa/termina, escape literal de `%`/`_` e sensibilidade à collation do banco, sem tradução de sintaxe. Junções `INNER`/`LEFT` com critérios coluna × coluna; alias, esquema e `RIGHT`/`FULL` como extensão futura. Fronteira construção/execução: receptor não genérico `db.Select(ent)`/`tx.Select(ent)` (limitação de Go: sem type params em métodos), tipagem nos destinos como `Rows.All`, `Build()` público para inspeção e novo sentinela `ErrInvalidQuery`. Itens adiados com destino: agregações e `krios.Paginate` em E05-P03; `SetValues` e retornos de escrita em E06-P01.

- [x] **E05-P02 — Implementar seleção, filtros e junções.** Construir consultas a partir de entidades, oferecer `Only`, `Where`, grupos com `OR` e junções, gerar parâmetros separados dos valores e preservar a ordem das colunas na seleção padrão. Implementar a geração equivalente nos três dialetos.

  > **Implementado e validado (2026-10-19):** pacote [internal/dialect](../internal/dialect/dialect.go) com `Dialect`, `For(config.DBType)` e os tipos `sqlite`/`mysql`/`postgres` (`?`/`?`/`$n`; aspas duplas/crase/duplas, incluindo nome qualificado `schema.tabela`), além da `MigrationDialect` de E08 em [migration.go](../internal/dialect/migration.go). Sentinela `ErrInvalidQuery` em [internal/errs/errors.go](../internal/errs/errors.go). AST de condições em [internal/entity/expression.go](../internal/entity/expression.go): `(*Entity).Col`, `Column`, `Condition` opaca, `Eq`/`Ne`/`Lt`/`Lte`/`Gt`/`Gte`/`IsNull`/`IsNotNull`/`In`/`Like`, `And`/`Or`, `Render`/`RenderInto` (numeração contínua de placeholders entre condições) e `Entities()`. Builder em [internal/query/query.go](../internal/query/query.go): `New`, `Only`, `Where` (AND acumulado), `Join` (`INNER`/`LEFT`), `Build`, `Rows`, `Row`, `All`, `One`; validação por nome de tabela (duplicata/repetição), colunas restritas à entidade base em `Only`, entidades referenciadas presentes em `Where`/`Join`, ordem textual dos argumentos e `In()` vazio como `FALSE`. Dialeto retido em `DB` e `Tx` com `Select(ent)` em [access](../internal/access/db.go) e [transaction](../internal/transaction/tx.go); `transaction.New` passou a receber o dialeto. Fachada em [krios.go](../krios.go) (`Column`, `Condition`, `LikeMode`, `Select`, `JoinKind`, consts, `And`/`Or`, `ErrInvalidQuery`). Testes unitários de geração nos três dialetos em [dialect_test.go](../internal/dialect/dialect_test.go), [expression_test.go](../internal/entity/expression_test.go) e [query_test.go](../internal/query/query_test.go); integração SQLite e exemplo em [tests/select_test.go](../tests/select_test.go) e [tests/examples_test.go](../tests/examples_test.go) (`Example_selectQueries`). `make check` e `make race` passaram (Go 1.26.8). Validação manual de MySQL/PostgreSQL segue para E09; `Paginate`, agregações e `OrderBy`/`GroupBy`/`Distinct`/`Limit` ficam em E05-P03.

- [ ] **E05-P03 — Implementar agregações e paginação.** Adicionar ordenação, agrupamento, `DISTINCT`, limite, `Count`, `Sum`, `Max`, `Min` e paginação por limite e deslocamento com total e estatísticas.

  > **Pendências:** definir campos de `krios.Paginate`, validação de limites e cálculo do total quando houver agrupamentos ou `DISTINCT` (§6.5).

- [ ] **E05-P04 — Validar construção e execução.** Verificar SQL e parâmetros gerados, executar cenários equivalentes nos três bancos e conferir filtros agrupados, junções, agregações, paginação, seleção padrão e execução dentro de transações.

**Critério de conclusão:** exemplos encadeados retornam os dados e totais esperados nos três bancos, respeitando as diferenças documentadas dos dialetos.

### E06 — Implementar operações de escrita

**Entrega:** `INSERT`, `UPDATE` e `DELETE` por entidade (struct mapeado por reflexão), com valores explícitos, filtros obrigatórios onde definidos e retorno documentado por banco.

- [ ] **E06-P01 — Especificar valores e resultados de escrita.** Definir `SetValues`, campos omitidos, valores zero, `false`, `NULL`, informações de execução e solicitação de campos gerados ou alterados.

  > **Pendências:** definir a assinatura de `SetValues` e a política de `nil`/omitido/`NULL` sobre o modelo de reflexão de [E04-P01_decisions.md](E04-P01_decisions.md), a validade de um `Where` vazio, os retornos disponíveis por banco e os destinos de resultados de uma ou de múltiplas linhas (§6.6).

- [ ] **E06-P02 — Implementar os construtores de escrita.** Gerar SQL parametrizado pelos dialetos, exigir `Where` em atualizações e exclusões e integrar a validação de valores e a execução na conexão ou na transação selecionada.

  > **Pendências:** aplicar E06-P01, incluindo rejeição antes da escrita quando um retorno solicitado não tiver suporte no banco.

- [ ] **E06-P03 — Validar as escritas e seus efeitos.** Cobrir inserção, atualização e exclusão de múltiplas linhas, campos omitidos, valores explícitos, filtros ausentes ou vazios, linhas afetadas e retornos suportados e não suportados. Repetir o cenário de rollback do grupo usando os construtores públicos.

**Critério de conclusão:** escritas funcionam nos três bancos, alterações sem filtro exigido são rejeitadas e pedidos de retorno incompatíveis falham antes de modificar dados.

### E07 — Executar arquivos SQL incorporados

**Entrega:** consumidor incorpora SQL ao executável e usa esses arquivos para escritas e consultas, inclusive dentro de transações.

- [ ] **E07-P01 — Especificar o recebimento de arquivos.** Definir o objeto que referencia um arquivo, a forma de receber os recursos incorporados e os métodos de execução, consulta e obtenção dos resultados.

  > **Pendências:** resolver o tipo público, o recebimento dos arquivos, a passagem de parâmetros e a combinação entre resultados nativos e mapeados (§6.8). Reutilizar os contratos de E03 e E04.

- [ ] **E07-P02 — Implementar a execução dos recursos.** Carregar o SQL selecionado, passar parâmetros separadamente e executá-lo sem tradução de sintaxe. Permitir selecionar a transação explicitamente e comunicar erros de recurso inexistente ou execução.

- [ ] **E07-P03 — Validar um consumidor com `go:embed`.** Compilar um exemplo que incorpore consultas e escritas, executá-lo sem depender dos arquivos SQL originais em tempo de execução e verificar resultados, erros e transações nos bancos correspondentes.

**Critério de conclusão:** o consumidor consegue utilizar SQL incorporado fora do mecanismo de migrações, com parâmetros, resultados e encerramento de recursos conforme o contrato.

### E08 — Implementar migrações versionadas

**Entrega:** consumidor configura versões e arquivos e executa avanço ou reversão de forma explícita, com histórico e tratamento de falhas.

- [ ] **E08-P01 — Especificar versões, arquivos e histórico.** Definir em `krios.Config` a versão desejada e a associação dos arquivos às versões, a ordem de execução, os pares `up`/`down`, a estrutura de `migrations` (com `status`, `filename`, `direction`, `checksum`, `error`), o estado `0`, identificação do esquema gerenciado, `ErrPartialMigration` e contrato de dialeto para capacidades transacionais (`SupportsTransactionalDDL`, `IsTransactionalStatement`).

  > **Decisão base:** ver [E08-P01_decisions.md](E08-P01_decisions.md). Promessa transacional condicional por dialeto. Prefixo numérico não é fonte definitiva; divergências tratadas por configuração.

- [ ] **E08-P02 — Implementar a preparação das migrações.** Receber os recursos incorporados, validar associações, detectar `in_progress`/`failed`, comparar versão atual/desejada, montar sequência (up/down), avaliar atomicidade por dialeto (MySQL: detectar DDL não-transacional por arquivo). Rejeitar ausência da versão desejada, esquemas preexistentes sem histórico e estado parcial.

  > **Base:** [E08-P01_decisions.md](E08-P01_decisions.md).

- [ ] **E08-P03 — Implementar execução e recuperação de falhas.** Executar com escopo máximo viável: PG/SQLite → transação por versão quando segura; MySQL → por arquivo se 100% transacional, senão sem transação. Registrar `in_progress` → `applied`/`failed`. Nunca tentar reverter DDL já committed no MySQL. Bloquear com `ErrPartialMigration` em estado parcial.

  > **Base:** [E08-P01_decisions.md](E08-P01_decisions.md). Não prometer rollback integral onde indisponível.

- [ ] **E08-P04 — Validar os percursos e falhas.** Cobrir banco vazio, avanço, reversão, retorno a `0`, versão já alcançada, arquivos inválidos, esquema sem histórico, falha com rollback integral (PG/SQLite), falha no meio com DDL no MySQL (estado parcial sem rollback integral), detecção de `in_progress`, bloqueio após estado parcial e recuperação guiada. Conferir estado real + histórico.

  > **Base:** [E08-P01_decisions.md](E08-P01_decisions.md).

**Critério de conclusão:** os percursos definidos no RF-05 são demonstrados nos três bancos, e cada cenário de falha tem um estado identificável e um procedimento de recuperação documentado. O uso da biblioteca sem migrações continua funcionando.

### E09 — Consolidar a qualidade e preparar a distribuição

**Entrega:** biblioteca com o escopo inicial validado, documentação de consumo e uma versão distribuível.

- [ ] **E09-P01 — Conferir todos os requisitos e critérios de atendimento.** Relacionar os treze critérios da seção 7 do planejamento aos exemplos e testes que os comprovam. Executar as verificações automatizadas com SQLite e os testes unitários dos dialetos; reunir os resultados dos cenários manuais de MySQL e PostgreSQL realizados pelo consumidor, corrigindo divergências encontradas.

  > **Pendências:** fechar as decisões que ainda afetem o comportamento do escopo inicial. Uma funcionalidade exigida não pode ser considerada entregue apenas porque sua definição foi adiada; eventuais mudanças de escopo devem ser registradas no planejamento.

- [ ] **E09-P02 — Finalizar os exemplos e a documentação.** Documentar instalação, importação de drivers, preenchimento de `krios.Config`, abertura, entidades, consultas, escritas, transações, SQL incorporado, migrações, erros e limitações por banco. Conferir que os exemplos correspondem à API implementada.

- [ ] **E09-P03 — Definir e preparar a primeira versão.** Registrar a política de versionamento e estabilidade da API, revisar o caminho do módulo, preparar notas da versão e documentar o procedimento de publicação.

  > **Pendências:** definir a versão inicial, a política de compatibilidade e o processo de publicação (§6.9). A escolha do caminho feita em E01 deve permanecer consistente com os imports e a localização do repositório.

- [ ] **E09-P04 — Publicar e validar o consumo externo.** No momento da entrega, executar o procedimento de publicação definido e instalar a versão em um projeto consumidor separado. Verificar imports, dependências e um exemplo funcional de acesso ao banco com `krios.Config` definida em código, sem distribuir arquivos externos de configuração.

  > **Pendências:** concluir E09-P03 e dispor do acesso necessário ao repositório de publicação. Este documento planeja essa ação futura; sua criação não publica código nem versões.

**Critério de conclusão:** os critérios de atendimento têm evidências, a documentação corresponde à implementação e a versão publicada pode ser instalada e utilizada por outro projeto Go.

## 5. Registro de progresso

O estado abaixo distingue implementação, revisão local e validação real pendente. Registros datados nas entregas preservam a sequência histórica; os contratos vigentes estão nas especificações de decisões revisadas.

Ao concluir uma entrega, registrar abaixo sua referência de implementação e validação. Usar commit, revisão, especificação ou resultado de testes que permita localizar a evidência.

| Entrega | Estado       | Evidências e observações |
| ------- | ------------ | ------------------------ |
| E01     | Concluída | E01-P01, E01-P02 e E01-P03 concluídos. Checks remotos Go 1.26/1.27 confirmados pelo consumidor e versão incorporada do SQLite aceita em 18/10/2026. Ver [Makefile](../Makefile), [testes](../tests/sqlite_test.go), [workflow](../.github/workflows/check.yml) e [README](../README.md). |
| E02 | Implementada; integração real parcial | Configuração, DSN e ciclo de vida corrigidos em M01–M06. SQLite e drivers simulados validados. E02-P04 permanece parcial até evidências reais MySQL/PostgreSQL em E09. Ver [decisões](E02-P01_decisions.md) e [melhorias](0003_improvement_plan.md). |
| E03     | Implementada e revisada localmente | E03-P01–E03-P04 concluídos. Contratos em [E03-P01_decisions.md](E03-P01_decisions.md) (inclui §3.4.5, Commit bloqueado com auto-rollback, e §3.4.6, erros de leitura tardios também bloqueiam); implementação em [tx.go](../internal/transaction/tx.go), [db.go](../internal/access/db.go), [execution.go](../internal/execute/execution.go) e [rows.go](../internal/scan/rows.go); validação em [tests/tx_test.go](../tests/tx_test.go), [tests/tx_group_test.go](../tests/tx_group_test.go) e [internal/transaction/tx_test.go](../internal/transaction/tx_test.go), com evidência local `go test -race`. Revisão M01–M06 validada localmente com check/race nas linhas Go 1.26/1.27; novo CI remoto pendente. Evidências manuais de MySQL/PostgreSQL pendentes para E09. |
| E04     | Concluída (evidência manual MySQL/PostgreSQL pendente para E09) | E04-P01–E04-P04 concluídos. Mapeamento por reflexão com `EntityOf`, tags `krios`, `TableName()`, `Row.Scan(*T)`, `Rows.All(*[]T)` e validação/formatação por campo (`Validator`/`Formatter`, `Validate`/`ValidateAll`/`Format`), verificada por testes de integração. Contratos em [E04-P01_decisions.md](E04-P01_decisions.md), [E04-P02_decisions.md](E04-P02_decisions.md), [E04-P03_decisions.md](E04-P03_decisions.md) e [E04-P04_decisions.md](E04-P04_decisions.md); implementação em [entity.go](../internal/entity/entity.go), [mapping.go](../internal/scan/mapping.go) e [validation.go](../internal/entity/validation.go); validação em [tests/mapping_test.go](../tests/mapping_test.go), [tests/mapping_validation_test.go](../tests/mapping_validation_test.go), [tests/validation_test.go](../tests/validation_test.go) e [tests/examples_test.go](../tests/examples_test.go). Evidência manual MySQL/PostgreSQL pendente para E09. |
| E05     | Especificação (E05-P01) e construção/execução (E05-P02) concluídas; agregações e paginação pendentes (E05-P03) | Contratos e dialetos em [E05-P01_decisions.md](E05-P01_decisions.md) (2026-10-19); builder `Select` validado por testes nos três dialetos e integração SQLite (2026-10-19). E05-P03–E05-P04 não iniciados. Evidência manual MySQL/PostgreSQL seguirá para E09. |
| E06     | Não iniciada | —                        |
| E07     | Não iniciada | —                        |
| E08     | Não iniciada | —                        |
| E09     | Não iniciada | —                        |

## 6. Lembretes para validação posterior

- [ ] Construir testes unitários das regras da biblioteca e da geração de SQL para SQLite, MySQL e PostgreSQL conforme as funcionalidades forem implementadas, incluindo parâmetros, filtros, escritas e capacidades não suportadas.

  > **Parcial (2026-10-19):** geração de `SELECT` coberta nos três dialetos em `internal/dialect`, `internal/entity` e `internal/query`; integração SQLite cobre filtros, `LIKE`, `IN`, `And`/`Or`, junções e execução em transação. Escritas (E06), SQL incorporado (E07) e migrações (E08) permanecem pendentes.
- [x] Preparar o [roteiro manual da base E01–E03](E09_manual_validation.md), com ambiente isolado, versões e registro de resultados.
- [ ] Ampliar o roteiro para construtores, SQL incorporado e migrações conforme E04–E08 forem implementados.
- [ ] Na preparação desses testes manuais, escolher os drivers e as versões dos bancos e registrar as combinações utilizadas e os resultados.
- [ ] Antes de concluir E09, reunir as evidências manuais dos dois bancos e corrigir incompatibilidades. O sucesso dos testes com SQLite ou dos testes unitários não comprova sozinho o funcionamento em MySQL e PostgreSQL.
