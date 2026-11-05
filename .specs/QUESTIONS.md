# Questões e decisões

## Tarefa: revisão técnica de DEFINITIONS.md — 2026-11-04

**Estado: encerrada.** As respostas e recomendações foram aprovadas pelo desenvolvedor e consolidadas neste documento em 2026-11-04. Os 24 itens mantêm a numeração da revisão para referência futura.

Este registro reúne as decisões finais, substituindo respostas informais, propostas e campos de validação. As alternativas não adotadas ficam fora do escopo inicial. O encerramento refere-se às questões da revisão; planejamento, implementação e verificações seguem o fluxo de aprovação do projeto. Os exemplos descrevem o contrato aprovado, não a disponibilidade atual da API.

O arquivo [DEFINITIONS.md](DEFINITIONS.md) não foi alterado nesta consolidação. Os ajustes documentais acordados estão registrados no item 24 para atualização pelo desenvolvedor.

### Configuração e execução

#### 1. API de configuração

O campo de configuração de timeout será público, com o nome `Timeout`. A criação do cliente terá a assinatura `New(config Config) (*Client, error)`.

Erros de configuração ou conexão serão retornados ao utilizador. Quando a criação falhar, o cliente retornado será `nil`.

#### 2. Conexão e versões mínimas

A conexão será validada em `New`, após a validação da configuração, por meio de `Db.PingContext` com o contexto e o timeout aplicáveis. O utilizador fornecerá o pool `*sql.DB` e será responsável por fechá-lo. O ping verifica a disponibilidade naquele momento, sem garantir execuções futuras.

| Dependência | Versão mínima |
| ----------- | ------------- |
| Go          | 1.26          |
| PostgreSQL  | 16            |
| MySQL       | 8.0           |
| SQLite      | 3.39          |

A aplicação escolherá e registrará o driver. O Krios não detectará automaticamente a versão do banco. A compatibilidade entre banco, versão, driver e dialeto informado será responsabilidade do utilizador.

#### 3. Contexto e timeout

Todas as operações usarão `Config.Context`. `Exec()` e `ExecTx(tx)` não receberão contexto adicional.

| Valor de `Timeout` | Comportamento                                                                   |
| ------------------ | ------------------------------------------------------------------------------- |
| Negativo           | Erro de configuração em `New`.                                                  |
| Zero               | Usar somente o prazo e o cancelamento de `Config.Context`.                      |
| Positivo           | Criar um contexto derivado por operação; prevalece o prazo que vencer primeiro. |

O timeout começará na execução, não na criação do builder. Abrangerá o ping em `New`, toda a execução das escritas e a chamada completa de migração, sem reiniciar por comando. No SELECT paginado, contagem, consulta e leitura das linhas compartilharão o mesmo prazo.

O cancelamento de `Config.Context` afetará operações em andamento e futuras daquele cliente. O Krios propagará o contexto às chamadas de `database/sql` e não iniciará novos comandos após observar seu cancelamento.

Ao retornar `*sql.Rows`, o Krios manterá válido o contexto derivado para permitir a leitura pelo consumidor. Não executará `cancel()` simplesmente ao retornar de `Exec`. Em falhas anteriores à entrega das linhas, cancelará imediatamente; em operações sem linhas, liberará o contexto derivado ao término.

Como `*sql.Rows` será retornado diretamente, o Krios não interceptará `Rows.Close()` para liberar seu timer. Está aceito que, após sucesso, o timer permaneça até o prazo vencer, mesmo quando as linhas forem fechadas antes. O campo `Timeout` será mantido; a alternativa de removê-lo não foi adotada.

Referência: [Cancelamento de operações no Go](https://go.dev/doc/database/cancel-operations).

#### 4. Assinaturas, retornos e erros

| Operação                | Assinatura                                     |
| ----------------------- | ---------------------------------------------- |
| Criação                 | `New(config Config) (*Client, error)`          |
| SELECT                  | `Exec() (*sql.Rows, *Page, error)`             |
| SELECT em transação     | `ExecTx(tx *sql.Tx) (*sql.Rows, *Page, error)` |
| INSERT, UPDATE e DELETE | `Exec() (sql.Result, error)`                   |
| Escritas em transação   | `ExecTx(tx *sql.Tx) (sql.Result, error)`       |
| Migração                | `Migrate() error`                              |
| Migração em transação   | `MigrateTx(tx *sql.Tx) error`                  |

- SELECT bem-sucedido sem paginação retornará `(rows, nil, nil)`; com paginação, `(rows, page, nil)`.
- Falhas de validação, contagem ou abertura do SELECT retornarão `(nil, nil, err)`, com liberação dos recursos intermediários.
- SELECT sem registros retornará `Rows` normalmente. O primeiro `Next()` retornará `false`; o consumidor verificará `Rows.Err()` para distinguir fim dos resultados de erro.
- Escrita com erro retornará `(nil, err)`.
- Erros poderão receber contexto com `%w`, preservando a causa para `errors.Is` e `errors.As`.
- O consumidor será responsável por `Rows.Close`, `Rows.Scan` e `Rows.Err`. Falhas posteriores à entrega das linhas não alterarão retroativamente o retorno de `Exec`.

Os nomes dos tipos concretos dos builders serão detalhados nas respectivas tarefas, preservando essas assinaturas.

Referência: [API de database/sql](https://pkg.go.dev/database/sql).

#### 5. Transações

A aplicação criará a transação com `db.BeginTx(...)` e será responsável por `Commit` ou `Rollback`, incluindo o tratamento dos respectivos erros. O argumento das operações transacionais será `*sql.Tx`.

O Krios não confirmará nem desfará uma transação recebida. Sucesso em `ExecTx` ou `MigrateTx` ainda dependerá do commit da aplicação.

Todas as operações da chamada usarão exclusivamente a transação fornecida, incluindo contagem de paginação, consulta principal e histórico de migrações. Não haverá fallback para o pool nem armazenamento da transação no cliente para chamadas futuras.

| Situação                             | Comportamento                                                    |
| ------------------------------------ | ---------------------------------------------------------------- |
| `tx == nil`                          | Retornar erro de argumento antes de executar SQL.                |
| Transação encerrada                  | Propagar o erro de `database/sql`, normalmente `sql.ErrTxDone`.  |
| Contexto cancelado                   | Retornar o erro e não iniciar novos comandos.                    |
| Transação incompatível com o cliente | A aplicação responderá pela compatibilidade com banco e dialeto. |

`*sql.Tx` não expõe publicamente seu pool de origem; o Krios não prometerá verificar se pertence a `Config.Db`. A aplicação manterá válido o contexto passado a `BeginTx` até encerrar a transação. O cancelamento desse contexto pode provocar rollback pelo próprio `database/sql`, independentemente do contexto de uma consulta.

Referências: [Transações no Go](https://go.dev/doc/database/execute-transactions) e [API de sql.Tx](https://pkg.go.dev/database/sql#Tx).

### Critérios e construção das consultas

#### 6. SQL e parâmetros

SQL e argumentos serão gerados e enviados separadamente. Valores não serão interpolados no texto SQL, inclusive strings, datas e `nil`.

| Dialeto    | Placeholders    |
| ---------- | --------------- |
| PostgreSQL | `$1`, `$2`, ... |
| MySQL      | `?`             |
| SQLite     | `?`             |

A ordem dos argumentos será definida na montagem da consulta completa, conforme a aparição dos parâmetros no SQL. Listas, joins, subconsultas, unions e valores de escrita participarão da mesma sequência. Critérios não guardarão índices definitivos quando forem criados.

A sequência será reiniciada para cada comando enviado ao driver. Contagem e seleção paginada terão argumentos próprios. Reutilizar um critério não reutilizará automaticamente o índice do parâmetro.

Combinar builders de clientes diferentes produzirá erro estrutural. Cada consulta usará o dialeto do seu cliente. Os formatos de placeholders serão verificados com os drivers escolhidos na integração.

Exemplo PostgreSQL, omitindo apenas as aspas dos identificadores:

```go
client.And(client.Eq("name", "Ana"), client.In("id", []int{2, 3}))
// SQL: (name = $1 AND id IN ($2, $3))
// Argumentos: []any{"Ana", 2, 3}
```

Referência: [Parâmetros separados do SQL](https://go.dev/doc/database/sql-injection).

#### 7. Tipos e validação de valores

Os valores serão encaminhados como argumentos, sem lista restritiva de tipos, conversões antecipadas de datas ou números, nem chamada manual a `driver.Valuer`.

A aceitação e conversão dos valores caberão a `database/sql`, ao driver e ao banco. Seus erros serão preservados e retornados. Isso não implica que todo valor seja aceito em qualquer banco.

O Krios validará a estrutura necessária à construção das consultas e as regras do próprio contrato, conforme o item 21. Essa responsabilidade inclui grupos vazios, escritas incompletas, transação nula e versões duplicadas.

Referência: [Conversão padrão de argumentos](https://pkg.go.dev/database/sql/driver#DefaultParameterConverter).

#### 8. IN, NOT IN e subconsultas

Slices e arrays serão aceitos como listas, com um placeholder por elemento e preservação da ordem e dos valores. Listas aninhadas não serão achatadas; elementos não serão convertidos pelo Krios. Listas heterogêneas e elementos `nil` serão encaminhados ao driver.

`[]byte` diretamente em `In` ou `NotIn` representará uma lista de bytes. Uma lista de blobs será representada por `[][]byte` ou `[]any{blob1, blob2}`. Em `Eq` e em escritas, `[]byte` continuará sendo um único valor.

Lista vazia ou slice nulo tipado produzirá `IN ()` ou `NOT IN ()`, sem remoção do filtro nem substituição por condição constante. O banco decidirá sua validade. SQLite aceita essas listas; `NOT IN ()` pode selecionar todas as linhas. Não haverá promessa de erro uniforme entre os bancos. [Semântica do SQLite](https://www.sqlite.org/lang_expr.html#the_in_and_not_in_operators).

Um argumento que não seja lista, ou `nil` sem subconsulta vinculada, produzirá erro estrutural na execução. A forma de subconsulta será:

```go
sub := client.Select().From("users").Distinct().Column("id")
query := client.Select().From("users")
query.Where(client.In("id", nil).SubSelect(sub))
// SQL equivalente: SELECT * FROM users
// WHERE id IN (SELECT DISTINCT id FROM users)
```

A mesma construção estará disponível para `NotIn`. O `nil` representará ausência de lista nessa construção. Lista não nula junto com subconsulta produzirá erro, sem descarte silencioso de argumentos. A última chamada a `SubSelect` substituirá a anterior; subconsulta nula produzirá erro.

O predicado produzirá somente a expressão `IN (SELECT ...)` e deverá ser associado à consulta externa. O Krios não executará o subselect separadamente para carregar itens em memória.

A subconsulta deverá produzir uma coluna para esse uso escalar de IN; poderá retornar várias linhas. A verificação de colunas e tipos ficará com o banco. Não será acrescentado DISTINCT automaticamente, e a semântica de NULL será preservada, inclusive em NOT IN.

Referência: [Subconsultas em IN e NOT IN](https://www.postgresql.org/docs/16/functions-subquery.html).

#### 9. NULL e composição lógica

`Eq("id", nil)` produzirá `id = $1`, com argumento `nil`, sem transformação em `IS NULL`. A mesma política valerá para os demais operadores parametrizados. `IsNull` e `IsNotNull` permanecerão explícitos.

Comparações normais com NULL geralmente produzem UNKNOWN, não erro. Em WHERE, isso não seleciona a linha, inclusive quando a coluna é nula. O Krios preservará essa semântica. [Comparações com NULL](https://www.postgresql.org/docs/16/functions-comparison.html).

`And()` e `Or()` vazios, assim como critérios nulos em grupos, `Where`, `On` ou `Having`, produzirão erro na execução. Nenhum desses elementos será descartado silenciosamente. Grupos explícitos serão envolvidos por parênteses para preservar a precedência.

Todos os predicados estarão disponíveis exclusivamente no cliente, conforme o dialeto da instância.

#### 10. Identificadores e expressões

Strings em posições de tabela, coluna, agrupamento, ordenação e primeiro operando de predicados representarão identificadores. Strings em posições de valor continuarão parametrizadas.

- `client.Ref("o.user_id")` representará uma coluna explicitamente. `client.Eq("u.id", client.Ref("o.user_id"))` comparará duas colunas, sem parâmetro textual. Referências também serão aceitas como primeiro operando.
- Cada componente do identificador será escapado separadamente: aspas duplas no PostgreSQL/SQLite e crases no MySQL, duplicando delimitadores internos. `u.id` terá dois componentes; `u.*` preservará o curinga. SQL já montado não será interpretado como identificador.
- Nomes simples do SELECT receberão o alias da tabela principal, quando definido. Nomes qualificados não receberão outro prefixo. Em ON, referências qualificadas serão recomendadas. Schema qualificará a origem da tabela, não o alias.
- `client.RefParts("u", "nome.com.ponto")` permitirá componentes literais com ponto. Cada argumento será indivisível; uma string simples com ponto continuará representando qualificação.
- `client.CountExpr`, `MaxExpr`, `MinExpr`, `AvgExpr` e `SumExpr` representarão agregações em operandos. Exemplo: `client.Gt(client.CountExpr("*"), 1)`.

O escopo inicial não incluirá parser de expressões SQL nem função genérica de SQL cru. Os tipos concretos dessas referências e expressões serão detalhados nas tarefas de implementação, preservando as operações aprovadas.

#### 11. Estado, encadeamento e reutilização dos builders

Builders serão mutáveis e reutilizáveis sequencialmente. O mesmo builder não deverá ser construído ou executado concorrentemente; cada goroutine usará builders separados. O cliente não manterá contadores globais de parâmetros nem paginação compartilhada entre builders.

| Método ou grupo                         | Comportamento                                       |
| --------------------------------------- | --------------------------------------------------- |
| `Where`, `On`, `Having`                 | Acumular condições, combinadas por AND.             |
| `Columns`, `GroupBy`, `OrderBy`         | Acumular itens na ordem recebida.                   |
| `Set` repetido para a mesma coluna      | Substituir o valor, preservando a posição original. |
| `From`, `Into` repetidos                | Substituir a tabela principal.                      |
| `Join`, `SubSelect` como origem         | Acumular origens.                                   |
| `As`, `Asc`, `Desc`, `Schema` repetidos | Substituir a opção anterior no alvo correspondente. |

`As` será aplicado a elementos que aceitam alias, como tabelas, colunas, joins e subconsultas. `Asc` e `Desc` serão aplicados somente a itens de `OrderBy`; `Schema`, a tabelas e joins, conforme o dialeto.

O alvo será o elemento compatível selecionado pelo encadeamento. Sem alvo válido, será registrado erro para a execução, sem busca silenciosa de outro alvo antigo. `As` após `Columns` com vários itens será erro; aliases individuais usarão `Column(...).As(...)`.

Trocar a origem invalidará o alvo de encadeamento anterior e a paginação. Métodos de tabela e coluna retornarão o builder, preservando o estilo dos exemplos.

Subconsultas e critérios serão guardados por referência, sem cópias profundas implícitas. Alterar uma subconsulta afetará seus pais. Nenhum componente deverá ser alterado enquanto um pai estiver sendo executado. Ciclos de referência produzirão erro estrutural.

Qualquer mudança na árvore da consulta invalidará os totais e reiniciará o offset dos pais, conforme o item 16. A propagação incluirá mudanças feitas diretamente nos filhos e deverá ser verificada nos testes.

Erros estruturais serão registrados e retornados por `Exec`/`ExecTx` antes do SQL. Um builder com erro permanecerá inválido; a correção exigirá outro builder. `Clone` não fará parte do escopo inicial.

#### 12. SELECT e subconsultas

Sem projeção explícita, a consulta usará `SELECT *`, inclusive com joins. A primeira chamada a `Column`, `Columns`, agregador ou `ColumnSelect` removerá a seleção implícita; chamadas seguintes acumularão projeções.

Para selecionar os campos principais e uma subconsulta, será necessário declarar ambos, por exemplo `Column("u.*").ColumnSelect(sub).As("total")`.

`Count("*")` sempre produzirá `COUNT(*)`, sem prefixo de alias. Os demais argumentos seguirão as regras de identificadores e expressões. O Krios não acrescentará colunas, agregações ou GROUP BY para corrigir a consulta.

`SubSelect(sub).As("o")` acrescentará uma origem derivada ao FROM, produzindo produto cartesiano com a origem principal. Origens derivadas e `Wrapper` exigirão alias explícito. Nenhuma condição de JOIN será criada automaticamente.

`ColumnSelect(sub)` representará subconsulta escalar. Para resultados portáveis, o utilizador fornecerá uma coluna e no máximo uma linha. A verificação ficará com o banco: SQLite pode usar a primeira linha em situações nas quais outro banco retorna erro. [Subconsultas escalares no SQLite](https://www.sqlite.org/lang_expr.html#subquery_expressions).

Correlação será permitida em subconsultas escalares e em IN/NOT IN por referências qualificadas, como `client.Ref("u.id")`. O alias interno não substituirá referências ao escopo externo. Subconsultas correlacionadas no FROM e LATERAL ficarão fora do escopo inicial.

#### 13. UNION, UNION ALL e Wrapper

`client.Union(query1, query2, ...)` e `UnionAll` exigirão pelo menos duas consultas não nulas. Cada parte manterá projeção, filtros, ordenação e limite/offset. A compatibilidade de colunas e tipos ficará com o banco.

Cada parte será compilada como origem derivada com alias interno, preservando suas cláusulas locais e uniões aninhadas:

```sql
SELECT * FROM (<query1>) AS _krios_u1
UNION
SELECT * FROM (<query2>) AS _krios_u2
```

Essa geração deverá ser testada nos três bancos mínimos. Erros do SQL do utilizador serão propagados. O uso de origens derivadas considera as restrições de ORDER BY/LIMIT em componentes de SELECT composto no SQLite. [SELECT composto no SQLite](https://sqlite.org/lang_select.html#compound_select_statements).

`Wrapper` será usado para ordenar e paginar o resultado combinado. Ordenação e limite de uma parte afetarão somente aquela parte; a ordem final dependerá de ORDER BY externo. UNION eliminará duplicatas; UNION ALL as preservará. [Semântica de UNION](https://www.postgresql.org/docs/16/queries-union.html).

`Union.Exec`/`ExecTx` terão o retorno de SELECT, com `page == nil`. Metadados globais serão obtidos executando o wrapper paginado. A compilação dos filhos usará seus LIMIT/OFFSET, sem executar suas contagens de paginação.

```go
combined := client.Union(query1, query2)
outer := client.Wrapper(combined).As("w").Columns("id")
outer.OrderBy("id").Asc().Paginate(0, 10)
```

#### 14. JOIN e HAVING

`Having(criterios...)` fará parte do escopo inicial e acumulará condições com AND. Agregações serão representadas pelas expressões do item 10, por exemplo `query.Having(client.Gt(client.CountExpr("*"), 1))`.

WHERE filtrará linhas antes da agregação; HAVING filtrará grupos. GROUP BY não será exigido em todo uso de HAVING; a validade da expressão ficará com o banco. [Agrupamento e HAVING](https://www.postgresql.org/docs/16/queries-table-expressions.html#QUERIES-GROUP).

Serão incluídos `Join`/`InnerJoin`, `LeftJoin`, `RightJoin` e `CrossJoin`. Os três primeiros tipos exigirão ON não vazio; CrossJoin não receberá ON. Comparações entre colunas usarão `client.Ref`. O Krios não moverá condições entre ON e WHERE.

`FullJoin`, `NaturalJoin`, `Using` e `Lateral` ficarão fora do escopo inicial. FULL OUTER JOIN existe em PostgreSQL e SQLite 3.39+, mas não é oferecido nativamente pelo MySQL 8.0. [SQLite 3.39](https://www.sqlite.org/releaselog/3_39_0.html), [JOIN no MySQL](https://dev.mysql.com/doc/refman/8.0/en/join.html).

Recursos indisponíveis não serão emulados. O dialeto orientará a geração do SQL; incompatibilidades observadas na execução serão propagadas. Formas não representáveis pela API produzirão erro estrutural, conforme o item 21.

### Paginação e escrita

#### 15. Offset, limite e metadados

A assinatura será `Paginate(offset, limit uint64)`, com offset iniciado em zero. `Paginate(0, 10)` produzirá LIMIT 10 OFFSET 0; `Paginate(1, 10)` pulará uma linha. O primeiro argumento não será convertido de número de página. A página convencional 25 com dez registros começará no offset 240.

Offset zero será válido. Limite zero produzirá erro estrutural na execução; a ausência de `Paginate` representará consulta sem paginação.

Offsets não múltiplos do limite serão aceitos. Nesses casos, `CurrentPage` identificará o bloco convencional que contém o início do intervalo, não uma janela única de resultados. Offset 15 e limite 10, por exemplo, começarão no bloco 2 e atravessarão dois blocos convencionais.

Para total `T`, offset `O` e limite `L > 0`:

| Campo             | Regra                                  |
| ----------------- | -------------------------------------- |
| `TotalRows`       | `T`                                    |
| `RowsPerPage`     | `L`                                    |
| `LastPage`        | `T/L`, acrescido de 1 se `T%L != 0`    |
| `CurrentPage`     | `O/L + 1` se `T > 0`; zero se `T == 0` |
| `HasNextPage`     | `O < T && L < T-O`                     |
| `HasPreviousPage` | `O > 0`                                |

`HasPreviousPage` indicará que é possível reduzir o offset, mesmo após remoção dos dados. Os indicadores refletirão a contagem da execução.

Offset além do total retornará Rows vazio, sem ajuste silencioso; `CurrentPage` poderá superar `LastPage`. Total vazio produzirá `LastPage == 0` e `CurrentPage == 0`, preservando o limite e o offset solicitado.

Os cálculos do Krios detectarão overflow e retornarão erro na execução, sem wraparound. Não será usada a fórmula `(T+L-1)/L`, que pode transbordar. A aceitação do offset/limite pelo banco ficará com o driver/banco.

Os campos de `Page` permanecerão `uint64` quando numéricos e booleanos nos indicadores. `TotalPages` será removido; haverá apenas `LastPage` para o total de páginas convencionais.

#### 16. Contagem, navegação e consistência

`TotalRows` será recalculado em cada execução paginada, contando o resultado completo antes da paginação externa:

```sql
SELECT COUNT(*) FROM (<consulta>) AS _krios_count
```

DISTINCT, GROUP BY, HAVING, joins e unions serão preservados. Somente ordenação e paginação do nível contado serão removidas; limites internos que definem subconsultas ou partes da união permanecerão.

O builder original não será alterado para gerar a contagem. O resultado da contagem será consumido e fechado antes da abertura das linhas da seleção. As duas operações compartilharão executor e prazo; no modo transacional, usarão exclusivamente `tx`.

Se o banco rejeitar a consulta derivada de contagem, seu erro será retornado. O utilizador fornecerá projeções e aliases válidos; o Krios não reescreverá expressões para ocultar erros.

- `Next()` e `Prev()` alterarão apenas o offset e retornarão o builder, sem executar SQL.
- `Next()` somará o limite somente se a última contagem conhecida indicar outra janela. Antes da primeira execução, após invalidação ou na última janela, não fará nada.
- `Prev()` subtrairá o limite, limitado a zero, mesmo sem contagem anterior.
- A navegação conservará a última contagem somente para decidir limites. A próxima execução sempre a atualizará; falha na execução invalidará essa contagem.
- Sem paginação, `Next`/`Prev` registrarão erro estrutural. O consumidor fechará o Rows anterior antes de executar novamente.
- Mudanças na consulta ou em filhos referenciados voltarão o offset a zero, preservarão o limite e invalidarão os totais.
- `Paginate` explícito substituirá offset/limite e invalidará os totais. `Next`/`Prev` não dispararão o reset por alteração da consulta.

A consulta externa paginada exigirá ORDER BY explícito. O utilizador escolherá chaves que determinem ordem única; o Krios não inspecionará índices para verificar isso. [LIMIT/OFFSET e ordenação](https://www.postgresql.org/docs/16/queries-limit.html).

Não será garantido snapshot consistente entre contagem e seleção pelo pool. Em transações, a consistência dependerá do isolamento escolhido pela aplicação. Alterações entre execuções poderão causar repetição ou omissão de linhas na paginação por offset; não haverá conversão automática para paginação por cursor.

#### 17. Validação das escritas

As validações estruturais ocorrerão em `Exec`/`ExecTx`, antes do primeiro comando SQL. Produzirão erro:

- Ausência da tabela de destino.
- INSERT sem colunas ou valores, ou com quantidade de valores incompatível com as colunas.
- UPDATE sem `Set`.
- UPDATE ou DELETE sem critérios WHERE.
- `Where()` vazio ou contendo grupo vazio.

`Set` repetido para a mesma coluna manterá o último valor, preservando sua posição original. Valores serão encaminhados ao driver conforme o item 7.

Não haverá bypass do WHERE obrigatório. Um WHERE não vazio poderá afetar toda a tabela; o Krios não tentará provar sua seletividade.

O `sql.Result` do driver será preservado, sem garantia universal de `LastInsertId` nem ocultação dos erros de seus métodos. [Contrato de sql.Result](https://pkg.go.dev/database/sql#Result).

### Arquivos SQL e migrações

#### 18. Execução de arquivos SQL

O builder de arquivo separará execução sem leitura de linhas e consulta de linhas:

| Método                             | Retorno               |
| ---------------------------------- | --------------------- |
| `Exec(args ...any)`                | `(sql.Result, error)` |
| `ExecTx(tx *sql.Tx, args ...any)`  | `(sql.Result, error)` |
| `Query(args ...any)`               | `(*sql.Rows, error)`  |
| `QueryTx(tx *sql.Tx, args ...any)` | `(*sql.Rows, error)`  |

O utilizador escolherá o método; o Krios não detectará SELECT pelo primeiro termo do arquivo. `Query` não produzirá `Page` nem paginação automática.

As operações seguirão o contexto e o timeout do item 3. O consumidor fechará e verificará Rows. Múltiplos conjuntos de linhas serão percorridos com `NextResultSet` quando suportados pelo driver. Vários `sql.Result` não serão agregados em um resultado artificial. [Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet).

`File(path)` resolverá e armazenará o caminho absoluto na criação do builder, usando o diretório de trabalho daquele momento para caminhos relativos. O conteúdo será lido a cada execução. Serão exigidos extensão `.sql`, arquivo regular e conteúdo não vazio. Erros de resolução ou leitura serão retornados na execução, preservando a causa.

O conteúdo inteiro será enviado ao driver em uma chamada, sem divisão por `;`, substituição de variáveis ou interpretação de comandos. O arquivo conterá placeholders corretos para seu dialeto/driver; os argumentos serão enviados separadamente, na ordem fornecida.

Múltiplos comandos dependerão do driver e da configuração escolhida pela aplicação. A integração deverá comprovar execução de todos os comandos e propagação de erros antes de anunciar suporte. Não se presumirá rejeição uniforme de conteúdo não suportado: alguns drivers podem executar apenas parte dele. O driver MySQL possui opção específica e restrições de parâmetros para múltiplos comandos. [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql#multistatements).

Comandos de ferramentas cliente, como DELIMITER, SOURCE e comandos de psql, ficarão fora do escopo. O arquivo conterá SQL aceito pelo driver. [Comandos do cliente mysql](https://dev.mysql.com/doc/refman/8.0/en/mysql-commands.html).

Erros identificarão caminho e operação, sem imprimir automaticamente conteúdo ou argumentos. Não será prometida identificação da linha ou comando exato quando o driver não fornecer essa informação. Falhas fora de transação poderão deixar efeitos parciais; não haverá repetição automática. Nas operações transacionais, a aplicação continuará responsável pelo encerramento da transação.

#### 19. Versões e histórico de migrações

`Config.DbVersion` representará a versão alvo exigida pela aplicação. A versão efetivamente aplicada será obtida da tabela `migrations`. Banco na versão 2 com alvo 8 executará os UPs necessários até 8; a configuração não declarará que o banco já está nessa versão.

As versões usarão `uint64` na API, limitadas a `1..math.MaxInt64`. Esse limite comum foi aprovado para o formato de controle do Krios, substituindo a proposta anterior de fallback para `MaxUint64`. Não será feita detecção genérica do limite do driver nem armazenamento textual para suportar toda a faixa de `uint64`.

BIGINT do PostgreSQL e INTEGER do SQLite possuem faixa com sinal; a conversão padrão de Go também pode rejeitar valores acima de `MaxInt64`. [PostgreSQL](https://www.postgresql.org/docs/16/datatype-numeric.html), [SQLite](https://www.sqlite.org/datatype3.html), [Conversor Go](https://go.dev/src/database/sql/driver/types.go).

As versões serão únicas, positivas e ordenadas, sem exigência de continuidade. O Krios ordenará uma cópia do cadastro. Uma entrada `[2, 8, 6]` será executada como `[2, 6, 8]`; versões repetidas produzirão erro.

O cadastro deverá representar o histórico completo. As versões aplicadas formarão um prefixo da lista ordenada: para `[2, 6, 8]`, serão válidos `[]`, `[2]`, `[2, 6]` e `[2, 6, 8]`; `[2, 8]` será divergente. Versões desconhecidas, duplicadas ou fora dessa sequência lógica bloquearão a execução. Novas migrations não serão inseridas abaixo da maior versão já distribuída ou aplicada.

Versão zero será inválida como alvo e cadastro. Tabela inexistente ou vazia representará estado inicial sem migrations. Não haverá downgrade completo para zero.

`Migrate`/`MigrateTx` retornarão erro para cadastro vazio, alvo ausente ou histórico divergente. `New` aceitará `Migrations: nil` para uso somente do query builder, conforme o item 22.

A tabela registrará somente as versões atualmente aplicadas. Downgrade removerá o registro correspondente; não haverá auditoria permanente de aplicações e reversões. Tabela inexistente será criada. Estrutura existente incompatível produzirá erro, sem alteração automática para apropriação da tabela.

| Coluna       | PostgreSQL               | MySQL                    | SQLite                    |
| ------------ | ------------------------ | ------------------------ | ------------------------- |
| `id`         | `BIGINT PRIMARY KEY`     | `BIGINT PRIMARY KEY`     | `INTEGER PRIMARY KEY`     |
| `version`    | `BIGINT NOT NULL UNIQUE` | `BIGINT NOT NULL UNIQUE` | `INTEGER NOT NULL UNIQUE` |
| `applied_at` | `TIMESTAMPTZ NOT NULL`   | `DATETIME(6) NOT NULL`   | `TEXT NOT NULL`           |

`id` terá o mesmo valor de `version`, sem sequência separada nem dependência de `LastInsertId`. Uma versão removida e reaplicada receberá o mesmo id e nova data.

A tabela usará a origem padrão da conexão, com `main` no SQLite. Não será imposto o schema `public` a todos os bancos. O horário de conclusão do passo será gerado pela aplicação em UTC: instante com fuso no PostgreSQL, representação UTC no MySQL e texto RFC3339Nano no SQLite. Os drivers de integração serão configurados para esses formatos.

As três colunas não detectarão alterações de conteúdo em scripts já aplicados nem toda falha parcial não registrada. Essas limitações integram o escopo aprovado no item 23.

#### 20. Aplicação e recuperação de migrações

O cadastro, o histórico e o plano completo serão validados antes do primeiro script. Todos os arquivos necessários à direção escolhida serão lidos previamente; arquivo UP/DOWN necessário ausente bloqueará o plano. Os conteúdos serão mantidos durante a chamada, sem releitura entre validação e execução.

UPs serão executados em ordem crescente; DOWNs, em ordem decrescente. O DOWN de uma versão desfará seu respectivo UP, retornando ao estado da versão cadastrada anterior, não ao número `version-1`. Com cadastro `[2, 6, 8]`, descer de 8 para 2 executará DOWN 8 e DOWN 6, preservando a versão 2.

Após UP bem-sucedido, o registro será inserido; após DOWN bem-sucedido, será removido. Alvo já aplicado com histórico válido retornará sucesso sem scripts. Downgrade seguirá o alvo informado, sem segunda confirmação durante a execução da biblioteca.

Em `Migrate`, a atomicidade será por migration declarada transacional, incluindo script e atualização do histórico na mesma transação interna. O Krios confirmará em sucesso e tentará rollback em falha. Migrations anteriores confirmadas permanecerão aplicadas; não haverá promessa de atomicidade do plano inteiro.

`Migration.NonTransactional bool`, com padrão `false`, declarará a necessidade de execução fora de transação. Para passos com `true`, o Krios reservará uma conexão do pool durante script e registro. Não analisará SQL para adivinhar sua capacidade transacional. Os scripts não conterão seu próprio BEGIN, COMMIT ou ROLLBACK.

`MigrateTx` verificará previamente se todos os passos necessários são declarados transacionais e rejeitará os demais antes de qualquer script. Histórico e scripts usarão somente a transação recebida; o Krios não criará transação interna nem executará commit/rollback. Caso a criação da tabela de controle exija DDL incompatível com a transação, será necessária preparação pela execução sem transação externa, sem fallback para o pool.

DDL com commit implícito do MySQL será declarado `NonTransactional`. Uma declaração incorreta poderá encerrar a transação; receber `*sql.Tx` não assegura rollback de SQL incompatível. [Commits implícitos do MySQL](https://dev.mysql.com/doc/refman/8.0/en/implicit-commit.html).

Scripts seguirão a política do item 18: envio integral ao driver configurado, com suporte a múltiplos comandos comprovado na integração. O timeout abrangerá a chamada completa. A execução parará no primeiro erro ou cancelamento, sem repetição automática.

Erros identificarão versão, direção, caminho e etapa: leitura, SQL, histórico, commit ou rollback. As causas serão preservadas. Se rollback também falhar, ambos os erros serão preservados, por exemplo com `errors.Join`. Erro de commit poderá deixar resultado incerto.

Falhas não transacionais ou resultados incertos exigirão verificação e reparo manual pela aplicação. Não haverá compensação automática nem DOWN baseado na suposição de que todo UP terminou. A ausência do registro não provará que o script não executou. Uma nova chamada não conseguirá detectar todos esses casos; a aplicação bloqueará novas execuções até verificar e reparar banco e histórico.

Haverá um único executor de migrations por banco, coordenado pela aplicação ou pelo deploy. A exclusão abrangerá leitura do histórico, scripts e registro; em `MigrateTx`, permanecerá até o commit/rollback externo. O Krios não fornecerá bloqueio distribuído automático. Uma chave única ou mutex local não será tratado como garantia contra execução duplicada entre processos.

### Decisões complementares e encerramento

#### 21. Fronteira das validações

O Krios validará configuração, formato dos operandos que precisa transformar, estrutura dos builders, cálculos próprios de paginação e histórico de migrations.

Isso incluirá alias de origem derivada, ORDER BY para paginação, lista versus subconsulta, ausência de origem, combinação de builders de clientes diferentes e demais erros estruturais definidos nesta revisão.

Tipos e valores SQL, existência de tabelas/colunas, compatibilidade de projeções e semântica de expressões ficarão com o driver/banco. O Krios não consultará schema para validar SQL do utilizador, exceto para inspecionar sua tabela de controle de migrations.

#### 22. Configuração, drivers e categorias de erro

`New` validará `Db != nil`, `Context != nil`, dialeto suportado, `Timeout >= 0` e versão alvo positiva dentro do limite do item 19. Depois fará o ping. Não executará migrations automaticamente.

Cadastro de migrations vazio será permitido na criação, mas rejeitado em `Migrate`/`MigrateTx`. A obrigatoriedade de `DbVersion >= 1` permanecerá também nos clientes usados somente para queries.

A struct de configuração e o slice de migrations serão copiados na criação, sem setters posteriores. Pool e contexto continuarão sendo referências; fechá-los ou cancelá-los afetará o cliente. Arquivos de migrations não serão lidos em `New`.

A biblioteca não imporá um driver obrigatório. Drivers e versões serão escolhidos e fixados na tarefa de integração, cobrindo os mínimos do item 2, múltiplos comandos e cancelamento.

As categorias públicas `ErrInvalidConfig`, `ErrInvalidQuery` e `ErrInvalidMigration` estarão acessíveis por `krios.go` e serão verificáveis com `errors.Is`. Erros do driver/banco preservarão sua causa e não serão classificados como validação do Krios. Mensagens não exporão argumentos ou credenciais automaticamente.

#### 23. Garantias e limites do escopo inicial de migrações

Foi aprovado o escopo simples dos itens 19 e 20: histórico com três colunas, exclusão coordenada pela aplicação e recuperação manual de falhas parciais ou incertas.

Não fazem parte desse escopo:

- Checksums para detectar alterações em scripts já aplicados.
- Tabela adicional para registrar operação em andamento ou falha persistente.
- Bloqueio distribuído automático e gestão de locks por banco.
- Operação automática de reparo ou regularização do histórico.

Essas possibilidades foram avaliadas como ampliação futura e não são requisitos da implementação inicial. Sua eventual inclusão exigirá novo planejamento e aprovação.

A documentação deverá declarar as limitações aprovadas. O Krios verificará a estrutura e a sequência do histórico, sem prometer detecção de toda divergência semântica do schema ou execução concorrente segura por conta própria.

A regra de uso exclusivo da transação recebida continuará válida: `MigrateTx` não gravará marcadores ou diagnósticos em outra conexão para escapar do rollback.

#### 24. Atualização documental acordada

A revisão de DEFINITIONS.md identificou correções a serem realizadas pelo desenvolvedor. A decisão sobre essas correções está encerrada; sua aplicação no arquivo não foi executada nesta consolidação.

Já estavam registrados no documento consultado: `DbVersion` e `Migration.Version` como `uint64`, remoção de `TotalPages`, correção de `Union(query1, query2)` e adição de subselect em IN.

Os ajustes acordados são:

- Uniformizar `Timeout` como campo exportado, incluindo a declaração de `Config` e o exemplo de migração.
- Atualizar todos os exemplos de `New` para receber e tratar seu erro.
- Corrigir os exemplos de paginação para offset/limite, com ORDER BY explícito. `Paginate(1, 10)` corresponde a OFFSET 1.
- Substituir `krios.Eq` por `client.Eq` nos exemplos lógicos.
- Corrigir o SELECT com alias para incluir `AS u` no FROM e alinhar os exemplos de WHERE às colunas e aliases efetivamente declarados.
- Associar o predicado de subselect em IN a uma consulta externa.
- Atualizar COUNT(\*) e seleção implícita conforme o item 12.
- Substituir “sem limites” pela faixa de versões aprovada no item 19 e registrar os mínimos de Go e dos bancos.
- Incorporar as assinaturas, regras, exemplos e limites aprovados nos itens 1–23.

Não restam questões de decisão abertas nesta revisão. Detalhes das tarefas, implementação e validação técnica serão tratados no planejamento do projeto.

## Tarefa 001 — Configuração e ciclo de vida do cliente

**Estado: implementação autorizada, concluída e verificada em 2026-11-04.**

O escopo geral de `PLANNING.md` foi aprovado pelo desenvolvedor em 2026-11-04. Após a preparação e revisão, o desenvolvedor autorizou a implementação da tarefa 001.

### 001.1 — Organização e verificação da primeira implementação

**Proposta:** adotar o detalhamento de [001.md](001.md), com implementação em `internal/core`, exposição dos tipos por aliases em `krios.go`, `Dialect` definido sobre `string` e testes com driver controlado usando somente a biblioteca padrão. Reservar `Page` para a tarefa 004 e os recursos funcionais para suas tarefas previstas no plano.

**Pergunta:** Sim esta aprovdo

**Resposta do desenvolvedor:** respondido.

**Autorização de implementação:** “pode implementar 001”, recebida em 2026-11-04.

### 001.2 — Ajustes identificados na validação do detalhamento

**Estado: ajustes aprovados e incorporados em 2026-11-04.**

Na validação de `001.md` contra o contrato e o plano aprovado, foram identificados três pontos para tornar a implementação e sua conclusão mais objetivas:

1. **Contexto de operações com linhas:** explicitar que a biblioteca não interceptará `Rows.Close()` e que o timer poderá permanecer até o prazo vencer. A expressão “sem cancelá-lo antes do consumo” deve preservar a possibilidade de timeout ou cancelamento do contexto pai durante a leitura, conforme `DEFINITIONS.md`.
2. **Limite da abstração de executor nesta tarefa:** confirmar se a entrega consiste apenas na interface compatível com pool/transação e no componente de contexto usado pelo ping, deixando seleção de executor e execução de SQL para as tarefas funcionais. Essa é a proposta da revisão; os critérios de conclusão deverão distinguir o que será verificado agora do que será verificado nessas tarefas futuras.
3. **Mensagens de erro:** incluir expressamente a regra já aprovada de não acrescentar argumentos ou credenciais às mensagens produzidas pelo Krios, preservando as causas dos erros do driver. Acrescentar a verificação correspondente ao escopo de criação do cliente, sem prometer sanitização do conteúdo de erros originados no driver.

**Pergunta:** podemos incorporar esses esclarecimentos ao detalhamento da tarefa 001, adotando o limite de entrega proposto no item 2?

**Resposta do desenvolvedor:** “sim pode”. Aprovação para incorporar os três esclarecimentos, incluindo o limite da entrega do executor à interface e ao componente de contexto nesta tarefa.

**Aplicação:** `001.md` atualizado com os esclarecimentos e os respectivos limites de verificação. Posteriormente, a implementação foi autorizada no item 001.1.

## Tarefa 002 — Dialetos, identificadores e expressões

**Estado: decisões e organização revisada aprovadas; implementação concluída e verificada em 2026-11-04.**

O planejamento detalhado está em [002.md](002.md). As decisões abaixo complementam o contrato sem alterar `DEFINITIONS.md`. A organização revisada está registrada em `002.md`.

### 002.1 — API e organização da implementação

**Proposta:** manter os componentes em `internal/core`, com arquivos para dialeto, referência, expressão e compilação. Expor `Reference` e `Expression` por aliases em `krios.go`. `Ref`/`RefParts` retornarão `*Reference`; os cinco agregadores `*Expr` receberão `any` e retornarão `*Expression`, aceitando strings, referências e expressões. Erros serão guardados para a compilação e para a execução dos futuros builders. Não haverá API pública de compilação nem execução SQL nesta tarefa.

**Pergunta:** aprova essas assinaturas, a organização e os limites descritos no detalhamento?

**Resposta do desenvolvedor:** aprovo as assinaturas, mas quero separar os pacotes, não acho que devam ficar em core, tudo referente aos dialetos, devem ficar em um pacote chamado `dialects`, e tbem quero separação por arquivos, o postresql vai ter o dele assim como os outros bancos, se for possivel, o mesmo para expressões, que devem ficar em um pacote chamado expressions, vamos colocar mais uma regra, eu quero arquivos e pacotes bem separados por responsabilidade.

### 002.2 — Componentes inválidos e curingas

**Proposta:** rejeitar componentes vazios, lista de componentes vazia e NUL; preservar espaços e nomes sem normalização. Em strings simples e `Ref`, reconhecer `*` como curinga apenas quando constituir o último componente inteiro. Em `RefParts`, todos os componentes serão literais, inclusive `*`, permitindo representar uma coluna com esse nome. O curinga simples permanecerá sem prefixo automático, e `CountExpr("*")` continuará produzindo `COUNT(*)`.

**Pergunta:** aprova essa distinção entre `Ref("u.*")` e `RefParts("u", "*")` e as validações propostas?

**Resposta do desenvolvedor:** aprovado.

### 002.3 — Origem de referências e expressões

**Proposta:** associar os operandos ao cliente criador e rejeitar seu uso por outro cliente, mesmo com o mesmo dialeto. O contrato já rejeita builders de clientes diferentes; esta proposta explicita a mesma restrição para referências e expressões.

**Pergunta:** referências e expressões também devem ficar restritas ao cliente que as criou, conforme a proposta?

**Resposta do desenvolvedor:** aprovado.

### 002.4 — Aprovação da organização revisada e implementação

Após a revisão de `002.md` e o esclarecimento sobre os métodos delegadores de `Client` em `core`, o desenvolvedor respondeu: “não precisamos fazer os ajustes, se estiver tudo certo pode implementar”.

**Decisão:** organização revisada aprovada, mantendo os dois arquivos delegadores em `core`, e implementação da tarefa 002 autorizada em 2026-11-04.
