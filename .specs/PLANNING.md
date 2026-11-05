# Plano de implementação do Krios

**Estado: escopo e guia de implementação aprovados pelo desenvolvedor em 2026-11-04.**

Este documento organiza o escopo das implementações e sua sequência sugerida. O contrato funcional está em [DEFINITIONS.md](DEFINITIONS.md), e as decisões da revisão estão em [QUESTIONS.md](QUESTIONS.md). Este plano não altera esses documentos nem autoriza o início da implementação.

## Forma de execução

- Executar uma tarefa por vez, após aprovação do desenvolvedor.
- Ao iniciar a preparação de uma tarefa, detalhá-la em `.specs/NNN.md`, usando sua numeração neste plano: tarefa 1 em `001.md`, tarefa 2 em `002.md` e assim por diante. Criar somente o detalhamento da tarefa em preparação.
- O detalhamento deverá apresentar as decisões técnicas propostas, os limites da tarefa, os critérios de conclusão e as verificações necessárias. A implementação dependerá da aprovação desse detalhamento.
- Registrar dúvidas e respostas em `QUESTIONS.md`, separadas por tarefa. Havendo dúvida que exija decisão, interromper a execução e aguardar a orientação do desenvolvedor.
- Disponibilizar os recursos públicos por `krios.go` e manter novos arquivos de implementação e testes em `internal`, respeitando as regras de `AGENTS.md`.
- Incluir em cada tarefa as verificações do comportamento que ela introduz. A integração final complementará essas verificações com os bancos e drivers escolhidos.

## Sequência de tarefas

### 001 — Configuração e ciclo de vida do cliente

**Escopo:** estabelecer o cliente, a configuração e os tipos públicos básicos previstos no contrato; validar a criação, realizar o ping e organizar as categorias de erro. Preparar o tratamento compartilhado de contexto, timeout e execução pelo pool ou pela transação recebida, incluindo a responsabilidade pelos recursos retornados.

**Dependências:** nenhuma.

### 002 — Dialetos, identificadores e expressões

**Escopo:** preparar a geração de SQL para SQLite, PostgreSQL e MySQL, com escape de identificadores, referências simples e por componentes, expressões de agregação e placeholders. Organizar a montagem de SQL e argumentos separados, com numeração própria por comando e preservação dos valores enviados ao driver.

**Dependências:** 001.

### 003 — Critérios e composição lógica

**Escopo:** implementar os predicados de comparação, NULL, listas, intervalos e grupos lógicos previstos no contrato. Cobrir a composição por AND/OR e suas validações estruturais, preservando a semântica aprovada para valores nulos e listas vazias. A associação de subconsultas a IN/NOT IN será concluída na tarefa 006.

**Dependências:** 002.

### 004 — SELECT básico e estado dos builders

**Escopo:** implementar SELECT com origem principal, schema, aliases, projeção implícita e explícita, DISTINCT, filtros e ordenação. Estabelecer as regras de encadeamento, acumulação, substituição, reutilização sequencial e registro persistente de erros estruturais. Disponibilizar execução pelo pool e por transação, com os retornos de `database/sql` e o ciclo de vida de contexto definido no contrato.

**Dependências:** 003.

### 005 — JOIN, agregações e agrupamento

**Escopo:** ampliar SELECT com os tipos de JOIN aprovados, condições ON, agregadores de projeção, GROUP BY e HAVING. Aplicar as regras de alias, comparação entre colunas, encadeamento e validação das combinações representadas pela API.

**Dependências:** 004.

### 006 — Subconsultas, UNION e Wrapper

**Escopo:** implementar subconsultas na projeção, no FROM e em IN/NOT IN, além de UNION, UNION ALL e Wrapper. Abranger referências correlacionadas permitidas, aliases obrigatórios, composição de argumentos e preservação das cláusulas locais dos filhos. Tratar referências compartilhadas, alterações nos filhos, ciclos e combinação de componentes de clientes diferentes.

**Dependências:** 005.

### 007 — Paginação, contagem e navegação

**Escopo:** implementar paginação por offset e limite, metadados de `Page`, contagem do resultado completo e navegação com Next/Prev. Cobrir ORDER BY obrigatório, cálculos com detecção de overflow, invalidação dos totais e reinício do offset após alterações na consulta ou em filhos. Garantir o compartilhamento de executor e prazo entre contagem e seleção, preservando os limites internos das consultas compostas.

**Dependências:** 006.

### 008 — INSERT, UPDATE e DELETE

**Escopo:** implementar os builders de escrita, incluindo INSERT com múltiplas linhas, atribuições de UPDATE e filtros de UPDATE/DELETE. Aplicar as validações estruturais aprovadas, a exigência de WHERE e a ordem dos argumentos. Disponibilizar execução pelo pool e por transação, preservando `sql.Result` e as causas dos erros.

**Dependências:** 003 e 004.

### 009 — Execução de arquivos SQL

**Escopo:** implementar File, resolução e armazenamento do caminho, leitura e validação do arquivo a cada execução. Disponibilizar Exec, ExecTx, Query e QueryTx, enviando o conteúdo integral e os argumentos ao driver. Cobrir contexto, timeout, resultados, identificação de falhas e responsabilidades do consumidor das linhas.

**Dependências:** 001 e 002.

### 010 — Cadastro, histórico e planejamento de migrações

**Escopo:** implementar a validação do cadastro de migrations, da versão alvo e do histórico aplicado. Preparar a criação e inspeção da tabela de controle nos três dialetos, a identificação de históricos divergentes e o cálculo dos passos UP/DOWN. Abranger a leitura prévia de todos os scripts necessários e a validação do plano antes de sua execução.

**Dependências:** 009.

### 011 — Execução e falhas de migrações

**Escopo:** implementar Migrate e MigrateTx sobre o plano validado, incluindo passos transacionais e não transacionais, atualização do histórico e execução na ordem prevista. Cobrir atomicidade por migration quando aplicável, uso exclusivo da transação externa, conexão reservada para passos não transacionais, timeout da chamada completa e preservação de erros de execução e encerramento. Respeitar os limites aprovados de recuperação manual e coordenação externa entre executores.

**Dependências:** 010.

### 012 — Integração com bancos e drivers

**Escopo:** escolher e fixar, mediante aprovação no detalhamento, os drivers e as versões de integração. Verificar o contrato nos mínimos definidos de Go, SQLite, PostgreSQL e MySQL, cobrindo consultas, escritas, paginação, transações, arquivos SQL e migrações. Comprovar especialmente placeholders, consultas compostas, múltiplos comandos, propagação de erros, cancelamento e formatos do histórico. Registrar os limites observados de cada combinação de banco, driver e configuração.

**Dependências:** 007, 008, 009 e 011.

### 013 — Documentação de uso e revisão da API pública

**Escopo:** revisar a exposição dos recursos por `krios.go` e consolidar orientações e exemplos de uso compatíveis com a implementação verificada. Documentar configuração, drivers, consumo de resultados, transações, paginação, arquivos SQL, migrações e suas limitações aprovadas. Conferir a cobertura do contrato e identificar eventuais pendências antes de considerar a implementação inicial concluída.

**Dependências:** 012.

## Guia de acompanhamento

A ordem numérica é a sequência sugerida de execução. As dependências indicam os recursos necessários a cada tarefa; não autorizam execução simultânea.

O escopo das 13 tarefas está aprovado. A aprovação deste plano não substitui a aprovação do detalhamento de cada tarefa. A conclusão dependerá dos critérios aprovados no respectivo arquivo numerado.

| Tarefa  | Situação                                                                     |
| ------- | ---------------------------------------------------------------------------- |
| 001     | Concluída e verificada em 2026-11-04. Resultados em [001.md](001.md).        |
| 002     | Concluída e verificada em 2026-11-04. Resultados em [002.md](002.md).        |
| 003     | Detalhamento aprovado em [003.md](003.md); implementação em andamento. |
| 004–013 | Escopo aprovado; aguardando preparação individual.                           |

Nomes de tipos concretos, organização dos pacotes internos, assinaturas ainda não fixadas pelo contrato, ferramentas de teste e configuração dos ambientes serão definidos no detalhamento da tarefa correspondente. Mudanças de escopo ou de sequência serão submetidas ao desenvolvedor antes da execução.
