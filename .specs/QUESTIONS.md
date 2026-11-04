# Dúvidas por tarefa

## Tarefa: revisão técnica de DEFINITIONS.md — 2026-11-04

Estado: revisão em validação pelo desenvolvedor. As respostas originais foram preservadas; os itens 3, 4 e 5 têm aprovação explícita. As novas sugestões e os complementos abaixo são propostas, não decisões aprovadas nem autorização de implementação. Referências às linhas de DEFINITIONS.md correspondem à revisão original e podem ter mudado. Esta revisão não atribui numeração às futuras tarefas de implementação.

Atualização de 2026-11-04: sugestões acrescentadas aos itens 6–20 e pontos adicionais registrados nos itens 21–24. Preencher os campos de validação para aprovar ou ajustar cada proposta. DEFINITIONS.md permanece inalterado.

### Configuração e execução

1. **API de configuração (linhas 15–38):** o campo `timeout` deve ser exportado como `Timeout`? Qual será a assinatura de `New` para retornar erros de configuração e de conexão?

- Resposta: Timeout foi ajustado, e foi corrigido sobre o erro, que deve retornar e ser tratado pelo utilizador.

2. **Conexão (linhas 5–12):** a validação será feita na criação, antes de cada execução ou em outro momento? Quem será responsável por fechar o `*sql.DB` recebido? Quais drivers e versões mínimas de Go e dos bancos serão suportados?

- Resposta: A validação será feito no momento da criação, do `New`, a versão minima do golang será a 1.26+, PostgreSQL 16+, MySQL 8+ e SQLite 3.39+. O utilizador é responsavel por fechar o `*sql.DB`.

3. **Contexto e timeout (linhas 19–20):** haverá contexto por operação? Como tratar timeout zero ou negativo, contexto cancelado e prazos já existentes? O timeout cobre ping, contagem da paginação, leitura de linhas e migrações? Como manter o contexto válido enquanto o consumidor lê `*sql.Rows`?

- Resposta: A sugestão da IA foi aprovado.

**Resposta sugerida pela IA — aprovada pelo desenvolvedor:**

- Usar `Config.Context` nas operações do cliente, sem contexto adicional nos argumentos de `Exec()` e `ExecTx(tx)`.
- `Timeout` negativo gera erro de configuração em `New`; zero usa somente o prazo/cancelamento de `Config.Context`; positivo cria um contexto derivado por operação. Havendo prazo anterior no contexto original, prevalece o que vencer primeiro.
- O timeout começa na execução, não na criação do builder. Cobre o ping em `New`, toda a execução de escritas e a chamada completa de migração, sem reiniciar a contagem por comando. Em SELECT paginado, contagem e consulta compartilham o mesmo prazo, que também abrange a leitura das linhas.
- Contexto cancelado impede iniciar novos comandos e retorna erro. O cancelamento de `Config.Context` afeta também as próximas operações do mesmo cliente. Propagar o contexto às chamadas de `database/sql`.
- Ao retornar `*sql.Rows`, não cancelar o contexto derivado na saída de `Exec`: ele precisa continuar válido enquanto o consumidor lê as linhas. Em falha anterior à entrega das linhas, cancelar imediatamente; nas operações sem linhas, liberar o contexto derivado ao término.
- Como `*sql.Rows` é retornado diretamente, o Krios não intercepta `Rows.Close()` para liberar seu timer. A proposta mantém `Timeout` automático e aceita que, em sucesso, o timer possa permanecer até o prazo vencer, mesmo após o fechamento das linhas. Documentar esse comportamento.
- Alternativa apresentada, não escolhida: a aplicação controlar integralmente o prazo pelo contexto e chamar `cancel()` após consumir as linhas, permitindo dispensar `Timeout` na configuração. A aprovação acima mantém `Timeout` conforme a recomendação principal.

Referência: [Cancelamento de operações no Go](https://go.dev/doc/database/cancel-operations).

4. **Erros e retornos (linhas 10–12, 188–205 e 283–285):** quais serão as assinaturas completas, incluindo ponteiros, de `Exec`, `ExecTx`, `Migrate` e `MigrateTx`? Qual valor de `Page` será retornado sem paginação? Os erros do banco serão preservados para `errors.Is`/`errors.As`? O consumidor ficará responsável por `Rows.Close`, `Rows.Scan` e `Rows.Err`?

- Resposta: Porem o valor de page quando não houver paginação deve ser nil, e sim, os erros do banco devem ser preservados e retornados para o utilizados, e sim, o consumidor ficará responsavel por tratar tudo referente ao `Rows`. A sugestão da IA foi aprovada

**Resposta sugerida pela IA — complemento aprovado pelo desenvolvedor:**

As propostas abaixo preservam as respostas do desenvolvedor sobre `page == nil`, preservação dos erros e responsabilidade do consumidor por `Rows`. Os nomes dos tipos dos builders ainda poderão ser definidos; `Client` é o nome proposto para o tipo retornado por `New`.

| Operação                | Assinatura sugerida                            |
| ----------------------- | ---------------------------------------------- |
| Criação                 | `New(config Config) (*Client, error)`          |
| SELECT                  | `Exec() (*sql.Rows, *Page, error)`             |
| SELECT em transação     | `ExecTx(tx *sql.Tx) (*sql.Rows, *Page, error)` |
| INSERT, UPDATE e DELETE | `Exec() (sql.Result, error)`                   |
| Escritas em transação   | `ExecTx(tx *sql.Tx) (sql.Result, error)`       |
| Migração                | `Migrate() error`                              |
| Migração em transação   | `MigrateTx(tx *sql.Tx) error`                  |

- Falha em `New`: retornar cliente `nil` e erro.
- SELECT bem-sucedido sem paginação: `(rows, nil, nil)`; com paginação: `(rows, page, nil)`.
- Falha de validação, contagem ou abertura do SELECT: `(nil, nil, err)`, liberando recursos intermediários.
- SELECT sem registros: retornar `Rows` normalmente; o primeiro `Next()` retorna `false`. O consumidor verifica `Rows.Err()` para distinguir fim dos resultados de falha na leitura.
- Escrita com erro: `(nil, err)`.
- Acrescentar contexto aos erros, quando útil, usando `%w`, por exemplo `fmt.Errorf("krios: executar select: %w", err)`, preservando a causa para `errors.Is` e `errors.As`.
- Falhas posteriores à entrega das linhas são verificadas pelo consumidor em `Scan` e `Rows.Err()`; não alteram retroativamente o erro retornado por `Exec`. O consumidor também fecha `Rows`.

Referência: [API de database/sql](https://pkg.go.dev/database/sql).

5. **Transações (ocorrências de `ExecTx` e `MigrateTx`):** o argumento será `*sql.Tx`? Quem executará commit e rollback? Como tratar transação nula, encerrada ou incompatível com o cliente? Toda operação auxiliar de paginação ou migração utilizará a mesma transação?

- Resposta: o usuario deve abrir um contexto, sql.Tx e ser responsavel por fechar e tratar o commit. Aprovado a sugestão da IA.

**Resposta sugerida pela IA — complemento aprovado pelo desenvolvedor:**

- A aplicação cria a transação com `db.BeginTx(...)` e fica responsável por `Commit` ou `Rollback`, incluindo o tratamento de seus erros. O argumento recebido por `ExecTx` e `MigrateTx` é `*sql.Tx`.
- O Krios não confirma nem desfaz a transação recebida. Sucesso no método indica que a operação terminou, mas ainda depende do commit da aplicação.
- Usar exclusivamente a transação fornecida, inclusive para contagem da paginação, consulta principal e leitura/escrita do histórico de migrações. Não usar o pool como alternativa em caso de erro.
- Não armazenar a transação no cliente para chamadas futuras. Chamadas sem transação continuam usando o pool configurado.
- `tx == nil`: retornar erro de argumento antes de executar SQL.
- Transação encerrada: propagar o erro retornado por `database/sql`, normalmente `sql.ErrTxDone`, preservando sua identificação.
- Contexto da operação cancelado: retornar o erro e não iniciar novos comandos.
- A aplicação deve fornecer uma transação compatível com o banco e o dialeto do cliente. `*sql.Tx` não expõe publicamente seu pool de origem; não prometer validação geral de que pertence a `Config.Db`.
- Manter válido o contexto passado a `BeginTx` até encerrar a transação. Seu cancelamento pode provocar rollback pelo próprio `database/sql`, independentemente do contexto usado em uma consulta. Prazo/cancelamento de uma operação e ciclo de vida da transação são responsabilidades distintas.

Referências: [Execução de transações no Go](https://go.dev/doc/database/execute-transactions) e [API de sql.Tx](https://pkg.go.dev/database/sql#Tx).

### Critérios e construção das consultas

6. **Parâmetros (linhas 43–54 e 173):** confirmar que SQL e argumentos serão enviados separadamente ao driver, sem interpolação textual. Qual será a sintaxe por dialeto/driver e como ocorrerá a numeração conjunta em subconsultas, joins e unions?

- Resposta: Aprovado.

**Sugestão para o item 6 — pendente de aprovação:**

- Gerar SQL e argumentos separadamente. Nunca substituir valores por texto SQL, mesmo para strings, datas ou `nil`. PostgreSQL usa `$1`, `$2` etc.; propor `?` para MySQL e SQLite. Esses formatos devem ser verificados com os drivers escolhidos para integração.
- Numerar somente ao montar a consulta completa, na ordem de aparição dos parâmetros no SQL. Cada lista, JOIN, subconsulta, UNION e valor de escrita participa da mesma sequência. Critérios não guardam um `$1` definitivo quando são criados.
- Reiniciar a sequência em cada comando enviado ao driver. Contagem e seleção paginada são comandos distintos e têm seus próprios argumentos. Reutilizar um critério não reutiliza automaticamente o índice do parâmetro.
- Manter o dialeto do cliente que executa a consulta; propor erro estrutural se forem combinados builders de clientes diferentes, evitando mistura implícita de configuração.

Exemplo PostgreSQL: `client.And(client.Eq("name", "Ana"), client.In("id", []int{2, 3}))` produz `(name = $1 AND id IN ($2, $3))`, com argumentos `[]any{"Ana", 2, 3}`. As aspas de identificadores foram omitidas apenas neste exemplo.

Referência: [Parâmetros separados do SQL e diferenças de placeholders](https://go.dev/doc/database/sql-injection).

- Validação do desenvolvedor:

7. **Tipos (linhas 43–61):** o contrato inclui `nil`, floats, inteiros sem sinal dentro dos limites de conversão, ponteiros, tipos que implementam `driver.Valuer` e extensões dos drivers? Quais validações cabem ao Krios e quais serão delegadas ao driver/banco?

- Resposta: Eu quero que toda a validação seja feita pelo driver, o krios é apenas um builder de queries, o driver já faz a validação necessaria, então ao executar, se algo estiver errado, deve retornar o erro do driver e/ou banco de dados. Aprovado proposta da IA.

**Complemento para o item 7 — pendente de aprovação:**

Preservar os valores recebidos e encaminhá-los como argumentos, sem whitelist de tipos, conversão antecipada de datas/números ou chamada manual a `driver.Valuer`. A aceitação e conversão dos valores cabem a `database/sql`, ao driver e ao banco; seus erros são propagados. Isso não garante que qualquer valor funcione em qualquer banco. [Conversão padrão de argumentos](https://pkg.go.dev/database/sql/driver#DefaultParameterConverter).

Proponho distinguir validação de valores de validação da estrutura do builder. Permanecem os erros que você já exigiu: grupos lógicos vazios, escritas incompletas, transação nula e versões duplicadas. Ver item 21 para confirmar esse limite, inclusive nas novas propostas.

- Validação do desenvolvedor:

8. **IN/NOT IN (linha 53):** listas serão expandidas em um argumento por elemento? Qual comportamento para listas vazias, nulas, heterogêneas, com `NULL`, arrays e `[]byte`? Haverá suporte a subconsultas nesses predicados?

- Resposta: deve aceitar apenas listas, novamente, o krios não valida nada, se o driver ou banco não aceitar, então deve estourar erro, retornando o que aconteceu, porem gostei da ideia de colocar uma subconsulta para carregar esses itens, vou implementar a estrategia nas definições, solicito a IA para validar a adição do subselect. Aprovado proposta da IA.

**Sugestão e revisão do subselect do item 8 — pendentes de aprovação:**

- Aceitar slices e arrays como listas, preservando ordem e valores; expandir um placeholder por elemento. Não achatar listas aninhadas nem converter elementos. Listas heterogêneas e elementos `nil` seguem para o driver.
- Para evitar ambiguidade, propor que `[]byte` diretamente em `In` represente uma lista de bytes; uma lista de blobs será `[][]byte` ou `[]any{blob1, blob2}`. Em `Eq` e em escritas, `[]byte` continua sendo um único valor. Essa é uma convenção de construção proposta, não uma conversão dos elementos.
- Preservando seu pedido de delegar ao banco: lista vazia ou slice nulo tipado gera `IN ()`/`NOT IN ()`, sem substituir por condição constante nem remover o filtro. SQLite aceita a lista vazia; não se pode prometer erro uniforme entre os bancos. Em particular, `NOT IN ()` pode selecionar todas as linhas no SQLite. [Semântica documentada pelo SQLite](https://www.sqlite.org/lang_expr.html#the_in_and_not_in_operators).
- Propor erro de estrutura para argumento que não seja lista, ou `nil` sem uma subconsulta posteriormente vinculada. Isso precisa de sua aprovação conforme o item 21; passar um slice inteiro em um único placeholder não implementa a expansão da lista.
- Manter a forma que você adicionou: `client.In("id", nil).SubSelect(sub)` e equivalente para `NotIn`. O `nil` funciona como ausência de lista nessa construção. Lista não nula junto com subconsulta gera erro estrutural na execução, para não descartar argumentos silenciosamente. O último `SubSelect` substitui o anterior; subconsulta nula gera erro.
- O predicado apenas produz `id IN (SELECT ...)`; ele não cria um SELECT externo. O novo exemplo em DEFINITIONS.md precisa de `Where` em uma consulta externa, como abaixo. O Krios não deve executar o subselect separadamente para carregar IDs em memória.

```go
sub := client.Select().From("users").Distinct().Column("id")
query := client.Select().From("users")
query.Where(client.In("id", nil).SubSelect(sub))
// SQL equivalente: SELECT * FROM users
// WHERE id IN (SELECT DISTINCT id FROM users)
```

O subselect deve produzir uma coluna para esse uso escalar de `IN`; quantidade de linhas pode variar. Proponho deixar a verificação de colunas/tipos com o banco, sem consultar o schema ou executar a subconsulta para validá-la. Não acrescentar `DISTINCT` automaticamente; ele é opcional. Preservar a semântica de `NULL`, inclusive em `NOT IN`. [Subconsultas em IN/NOT IN](https://www.postgresql.org/docs/16/functions-subquery.html).

- Validação do desenvolvedor:

9. **NULL e composição (linhas 67–89):** `Eq(campo, nil)` e `Ne(campo, nil)` serão traduzidos para `IS NULL`/`IS NOT NULL`? Como tratar `And()`/`Or()` vazios e critérios nulos? Os predicados existirão apenas no cliente ou também no pacote, como mostram os exemplos com `krios.Eq`?

- Resposta: `Eq(campo, nil)` e outros devem tratar como se o valor fosse normal, como eu falei, o crieu passa para o driver, se retornar erro, reporta o erro, nesse caso, deve fazer isso `id = null`, o mesmo para todos, `And()` ou `Or()`, devem retornar erro, eu quero que seja apenas no cliente, por conta do dialeto. Aprovado proposta da IA.

**Esclarecimento e complemento para o item 9 — pendentes de validação:**

Manter a comparação solicitada: `Eq("id", nil)` produz `id = $1`, com argumento `nil`, equivalente semanticamente a comparar com SQL NULL. Não transformar em `IS NULL`. A mesma regra vale para os demais operadores parametrizados; `IsNull` e `IsNotNull` continuam explícitos.

Uma comparação normal com NULL em geral produz UNKNOWN, não erro. Em `WHERE`, isso não seleciona a linha, inclusive quando a coluna é nula. Portanto, não há necessariamente um erro de driver para reportar. [Comparações com NULL](https://www.postgresql.org/docs/16/functions-comparison.html).

Proponho retornar na execução erros de `And()`/`Or()` vazios e de critérios nulos inseridos em grupos, `Where`, `On` ou `Having`, sem descartar esses elementos. Usar parênteses em cada grupo explícito para preservar a precedência. Os predicados permanecem exclusivamente no cliente, conforme sua resposta.

- Validação do desenvolvedor:

10. **Identificadores e expressões (linhas 98–144):** strings representam nomes de colunas/tabelas, expressões SQL ou ambos? Como representar comparação entre duas colunas no `ON` sem transformar a segunda coluna em valor textual? Como tratar nomes qualificados, curingas, funções, schemas, aliases e escape de identificadores por dialeto?

- Resposta: Aprovado.

**Sugestão para o item 10 — pendente de aprovação:**

- Strings em posições de tabela, coluna, agrupamento, ordenação e primeiro operando dos predicados representam identificadores. Valores nos outros operandos continuam parametrizados; nunca interpretar uma string de valor como nome de coluna.
- Acrescentar `client.Ref("o.user_id")` para referência explícita a coluna. Exemplo: `client.Eq("u.id", client.Ref("o.user_id"))` gera comparação entre colunas sem argumento textual. Aceitar uma referência também como primeiro operando.
- Qualificar e escapar cada componente: PostgreSQL/SQLite usam aspas duplas e MySQL usa crases. `u.id` tem dois componentes; `u.*` preserva o curinga. Duplicar o delimitador dentro de cada identificador. Não aceitar SQL já montado como se fosse identificador.
- Nos nomes simples do SELECT, usar o alias da tabela principal, se definido; nomes já qualificados não recebem outro prefixo. Em `On`, recomendar referências qualificadas para evitar ambiguidade. `Schema` afeta a origem da tabela, não o texto do alias.
- Para nomes literais contendo ponto, propor `client.RefParts("u", "nome.com.ponto")`; cada argumento é um componente indivisível. Uma string simples com ponto segue a regra de qualificação. Não acrescentar parser de expressões SQL.
- Acrescentar expressões estruturadas `client.CountExpr`, `MaxExpr`, `MinExpr`, `AvgExpr` e `SumExpr` para operandos de predicados. Assim, `client.Gt(client.CountExpr("*"), 1)` representa uma agregação parametrizada, sem confundir `"COUNT(*)"` com um identificador.
- Não incluir uma função genérica de SQL cru no escopo inicial. Os nomes dessas novas funções e seus tipos públicos precisam de aprovação; são propostas de API, não funções já existentes.

- Validação do desenvolvedor:

11. **Estado do builder (linhas 97–185):** chamadas sucessivas de `Where`, `On`, `Columns`, `GroupBy` e `Set` acumulam ou substituem? A que elemento `As`, `Asc`, `Desc` e `Schema` se aplicam? Os builders e critérios são mutáveis, reutilizáveis e seguros para uso concorrente? Uma subconsulta é copiada ou mantida por referência?

- Resposta:`Where`, `On`, `Columns`, `GroupBy` sempre acumulam e `Set` troca, se for colocado o mesmo, `As` colunas, table, joins, e tudo mais que pode-se colocar um alias em uma instrução sql, `Asc`, `Desc` apenas em `OrderBy`, e `Schema` em tabelas e joins, se o dialeto permitir. Aprovado proposta da IA.

**Complemento para o item 11 — pendente de aprovação:**

- Manter builders mutáveis e reutilizáveis sequencialmente. Não permitir construção/execução concorrente do mesmo builder; usar builders separados por goroutine. O cliente não mantém contadores globais de parâmetros nem estado de paginação compartilhado entre builders.
- Preservar as regras já respondidas: acumular `Where`, `On`, `Columns`, `GroupBy`; unir condições acumuladas com AND. `OrderBy` também acumula. `Set` repetido substitui o valor, preservando a posição original da coluna na ordem dos argumentos.
- `As`, `Asc`, `Desc` e `Schema` alteram o último elemento compatível selecionado pelo encadeamento. Uma segunda chamada substitui a opção anterior. Uma chamada sem alvo válido registra erro para a execução; não procurar silenciosamente outro alvo antigo. `As` após `Columns` com vários itens é ambíguo: propor erro, usando `Column(...).As(...)` para cada alias.
- `From`/`Into` repetidos substituem a tabela principal; `Join` e `SubSelect` acumulam origens. A troca da origem invalida o alvo de encadeamento anterior e a paginação. Métodos de tabela/coluna retornam o builder para preservar o estilo atual dos exemplos.
- Guardar subconsultas e critérios por referência, sem cópias profundas implícitas; isso preserva valores personalizados dos drivers. Alterar uma subconsulta afeta os pais. Não mutar nenhum componente enquanto um pai estiver sendo executado. Ciclos de referência geram erro estrutural em vez de recursão infinita.
- Qualquer mudança na árvore da consulta invalida a contagem e reinicia o offset do pai conforme o item 16, inclusive quando o filho é alterado. Essa propagação de alteração deve constar dos testes; não basta observar apenas chamadas feitas no pai.
- Erros estruturais ficam registrados e são retornados por `Exec`/`ExecTx` antes do SQL. Propor que um builder com erro permaneça inválido; criar outro para corrigir, em vez de limpar erros silenciosamente. Não incluir `Clone` nesta etapa.

- Validação do desenvolvedor:

12. **SELECT e subconsultas (linhas 110–159):** qual regra determina a seleção implícita de `*` ou `u.*` e sua remoção após adicionar colunas? `Count("*")` deve preservar `COUNT(*)`? `SubSelect` pretende gerar produto cartesiano? Como representar correlação e restringir o formato de subconsultas escalares?

- Resposta: Aprovado.

**Sugestão para o item 12 — pendente de aprovação:**

- Sem projeção explícita, usar `SELECT *`, inclusive com joins. A primeira chamada a `Column`, `Columns`, agregador ou `ColumnSelect` remove a seleção implícita; chamadas posteriores acumulam projeções. Para selecionar os campos principais mais uma subconsulta, escrever explicitamente `Column("u.*").ColumnSelect(sub).As("total")`. Essa regra exige ajustar o exemplo que hoje acrescenta `u.*` implicitamente.
- `Count("*")` sempre gera `COUNT(*)`, sem prefixar o alias. Outros argumentos seguem as regras de identificadores/expressões. Não acrescentar colunas, agregações ou `GROUP BY` para corrigir a consulta do utilizador.
- `SubSelect(sub).As("o")` acrescenta uma origem derivada ao FROM, produzindo produto cartesiano com a origem principal. Exigir alias explícito para a origem derivada e para `Wrapper`, como regra estrutural portátil. Não inserir uma condição de JOIN automaticamente.
- `ColumnSelect(sub)` representa subconsulta escalar. O utilizador deve fornecer uma coluna e no máximo uma linha para resultados portáveis; o banco verifica a execução. O SQLite pode usar a primeira linha onde outro banco retorna erro, portanto não prometer erro uniforme. [Subconsultas escalares no SQLite](https://www.sqlite.org/lang_expr.html#subquery_expressions).
- Permitir correlação em subconsultas escalares e em IN/NOT IN por `client.Ref("u.id")`, preservando referências qualificadas ao escopo externo. Não prefixar tudo com o alias interno. Não incluir subconsultas correlacionadas no FROM nem `LATERAL` inicialmente.

- Validação do desenvolvedor:

13. **UNION (linhas 148–159):** como `query1` participará da união, já que não é passado no exemplo? Qual será a assinatura de `Union`/`UnionAll`, e onde se aplicarão ordenação, paginação e parênteses?

- Resposta: Falha minha de definição, sim `query1` participa do union `selectQuery := client.Union(query1, query2)`, cada query deve do union deve cudar da paginação ordenação e tal, porem isso pode falhar, por isso criei a função Wrapper, para absorver esse tipo de problema. Quero a opinião da IA, aprovado.

**Sugestão para o item 13 — pendente de aprovação:**

- Manter `client.Union(query1, query2, ...)` e `UnionAll`, exigindo pelo menos duas consultas não nulas. Cada parte mantém sua própria projeção, filtros, ordenação e limite/offset. A compatibilidade de colunas e tipos fica com o banco.
- Concordo com `Wrapper` para ordenar e paginar o resultado combinado. Isso não corrige automaticamente a sintaxe das partes: SQLite, por exemplo, restringe ORDER BY/LIMIT em componentes de um SELECT composto. [SELECT composto no SQLite](https://sqlite.org/lang_select.html#compound_select_statements).
- Propor que cada parte seja compilada como uma origem derivada, com alias interno, para manter suas cláusulas locais: `SELECT * FROM (<query1>) AS _krios_u1 UNION SELECT * FROM (<query2>) AS _krios_u2`. Preservar também uniões aninhadas. Essa estratégia deve ser testada nos três bancos mínimos; erros de projeção/nomes do SQL do utilizador continuam sendo propagados.
- Ordenação/limite de uma parte afetam aquela parte; somente ordenação externa garante a ordem do conjunto final. `UNION` elimina duplicatas e `UNION ALL` as preserva. [Semântica de UNION](https://www.postgresql.org/docs/16/queries-union.html).
- Propor o mesmo retorno de SELECT em `Union.Exec`/`ExecTx`, com `page == nil`; para obter metadados globais, executar o wrapper paginado. Ao compilar filhos, não executar suas contagens de paginação: somente seu LIMIT/OFFSET participa do SQL.

```go
combined := client.Union(query1, query2)
outer := client.Wrapper(combined).As("w").Columns("id")
outer.OrderBy("id").Asc().Paginate(0, 10)
// Ordenação e paginação do conjunto final ficam no SELECT externo.
```

- Validação do desenvolvedor:

14. **JOIN e HAVING (linhas 127–144 e 163–164):** `Having` fará parte do escopo inicial? Como representar agregações nos critérios? Quais tipos de join serão incluídos e como tratar recursos indisponíveis na versão do banco?

- Resposta: Aprovado.

**Sugestão para o item 14 — pendente de aprovação:**

- Incluir `Having(criterios...)` no escopo inicial, acumulando condições com AND como `Where`. Usar as expressões do item 10: `query.Having(client.Gt(client.CountExpr("*"), 1))`. WHERE filtra linhas antes da agregação; HAVING filtra grupos. Não exigir GROUP BY sempre que houver HAVING; a validade da expressão fica com o banco. [Agrupamento e HAVING](https://www.postgresql.org/docs/16/queries-table-expressions.html#QUERIES-GROUP).
- Incluir `Join`/`InnerJoin`, `LeftJoin`, `RightJoin` e `CrossJoin`. Exigir ON não vazio nos três primeiros; `CrossJoin` não recebe ON. Usar `client.Ref` para comparações entre colunas. Não mover condições entre ON e WHERE, pois isso pode alterar o resultado de joins externos.
- Adiar `FullJoin`, `NaturalJoin`, `Using` e `Lateral` para manter o escopo inicial pequeno. FULL OUTER JOIN existe em PostgreSQL e SQLite 3.39+, mas não é oferecido nativamente pelo MySQL 8.0. [SQLite 3.39](https://www.sqlite.org/releaselog/3_39_0.html), [JOIN no MySQL](https://dev.mysql.com/doc/refman/8.0/en/join.html).
- Não emular recursos indisponíveis. O dialeto determina a geração do SQL; o utilizador garante as versões mínimas e a configuração do driver. Propor não detectar a versão automaticamente. Incompatibilidades observadas na execução são propagadas. Formas que a API não consegue representar geram erro estrutural, conforme o item 21.

- Validação do desenvolvedor:

### Paginação e escrita

15. **Semântica da paginação (linhas 177–205):** os argumentos são página/tamanho ou offset/limite? Se forem página/tamanho, confirmar `offset = (pagina - 1) * tamanho`. Como tratar zero, valores fora do intervalo, resultado vazio e overflow? Qual diferença entre `LastPage` e `TotalPages`?

- Resposta: é `offset/limite`. "Como tratar zero, valores fora do intervalo, resultado vazio e overflow?" aceito sugestão da IA. Deve existir apenas `LastPage`, foi um erro, vou remover `TotalPages`. Aprovado proposta da IA.

**Sugestão para o item 15 — pendente de aprovação:**

- Propor `Paginate(offset, limit uint64)`, com offset iniciado em zero. `Paginate(0, 10)` produz LIMIT 10 OFFSET 0; `Paginate(1, 10)` pula uma linha. Não subtrair 1 do primeiro argumento. A página convencional 25 com dez registros começa no offset 240.
- Aceitar offset zero; limite zero gera erro estrutural na execução. Não usar limite zero como comando para desligar paginação. A ausência de `Paginate` representa consulta sem paginação.
- Aceitar offsets que não sejam múltiplos do limite, preservando o contrato offset/limite. Nesse caso, `CurrentPage` indica o bloco convencional que contém o início do intervalo, e não identifica unicamente a janela pedida. Exemplo: offset 15/limite 10 começa no bloco 2, mas lê posições que atravessam dois blocos.
- Para total `T`, offset `O` e limite `L > 0`: `RowsPerPage = L`; `LastPage = T/L`, acrescido de 1 quando `T%L != 0`; `CurrentPage = O/L + 1` quando `T > 0`, e zero quando `T == 0`. Não usar `(T+L-1)/L`, que pode transbordar.
- `HasNextPage = O < T && L < T-O`; `HasPreviousPage = O > 0`, indicando que é possível reduzir o offset, mesmo se os dados tiverem sido removidos. Os indicadores refletem a contagem daquela execução.
- Offset além do total retorna Rows vazio, sem ajustar silenciosamente a posição; `CurrentPage` pode superar `LastPage`. Total vazio resulta em `LastPage == 0` e `CurrentPage == 0`; preservar `RowsPerPage` e o offset solicitado.
- Detectar overflow dos cálculos que o Krios faz e retornar erro na execução, sem wraparound. A capacidade do banco de aceitar um offset/limite recebido é validada pelo driver/banco. Preservar os campos de `Page` como `uint64` e manter a remoção de `TotalPages` já decidida.

Esta proposta aceita metadados menos intuitivos em offsets não alinhados. Se você quiser que `CurrentPage` identifique sempre uma página convencional, seria necessário restringir o offset a múltiplos do limite ou mudar o contrato; não estou impondo nenhuma dessas mudanças.

- Validação do desenvolvedor:

16. **Contagem e navegação (linhas 182–205):** `TotalRows` contará as linhas do resultado completo, inclusive após `DISTINCT`, `GROUP BY`, `HAVING` e `UNION`? Quando recalcular a contagem? `Next`/`Prev` executam SQL ou apenas mudam o estado? Como funcionam antes da primeira execução? O que significa zerar a paginação? Qual consistência será prometida diante de alterações concorrentes dos dados e qual será a regra de ordenação?

- Resposta: Aprovado

**Sugestão para o item 16 — pendente de aprovação:**

- Recalcular `TotalRows` em cada execução paginada. Contar as linhas da consulta completa antes do LIMIT/OFFSET externo: `SELECT COUNT(*) FROM (<consulta>) AS _krios_count`. Preservar DISTINCT, GROUP BY, HAVING, joins e unions; não contar apenas a tabela principal. Remover somente a ordenação e paginação do nível que está sendo contado, preservando os limites internos que definem o conteúdo das subconsultas/partes da união.
- Não alterar o builder original para gerar a contagem. Fechar/consumir o resultado da contagem antes de abrir Rows da consulta principal. Ambas usam o mesmo executor e prazo da chamada. No modo transacional, ambas usam exclusivamente `tx`.
- Se o banco rejeitar a consulta derivada de contagem, retornar seu erro. O utilizador deve fornecer nomes/aliases válidos para a projeção; não esconder erros reescrevendo suas expressões.
- `Next()` e `Prev()` apenas alteram o offset e retornam o builder, sem executar SQL. `Next()` soma o limite somente quando a última contagem conhecida indica uma próxima janela; antes da primeira execução ou após invalidação, não faz nada. Na última janela também não faz nada. `Prev()` subtrai o limite, limitado a zero, mesmo sem contagem anterior.
- Navegação conserva a última contagem conhecida apenas para decidir limites; a próxima execução sempre a atualiza. Falha na execução invalida essa contagem. Sem paginação, `Next`/`Prev` registram erro estrutural. O consumidor fecha o Rows anterior antes de executar novamente.
- Mudanças na consulta, inclusive nos filhos referenciados, voltam o offset para zero, preservam o limite e invalidam os totais. `Paginate` explícito substitui offset/limite e invalida os totais; `Next`/`Prev` não disparam esse reset.
- Propor exigir ORDER BY explícito na consulta externa paginada. Cabe ao utilizador escolher chaves que determinem ordem única; o Krios não consulta índices para verificar isso. [LIMIT/OFFSET e ordenação](https://www.postgresql.org/docs/16/queries-limit.html).
- Não prometer snapshot consistente entre contagem e seleção no uso pelo pool. Uma transação também depende do nível de isolamento do banco; para consistência mais forte, a aplicação deve escolher um isolamento que forneça snapshot apropriado. Alterações entre execuções podem causar repetição/omissão de linhas em paginação por offset; não transformar automaticamente em paginação por cursor.

- Validação do desenvolvedor:

17. **Validações de escrita (linhas 208–240):** como tratar `Insert` sem valores, quantidade de valores diferente das colunas, `Update` sem `Set` e `Set` repetido para a mesma coluna? `Where()` vazio ou um grupo vazio satisfaz a exigência de filtro? Em que momento esses erros serão retornados?

- Resposta: `Insert` sem valores retorna erro, quantidade de colunas diferentes de valores, tbem retornar erro, `Update` sem `Set` retorna erro, `Set` repetido substitui mantendo o ultimo valor, `Where()` vazio ou um grupo vazio retorna erro. E deve-se retornar o erro no momendo da execução. Aprovado proposta da IA.

**Complemento para o item 17 — pendente de aprovação:**

Aplicar as validações respondidas antes do primeiro comando SQL em `Exec`/`ExecTx`. Propor também erro para ausência de tabela, insert sem colunas e UPDATE/DELETE sem nenhum critério WHERE. O conteúdo dos valores segue para o driver, conforme o item 7. Não oferecer bypass do WHERE obrigatório nesta etapa.

Um WHERE não vazio não garante que poucas linhas serão afetadas: pode ser verdadeiro para toda a tabela. Não tentar provar seletividade. Preservar o `sql.Result` do driver, sem inventar suporte universal a `LastInsertId` ou transformar erros dos métodos do resultado em sucesso. [Contrato de sql.Result](https://pkg.go.dev/database/sql#Result).

- Validação do desenvolvedor:

### Arquivos SQL e migrações

18. **Execução de arquivos (linhas 243–250):** qual retorno atende arquivos com SELECT, comandos sem linhas e múltiplos resultados? Arquivos podem conter vários comandos, funções, procedures e comandos de clientes como `DELIMITER`? Serão enviados inteiros ao driver ou interpretados pelo Krios? Como resolver caminhos relativos, passar parâmetros e reportar falhas parciais?

- Resposta: Aprovado.

**Sugestão para o item 18 — pendente de aprovação:**

Separar explicitamente execução sem leitura de linhas e consulta de linhas, mantendo os tipos de `database/sql`:

| Método do builder de arquivo | Retorno sugerido |
|---|---|
| `Exec(args ...any)` | `(sql.Result, error)` |
| `ExecTx(tx *sql.Tx, args ...any)` | `(sql.Result, error)` |
| `Query(args ...any)` | `(*sql.Rows, error)` |
| `QueryTx(tx *sql.Tx, args ...any)` | `(*sql.Rows, error)` |

- O utilizador escolhe o método. Não detectar SELECT pelo primeiro termo do arquivo: CTEs, procedures e comandos que retornam linhas tornam essa heurística insuficiente. `Query` não produz `Page` nem paginação automática.
- Usar o contexto e as regras de timeout já aprovadas, mantendo o contexto válido durante a leitura de Rows. O consumidor fecha e verifica Rows. Múltiplos conjuntos de linhas usam `NextResultSet` quando o driver os suporta; não agregar vários `sql.Result` em um resultado inventado. [Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet).
- `File(path)` armazena o caminho absoluto resolvido na criação do builder; caminhos relativos partem do diretório de trabalho naquele momento. Ler o arquivo a cada execução; alterações no arquivo passam a valer na execução seguinte. Exigir extensão `.sql`, arquivo regular e conteúdo não vazio. Erros de resolução/leitura são retornados na execução, preservando sua causa.
- Enviar o conteúdo inteiro ao driver em uma chamada, sem separar por `;`, substituir variáveis ou reinterpretar comandos. Os placeholders já devem estar corretos para o dialeto/driver do arquivo; `args` são enviados separadamente, na ordem fornecida.
- Suporte a vários comandos depende do driver e de sua configuração, escolhidos pelo utilizador. Antes de anunciar suporte, a integração precisa comprovar que todos os comandos são executados e que erros são propagados. Não presumir que todo driver rejeita conteúdo não suportado: alguns podem executar apenas parte dele. No driver MySQL, por exemplo, múltiplos comandos exigem opção específica e têm restrições de parâmetros. [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql#multistatements).
- Excluir comandos de ferramentas cliente, como `DELIMITER`, `SOURCE` e comandos de psql. O Krios não os interpreta; os arquivos devem conter SQL que o driver aceite. `DELIMITER` pertence ao cliente mysql, não à gramática SQL do servidor. [Comandos do cliente mysql](https://dev.mysql.com/doc/refman/8.0/en/mysql-commands.html).
- Identificar caminho e operação no erro, sem imprimir argumentos/conteúdo automaticamente. Não prometer identificar a linha/comando exato se o driver não informar. Fora de transação, falhas podem deixar efeitos parciais; não repetir automaticamente. Em `ExecTx`/`QueryTx`, o utilizador continua responsável por encerrar a transação.

- Validação do desenvolvedor:

19. **Versões e histórico (linhas 6, 17 e 253–277):** `DbVersion` é a versão desejada do schema? Como representar banco sem migrações e retorno à versão zero? Versões precisam ser contíguas? Como tratar versões duplicadas, alvo ausente, lista vazia e histórico divergente? Qual limite numérico será adotado, considerando `uint` e a coluna SQL? A tabela `migrations` guarda versão atual, migrações aplicadas ou histórico de eventos? Quais seus nomes de colunas, tipos, chave, schema e regra de data/fuso?

- Resposta: `DBVersion` é a versão corrente do banco, versão zero retorna erro, a tabela `migrations` não existe, isso signifca que o banco não tem migração, ela deve ser criada, e aplicado a migração até concronizar com a versão informada. Versões duplicadas da erro, alvo ausente erro, lista vazia retorna erro, histórico divergente retorna erro. Limite numérico usar o mesmo do driver, se não houver, usar o `math.MaxUint64` (sempre positivo), troquei para uint64. `migrations` registrará tudo que foi feito, se tiver 3 versẽos, devem estar todas elas lá, com a data que foi aplicada, se for feito downgrade, deve-se remover o registro da versão removida. Colunas: `id`, `version`, `applied_at`. "Versões precisam ser contíguas?" se entendi bem, devem sim, mas não precisa segir uma order, só deve ser superior, exemplo, ode ter isso, versão 2, 6, 8, entende, o indicado é 1,2,3, porem devemos validar, se o usuário colocar assim, 2,8,6, deve executar 2,6,8, entende, devemos ordenar, se existir isso, 1,1,2,2, versões repetidas, deve dar erro. Aprovado proposta da IA.

**Complemento para o item 19 — pendente de aprovação:**

- Sua descrição de sincronização indica que `Config.DbVersion` funciona como versão alvo exigida pela aplicação; a versão efetivamente aplicada é obtida da tabela. Recomendo formalizar essa distinção, mantendo o nome `DbVersion`. Exemplo: banco na versão 2 e configuração 8 executam os UPs necessários até 8; o valor configurado não declara que o banco já está em 8.
- Seus exemplos aceitam lacunas: as versões são únicas e ordenadas, mas não precisam ser contíguas. Ordenar uma cópia do cadastro, sem alterar o slice do utilizador. Versão zero permanece inválida como alvo e como cadastro. Tabela inexistente ou vazia representa estado inicial sem migrations, não uma migration de versão zero. Não oferecer downgrade completo para zero nesta API.
- Usar um cadastro completo do histórico: as versões aplicadas devem formar um prefixo da lista ordenada. Cadastro `[2, 6, 8]` admite histórico `[]`, `[2]`, `[2, 6]` ou `[2, 6, 8]`; `[2, 8]` é divergente. Uma versão desconhecida, duplicada ou fora de ordem lógica bloqueia antes de executar scripts. Não inserir migrations novas abaixo da maior já distribuída/aplicada.
- Preservar erros para duplicatas, alvo ausente, histórico divergente e lista vazia ao chamar migração. Propor que `New` aceite `Migrations: nil` para permitir uso somente do query builder, como no exemplo atual; `Migrate`/`MigrateTx` retornam erro se não houver cadastro. Confirmar isso no item 22.
- A tabela descrita registra o conjunto atualmente aplicado, não uma auditoria permanente: remover uma linha no downgrade apaga aquele evento do histórico. Preservar essa escolha. Tabela inexistente será criada; tabela existente com estrutura incompatível gera erro, sem alterações automáticas para apropriar-se dela.

**Limite numérico — precisa de decisão específica:** `database/sql` não oferece uma consulta genérica ao limite de versões aceito pelo driver. `uint64` comporta `MaxUint64`, mas BIGINT do PostgreSQL e INTEGER do SQLite são inteiros com sinal; a conversão padrão de Go também pode rejeitar `uint64` acima de `MaxInt64`. [PostgreSQL](https://www.postgresql.org/docs/16/datatype-numeric.html), [SQLite](https://www.sqlite.org/datatype3.html), [Conversor Go](https://go.dev/src/database/sql/driver/types.go).

Recomendo manter `uint64` na API e limitar apenas a versão de migration a `1..math.MaxInt64`, como regra do formato de controle do Krios. Isso revisa o fallback `MaxUint64` que você propôs e NÃO está aprovado. Se você exigir toda a faixa `uint64`, a alternativa é armazenar versões como texto decimal canônico nos três bancos e fazer comparação/ordenação em Go; não usar REAL no SQLite, por perda de precisão. Prefiro o limite comum e colunas inteiras pela simplicidade.

Formato físico proposto, condicionado à aprovação desse limite:

| Coluna | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| `id` | `BIGINT PRIMARY KEY` | `BIGINT PRIMARY KEY` | `INTEGER PRIMARY KEY` |
| `version` | `BIGINT NOT NULL UNIQUE` | `BIGINT NOT NULL UNIQUE` | `INTEGER NOT NULL UNIQUE` |
| `applied_at` | `TIMESTAMPTZ NOT NULL` | `DATETIME(6) NOT NULL` | `TEXT NOT NULL` |

Propor `id = version`, sem sequência separada ou dependência de `LastInsertId`. A linha removida e reaplicada recebe o mesmo id e uma nova data. Usar a origem padrão da conexão para a tabela `migrations` (`main` no SQLite), sem impor `public` a todos os bancos. Gravar horário de conclusão do passo em UTC, gerado pela aplicação; PostgreSQL usa instante com fuso, MySQL recebe representação UTC e SQLite texto RFC3339Nano. O driver deve estar configurado para os formatos usados na integração.

Com apenas essas três colunas, não se detecta alteração de conteúdo de um script já aplicado nem falha parcial não registrada. Não afirmar que o Krios verifica isso. Ampliar integridade/recuperação exige sua decisão no item 23.

- Validação do desenvolvedor:

20. **Aplicação e recuperação de migrações (linhas 255–285):** confirmar ordem de `Up` e `Down` e qual versão identifica cada arquivo de descida. A atomicidade será por arquivo ou por conjunto? Como registrar falha parcial, retomar a execução e impedir duas instâncias de migrarem simultaneamente? Como lidar com DDL que causa commit implícito no MySQL e com uma tabela `migrations` já existente? Qual contrato de erro e de controle da transação para `MigrateTx`?

- Resposta: Aprovado.

**Sugestão para o item 20 — pendente de aprovação:**

- Validar cadastro, histórico e plano inteiro antes do primeiro script. Ler todos os arquivos necessários à direção escolhida antes de iniciar; arquivo UP/DOWN necessário ausente bloqueia o plano. Guardar esses conteúdos durante a chamada, sem reler cada arquivo entre validação e execução.
- Subida em ordem crescente; descida em ordem decrescente. `Down` de uma versão desfaz o respectivo `Up`, retornando ao estado da versão cadastrada anterior, não ao número `version-1`. Com cadastro `[2, 6, 8]`, descer de 8 para 2 executa DOWN 8 e DOWN 6. Não executar DOWN 2 nesse caso.
- Após UP bem-sucedido, inserir o registro; após DOWN bem-sucedido, remover somente aquele registro. Se o alvo já estiver aplicado e o histórico for válido, retornar sucesso sem scripts. Downgrade segue o alvo pedido, sem segunda confirmação durante a execução da biblioteca.
- Em `Migrate`, propor atomicidade por migration transacional, incluindo script e atualização do histórico na mesma transação interna. O Krios confirma em sucesso e tenta rollback em falha; migrations anteriores confirmadas permanecem. Não prometer atomicidade do conjunto inteiro.
- Acrescentar um campo declarado pelo autor, proposto como `Migration.NonTransactional bool` (padrão `false`). Passos com `true` são executados fora de transação interna, reservando uma conexão do pool durante script e registro. Não analisar SQL para adivinhar sua capacidade transacional. Scripts não devem conter seu próprio BEGIN/COMMIT/ROLLBACK.
- Em `MigrateTx`, verificar previamente se todos os passos a executar são declarados transacionais e rejeitar os demais antes de qualquer script. Usar somente a transação recebida para histórico e scripts; não criar transação interna nem fazer commit/rollback. Sucesso ainda depende do commit do utilizador. Se for preciso criar a tabela de controle com DDL incompatível com a transação, exigir sua preparação pela execução sem transação externa, em vez de usar o pool como fallback.
- DDL com commit implícito do MySQL deve ser declarado `NonTransactional`. Um autor que declare o contrário pode encerrar a transação sem intenção; o Krios não consegue assegurar rollback apenas por receber `*sql.Tx`. [DDL e commits implícitos do MySQL](https://dev.mysql.com/doc/refman/8.0/en/implicit-commit.html).
- Usar a mesma política do item 18 para executar scripts: conteúdo inteiro, driver configurado e suporte a múltiplos comandos comprovado nos testes. Cancelamento/prazo para a chamada inteira segue o item 3. Parar no primeiro erro e não repetir comandos automaticamente.
- Erros identificam versão, direção, caminho e etapa (leitura, SQL, histórico, commit ou rollback), preservando causas. Se rollback também falhar, preservar os dois erros, por exemplo com `errors.Join`. Erro de commit pode deixar resultado incerto; não assumir que nada foi aplicado.

**Recuperação proposta para o escopo atual:** preservar as três colunas aprovadas e exigir reparo manual após falha não transacional ou resultado incerto. Não executar compensação automática nem um DOWN presumindo que todo o UP terminou. Se o script terminar e o registro falhar, a ausência da versão na tabela NÃO prova que o script não executou. Na versão simples, uma nova chamada não consegue detectar todos esses casos; a aplicação deve bloquear novas execuções até verificar/reparar banco e histórico. Para um bloqueio persistente garantido pelo Krios, ver a alternativa de ampliação no item 23.

**Concorrência proposta para o escopo inicial:** exigir um único executor de migrations por banco, coordenado pela aplicação/deploy, cobrindo leitura do histórico, execução e registro; com `MigrateTx`, manter a exclusão até o commit/rollback externo. Essa recomendação reduz o escopo e não oferece bloqueio distribuído automático no Krios. Uma chave única ou um mutex local não impede dois processos de executar o mesmo script antes de registrar a versão. Se a exclusão precisar ser garantida pela biblioteca, aprovar a ampliação do item 23 antes de implementar migrations.

- Validação do desenvolvedor:

### Pontos adicionais para validação

21. **Fronteira das validações do Krios.**

Há uma diferença entre “não validar valores” nos itens 7–9 e os erros estruturais exigidos nos itens 9 e 17. Proponho formalizar: o Krios valida configuração, formato dos operandos que precisa transformar, estrutura dos builders, seus cálculos de paginação e o próprio histórico de migrations. Tipos/valores SQL, existência de tabelas/colunas, compatibilidade de projeções e semântica das expressões ficam com o driver/banco. O Krios não faz consultas de schema para validar SQL do utilizador, exceto a inspeção de sua tabela de controle.

Essa fronteira também inclui as novas exigências propostas: alias de origem derivada, ORDER BY para paginação, lista versus subconsulta, ausência de origem e builder de outro cliente. Aprovar esses casos explicitamente; ainda não são decisões. Recomendo essa fronteira para retornar erros úteis onde o builder não consegue gerar a estrutura, sem duplicar validações dos valores.

- Validação do desenvolvedor:

22. **Configuração, drivers e erros públicos ainda precisam de detalhes.**

- Propor que `New` valide `Db != nil`, `Context != nil`, dialeto suportado, `Timeout >= 0` e versão alvo positiva, depois faça o ping já aprovado. Não executar migrations automaticamente em `New`.
- Permitir cadastro de migrations vazio na criação para clientes que só usam queries; exigir cadastro não vazio apenas em `Migrate`/`MigrateTx`. Manter a obrigatoriedade atual de `DbVersion >= 1` também nesses clientes, até você decidir simplificá-la.
- Copiar a struct de configuração e o slice de migrations na criação, sem setters. Pool/contexto continuam referências: cancelar o contexto ou fechar o pool afeta o cliente. Não ler arquivos de migrations no `New`.
- A aplicação registra o driver e fornece o pool. Não fixar um driver obrigatório na biblioteca. Escolher/fixar drivers na tarefa de integração e verificar os mínimos Go 1.26, PostgreSQL 16, MySQL 8.0 e SQLite 3.39, inclusive scripts múltiplos e cancelamento. Nenhuma dependência foi escolhida nesta revisão.
- Propor categorias públicas `ErrInvalidConfig`, `ErrInvalidQuery` e `ErrInvalidMigration`, acessíveis por `krios.go` e verificáveis por `errors.Is`. Erros de driver/banco preservam sua causa, sem serem classificados como validação do Krios. Não expor argumentos ou credenciais nas mensagens automaticamente.

- Validação do desenvolvedor:

23. **Nível de garantia das migrations: escolher explicitamente o escopo.**

Recomendo inicialmente o contrato simples dos itens 19–20: histórico de versões aplicadas com três colunas, executor único organizado pela aplicação e recuperação manual de falhas parciais/incertas. Isso precisa constar da documentação; não equivale a garantir detecção automática de histórico semanticamente divergente ou execução concorrente segura.

Se você exigir essas garantias dentro do Krios, proponho ampliar o planejamento antes da implementação:

- Guardar checksums de UP/DOWN para detectar alteração de scripts já aplicados; essa mudança acrescenta dados ao formato de controle aprovado.
- Guardar estado persistente de operação em andamento/falha, com versão e direção, em uma tabela de controle adicional. Para passos não transacionais, confirmar esse estado antes de executar o script e limpar apenas após registro de sucesso. Estado pendente deve bloquear novas chamadas até reparo explícito. Definir uma operação de regularização que nunca execute nem presuma o reparo do schema.
- Definir exclusão entre processos por banco. PostgreSQL/MySQL podem exigir locks e afinidade de sessão; SQLite exige estratégia compatível com arquivos, transações e processos distintos. O desenho precisa cobrir criação inicial das tabelas, timeout de espera, conexão perdida, liberação e transação externa ainda aberta. Não assumir que um mesmo lock SQL funciona nos três bancos.
- Em `MigrateTx`, não gravar marcador de falha por outra conexão para escapar do rollback. A política de recuperação precisa tratar a confirmação externa e commits de resultado incerto, preservando a regra aprovada de uso exclusivo da transação recebida.

Essa alternativa é expansão de escopo, não implementação autorizada nem uma garantia já oferecida. Minha recomendação atual é a versão simples, com suas limitações explícitas; se ela for insuficiente, as decisões de formato/locks precisam ser detalhadas em tarefa própria antes de codificar.

- Validação do desenvolvedor (escopo simples ou ampliação):

24. **Ajustes ainda necessários em DEFINITIONS.md, sem modificá-lo nesta revisão.**

Conferido o conteúdo atual: `DbVersion` e `Migration.Version` já usam `uint64`, `TotalPages` já foi removido, `Union(query1, query2)` já foi corrigido e o exemplo de subselect em IN foi acrescentado. Ainda proponho que o desenvolvedor atualize:

- `Config` ainda declara `timeout` minúsculo; o primeiro exemplo usa `Timeout`. Uniformizar o campo exportado e corrigir também o exemplo de migração, que ainda usa `timeout` e ignora o erro de `New`.
- O exemplo `Paginate(1, 10)` ainda mostra OFFSET 0 e descreve índice de página; ajustar ao contrato offset/limite aprovado. Com as propostas do item 16, acrescentar ORDER BY aos exemplos paginados.
- Os exemplos lógicos ainda usam `krios.Eq`; trocar por `client.Eq`, conforme sua decisão.
- O SELECT com alias ainda omite `AS u` no SQL esperado. Os exemplos de WHERE ainda introduzem alias/colunas que não foram definidos no builder.
- Atualizar o SQL esperado do subselect de IN com a consulta externa, conforme item 8; atualizar COUNT(*) e a seleção implícita conforme a eventual aprovação do item 12.
- Substituir a expressão “sem limites” da versão por uma regra aprovada no item 19 e registrar os mínimos dos bancos no documento de definições.
- Registrar as assinaturas e as políticas já aprovadas nos itens 3–5; incorporar as demais propostas somente após sua validação. Os exemplos aqui apresentados são propostas de contrato, não código executável na biblioteca atual.

- Validação do desenvolvedor:
