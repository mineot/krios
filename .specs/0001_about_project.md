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

- Configuração, abertura e encerramento de conexões.
- Integração dos drivers de SQLite3, MySQL e PostgreSQL.
- Dialetos específicos para os três bancos suportados.
- Controle explícito de transações.
- Migrações de avanço (`up`) e reversão (`down`).
- Construção de consultas `SELECT`, `INSERT`, `UPDATE` e `DELETE`.
- Execução de SQL fornecido pelo consumidor da biblioteca.
- Mapeamento automático entre resultados de consultas e structs Go.
- Distribuição como módulo Go hospedado no GitHub.

A adoção de esquemas preexistentes sem histórico de migrações do Krios não faz parte do escopo inicial.

## 4. Diretrizes técnicas

### 4.1. Linguagem e acesso a dados

A biblioteca deve ser desenvolvida em Go e utilizar o pacote padrão `database/sql` como base para acesso aos bancos de dados e controle de transações.

Os drivers específicos dos bancos devem ser integrados pela biblioteca. O consumidor deve selecionar o banco na configuração, sem precisar importar ou registrar manualmente o driver correspondente.

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

O consumidor deve informar o banco de dados utilizado e os parâmetros de conexão correspondentes.

| Banco      | Parâmetros obrigatórios                       |
| ---------- | --------------------------------------------- |
| SQLite3    | Caminho do arquivo do banco de dados          |
| MySQL      | Host, usuário, senha e nome do banco de dados |
| PostgreSQL | Host, usuário, senha e nome do banco de dados |

Parâmetros adicionais, como porta e opções de conexão, serão definidos na especificação de configuração.

### RF-02 — Gerenciar o ciclo de vida da conexão

A API pública deve disponibilizar:

- `Open`: validar a configuração, inicializar os recursos de acesso e verificar a conectividade com o banco de dados.
- `Close`: encerrar o acesso e liberar os recursos associados.

Se qualquer etapa de `Open` falhar, a biblioteca deve liberar os recursos criados e retornar erro. A verificação de conectividade deve aceitar cancelamento e prazo limite. Seu sucesso confirma o acesso naquele momento, mas não garante a disponibilidade futura do banco nem a existência do esquema esperado.

As assinaturas e o tratamento de erros dessas operações serão definidos na especificação da API.

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

Após `Commit` ou `Rollback`, novas operações na transação devem retornar erro.

As assinaturas do objeto de transação, o comportamento de chamadas concorrentes no mesmo objeto e o tratamento de falhas na confirmação ou reversão serão definidos em uma especificação própria.

### RF-04 — Receber arquivos de migração

O consumidor deve fornecer arquivos `.sql` para as operações de avanço (`up`) e reversão (`down`).

Esses arquivos devem ser compatíveis com o banco configurado. O consumidor pode fornecer conjuntos distintos de arquivos para cada banco; a biblioteca não deve traduzir sua sintaxe entre dialetos.

A incorporação desses arquivos ao executável, por meio de `embed`, é responsabilidade do consumidor. A biblioteca deve disponibilizar uma forma de receber e carregar esses recursos.

Convenções de nomes, organização dos arquivos e associação entre arquivos e versões serão definidas na especificação de migrações.

### RF-05 — Executar e controlar migrações

A configuração de migrações é opcional para o uso da biblioteca. O consumidor deve decidir quando executar as migrações, chamando `RunMigrations()`.

Ao chamar `RunMigrations()`, o consumidor deve ter informado explicitamente a versão desejada do esquema na configuração e os recursos de migração necessários. A ausência da versão desejada deve produzir erro antes de executar migrações e não deve ser interpretada como uma solicitação de reversão.

A versão `0` representa o estado anterior à primeira migração. Informar explicitamente essa versão deve solicitar a reversão de todas as migrações aplicadas.

O Krios deve gerenciar uma tabela chamada `migrations`, responsável por registrar a versão aplicada e a data de aplicação.

Na ausência de histórico de migrações, a inicialização automática somente será permitida quando não existirem objetos de aplicação no esquema gerenciado. Caso existam, a biblioteca deve retornar erro antes de executar migrações. A adoção de esquemas preexistentes sem esse histórico não faz parte do escopo inicial.

A execução deve comparar a versão atual do banco com a versão desejada:

| Condição                         | Comportamento esperado                                                 |
| -------------------------------- | ---------------------------------------------------------------------- |
| Não há histórico e não existem objetos de aplicação | Considerar a versão atual como `0` e aplicar as regras de comparação abaixo |
| Não há histórico e existem objetos de aplicação | Retornar erro sem executar migrações |
| Versão atual inferior à desejada | Executar `up` até alcançar a versão desejada                           |
| Versão atual superior à desejada | Executar `down` até alcançar a versão desejada                         |
| Versão atual igual à desejada    | Não executar migrações                                                 |

A estrutura da tabela, o registro do estado na versão `0`, a delimitação do esquema gerenciado, os objetos considerados na verificação inicial, a ordem de execução e o tratamento de falhas serão definidos na especificação de migrações.

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

O construtor deve utilizar o dialeto do banco configurado para gerar o SQL correspondente.

O exemplo abaixo expressa os recursos desejados e não representa uma assinatura definitiva da API Go:

```text
SELECT
  destino: coleção de structs fornecida por ponteiro
  campos: id, name e funções de agregação, conforme a consulta
  origem: tabela
  junções: lista de junções
  filtros: condições simples e grupos lógicos
  ordenação: lista de campos e direções
  agrupamento: lista de campos
  limite: quantidade máxima de resultados
```

### RF-07 — Construir operações de escrita

A biblioteca deve oferecer construtores para `INSERT`, `UPDATE` e `DELETE`.

No escopo inicial:

- `INSERT` deve receber uma struct representando uma linha.
- `UPDATE` deve receber os campos a alterar e os filtros, podendo atingir várias linhas.
- `DELETE` deve receber filtros, podendo atingir várias linhas.

Operações de escrita devem retornar informações sobre sua execução, incluindo a quantidade de linhas afetadas quando disponível.

O preenchimento de structs com campos gerados ou alterados pelo banco deve ser solicitado explicitamente e ter suporte documentado por banco. Quando solicitado e não suportado, deve produzir erro antes de executar a escrita.

A especificação dessas operações deve definir os campos cujo retorno é garantido e os destinos de resultados para escritas de uma ou de múltiplas linhas. Também deve distinguir campos omitidos de valores intencionais como `0`, `false` e `NULL` em atualizações.

### RF-08 — Executar SQL fornecido pelo consumidor

A biblioteca deve permitir a execução de arquivos `.sql` incorporados pelo consumidor via `embed`, inclusive para consultas complexas que não sejam expressas pelo construtor.

Esse recurso deve estar disponível além do mecanismo de migrações.

Os arquivos devem conter SQL compatível com o banco configurado, sem tradução de sintaxe pela biblioteca. O consumidor pode fornecer conjuntos distintos de arquivos para cada banco.

A execução deve permitir fornecer valores de parâmetros separadamente do texto SQL, utilizando os marcadores aceitos pelo driver configurado. Consultas de leitura devem permitir mapear seus resultados conforme o RF-09.

### RF-09 — Mapear resultados para structs

O mapeamento dos resultados deve ser automático, utilizando structs Go como representação dos dados.

Para consultas `SELECT`, a API deve permitir informar um destino por ponteiro e obter uma coleção de resultados, representada por um slice de structs.

As regras de correspondência entre colunas e campos, uso de tags, valores nulos e conversão de tipos serão definidas em uma especificação própria.

## 6. Definições pendentes

As próximas especificações devem detalhar:

| Tema               | Definições necessárias                                                               |
| ------------------ | ------------------------------------------------------------------------------------ |
| API pública        | Tipos exportados, assinaturas, erros, cancelamento, prazos limite e exemplos válidos em Go |
| Configuração       | Parâmetros adicionais, valores padrão e validações                                   |
| Drivers e dialetos | Seleção dos drivers e diferenças entre os bancos suportados                          |
| Transações         | Assinaturas do objeto de transação, concorrência no mesmo objeto e falhas em `Commit` ou `Rollback` |
| Migrações          | Convenções dos arquivos, histórico de versões, registro da versão `0`, esquema gerenciado, verificação inicial e tratamento de falhas |
| Consultas          | Composição de expressões, parâmetros, funções e operadores                           |
| Escritas           | Campos omitidos e valores explícitos, informações de execução e suporte ao retorno de campos por banco |
| Mapeamento         | Tags, tipos, valores nulos e destinos de resultados de escritas de uma ou de múltiplas linhas |
| Distribuição       | Caminho do módulo, publicação, versionamento e guia de consumo                       |

## 7. Critérios de atendimento

O escopo descrito neste documento será atendido quando for possível:

1. Importar a biblioteca em outro projeto Go pelo caminho do módulo publicado no GitHub.
2. Configurar e utilizar cada um dos três bancos suportados sem registrar drivers manualmente.
3. Abrir o acesso com verificação de conectividade, cancelamento e prazo limite, liberar recursos em caso de falha e encerrar o acesso pela API pública.
4. Executar operações por meio de um objeto de transação e confirmar ou reverter seus efeitos, mantendo operações da instância principal independentes e rejeitando novas operações na transação encerrada.
5. Utilizar a biblioteca sem configurar migrações e, ao executá-las, exigir uma versão desejada explícita, avançar ou reverter até ela e permitir retornar à versão `0` por meio de arquivos SQL incorporados pelo consumidor.
6. Construir operações de leitura e escrita utilizando o dialeto correspondente ao banco configurado, incluindo atualizações e exclusões de múltiplas linhas e informações sobre linhas afetadas quando disponíveis.
7. Executar SQL personalizado compatível com o banco a partir de recursos incorporados, recebendo parâmetros separadamente e permitindo mapear os resultados de leitura.
8. Mapear resultados para structs, incluindo coleções em consultas de seleção.
9. Inicializar migrações sem histórico somente na ausência de objetos de aplicação no esquema gerenciado e rejeitar esquemas preexistentes sem histórico antes de executar migrações.
10. Solicitar explicitamente o retorno de campos gerados ou alterados em escritas, conforme o suporte documentado por banco, e rejeitar solicitações não suportadas antes de executar a escrita.

Esses critérios orientam as especificações seguintes; sua validação depende da definição dos contratos pendentes.
