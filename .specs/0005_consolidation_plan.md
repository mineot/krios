# Krios — Plano de consolidação antes da E09

Data: 2026-10-20.

**Estado: planejado; correções ainda não executadas.** A preparação deste plano incluiu leitura dos documentos, inspeção dirigida do código, consulta à suíte existente e uma reprodução isolada em SQLite. Não equivale a uma auditoria completa nem à validação final da biblioteca.

## 1. Objetivo e fronteira com E09

Consolidar E01–E08 antes de iniciar a validação final e a distribuição: corrigir defeitos locais, reconciliar contratos, fechar lacunas de regressão e preparar evidências reproduzíveis. Este plano complementa [0002_execution_plan.md](0002_execution_plan.md), sem renumerar as entregas ou apagar suas evidências históricas.

A entrada na validação final depende da conclusão de C01–C07. **Revisão solicitada pelo consumidor em 2026-10-20:** a integração real automatizada dos três bancos passa à [E10](E10_integration_plan.md), executada depois da consolidação e antes da conclusão/publicação de E09. A conferência final dos treze critérios de atendimento, os casos manuais residuais, as decisões de versionamento e a publicação continuam em E09. Preparar cenários aqui não significa executá-los nem concluir essas entregas antecipadamente.

Diretrizes:

- Preservar o import público, a fachada em `krios.go`, a organização por responsabilidade e a ausência de imports de drivers em produção.
- Corrigir comportamento comprovadamente incorreto; registrar separadamente mudanças de contrato público. Recomendações deste plano não são decisões já aprovadas pelo consumidor.
- Antes de cada correção, criar uma regressão que demonstre o defeito; verificar resultados observáveis, recursos liberados e estado persistido.
- Usar SQLite temporário e drivers simulados para falhas determinísticas. Não usar `dev.db` nem bancos do consumidor.
- Manter evidência local, simulada e remota separadas. Um teste existente aprovado não comprova cenários que ele não exercita.
- Registrar extensões futuras sem incorporá-las à consolidação: self-join/aliases públicos, novos tipos de JOIN, `HAVING`, inserção em lote, seleção parcial em `RETURNING`, divisão de SQL em execuções e extensibilidade pública de dialetos.

## 2. Base factual e prioridades

Os achados abaixo têm alcances diferentes. “Inspeção” identifica um caminho no código; seus cenários ainda precisam de regressões executáveis. Nenhum deles representa evidência contra servidores MySQL/PostgreSQL reais.

| ID  | Prioridade | Achado e evidência                                                                                                                                                                                                                                                                        | Encaminhamento                                                                                   |
| --- | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| A01 | Crítica    | [lastApplied](../internal/migrate/migrate.go) usa a maior versão `applied`, ignorando `direction`; [executeFile](../internal/migrate/apply.go) também grava `down` como `applied`. Reprodução SQLite: `0 → 1 → 0 → 1` termina com sucesso e nenhum passo, embora a tabela esteja ausente. | C02: versão efetiva, histórico e ciclos repetidos.                                               |
| A02 | Alta       | [MigrationsConfig.Validate](../internal/config/config.go) não valida `TargetVersion`. Alvos `-1` e `2`, com apenas a versão `1` declarada, retornaram `nil`. A preparação filtra versões sem exigir que alcance o alvo.                                                                   | C01/C02: presença, domínio e alcançabilidade do alvo.                                            |
| A03 | Alta       | Um arquivo aplicado já eleva a versão em `lastApplied`; não há conferência de que todos os arquivos da direção tenham terminado. No caminho MySQL por arquivo, uma falha transacional posterior pode reverter seu próprio registro e deixar apenas arquivos anteriores `applied`.         | C02/C03: reproduzir versão incompleta e impedir sucesso/avanço indevidos.                        |
| A04 | Alta       | [existenceQuery](../internal/migrate/migrate.go) interpola nomes em literais SQL e, para MySQL/PostgreSQL, consulta `information_schema.tables` sem delimitar banco/esquema. `isMissingTable` depende de fragmentos amplos da mensagem de erro.                                           | C03: parametrização, escopo e diagnóstico de ausência.                                           |
| A05 | Alta       | [Rows.All](../internal/scan/mapping.go) retorna antes de fechar em falhas de destino, derivação ou metadados. [Row.Scan](../internal/scan/rows.go) também pode rejeitar a entidade antes de consumir a linha nativa. Os atalhos nem sempre devolvem o recurso ao consumidor.              | C04: fechamento e tratamento uniforme das falhas de mapeamento.                                  |
| A06 | Alta       | [fileTransactional](../internal/migrate/migrate.go) aceita todo arquivo em SQLite/PostgreSQL pelo sinalizador geral. O classificador MySQL aceita palavras não presentes na lista de exclusão, embora seu contrato diga “na dúvida, false”.                                               | C03: delimitar comandos suportados, classificação e controle transacional.                       |
| A07 | Alta       | [apply.go](../internal/migrate/apply.go) nem sempre preserva `ErrPartialMigration` quando o SQL sem transação executou e a atualização do histórico falhou. Os escopos transacionais não registram limpeza logo após `Begin`.                                                             | C03: verificar falhas de histórico, cancelamento e finalização; corrigir caminhos comprovados.   |
| A08 | Média      | O manual e a execução usam transação por versão em SQLite/PostgreSQL; E08 e o cenário M05 ainda contêm promessas de rollback de todo o percurso. O teste `TestMigrationsFailureRollsBackSQLite` exige que a versão anterior permaneça aplicada.                                           | C01/C05: explicitar a unidade de atomicidade.                                                    |
| A09 | Média      | [NewSQLFile](../internal/sqlfile/sqlfile.go) e o manual usam `ErrFileNotFound` para conteúdo vazio; E07 §8 cita `ErrInvalidQuery`. E07 também generaliza o mapeamento por nome para `One`, que é posicional.                                                                              | C05: corrigir contratos documentais.                                                             |
| A10 | Média      | Exemplos históricos ainda afirmam validação somente explícita, rollback após commit como no-op e fechamento incondicional de `All`; parte dos trechos de escrita usa SQL direto, apesar de se apresentar como extraída da implementação atual.                                            | C05: conferir exemplos com código executável e contratos corrigidos.                             |
| A11 | Média      | [DB.Migrations](../internal/access/db.go) devolve o mesmo ponteiro retido de `Open`, incluindo slices mutáveis; há risco de documentação sugerir imutabilidade ou concorrência não oferecidas.                                                                                            | C01/C05: definir propriedade e mutabilidade; alterar código somente conforme contrato escolhido. |

### Evidência inicial

- Go local: `go1.26.8 linux/amd64`.
- `go test ./internal/config ./internal/scan ./internal/sqlfile ./internal/migrate ./tests`: aprovado, com resultados em cache. Não é uma nova execução sem cache nem uma execução de `make check`/`make race`.
- Consumidor temporário, importando a API pública e `modernc.org/sqlite`, executado com `go run`: uma versão com `CREATE TABLE probe`/`DROP TABLE probe`, reabertura do mesmo banco temporário a cada alvo e inspeção de existência da tabela. Resultado:

```text
target=1 current=0 steps=1 table_exists=1 error=<nil>
target=0 current=1 steps=1 table_exists=0 error=<nil>
target=1 current=1 steps=0 table_exists=0 error=<nil>
validate_target=-1 error=<nil>
validate_target=2 error=<nil>
```

A reprodução usou somente arquivos temporários e removeu seu banco ao terminar. O teste permanente que reproduz A01 é entrega de C02. Os testes atuais de reversão em [tests/migrations_test.go](../tests/migrations_test.go) conferem a remoção das tabelas, mas não repetem a migração nem verificam a versão efetiva após reabertura.

## 3. Decisões de contrato necessárias — C01

Resolver cada decisão antes de implementar a parte dependente. Trabalhos independentes, como regressões de recursos, podem avançar. A compatibilidade deve ser avaliada contra exemplos e consumidores existentes, mesmo antes da primeira versão publicada.

| Decisão                                  | Direção recomendada                                                                                                                                                                                                                                        | Impacto a registrar                                                                                                                                                                   |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D01 — Alvo omitido versus zero explícito | Preservar a exigência de RF-05: representar presença, preferencialmente com `*int`; `nil` é inválido e ponteiro para `0` solicita reversão. Se o consumidor escolher manter `int`, revisar explicitamente RF-05 e assumir que zero omitido também reverte. | Mudar para ponteiro altera a API e todos os exemplos de configuração; não implementar a mudança como simples correção documental.                                                     |
| D02 — Histórico e versão efetiva         | Distinguir execução histórica de estado aplicado. A versão só avança ao completar todos os arquivos; uma reversão completa reduz o estado e permite reaplicação. Definir esquema/chaves que suportem ciclos repetidos, sem apagar falhas silenciosamente.  | Definir preservação do histórico, tratamento de bancos já criados pelo Krios e recuperação de registros antigos ambíguos. A estrutura exata precisa ser registrada antes da correção. |
| D03 — Atomicidade                        | Manter uma transação por versão em SQLite/PostgreSQL para comandos suportados; versões já confirmadas permanecem. MySQL mantém escopo por arquivo quando viável e estado parcial explícito.                                                                | Corrigir promessas de rollback global; documentar comportamento da criação da tabela de infraestrutura e de falhas de commit.                                                         |
| D04 — Falhas de leitura/mapeamento       | Fechar recursos em todos os caminhos de `All` e concluir o ciclo de `Row.Scan` mesmo com destino inválido. Definir registro na Tx quando a consulta já foi iniciada, preservando erros primário e de limpeza.                                              | Diferenciar falha anterior ao executor de falha após consulta; preservar `sql.ErrNoRows` como exclusão do bloqueio de commit.                                                         |
| D05 — Configuração retida                | Preferir snapshot dos campos e slices de migração em `Open` e cópia defensiva em `Migrations`; documentar o limite da cópia de `fs.FS`.                                                                                                                    | Avaliar consumidores que alteram o alvo pelo ponteiro retornado; se a mutabilidade for mantida, definir sincronização e limites explícitos.                                           |

- [ ] **C01-P01 — Registrar as decisões D01–D05**, com comportamento escolhido, alternativas rejeitadas, impacto de compatibilidade e exemplos de migração de código quando necessário.
- [ ] **C01-P02 — Reconciliar os requisitos afetados**, com referência à decisão vigente e identificação dos trechos históricos superados em RF-05, E03/E04/E07/E08 e manual.

**Critério de conclusão:** cada comportamento necessário às correções tem contrato verificável; nenhuma recomendação aparece como aprovação já obtida. Mudanças que dependem de escolha do consumidor são apresentadas concretamente antes de sua implementação.

## 4. Histórico, alvos e repetição de migrações — C02

Depende de D01/D02. Prioridade: crítica.

- [ ] **C02-P01 — Reproduzir os defeitos de estado pela API pública.** Criar regressões SQLite para `0 → 1 → 0 → 1`, `0 → 2 → 1 → 2`, repetição no mesmo alvo e reabertura entre chamadas. Conferir esquema, dados, histórico e `CurrentVersion`; não alterar o histórico manualmente para fazer o fluxo normal passar.
- [ ] **C02-P02 — Corrigir validação e planejamento.** Rejeitar alvo negativo, ausente conforme D01 e alvo positivo não declarado. Aceitar versões não contíguas crescentes, calculando a versão anterior pela sequência declarada, sem `version - 1`. Verificar histórico incompatível com a configuração, direção necessária ausente e versão atual sem correspondência antes de executar alterações.
- [ ] **C02-P03 — Implementar o estado efetivo e os ciclos de D02.** Avanço/reversão só completam uma versão quando todos os arquivos necessários terminarem. Resolver conflito da chave histórica em reaplicações. Detectar histórico incompleto, status/direções inválidos e versões parcialmente executadas; não aceitar a mera existência de uma linha `applied` como conclusão da versão.
- [ ] **C02-P04 — Verificar recuperação e compatibilidade do histórico.** Cobrir vários arquivos por versão, interrupção entre arquivos, repetição após rollback e recuperação explícita após aplicação parcial. Definir o tratamento do checksum já armazenado: registro informativo ou verificação de integridade, sem prometer detecção de alterações que não exista. Documentar o procedimento para históricos anteriores ao ajuste.

**Critério de conclusão:** ciclos repetidos chegam ao alvo real, reabertura reconhece o estado correto e nenhum alvo inválido ou versão incompleta produz sucesso aparente. O estado `0` e a retenção do histórico seguem D02 em código, testes e documentação.

## 5. Preparação, escopo e falhas de migração — C03

Depende de D02/D03 e do modelo de estado de C02; a reprodução pode começar antes.

- [ ] **C03-P01 — Corrigir a inspeção de objetos.** Parametrizar valores nas consultas de existência; validar nomes vazios/inválidos e definir suporte a nomes qualificados. Delimitar banco/esquema coerente com a conexão e com a tabela de histórico. Diferenciar ausência de tabela de permissões, conexão, histórico malformado e outros erros, sem depender exclusivamente de fragmentos genéricos de mensagens. Verificar SQL/argumentos nos três dialetos e preparar cenário real com tabelas homônimas em esquemas distintos.
- [ ] **C03-P02 — Tornar explícita a classificação transacional.** Conferir comandos e exceções nas fontes oficiais dos bancos ao implementar. Testar palavras desconhecidas, comentários, aspas, múltiplos statements e comandos de controle de transação fornecidos pelo consumidor. Rejeitar comandos incompatíveis com o escopo escolhido antes de aplicá-los, ou encaminhá-los a um caminho sem transação definido no contrato. A análise não deve modificar nem dividir o texto enviado ao driver; não construir um parser SQL genérico nesta entrega.
- [ ] **C03-P03 — Preservar diagnóstico e limpeza.** Injetar falhas em registro inicial, SQL, marcação de sucesso/falha, cancelamento entre passos, commit e rollback. Garantir tentativa de limpeza de transações ainda ativas e preservar causas múltiplas. Após efeitos sem rollback garantido, reportar estado parcial/incerto com contexto, mesmo quando a atualização do histórico também falhar; bloquear retomada automática insegura.
- [ ] **C03-P04 — Conferir progresso parcial.** No MySQL simulado, aplicar o primeiro arquivo e falhar no seguinte, tanto no caminho transacional quanto sem transação; reiniciar a execução e comprovar que a versão não é tratada como completa. Verificar `LastVersion`/`LastFile` inclusive em `down` com versões não contíguas. Em SQLite, falhar na segunda versão e comprovar preservação da primeira conforme D03.

**Critério de conclusão:** preparação delimita os objetos corretos e preserva erros; escopo, limpeza e estado parcial correspondem ao contrato escolhido. Evidência simulada permanece identificada como tal; a confirmação em servidores reais segue para E09.

## 6. Recursos de leitura e propagação de erros — C04

Depende de D04. Pode avançar independentemente de C02/C03.

- [ ] **C04-P01 — Demonstrar recursos retidos.** Usar contadores do driver simulado para conferir fechamento após destino inválido, entidade inválida, erro de metadados, conversão, iteração e fechamento. Cobrir `Rows.All` e `Row.Scan`, incluindo ponteiro de entidade nil. O teste deve observar a limpeza antes de qualquer `defer` do próprio teste que possa mascarar o defeito.
- [ ] **C04-P02 — Corrigir o ciclo de vida.** Garantir fechamento/consumo na responsabilidade do wrapper e propagação conforme D04, sem substituir a causa original por erro de limpeza. Preservar o destino de `All` em falha e a substituição por slice vazio em sucesso sem linhas. Manter leitura posicional de `One` e por nome de `All`.
- [ ] **C04-P03 — Exercitar os atalhos públicos.** Verificar `Select.One/All/Paginate`, `File.One/All` e escritas com `Returning().One/All`, em DB e Tx. Conferir liberação de recursos, bloqueio de commit quando aplicável e exclusão de `sql.ErrNoRows`. Erros de construção anteriores ao executor continuam sem iniciar consultas ou escritas.

**Critério de conclusão:** cada recurso obtido pelos atalhos tem encerramento verificável em sucesso e falha, e o consumidor recebe a causa correta com a política transacional documentada.

## 7. Documentação, exemplos e rastreabilidade — C05

Depende das decisões e das correções correspondentes; revisão pode ser incremental.

- [ ] **C05-P01 — Consolidar a descrição vigente.** Revisar planejamento, decisões, README, manual e comentários de API. Corrigir A08–A11, tabela via `TableName` versus proposta antiga de tag de struct, validação automática em `SetValues`, retorno de `Rollback` após finalização e disponibilidade de `Result` conforme driver. Distinguir preparação com consultas ao banco de operação puramente local e plano preparado de confirmação de todos os passos.
- [ ] **C05-P02 — Conferir os exemplos de consumo.** Alinhar [0001_use_examples.md](0001_use_examples.md) a `examples/`; verificar assinaturas, dois retornos de `Exec`, ordem dos campos em `One`, uso de `Omit` e configuração corrigida de migrações. Manter exemplos executáveis para os contratos centrais e um exemplo de avanço/reversão/reaplicação; não declarar compilado um fragmento apenas por ser sintaticamente válido.
- [ ] **C05-P03 — Criar matriz de rastreabilidade.** Mapear RF-01–RF-09 e os treze critérios do planejamento para contrato vigente, implementação, teste/exemplo, evidência local e pendência E09. Revisar o restante da API pública de forma dirigida; novos achados entram com impacto e reprodução, sem expansão automática de escopo. Classificar adiamentos como extensão, mudança de escopo registrada ou requisito ainda não atendido, incluindo `ColumnTypes` e aliases/esquemas.

**Critério de conclusão:** os documentos descrevem os mesmos contratos e limitações; o histórico permanece rastreável e nenhum requisito desaparece sob a classificação genérica de melhoria futura.

## 8. Preparação dos cenários para E10/E09 — C06

Depende dos contratos estabilizados de C02–C05.

- [ ] **C06-P01 — Ampliar o roteiro para E06/E07.** Acrescentar consumidores reproduzíveis com escritas, zeros/nulos/omissão, validação, filtros obrigatórios, retornos suportados/rejeitados e transações; SQL incorporado com parâmetros, `One/All`, recurso inválido e falha tardia. Usar módulo consumidor separado para os drivers dos servidores.
- [ ] **C06-P02 — Revisar M01–M07 e acrescentar ciclos/falhas.** Adequar M05 à atomicidade decidida e M06 à presença do alvo. Cobrir os ciclos de C02, múltiplos arquivos por versão, recuperação parcial, falha ao persistir histórico e escopo de objetos. Preferir um statement por arquivo nos cenários portáveis; tratar multi-statement separadamente com suporte do driver explicitamente registrado.
- [ ] **C06-P03 — Preparar registro de execução.** Para cada cenário: ambiente/versões, revisão, pré-condições, passos, resultado esperado, limpeza e campo de resultado observado. Compilar os consumidores quando o ambiente permitir; manter MySQL/PostgreSQL como “não executado” até evidência real. Diferenças de collation, configuração e driver devem ser medidas, sem expectativas universais não justificadas.

**Critério de conclusão:** E10 recebe cenários suficientes para automatizar os contratos corrigidos; E09 mantém os procedimentos e registros dos casos residuais manuais. A limpeza fica limitada aos objetos de teste, sem confundir preparação com validação real. A implementação do harness e dos jobs pertence à E10, não a C06.

## 9. Verificação consolidada e passagem — C07

Depende de C01–C06.

- [ ] **C07-P01 — Executar a verificação local.** Após as regressões de cada correção, executar `make check`, `go test -count=1 ./...` e `make race` nas linhas Go 1.26/1.27 disponíveis, registrando versões exatas. Conferir dependências compiladas de produção para ausência de drivers. Uma linha indisponível permanece pendente, com execução correspondente no CI antes de encerrar a consolidação.
- [ ] **C07-P02 — Obter evidência do CI revisado.** Registrar revisão e resultado dos jobs de check/race nas duas linhas previstas. Usar o fluxo normal de envio/revisão do projeto; não atribuir sucesso a uma execução antiga ou não observada. Preservar as condições de execução em repositório público. Falhas e indisponibilidades ficam explícitas até resolução.
- [ ] **C07-P03 — Registrar a passagem para E10/E09.** Atualizar esta matriz de progresso e o plano principal com correções, decisões, regressões, revisão e limitações remanescentes. Confirmar que não há defeito local conhecido do escopo sem resolução ou mudança de escopo explicitamente registrada. Transferir a automação da integração real para E10 e a conferência final, casos manuais residuais, acabamento de distribuição e publicação para E09.

**Critério de conclusão:** regressões e verificações consolidadas passaram, contratos e exemplos estão coerentes, e as pendências restantes têm responsável/etapa e evidência exigida. E02-P04 e os itens E09/E10 não são marcados como concluídos por esta passagem. C07 verifica o CI existente de check/race; os novos jobs de integração são entregues em E10-P07, evitando dependência circular.

## 10. Ordem e acompanhamento

Sequência: C01 → C02 → C03 → C05 → C06 → C07 → E10 → conclusão de E09. C04 pode avançar após D04 e converge em C05. A divisão representa frentes de trabalho; não exige agentes paralelos nem publicação por etapa.

| Entrega | Resultado                                                       | Estado   | Evidência de conclusão |
| ------- | --------------------------------------------------------------- | -------- | ---------------------- |
| C01     | Contratos e impactos decididos                                  | Pendente | —                      |
| C02     | Estado de migração e ciclos corretos                            | Pendente | —                      |
| C03     | Preparação, atomicidade e falhas coerentes                      | Pendente | —                      |
| C04     | Recursos e erros de leitura corrigidos                          | Pendente | —                      |
| C05     | Documentos, exemplos e rastreabilidade reconciliados            | Pendente | —                      |
| C06     | Cenários reproduzíveis para automação E10 e casos residuais E09 | Pendente | —                      |
| C07     | Verificações e passagem registradas                             | Pendente | —                      |

Ao concluir um passo, registrar revisão/commit, comandos e resultados, versão de Go, testes que reproduziram o defeito antes da correção e eventuais limites. A inspeção e a reprodução registradas na seção 2 justificam este plano; não encerram suas entregas.
