# Krios — Visão geral e requisitos do projeto

## 1. Identificação

| Campo                 | Valor                                                           |
| --------------------- | --------------------------------------------------------------- |
| Projeto               | Krios                                                           |
| Tipo                  | Biblioteca Go para gerenciamento de bancos de dados relacionais |
| Status                | Rascunho para definição da arquitetura e da API pública         |
| Projeto de referência | [Krios](https://github.com/krios/krios)                         |

Este documento define a visão geral, o escopo inicial e os requisitos funcionais do projeto. Detalhes de implementação e contratos definitivos da API serão definidos em especificações próprias.

## 2. Objetivo

Desenvolver uma biblioteca Go que centralize a conexão com bancos de dados, o controle de transações, a execução de migrações e a construção de consultas SQL.

A biblioteca deve oferecer uma API pública consistente para SQLite3, MySQL e PostgreSQL, considerando as particularidades de cada banco por meio de dialetos.

## 3. Escopo inicial

A primeira versão deve contemplar:

- Configuração pela struct `krios.Config`, abertura e encerramento de conexões.
- Uso de drivers compatíveis com `database/sql`, escolhidos e importados pelo consumidor para SQLite3, MySQL e PostgreSQL.
- Dialetos específicos para os três bancos suportados.
- Controle explícito de transações.
- Migrações de avanço (`up`) e reversão (`down`).
- Construção encadeada de consultas `SELECT`, `INSERT`, `UPDATE` e `DELETE`, incluindo paginação.
- Execução de SQL fornecido pelo consumidor da biblioteca.
- Mapeamento de entidades, tabelas, junções e colunas, com acesso tipado aos valores, validação e formatação configuráveis.
- Distribuição como módulo Go hospedado no GitHub.

A adoção de esquemas preexistentes sem histórico de migrações do Krios não faz parte do escopo inicial.

## 4. Diretrizes técnicas

### 4.1. Linguagem e acesso a dados

A biblioteca deve ser desenvolvida em Go e utilizar o pacote padrão `database/sql` como base para acesso aos bancos de dados e controle de transações.

O consumidor deve escolher e importar o driver que deseja utilizar, compatível com `database/sql` e com um dos bancos suportados, e informar sua seleção na configuração. O Krios não deve importar os drivers automaticamente. O contrato de registro e identificação dos drivers será definido na especificação de acesso a dados.

### 4.2. Dialetos SQL

Cada banco suportado deve possuir um dialeto responsável por adaptar o SQL gerado às suas particularidades, incluindo marcadores de parâmetros, identificação de tabelas e colunas, funções e demais diferenças de sintaxe necessárias às operações suportadas.

Essa adaptação se aplica ao SQL construído pela biblioteca. O SQL fornecido pelo consumidor, inclusive em migrações, deve ser compatível com o banco configurado e não deve ter sua sintaxe traduzida entre dialetos.

### 4.3. Distribuição e consumo

O projeto deve ser distribuído como módulo Go, permitindo sua utilização por outros projetos mediante importação pelo caminho do repositório no GitHub.

A documentação deve explicar:

- Como o caminho do módulo se relaciona com o repositório e com os imports.
- Como instalar uma versão da biblioteca em um projeto consumidor.
- Como importar e utilizar a API pública.
- Como publicar e consumir versões do módulo.

O caminho definitivo do módulo e a política de versionamento serão definidos em uma especificação própria.

## 5. Requisitos funcionais

### RF-01 — Configurar a conexão

Antes de abrir o acesso ao banco e executar operações que dependam dele, o consumidor deve preparar a configuração da conexão.

O Krios deve disponibilizar a struct pública `krios.Config`, preenchida pelo consumidor em código e fornecida à biblioteca para inicializar o acesso. Ela deve centralizar o banco, o driver, os parâmetros de conexão, a porta quando aplicável, o tempo máximo de espera pela conexão e a configuração opcional de migrações.

O consumidor deve poder definir esses valores no código da aplicação e distribuí-la como executável sem arquivos externos de configuração. O Krios deve receber os valores por `krios.Config`, sem exigir leitura de arquivos ou variáveis de ambiente. Se a aplicação optar por outras fontes de valores, cabe a ela preencher a struct antes de entregá-la à biblioteca. Valores definidos diretamente no código acompanham a compilação; alterá-los no código exige gerar um novo executável.

| Banco      | Parâmetros obrigatórios                       |
| ---------- | --------------------------------------------- |
| SQLite3    | Caminho do arquivo do banco de dados          |
| MySQL      | Host, usuário, senha e nome do banco de dados |
| PostgreSQL | Host, usuário, senha e nome do banco de dados |

Os nomes e tipos dos campos de `krios.Config`, sua organização, as unidades de tempo, os valores padrão, a distinção entre valores omitidos e explícitos e as validações serão definidos na especificação de configuração.

### RF-02 — Gerenciar o ciclo de vida da conexão

A API pública deve disponibilizar:

- `Open`: validar a configuração, inicializar os recursos de acesso e verificar a conectividade com o banco de dados.
- `Close`: encerrar o acesso e liberar os recursos associados.

Se qualquer etapa de `Open` falhar, a biblioteca deve liberar os recursos criados e retornar erro. A verificação de conectividade deve aceitar cancelamento e prazo limite. Seu sucesso confirma o acesso naquele momento, mas não garante a disponibilidade futura do banco nem a existência do esquema esperado.

O cancelamento ou o esgotamento do tempo de espera deve ser comunicado como erro retornado por `Open`, para tratamento pelo consumidor. O mecanismo para solicitar cancelamento, as assinaturas e os tipos de erro serão definidos na especificação da API.

### RF-03 — Controlar transações

A biblioteca deve permitir iniciar, confirmar e reverter transações por meio das operações `Begin`, `Commit` e `Rollback`.

`Begin` deve retornar um objeto de transação por meio do qual o consumidor possa executar consultas e operações de escrita.

Fluxo conceitual:

```text
Obter um objeto de transação com Begin
Executar operações por meio desse objeto
Confirmar com Commit ou reverter com Rollback no objeto de transação
```

Apenas operações executadas por meio do objeto de transação devem participar dela. Iniciar uma transação não deve alterar o comportamento de operações executadas pela instância principal da biblioteca, inclusive quando ocorrerem de forma concorrente.

O consumidor deve poder executar operações sem abrir explicitamente uma transação, inclusive consultas `SELECT`. Quando precisar confirmar ou desfazer um conjunto de operações como uma unidade, deve associá-las à mesma transação.

Por exemplo, três inserts em tabelas relacionadas devem ser confirmados somente após o sucesso de todos. Se um deles falhar, as alterações ainda não confirmadas desse grupo devem ser revertidas. Operações executadas fora do grupo ou já confirmadas não participam dessa reversão. A responsabilidade por iniciar o rollback após uma falha e a política de execução simultânea dentro do grupo permanecem pendentes.

Após `Commit` ou `Rollback`, novas operações na transação devem retornar erro.

As assinaturas do objeto de transação, o comportamento de chamadas concorrentes no mesmo objeto e o tratamento de falhas na confirmação ou reversão serão definidos em uma especificação própria.

### RF-04 — Receber arquivos de migração

O consumidor deve fornecer arquivos `.sql` para as operações de avanço (`up`) e reversão (`down`).

Esses arquivos devem ser compatíveis com o banco configurado. O consumidor pode fornecer conjuntos distintos de arquivos para cada banco; a biblioteca não deve traduzir sua sintaxe entre dialetos.

A incorporação desses arquivos ao executável, por meio de `embed`, é responsabilidade do consumidor. A biblioteca deve disponibilizar uma forma de receber e carregar esses recursos.

Os arquivos devem seguir o padrão de nomes `0001_create_table.up.sql` e `0001_create_table.down.sql`, com prefixo numérico, descrição e indicação da direção. O consumidor deve informar em `krios.Config` a associação dos arquivos às versões do esquema.

A numeração no nome foi proposta como convenção de organização; ela não deve ser presumida como fonte definitiva da versão ou da ordem de execução. O formato da associação em `krios.Config`, a ordem dentro de cada versão, a validação dos pares `up`/`down` e o tratamento de divergências entre nomes e configuração permanecem pendentes.

A incorporação ocorre durante a compilação, por meio de `go:embed`. A configuração pode referenciar arquivos já incorporados, mas não adiciona novos arquivos ao executável em tempo de execução. Essa distinção segue o funcionamento do [pacote `embed`](https://pkg.go.dev/embed).

### RF-05 — Executar e controlar migrações

A configuração de migrações é opcional para o uso da biblioteca. O consumidor deve decidir quando executar as migrações, chamando `RunMigrations()`.

Ao chamar `RunMigrations()`, o consumidor deve ter informado explicitamente a versão desejada do esquema em `krios.Config` e fornecido os recursos de migração necessários conforme o contrato de configuração. A ausência da versão desejada deve produzir erro antes de executar migrações e não deve ser interpretada como uma solicitação de reversão.

A versão `0` representa o estado anterior à primeira migração. Informar explicitamente essa versão deve solicitar a reversão de todas as migrações aplicadas.

O Krios deve gerenciar uma tabela chamada `migrations`, responsável por registrar a versão aplicada e a data de aplicação.

Na ausência de histórico de migrações, a inicialização automática somente será permitida quando não existirem objetos de aplicação no esquema gerenciado. Caso existam, a biblioteca deve retornar erro antes de executar migrações. A adoção de esquemas preexistentes sem esse histórico não faz parte do escopo inicial.

A execução deve comparar a versão atual do banco com a versão desejada:

| Condição                                            | Comportamento esperado                                                      |
| --------------------------------------------------- | --------------------------------------------------------------------------- |
| Não há histórico e não existem objetos de aplicação | Considerar a versão atual como `0` e aplicar as regras de comparação abaixo |
| Não há histórico e existem objetos de aplicação     | Retornar erro sem executar migrações                                        |
| Versão atual inferior à desejada                    | Executar `up` até alcançar a versão desejada                                |
| Versão atual superior à desejada                    | Executar `down` até alcançar a versão desejada                              |
| Versão atual igual à desejada                       | Não executar migrações                                                      |

A estrutura da tabela, o registro do estado na versão `0`, a delimitação do esquema gerenciado, os objetos considerados na verificação inicial, a ordem de execução e o tratamento de falhas serão definidos na especificação de migrações.

A intenção é executar migrações em transações, revertendo as alterações do grupo em caso de falha e retornando um erro ao consumidor. Essa garantia depende do suporte transacional do banco e dos comandos executados. No MySQL, operações de definição de estrutura podem causar confirmação implícita e não ser desfeitas pelo rollback da aplicação, conforme a [documentação do MySQL](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html).

O alcance de cada transação, o registro de falhas e a recuperação de migrações parcialmente aplicadas devem ser definidos antes da implementação. Não se deve prometer rollback integral para comandos que não o suportem.

### RF-06 — Construir consultas de seleção

A biblioteca deve oferecer um construtor de consultas `SELECT`, permitindo representar:

- Campos selecionados e tabela de origem.
- Junções (`JOIN`).
- Condições de filtro e agrupamentos lógicos, incluindo alternativas com `OR`.
- Operadores de comparação, como igualdade e diferença.
- Busca textual com `LIKE`, distinguindo os modos “contém”, “começa com” e “termina com”.
- Funções de agregação, como `COUNT`, `SUM`, `MAX` e `MIN`.
- Ordenação (`ORDER BY`).
- Agrupamento (`GROUP BY`).
- Limite de resultados (`LIMIT`).
- Eliminação de duplicatas (`DISTINCT`).
- Paginação por limite e deslocamento (`offset`), com total de resultados calculado pela biblioteca e estatísticas de paginação.

O construtor deve utilizar o dialeto do banco configurado para gerar o SQL correspondente.

A construção deve partir de uma entidade mapeada e permitir encadear seleção de campos, filtros, ordenação e agrupamento. Sem seleção explícita de campos, devem ser selecionadas todas as colunas mapeadas, na ordem definida no mapeamento. A estrutura usada para guardar essa ordem ainda será definida.

Os nomes conceituais propostos incluem `Select`, `Only`, `Where`, `OrderBy`, `GroupBy`, `Distinct`, `Limit`, `Page`, `Count`, `Sum`, `Max` e `Min`. `Page(limit, offset)` deve produzir um resultado de paginação, proposto como `krios.Paginate`. Os campos desse resultado e o cálculo do total em consultas agrupadas ou com `DISTINCT` permanecem pendentes.

O exemplo abaixo expressa os recursos desejados e não representa uma assinatura definitiva da API Go:

```text
SELECT
  entidade: definição de tabela, junções e colunas
  resultado: representação de registros a definir no RF-09
  campos: id, name e funções de agregação, conforme a consulta
  origem: tabela
  junções: lista de junções
  filtros: condições simples e grupos lógicos
  ordenação: lista de campos e direções
  agrupamento: lista de campos
  limite: quantidade máxima de resultados
  paginação: limite, deslocamento, total e estatísticas
```

### RF-07 — Construir operações de escrita

A biblioteca deve oferecer construtores para `INSERT`, `UPDATE` e `DELETE`.

No escopo inicial:

- `INSERT` deve receber valores associados aos nomes dos campos, representando uma linha.
- `UPDATE` deve receber valores associados aos nomes dos campos e exigir filtros com `Where`, podendo atingir várias linhas.
- `DELETE` deve exigir filtros com `Where`, podendo atingir várias linhas.

Os construtores devem partir da entidade mapeada e permitir encadeamento. A operação conceitual `SetValues` deve atribuir os valores para inserção e atualização. Seu formato em Go e a manutenção da entrada por struct, prevista anteriormente, permanecem pendentes. Operações de `UPDATE` ou `DELETE` sem `Where` devem ser rejeitadas antes da execução.

Operações de escrita devem retornar informações sobre sua execução, incluindo a quantidade de linhas afetadas quando disponível.

O preenchimento de structs com campos gerados ou alterados pelo banco deve ser solicitado explicitamente e ter suporte documentado por banco. Quando solicitado e não suportado, deve produzir erro antes de executar a escrita.

A especificação dessas operações deve definir os campos cujo retorno é garantido e os destinos de resultados para escritas de uma ou de múltiplas linhas. Também deve distinguir campos omitidos de valores intencionais como `0`, `false` e `NULL` em atualizações.

### RF-08 — Executar SQL fornecido pelo consumidor

A biblioteca deve permitir a execução de arquivos `.sql` incorporados pelo consumidor via `embed`, inclusive para consultas complexas que não sejam expressas pelo construtor.

Esse recurso deve estar disponível além do mecanismo de migrações.

Os arquivos devem conter SQL compatível com o banco configurado, sem tradução de sintaxe pela biblioteca. O consumidor pode fornecer conjuntos distintos de arquivos para cada banco.

A execução deve permitir fornecer valores de parâmetros separadamente do texto SQL, utilizando os marcadores aceitos pelo driver configurado. Consultas de leitura devem permitir mapear seus resultados conforme o RF-09.

Deve ser possível definir um objeto que identifique um arquivo SQL incorporado e permita executá-lo. O nome do arquivo apenas identifica um recurso já incorporado pelo consumidor; aplica-se a distinção entre compilação e configuração descrita no RF-04.

A preferência é disponibilizar resultados compatíveis com `database/sql`. Os contratos de execução de escrita, consulta com retorno de linhas, mapeamento dos resultados e associação a transações permanecem pendentes; um método conceitual `Exec` ainda não resolve todos esses contratos.

### RF-09 — Mapear entidades, colunas e resultados

O consumidor deve poder descrever o mapeamento por meio de uma entidade, proposta como `krios.Entity`, contendo:

- Tabela de origem, com esquema e alias opcionais.
- Junções, cada uma identificando uma tabela e um ou mais critérios de associação.
- Colunas identificáveis por nome, com referências opcionais a esquema, tabela e alias. Na referência da coluna, somente seu nome é obrigatório.
- Função de validação por coluna, fornecida pelo consumidor, com indicação de validade e erro.
- Função de formatação por coluna, fornecida pelo consumidor, com retorno de texto e erro para apresentação do valor.

O acesso aos valores das colunas deve oferecer operações tipadas de leitura e atribuição, como texto e data/hora, além da leitura do valor formatado. Os nomes conceituais incluem `Column`, `GetString`, `SetString`, `GetTime`, `SetTime` e `GetFormatted`; as assinaturas definitivas serão especificadas posteriormente.

A proposta ainda não define onde os valores de cada registro são armazenados, como selecionar uma linha entre múltiplos resultados ou como representar `NULL`. Também permanece pendente se esse modelo substituirá ou complementará o requisito anterior de mapeamento automático para structs do consumidor, incluindo slices recebidos por ponteiro.

A estrutura que preservará a ordem das colunas, a correspondência com campos Go, as conversões de tipos, o momento de execução da validação e da formatação e os destinos de resultados de escrita serão definidos na especificação de mapeamento.

## 6. Definições pendentes

As respostas do consumidor foram incorporadas às seções anteriores. Esta seção reúne somente questões ainda não resolvidas. Os nomes de tipos e métodos propostos expressam a intenção da API, sem constituir assinaturas definitivas. Sugestões apresentadas durante a revisão não são decisões aprovadas automaticamente.

### 6.1. API pública e configuração

- Quais serão os campos e tipos de `krios.Config`, incluindo a organização das opções específicas de cada banco e das migrações?
- Quais campos serão obrigatórios e quais serão as portas padrão, as unidades e os valores padrão de tempo de espera?
- Como distinguir valores omitidos de valores explicitamente definidos, aplicar os padrões e retornar erros de configuração inválida?
- Como o consumidor solicita cancelamento antecipado de `Open`? Como esse mecanismo interage com o tempo de espera configurado?
- Quais serão os tipos exportados, as assinaturas e os erros distinguíveis pelo consumidor, incluindo cancelamento e prazo esgotado?

### 6.2. Drivers e dialetos

- Como identificar na configuração o driver importado pelo consumidor e o banco correspondente? Como validar incompatibilidades ou ausência de registro?
- Qual será a interface dos dialetos e como cada dialeto será selecionado ou associado ao driver?
- Quais drivers e versões dos bancos serão usados para validar o suporte? Quais capacidades serão garantidas por banco, especialmente retorno de campos e transações em migrações?

A responsabilidade de escolher e importar o driver já está definida como sendo do consumidor. O que permanece pendente é o contrato de integração e a estratégia dos dialetos.

### 6.3. Transações

- Como associar uma entidade, um construtor ou um arquivo SQL ao objeto de transação retornado por `Begin`?
- Operações na mesma transação devem ser sequenciais, serializadas pela biblioteca ou poderão ser solicitadas simultaneamente? Qual será o comportamento nesses casos?
- Após falha em uma operação do grupo, quem solicita o rollback: o consumidor ou um mecanismo da biblioteca? Como impedir a confirmação de um grupo que deveria ter sido revertido?
- Como comunicar e tratar falhas em `Commit` ou `Rollback`, inclusive quando não for possível confirmar o estado final das alterações?

A intenção já definida é confirmar ou reverter em conjunto operações da mesma transação, preservando a independência das operações executadas fora dela.

### 6.4. Migrações

- Como representar em `krios.Config` a versão desejada e a associação de arquivos a cada versão, distinguindo versão ausente de versão `0`?
- Qual será o papel do prefixo numérico dos arquivos diante dessa associação? Como tratar divergências, arquivos ausentes ou duplicados e pares `up`/`down` incompletos?
- Em qual ordem os arquivos de uma versão serão executados no avanço e na reversão?
- A transação abrangerá um arquivo, uma versão ou todo o percurso até a versão desejada?
- Como agir quando o banco ou os comandos não permitirem rollback integral? Como registrar falha, impedir que uma aplicação parcial seja considerada concluída e permitir recuperação?
- Qual será a estrutura da tabela `migrations`, incluindo histórico, datas e registro da versão `0`?
- Como delimitar o esquema gerenciado e quais objetos serão considerados na verificação de um banco sem histórico?
- Como `krios.Config` receberá ou referenciará os recursos incorporados pelo consumidor e como a biblioteca localizará os arquivos configurados?

O padrão de nomes, a associação de arquivos a versões por `krios.Config`, a execução explícita e a intenção de rollback em caso de falha já foram registrados nos RF-04 e RF-05. A garantia efetiva de reversão depende das limitações descritas nesses requisitos.

### 6.5. Consultas e paginação

- Quais serão as assinaturas e quais métodos apenas constroem a consulta ou efetivamente a executam? Como obter os resultados e erros?
- Condições passadas juntas a `Where` serão combinadas com `AND`? Como expressar grupos lógicos aninhados?
- Como diferenciar referências a colunas e valores de parâmetros, especialmente nas comparações de `JOIN`?
- Quais tipos de junção, operadores, funções e regras de busca textual serão suportados?
- Quais campos e estatísticas estarão presentes em `krios.Paginate`? Como validar limite e deslocamento e calcular o total com filtros, agrupamentos e `DISTINCT`?

O encadeamento, a seleção de todas as colunas mapeadas por padrão, sua ordem definida no mapeamento e os recursos listados no RF-06 já fazem parte do escopo.

### 6.6. Escritas

- Qual será o formato de `SetValues` em Go? A entrada por struct continuará disponível além da atribuição por nome de campo?
- Como distinguir campo omitido, valor zero, `false` e `NULL`? Como representar explicitamente a intenção de gravar cada um deles?
- O que será considerado um `Where` válido para autorizar `UPDATE` e `DELETE`, inclusive diante de um grupo de condições vazio?
- Quais informações serão retornadas pela execução e como indicar que a quantidade de linhas afetadas não está disponível?
- Como solicitar campos gerados ou alterados, quais bancos oferecerão esse retorno e quais destinos receberão resultados de uma ou de múltiplas linhas?

A exigência de `Where` em atualizações e exclusões e o retorno de campos apenas quando solicitado e suportado já estão definidos no RF-07.

### 6.7. Mapeamento, validação e formatação

- Como representar a definição da entidade e os valores de cada registro? Como acessar um registro específico entre vários resultados?
- O modelo de entidades substituirá ou complementará o mapeamento automático para structs do consumidor? Caso esse suporte permaneça, quais serão as regras de tags, correspondência de campos e preenchimento de slices por ponteiro?
- Como representar `NULL`, diferenciar coluna não selecionada de valor nulo e tratar conversões incompatíveis nos acessos tipados?
- Qual estrutura preservará a ordem das colunas e permitirá sua busca por nome? Um `map` isolado não garante ordem de iteração, conforme a [especificação de Go](https://go.dev/ref/spec#For_statements).
- Quando a validação será executada: na atribuição, antes da escrita ou também na leitura? Como comunicar valor inválido e erro da função de validação?
- Quando a formatação será executada e como seu resultado se relacionará com o valor original utilizado nas operações de banco?

As entidades, junções, metadados das colunas, funções fornecidas pelo consumidor e operações tipadas já estão definidos como capacidades no RF-09.

### 6.8. SQL incorporado

- Qual será o tipo público que referencia o arquivo e como ele receberá os recursos incorporados e os parâmetros?
- Como distinguir execução de escrita e consulta que devolve linhas? Quais tipos de `database/sql` serão expostos e quem encerrará os recursos de leitura?
- Como combinar os resultados nativos com o mapeamento do RF-09 e executar essas operações na transação selecionada?

O consumidor continua responsável pela incorporação dos arquivos durante a compilação. O nome fornecido na configuração apenas seleciona um recurso já incorporado.

### 6.9. Distribuição

- Qual será o caminho definitivo do módulo?
- Qual será a política de versionamento e de estabilidade da API pública?
- Qual será o procedimento de publicação e quais exemplos de instalação e importação serão fornecidos?

O consumidor solicitou orientação sobre esses pontos; ainda não há uma escolha definitiva registrada.

## 7. Critérios de atendimento

O escopo descrito neste documento será atendido quando for possível:

1. Importar a biblioteca em outro projeto Go pelo caminho do módulo publicado no GitHub.
2. Configurar o acesso por `krios.Config`, sem exigir arquivos externos de configuração, incluindo driver, parâmetros de conexão e tempo de espera, e utilizar cada banco suportado com um driver escolhido e importado pelo consumidor.
3. Abrir o acesso com verificação de conectividade, cancelamento e prazo limite, receber erro em caso de cancelamento ou prazo esgotado, liberar recursos em falhas e encerrar o acesso pela API pública.
4. Executar operações por meio de um objeto de transação, confirmar ou reverter o grupo, manter operações externas independentes e rejeitar novas operações na transação encerrada.
5. Utilizar a biblioteca sem configurar migrações e, ao executá-las, exigir uma versão desejada explícita, receber a associação de arquivos às versões por `krios.Config` e avançar ou reverter até a versão desejada, inclusive `0`, usando arquivos incorporados pelo consumidor.
6. Construir operações de leitura e escrita por encadeamento a partir de entidades mapeadas, utilizando o dialeto correspondente ao banco configurado.
7. Selecionar campos, filtrar, ordenar, agrupar, agregar, eliminar duplicatas e paginar consultas com total e estatísticas, preservando a ordem definida das colunas quando não houver seleção explícita.
8. Fornecer valores por nome de campo para inserções e atualizações, permitir atualizações e exclusões de múltiplas linhas e rejeitar `UPDATE` e `DELETE` sem `Where` antes da execução.
9. Executar SQL personalizado compatível com o banco a partir de recursos incorporados, recebendo parâmetros separadamente e disponibilizando os resultados conforme os contratos de consulta, execução e mapeamento a definir.
10. Definir entidades, tabelas, junções e colunas, oferecer acesso tipado aos valores e aplicar validação e formatação fornecidas pelo consumidor conforme os contratos a definir.
11. Inicializar migrações sem histórico somente na ausência de objetos de aplicação no esquema gerenciado e rejeitar esquemas preexistentes sem histórico antes de executar migrações.
12. Retornar erro quando uma migração falhar e reverter as alterações quando houver suporte transacional, tratando aplicações parciais conforme a política de recuperação a definir.
13. Retornar informações sobre linhas afetadas quando disponíveis, permitir a solicitação explícita de campos gerados ou alterados em escritas e rejeitar solicitações não suportadas antes da execução.

Esses critérios orientam as especificações seguintes; sua validação depende da definição dos contratos pendentes na seção 6.
