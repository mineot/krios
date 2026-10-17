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

## 4. Diretrizes técnicas

### 4.1. Linguagem e acesso a dados

A biblioteca deve ser desenvolvida em Go e utilizar o pacote padrão `database/sql` como base para acesso aos bancos de dados e controle de transações.

Os drivers específicos dos bancos devem ser integrados pela biblioteca. O consumidor deve selecionar o banco na configuração, sem precisar importar ou registrar manualmente o driver correspondente.

### 4.2. Dialetos SQL

Cada banco suportado deve possuir um dialeto responsável por adaptar o SQL gerado às suas particularidades, incluindo marcadores de parâmetros, identificação de tabelas e colunas, funções e demais diferenças de sintaxe necessárias às operações suportadas.

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

- `Open`: inicializar o acesso ao banco de dados com base na configuração fornecida.
- `Close`: encerrar o acesso e liberar os recursos associados.

As assinaturas e o tratamento de erros dessas operações serão definidos na especificação da API.

### RF-03 — Controlar transações

A biblioteca deve permitir iniciar, confirmar e reverter transações por meio das operações `Begin`, `Commit` e `Rollback`.

Fluxo conceitual:

```text
Iniciar transação com Begin
Executar operações no escopo da transação
Confirmar com Commit ou reverter com Rollback
```

Todas as operações realizadas no escopo da transação após `Begin` devem participar dela até sua confirmação ou reversão.

A forma de associar as operações à transação deve ser definida em uma especificação própria, incluindo o comportamento de operações concorrentes. A intenção original de aplicar a transação às operações posteriores a `Begin` deve ser traduzida em um contrato explícito da API.

### RF-04 — Receber arquivos de migração

O consumidor deve fornecer arquivos `.sql` para as operações de avanço (`up`) e reversão (`down`).

A incorporação desses arquivos ao executável, por meio de `embed`, é responsabilidade do consumidor. A biblioteca deve disponibilizar uma forma de receber e carregar esses recursos.

Convenções de nomes, organização dos arquivos e associação entre arquivos e versões serão definidas na especificação de migrações.

### RF-05 — Executar e controlar migrações

O consumidor deve informar a versão desejada do esquema na configuração e decidir quando executar as migrações, chamando `RunMigrations()`.

O Krios deve gerenciar uma tabela chamada `migrations`, responsável por registrar a versão aplicada e a data de aplicação.

A execução deve comparar a versão atual do banco com a versão desejada:

| Condição                         | Comportamento esperado                                                 |
| -------------------------------- | ---------------------------------------------------------------------- |
| Não há versão registrada         | Executar as migrações `up` necessárias para alcançar a versão desejada |
| Versão atual inferior à desejada | Executar `up` até alcançar a versão desejada                           |
| Versão atual superior à desejada | Executar `down` até alcançar a versão desejada                         |
| Versão atual igual à desejada    | Não executar migrações                                                 |

A estrutura da tabela, a representação do estado inicial, a ordem de execução e o tratamento de falhas serão definidos na especificação de migrações.

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

Para `INSERT` e `UPDATE`, deve ser possível fornecer uma struct com os dados da operação e receber nela os valores resultantes, conforme o contrato a ser definido para cada banco.

O comportamento de retorno de campos gerados ou alterados pelo banco será detalhado na especificação dessas operações.

### RF-08 — Executar SQL fornecido pelo consumidor

A biblioteca deve permitir a execução de arquivos `.sql` incorporados pelo consumidor via `embed`, inclusive para consultas complexas que não sejam expressas pelo construtor.

Esse recurso deve estar disponível além do mecanismo de migrações.

### RF-09 — Mapear resultados para structs

O mapeamento dos resultados deve ser automático, utilizando structs Go como representação dos dados.

Para consultas `SELECT`, a API deve permitir informar um destino por ponteiro e obter uma coleção de resultados, representada por um slice de structs.

As regras de correspondência entre colunas e campos, uso de tags, valores nulos e conversão de tipos serão definidas em uma especificação própria.

## 6. Definições pendentes

As próximas especificações devem detalhar:

| Tema               | Definições necessárias                                                               |
| ------------------ | ------------------------------------------------------------------------------------ |
| API pública        | Tipos exportados, assinaturas, erros e exemplos válidos em Go                        |
| Configuração       | Parâmetros adicionais, valores padrão e validações                                   |
| Drivers e dialetos | Seleção dos drivers e diferenças entre os bancos suportados                          |
| Transações         | Associação das operações à transação e comportamento concorrente                     |
| Migrações          | Convenções dos arquivos, histórico de versões, estado inicial e tratamento de falhas |
| Consultas          | Composição de expressões, parâmetros, funções e operadores                           |
| Mapeamento         | Tags, tipos, valores nulos e atualização de structs após operações de escrita        |
| Distribuição       | Caminho do módulo, publicação, versionamento e guia de consumo                       |

## 7. Critérios de atendimento

O escopo descrito neste documento será atendido quando for possível:

1. Importar a biblioteca em outro projeto Go pelo caminho do módulo publicado no GitHub.
2. Configurar e utilizar cada um dos três bancos suportados sem registrar drivers manualmente.
3. Abrir e encerrar o acesso ao banco pela API pública.
4. Executar operações em uma transação e confirmar ou reverter seus efeitos.
5. Avançar ou reverter o esquema até a versão desejada por meio de arquivos SQL incorporados pelo consumidor.
6. Construir operações de leitura e escrita utilizando o dialeto correspondente ao banco configurado.
7. Executar SQL personalizado a partir de recursos incorporados.
8. Mapear resultados para structs, incluindo coleções em consultas de seleção.

Esses critérios orientam as especificações seguintes; sua validação depende da definição dos contratos pendentes.
