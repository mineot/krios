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

- [ ] **E02-P01 — Especificar a configuração e o ciclo de vida.** Definir os campos e tipos da struct pública `krios.Config`, sua passagem à biblioteca, as assinaturas de `Open` e `Close`, o cancelamento e os erros relevantes. Produzir um exemplo mínimo de uso antes de implementar o contrato.

  > **Pendências:** definir organização e tipos dos campos de `krios.Config`, valores omitidos e explícitos, padrões, validações, unidades de tempo, cancelamento e erros (§6.1). Definir como identificar o driver importado e sua relação com o banco (§6.2).

- [ ] **E02-P02 — Implementar o fluxo de configuração.** Receber `krios.Config` preenchida pelo consumidor, aplicar os padrões, validar os dados de cada banco e disponibilizar um exemplo de configuração em Go com valores ilustrativos, sem credenciais reais. Manter a configuração de migrações opcional.

  > **Pendências:** aplicar E02-P01. Reservar a integração da configuração de versões e arquivos de migração para E08; ela não deve impedir o uso de conexões nesta entrega.

- [ ] **E02-P03 — Implementar `Open` e `Close`.** Integrar `database/sql` ao driver informado, verificar conectividade, respeitar cancelamento e tempo de espera e liberar recursos criados quando a abertura falhar.

  > **Pendências:** definir o comportamento de chamadas repetidas a `Close` e de operações após o encerramento, junto ao contrato de ciclo de vida (§6.1).

- [ ] **E02-P04 — Validar o ciclo de vida nos três bancos.** Cobrir configuração válida e inválida, driver indisponível, falha de conexão, cancelamento, prazo esgotado e encerramento. Demonstrar que abrir o banco não exige configurar nem executar migrações e que a configuração em código funciona sem arquivos externos ou variáveis de ambiente obrigatórias.

**Critério de conclusão:** exemplos de consumidores com `krios.Config` definida em código e drivers explicitamente importados funcionam em SQLite3, MySQL e PostgreSQL, sem arquivos externos de configuração; falhas de abertura são retornadas e não deixam recursos associados à tentativa abertos.

### E03 — Criar a execução básica e as transações

**Entrega:** operações podem ser executadas no acesso principal ou em uma transação explicitamente escolhida.

- [ ] **E03-P01 — Definir os contratos de execução.** Distinguir comandos de escrita e consultas com linhas, especificando parâmetros, resultados, erros e encerramento dos recursos de leitura. Criar a base compartilhada que será utilizada por construtores e arquivos SQL.

  > **Pendências:** definir tipos expostos de `database/sql`, assinaturas de execução, propriedade dos recursos de leitura (§6.8) e a associação dos executores à conexão ou à transação (§6.3). A exposição pública de SQL em texto não é um requisito adicional desta etapa.

- [ ] **E03-P02 — Implementar `Begin`, `Commit` e `Rollback`.** Criar o objeto de transação e a execução vinculada a ele, mantendo operações do acesso principal independentes. Rejeitar novas operações em uma transação encerrada.

  > **Pendências:** definir concorrência no mesmo objeto, responsabilidade pelo rollback após erro e tratamento de falhas em confirmação ou reversão (§6.3).

- [ ] **E03-P03 — Validar os limites da transação.** Demonstrar três inserts relacionados confirmados em conjunto e a reversão do grupo quando um deles falha. Cobrir operações fora do grupo, transações encerradas e o comportamento concorrente definido no contrato.

**Critério de conclusão:** a validação demonstra confirmação e reversão do grupo, sem incluir indevidamente operações externas, por testes automatizados em SQLite e execução manual posterior nos demais bancos. As regras de erro e de concorrência estão documentadas; evidências manuais ainda ausentes permanecem pendentes para E09.

### E04 — Definir entidades e mapear os resultados

**Entrega:** tabelas e colunas podem ser descritas por entidades, e resultados podem ser consumidos sem confundir valores de registros distintos.

- [ ] **E04-P01 — Especificar entidades e registros.** Definir tabela, esquema, alias, junções, colunas, ordem do mapeamento e representação de uma ou de múltiplas linhas. Decidir como o modelo se relaciona com structs do consumidor.

  > **Pendências:** resolver a separação entre definição da entidade e valores, preservação da ordem das colunas, acesso a linhas e manutenção ou substituição do mapeamento automático para structs (§6.7).

- [ ] **E04-P02 — Implementar o mapeamento e o acesso tipado.** Converter resultados obtidos pela execução básica em registros, disponibilizar operações tipadas de leitura e atribuição e tratar valores nulos e conversões incompatíveis.

  > **Pendências:** definir `NULL`, coluna não selecionada, conversões e, caso o suporte a structs seja mantido, tags e correspondência entre colunas e campos (§6.7).

- [ ] **E04-P03 — Implementar validação e formatação por coluna.** Receber as funções fornecidas pelo consumidor e integrar seus resultados e erros ao fluxo definido para atribuição, leitura ou escrita.

  > **Pendências:** decidir o momento de execução dessas funções e a relação entre o valor formatado para apresentação e o valor utilizado no banco (§6.7).

- [ ] **E04-P04 — Validar o mapeamento.** Cobrir múltiplos registros, ordem das colunas, aliases, nulos, tipos incompatíveis, erros de validação e formatação. Usar consultas pela base de E03, sem depender do construtor de E05.

**Critério de conclusão:** um exemplo consegue ler e modificar valores de registros distintos por meio do modelo escolhido; a representação dos resultados está definida e demonstrada.

### E05 — Construir consultas de leitura e paginação

**Entrega:** consultas `SELECT` encadeadas, geradas para cada dialeto e integradas ao mapeamento e às transações.

- [ ] **E05-P01 — Definir expressões e contratos dos dialetos.** Especificar referências a tabelas e colunas, valores parametrizados, comparações, grupos lógicos, funções e a fronteira entre construção e execução.

  > **Pendências:** definir seleção e interface dos dialetos (§6.2), combinação de condições em `Where`, distinção entre coluna e valor, tipos de junção, operadores e modos de busca textual (§6.5).

- [ ] **E05-P02 — Implementar seleção, filtros e junções.** Construir consultas a partir de entidades, oferecer `Only`, `Where`, grupos com `OR` e junções, gerar parâmetros separados dos valores e preservar a ordem das colunas na seleção padrão. Implementar a geração equivalente nos três dialetos.

  > **Pendências:** aplicar os contratos de E05-P01 e a forma escolhida em §6.3 para vincular o construtor ao objeto de transação.

- [ ] **E05-P03 — Implementar agregações e paginação.** Adicionar ordenação, agrupamento, `DISTINCT`, limite, `Count`, `Sum`, `Max`, `Min` e paginação por limite e deslocamento com total e estatísticas.

  > **Pendências:** definir campos de `krios.Paginate`, validação de limites e cálculo do total quando houver agrupamentos ou `DISTINCT` (§6.5).

- [ ] **E05-P04 — Validar construção e execução.** Verificar SQL e parâmetros gerados, executar cenários equivalentes nos três bancos e conferir filtros agrupados, junções, agregações, paginação, seleção padrão e execução dentro de transações.

**Critério de conclusão:** exemplos encadeados retornam os dados e totais esperados nos três bancos, respeitando as diferenças documentadas dos dialetos.

### E06 — Implementar operações de escrita

**Entrega:** `INSERT`, `UPDATE` e `DELETE` por entidade, com valores explícitos, filtros obrigatórios onde definidos e retorno documentado por banco.

- [ ] **E06-P01 — Especificar valores e resultados de escrita.** Definir `SetValues`, campos omitidos, valores zero, `false`, `NULL`, informações de execução e solicitação de campos gerados ou alterados.

  > **Pendências:** decidir a manutenção da entrada por struct, a validade de um `Where` vazio, os retornos disponíveis por banco e os destinos de resultados de uma ou de múltiplas linhas (§6.6). Conciliar esses destinos com o modelo de E04.

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

- [ ] **E08-P01 — Especificar versões, arquivos e histórico.** Definir em `krios.Config` a versão desejada e a associação dos arquivos às versões, a ordem de execução, os pares `up`/`down`, a estrutura de `migrations`, o estado `0` e a identificação do esquema gerenciado.

  > **Pendências:** resolver os campos de migração em `krios.Config`, a referência aos recursos incorporados, a distinção entre versão ausente e versão `0`, a relação com os prefixos numéricos, arquivos ausentes ou duplicados, histórico e objetos considerados na verificação inicial (§6.4). O prefixo não deve ser tratado como regra de execução sem essa decisão.

- [ ] **E08-P02 — Implementar a preparação das migrações.** Receber os recursos incorporados, validar as associações, comparar versão atual e desejada e montar a sequência de avanço ou reversão. Rejeitar ausência da versão desejada e esquemas preexistentes sem histórico antes de executar migrações.

  > **Pendências:** aplicar E08-P01, distinguindo versão ausente de versão `0` e excluindo da verificação os objetos que o contrato não considere objetos de aplicação.

- [ ] **E08-P03 — Implementar execução e recuperação de falhas.** Executar a sequência, registrar os resultados no histórico e aplicar confirmação ou reversão conforme as capacidades do banco e dos comandos.

  > **Pendências:** definir se cada transação cobre arquivo, versão ou percurso completo; definir registro de falha, estado parcialmente aplicado e procedimento de recuperação (§6.4). A política deve tratar explicitamente comandos sem rollback integral e não registrar uma aplicação parcial como concluída.

- [ ] **E08-P04 — Validar os percursos e falhas.** Cobrir banco vazio, avanço, reversão, retorno a `0`, versão já alcançada, arquivos inválidos, esquema sem histórico, falha com rollback e falha com aplicação parcial. Conferir o estado real do banco e o histórico, não apenas o erro retornado.

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

Todas as entregas começam como **não iniciadas**. A elaboração deste plano não conclui passos de implementação.

Ao concluir uma entrega, registrar abaixo sua referência de implementação e validação. Usar commit, revisão, especificação ou resultado de testes que permita localizar a evidência.

| Entrega | Estado       | Evidências e observações |
| ------- | ------------ | ------------------------ |
| E01     | Concluída | E01-P01, E01-P02 e E01-P03 concluídos. Checks remotos Go 1.26/1.27 confirmados pelo consumidor e versão incorporada do SQLite aceita em 18/10/2026. Ver [Makefile](../Makefile), [testes](../tests/sqlite_test.go), [workflow](../.github/workflows/check.yml) e [README](../README.md). |
| E02     | Não iniciada | —                        |
| E03     | Não iniciada | —                        |
| E04     | Não iniciada | —                        |
| E05     | Não iniciada | —                        |
| E06     | Não iniciada | —                        |
| E07     | Não iniciada | —                        |
| E08     | Não iniciada | —                        |
| E09     | Não iniciada | —                        |

## 6. Lembretes para validação posterior

- [ ] Construir testes unitários das regras da biblioteca e da geração de SQL para SQLite, MySQL e PostgreSQL conforme as funcionalidades forem implementadas, incluindo parâmetros, filtros, escritas e capacidades não suportadas.
- [ ] Preparar roteiros reproduzíveis para o consumidor testar manualmente MySQL e PostgreSQL, incluindo conexão, transações, consultas, escritas, SQL incorporado, migrações e recuperação de falhas.
- [ ] Na preparação desses testes manuais, escolher os drivers e as versões dos bancos e registrar as combinações utilizadas e os resultados.
- [ ] Antes de concluir E09, reunir as evidências manuais dos dois bancos e corrigir incompatibilidades. O sucesso dos testes com SQLite ou dos testes unitários não comprova sozinho o funcionamento em MySQL e PostgreSQL.
