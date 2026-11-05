# Definições do Krios

Este documento descreve o contrato da biblioteca Krios, consolidado a partir das decisões aprovadas em [QUESTIONS.md](QUESTIONS.md), revisão encerrada em 2026-11-04. A atualização deste arquivo foi autorizada expressamente pelo desenvolvedor após o encerramento da revisão.

O Krios é uma biblioteca Go para construção de consultas SQL, execução de arquivos SQL e gerenciamento de migrações. O escopo inicial atende SQLite, PostgreSQL e MySQL. Recursos adicionais serão tratados em tarefas próprias, conforme necessidade e aprovação.

A API pública será disponibilizada por `krios.go`; a implementação ficará em `internal`. A aplicação fornecerá conexão, driver e contexto. Resultados de banco manterão os tipos de `database/sql`, e os erros serão propagados com suas causas.

## Convenções dos exemplos

Os exemplos representam o contrato aprovado, não código já implementado. Cada bloco de uso é independente, salvo indicação de continuidade. Imports e o código da aplicação que obtém `db` ou `tx` podem ser omitidos; trechos com `return err` pertencem a funções que retornam erro.

Nos exemplos do query builder, `client` usa PostgreSQL. Os comentários mostram SQL equivalente, omitindo aspas de identificadores para facilitar a leitura; a geração efetiva deve aplicar o escape e os placeholders de cada dialeto. Exemplos de execução pelo pool e por transação são alternativas, não instruções para repetir a operação.

## Configuração

```go
type Config struct {
    Db         *sql.DB
    DbVersion  uint64
    Dialect    Dialect
    Timeout    time.Duration
    Context    context.Context
    Migrations []Migration
}
```

| Constante pública         | Valor do dialeto |
| ------------------------- | ---------------- |
| `krios.DialectSQLite3`    | `"sqlite3"`      |
| `krios.DialectPostgreSQL` | `"postgres"`     |
| `krios.DialectMySQL`      | `"mysql"`        |

`DbVersion` representa a versão alvo do schema da aplicação, não a versão do motor PostgreSQL, MySQL ou SQLite.

### Criação e validação

O campo de configuração de timeout será público, com o nome `Timeout`. A criação do cliente terá a assinatura `New(config Config) (*Client, error)`.

Erros de configuração ou conexão serão retornados ao utilizador. Quando a criação falhar, o cliente retornado será `nil`.

`New` validará `Db != nil`, `Context != nil`, dialeto suportado, `Timeout >= 0` e versão alvo positiva dentro do limite da seção **Versões e histórico**. Depois fará o ping. Não executará migrations automaticamente.

Cadastro de migrations vazio será permitido na criação, mas rejeitado em `Migrate`/`MigrateTx`. A obrigatoriedade de `DbVersion >= 1` permanecerá também nos clientes usados somente para queries.

A struct de configuração e o slice de migrations serão copiados na criação, sem setters posteriores. Pool e contexto continuarão sendo referências; fechá-los ou cancelá-los afetará o cliente. Arquivos de migrations não serão lidos em `New`.

A biblioteca não imporá um driver obrigatório. Drivers e versões serão escolhidos e fixados na tarefa de integração, cobrindo os mínimos da seção **Conexão e versões mínimas**, múltiplos comandos e cancelamento.

As categorias públicas `ErrInvalidConfig`, `ErrInvalidQuery` e `ErrInvalidMigration` estarão acessíveis por `krios.go` e serão verificáveis com `errors.Is`. Erros do driver/banco preservarão sua causa e não serão classificados como validação do Krios. Mensagens não exporão argumentos ou credenciais automaticamente.

```go
// db foi criado pela aplicação com um driver PostgreSQL registrado.
client, err := krios.New(krios.Config{
    Db:         db,
    DbVersion:  1,
    Dialect:    krios.DialectPostgreSQL,
    Timeout:    5 * time.Second,
    Context:    context.Background(),
    Migrations: nil,
})
if err != nil {
    return err
}
```

A configuração acima permite usar o query builder. Uma chamada a `Migrate` com esse cadastro vazio retorna erro.

### Conexão e versões mínimas

A conexão será validada em `New`, após a validação da configuração, por meio de `Db.PingContext` com o contexto e o timeout aplicáveis. O utilizador fornecerá o pool `*sql.DB` e será responsável por fechá-lo. O ping verifica a disponibilidade naquele momento, sem garantir execuções futuras.

| Dependência | Versão mínima |
| ----------- | ------------- |
| Go          | 1.26          |
| PostgreSQL  | 16            |
| MySQL       | 8.0           |
| SQLite      | 3.39          |

A aplicação escolherá e registrará o driver. O Krios não detectará automaticamente a versão do banco. A compatibilidade entre banco, versão, driver e dialeto informado será responsabilidade do utilizador.

### Contexto e timeout

Todas as operações usarão `Config.Context`. `Exec()` e `ExecTx(tx)` não receberão contexto adicional.

| Valor de `Timeout` | Comportamento                                                                   |
| ------------------ | ------------------------------------------------------------------------------- |
| Negativo           | Erro de configuração em `New`.                                                  |
| Zero               | Usar somente o prazo e o cancelamento de `Config.Context`.                      |
| Positivo           | Criar um contexto derivado por operação; prevalece o prazo que vencer primeiro. |

O timeout começará na execução, não na criação do builder. Abrangerá o ping em `New`, toda a execução das escritas e a chamada completa de migração, sem reiniciar por comando. No SELECT paginado, contagem, consulta e leitura das linhas compartilharão o mesmo prazo.

O cancelamento de `Config.Context` afetará operações em andamento e futuras daquele cliente. O Krios propagará o contexto às chamadas de `database/sql` e não iniciará novos comandos após observar seu cancelamento.

Ao retornar `*sql.Rows`, o Krios manterá válido o contexto derivado para permitir a leitura pelo consumidor. Não executará `cancel()` simplesmente ao retornar de `Exec`. Em falhas anteriores à entrega das linhas, cancelará imediatamente; em operações sem linhas, liberará o contexto derivado ao término.

Como `*sql.Rows` será retornado diretamente, o Krios não interceptará `Rows.Close()` para liberar seu timer. Está aceito que, após sucesso, o timer permaneça até o prazo vencer, mesmo quando as linhas forem fechadas antes. O campo `Timeout` faz parte da configuração pública.

Referência: [Cancelamento de operações no Go](https://go.dev/doc/database/cancel-operations).

## Execução e resultados

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

Exemplo de consumo de um SELECT sem paginação:

```go
query := client.Select().From("users").Columns("id", "name")
rows, _, err := query.Exec()
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int64
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    // A aplicação utiliza id e name.
}
if err := rows.Err(); err != nil {
    return err
}
```

O consumidor deve fechar as linhas também quando interromper a leitura antes do fim.

### Transações

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

Exemplo de escrita em uma transação criada pela aplicação, usando o contexto que também foi fornecido na configuração do cliente:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

query := client.Update().From("users").Set("name", "Ana")
query.Where(client.Eq("users.id", 1))
if _, err := query.ExecTx(tx); err != nil {
    return err
}
if err := tx.Commit(); err != nil {
    return err
}
```

O `defer` garante uma tentativa de rollback nas saídas anteriores ao commit; a aplicação continua responsável pelo tratamento dos erros de encerramento.

## Regras do query builder

### Responsabilidades e validações

O Krios validará configuração, formato dos operandos que precisa transformar, estrutura dos builders, cálculos próprios de paginação e histórico de migrations.

Isso incluirá alias de origem derivada, ORDER BY para paginação, lista versus subconsulta, ausência de origem, combinação de builders de clientes diferentes e demais erros estruturais definidos nesta revisão.

Tipos e valores SQL, existência de tabelas/colunas, compatibilidade de projeções e semântica de expressões ficarão com o driver/banco. O Krios não consultará schema para validar SQL do utilizador, exceto para inspecionar sua tabela de controle de migrations.

### Tipos e valores

Os valores serão encaminhados como argumentos, sem lista restritiva de tipos, conversões antecipadas de datas ou números, nem chamada manual a `driver.Valuer`.

A aceitação e conversão dos valores caberão a `database/sql`, ao driver e ao banco. Seus erros serão preservados e retornados. Isso não implica que todo valor seja aceito em qualquer banco.

O Krios validará a estrutura necessária à construção das consultas e as regras do próprio contrato, conforme a seção **Responsabilidades e validações**. Essa responsabilidade inclui grupos vazios, escritas incompletas, transação nula e versões duplicadas.

Referência: [Conversão padrão de argumentos](https://pkg.go.dev/database/sql/driver#DefaultParameterConverter).

### SQL e parâmetros

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

### Identificadores e expressões

Strings em posições de tabela, coluna, agrupamento, ordenação e primeiro operando de predicados representarão identificadores. Strings em posições de valor continuarão parametrizadas.

- `client.Ref("o.user_id")` representará uma coluna explicitamente. `client.Eq("u.id", client.Ref("o.user_id"))` comparará duas colunas, sem parâmetro textual. Referências também serão aceitas como primeiro operando.
- Cada componente do identificador será escapado separadamente: aspas duplas no PostgreSQL/SQLite e crases no MySQL, duplicando delimitadores internos. `u.id` terá dois componentes; `u.*` preservará o curinga. SQL já montado não será interpretado como identificador.
- Nomes simples do SELECT receberão o alias da tabela principal, quando definido. Nomes qualificados não receberão outro prefixo. Em ON, referências qualificadas serão recomendadas. Schema qualificará a origem da tabela, não o alias.
- `client.RefParts("u", "nome.com.ponto")` permitirá componentes literais com ponto. Cada argumento será indivisível; uma string simples com ponto continuará representando qualificação.
- `client.CountExpr`, `MaxExpr`, `MinExpr`, `AvgExpr` e `SumExpr` representarão agregações em operandos. Exemplo: `client.Gt(client.CountExpr("*"), 1)`.

O escopo inicial não incluirá parser de expressões SQL nem função genérica de SQL cru. Os tipos concretos dessas referências e expressões serão detalhados nas tarefas de implementação, preservando as operações aprovadas.

### Estado, encadeamento e reutilização

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

Qualquer mudança na árvore da consulta invalidará os totais e reiniciará o offset dos pais, conforme a seção **Contagem, navegação e consistência**. A propagação incluirá mudanças feitas diretamente nos filhos e deverá ser verificada nos testes.

Erros estruturais serão registrados e retornados por `Exec`/`ExecTx` antes do SQL. Um builder com erro permanecerá inválido; a correção exigirá outro builder. `Clone` não fará parte do escopo inicial.

## Critérios

Os predicados são criados pelo cliente, que fornece o dialeto da consulta.

| Predicado    | Operação SQL  |
| ------------ | ------------- |
| `Eq`         | `=`           |
| `Ne`         | `!=`          |
| `Lt`         | `<`           |
| `Lte`        | `<=`          |
| `Gt`         | `>`           |
| `Gte`        | `>=`          |
| `Like`       | `LIKE`        |
| `NotLike`    | `NOT LIKE`    |
| `IsNull`     | `IS NULL`     |
| `IsNotNull`  | `IS NOT NULL` |
| `In`         | `IN`          |
| `NotIn`      | `NOT IN`      |
| `Between`    | `BETWEEN`     |
| `NotBetween` | `NOT BETWEEN` |

```go
client.Eq("id", 1)                    // id = $1; argumento: 1
client.Eq("name", "Ana")              // name = $1; argumento: "Ana"
client.Eq("created_at", time.Now())   // created_at = $1; argumento: time.Time
client.Eq("active", true)             // active = $1; argumento: true
client.Like("name", "%Ana%")          // name LIKE $1; argumento: "%Ana%"
client.In("id", []int{1, 2, 3})        // id IN ($1, $2, $3); argumentos: 1, 2, 3
client.Between("id", 1, 2)            // id BETWEEN $1 AND $2; argumentos: 1, 2
```

Cada expressão acima é independente. Os índices definitivos serão atribuídos ao montar a consulta completa.

### NULL e composição lógica

`Eq("id", nil)` produzirá `id = $1`, com argumento `nil`, sem transformação em `IS NULL`. A mesma política valerá para os demais operadores parametrizados. `IsNull` e `IsNotNull` permanecerão explícitos.

Comparações normais com NULL geralmente produzem UNKNOWN, não erro. Em WHERE, isso não seleciona a linha, inclusive quando a coluna é nula. O Krios preservará essa semântica. [Comparações com NULL](https://www.postgresql.org/docs/16/functions-comparison.html).

`And()` e `Or()` vazios, assim como critérios nulos em grupos, `Where`, `On` ou `Having`, produzirão erro na execução. Nenhum desses elementos será descartado silenciosamente. Grupos explícitos serão envolvidos por parênteses para preservar a precedência.

Todos os predicados estarão disponíveis exclusivamente no cliente, conforme o dialeto da instância.

```go
query := client.Select().From("users").As("u").Columns("id", "name")
query.Where(client.Or(
    client.And(client.Eq("u.id", 1), client.Eq("u.name", "Ana")),
    client.Eq("u.active", true),
))
// SQL: SELECT u.id, u.name FROM users AS u
// WHERE ((u.id = $1 AND u.name = $2) OR u.active = $3)
```

Condições passadas separadamente a `Where` serão combinadas por AND:

```go
query := client.Select().From("users").As("u").Columns("id", "name")
query.Where(client.Eq("u.id", 1), client.Eq("u.name", "Ana"))
// SQL: SELECT u.id, u.name FROM users AS u
// WHERE u.id = $1 AND u.name = $2
```

### IN, NOT IN e subconsultas

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

## SELECT

### Projeção e subconsultas

Sem projeção explícita, a consulta usará `SELECT *`, inclusive com joins. A primeira chamada a `Column`, `Columns`, agregador ou `ColumnSelect` removerá a seleção implícita; chamadas seguintes acumularão projeções.

Para selecionar os campos principais e uma subconsulta, será necessário declarar ambos, por exemplo `Column("u.*").ColumnSelect(sub).As("total")`.

`Count("*")` sempre produzirá `COUNT(*)`, sem prefixo de alias. Os demais argumentos seguirão as regras de identificadores e expressões. O Krios não acrescentará colunas, agregações ou GROUP BY para corrigir a consulta.

`SubSelect(sub).As("o")` acrescentará uma origem derivada ao FROM, produzindo produto cartesiano com a origem principal. Origens derivadas e `Wrapper` exigirão alias explícito. Nenhuma condição de JOIN será criada automaticamente.

`ColumnSelect(sub)` representará subconsulta escalar. Para resultados portáveis, o utilizador fornecerá uma coluna e no máximo uma linha. A verificação ficará com o banco: SQLite pode usar a primeira linha em situações nas quais outro banco retorna erro. [Subconsultas escalares no SQLite](https://www.sqlite.org/lang_expr.html#subquery_expressions).

Correlação será permitida em subconsultas escalares e em IN/NOT IN por referências qualificadas, como `client.Ref("u.id")`. O alias interno não substituirá referências ao escopo externo. Subconsultas correlacionadas no FROM e LATERAL ficarão fora do escopo inicial.

### Origens, aliases e agregações

Cada trecho abaixo representa uma consulta independente:

```go
query := client.Select().From("users")
// SQL: SELECT * FROM users
```

```go
query := client.Select().From("users").Distinct()
// SQL: SELECT DISTINCT * FROM users
```

```go
query := client.Select().From("users").As("u").Schema("public").Distinct()
// SQL: SELECT DISTINCT * FROM public.users AS u
// public é informado explicitamente neste exemplo PostgreSQL.
```

```go
query := client.Select().From("users").As("u").Columns("id", "name")
// SQL: SELECT u.id, u.name FROM users AS u
```

```go
query := client.Select().From("users").As("u").Column("id").As("user_id")
// SQL: SELECT u.id AS user_id FROM users AS u
```

```go
query := client.Select().From("users").As("u").Count("*").As("total")
// SQL: SELECT COUNT(*) AS total FROM users AS u
```

Os agregadores de projeção serão `Count`, `Max`, `Min`, `Avg` e `Sum`. Para agregações nos operandos dos predicados, usar as expressões `CountExpr`, `MaxExpr`, `MinExpr`, `AvgExpr` e `SumExpr` do cliente.

```go
query := client.Select().From("users").Columns("id", "name")
query.OrderBy("id").OrderBy("name").Asc().OrderBy("created_at").Desc()
// SQL: SELECT id, name FROM users ORDER BY id, name ASC, created_at DESC
```

```go
query := client.Select().From("users").Columns("id", "name").GroupBy("id", "name")
// SQL: SELECT id, name FROM users GROUP BY id, name
```

Exemplos de origem derivada e subconsulta na projeção:

```go
sub := client.Select().From("others")
query := client.Select().From("users").As("u").SubSelect(sub).As("o")
// SQL: SELECT * FROM users AS u, (SELECT * FROM others) AS o
```

```go
sub := client.Select().From("others").Count("*")
query := client.Select().From("users").As("u")
query.Column("u.*").ColumnSelect(sub).As("total")
// SQL: SELECT u.*, (SELECT COUNT(*) FROM others) AS total FROM users AS u
```

### JOIN e HAVING

`Having(criterios...)` fará parte do escopo inicial e acumulará condições com AND. Agregações serão representadas pelas expressões da seção **Identificadores e expressões**, por exemplo `query.Having(client.Gt(client.CountExpr("*"), 1))`.

WHERE filtrará linhas antes da agregação; HAVING filtrará grupos. GROUP BY não será exigido em todo uso de HAVING; a validade da expressão ficará com o banco. [Agrupamento e HAVING](https://www.postgresql.org/docs/16/queries-table-expressions.html#QUERIES-GROUP).

Serão incluídos `Join`/`InnerJoin`, `LeftJoin`, `RightJoin` e `CrossJoin`. Os três primeiros tipos exigirão ON não vazio; CrossJoin não receberá ON. Comparações entre colunas usarão `client.Ref`. O Krios não moverá condições entre ON e WHERE.

`FullJoin`, `NaturalJoin`, `Using` e `Lateral` ficarão fora do escopo inicial. FULL OUTER JOIN existe em PostgreSQL e SQLite 3.39+, mas não é oferecido nativamente pelo MySQL 8.0. [SQLite 3.39](https://www.sqlite.org/releaselog/3_39_0.html), [JOIN no MySQL](https://dev.mysql.com/doc/refman/8.0/en/join.html).

Recursos indisponíveis não serão emulados. O dialeto orientará a geração do SQL; incompatibilidades observadas na execução serão propagadas. Formas não representáveis pela API produzirão erro estrutural, conforme a seção **Responsabilidades e validações**.

```go
join := client.Join("orders").As("o").On(
    client.Eq("u.id", client.Ref("o.user_id")),
)
query := client.Select().From("users").As("u").Columns("u.id", "o.total")
query.Join(join)
// SQL: SELECT u.id, o.total FROM users AS u
// INNER JOIN orders AS o ON u.id = o.user_id
```

```go
query := client.Select().From("orders").Column("user_id").Count("*").As("total")
query.GroupBy("user_id").Having(client.Gt(client.CountExpr("*"), 1))
// SQL: SELECT user_id, COUNT(*) AS total FROM orders
// GROUP BY user_id HAVING COUNT(*) > $1
// Argumentos: []any{1}
```

### UNION, UNION ALL e Wrapper

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
query1 := client.Select().From("users").Columns("id")
query2 := client.Select().From("archived_users").Columns("id")
combined := client.Union(query1, query2)
outer := client.Wrapper(combined).As("w").Columns("id")
outer.OrderBy("id").Asc().Paginate(0, 10)
```

`Wrapper` também aceita um SELECT simples:

```go
main := client.Select().From("users")
query := client.Wrapper(main).As("w").Columns("id")
// SQL: SELECT w.id FROM (SELECT * FROM users) AS w
```

## Paginação

```go
type Page struct {
    TotalRows       uint64
    RowsPerPage     uint64
    CurrentPage     uint64
    LastPage        uint64
    HasNextPage     bool
    HasPreviousPage bool
}
```

### Offset, limite e metadados

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

Os campos de `Page` permanecerão `uint64` quando numéricos e booleanos nos indicadores. O total de páginas convencionais será representado por `LastPage`.

```go
query := client.Select().From("users").OrderBy("id").Asc().Paginate(0, 10)
// SQL: SELECT * FROM users ORDER BY id ASC LIMIT 10 OFFSET 0
rows, page, err := query.Exec()
if err != nil {
    return err
}
defer rows.Close()
// A aplicação consome Rows, verifica Rows.Err() e utiliza page.
```

```go
query := client.Select().From("users").OrderBy("id").Asc().Paginate(1, 10)
// SQL: SELECT * FROM users ORDER BY id ASC LIMIT 10 OFFSET 1
```

A forma transacional do SELECT é `rows, page, err := query.ExecTx(tx)`; segue o mesmo contrato de paginação e de consumo das linhas.

### Contagem, navegação e consistência

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

## Escritas

### Validações comuns

As validações estruturais ocorrerão em `Exec`/`ExecTx`, antes do primeiro comando SQL. Produzirão erro:

- Ausência da tabela de destino.
- INSERT sem colunas ou valores, ou com quantidade de valores incompatível com as colunas.
- UPDATE sem `Set`.
- UPDATE ou DELETE sem critérios WHERE.
- `Where()` vazio ou contendo grupo vazio.

`Set` repetido para a mesma coluna manterá o último valor, preservando sua posição original. Valores serão encaminhados ao driver conforme a seção **Tipos e valores**.

Não haverá bypass do WHERE obrigatório. Um WHERE não vazio poderá afetar toda a tabela; o Krios não tentará provar sua seletividade.

O `sql.Result` do driver será preservado, sem garantia universal de `LastInsertId` nem ocultação dos erros de seus métodos. [Contrato de sql.Result](https://pkg.go.dev/database/sql#Result).

### INSERT

`Values` acumulará linhas na ordem recebida:

```go
query := client.Insert().Into("users").Columns("name", "email")
query.Values("Ana", "ana@example.com")
query.Values("Bruno", "bruno@example.com")
// SQL: INSERT INTO users (name, email) VALUES ($1, $2), ($3, $4)
result, err := query.Exec()
if err != nil {
    return err
}
// A aplicação utiliza result e trata erros de seus métodos.
```

### UPDATE

```go
query := client.Update().From("users").Set("name", "Ana").Set("email", "ana@example.com")
query.Where(client.Eq("users.id", 1))
// SQL: UPDATE users SET name = $1, email = $2 WHERE users.id = $3
result, err := query.Exec()
if err != nil {
    return err
}
// A aplicação utiliza result.
```

### DELETE

```go
query := client.Delete().From("users")
query.Where(client.Eq("users.id", 1))
// SQL: DELETE FROM users WHERE users.id = $1
result, err := query.Exec()
if err != nil {
    return err
}
// A aplicação utiliza result.
```

Para executar qualquer uma dessas escritas em uma transação fornecida, usar `result, err := query.ExecTx(tx)` em substituição à execução pelo pool.

## Execução de arquivos SQL

O builder de arquivo separará execução sem leitura de linhas e consulta de linhas:

| Método                             | Retorno               |
| ---------------------------------- | --------------------- |
| `Exec(args ...any)`                | `(sql.Result, error)` |
| `ExecTx(tx *sql.Tx, args ...any)`  | `(sql.Result, error)` |
| `Query(args ...any)`               | `(*sql.Rows, error)`  |
| `QueryTx(tx *sql.Tx, args ...any)` | `(*sql.Rows, error)`  |

O utilizador escolherá o método; o Krios não detectará SELECT pelo primeiro termo do arquivo. `Query` não produzirá `Page` nem paginação automática.

As operações seguirão o contexto e o timeout da seção **Contexto e timeout**. O consumidor fechará e verificará Rows. Múltiplos conjuntos de linhas serão percorridos com `NextResultSet` quando suportados pelo driver. Vários `sql.Result` não serão agregados em um resultado artificial. [Rows.NextResultSet](https://pkg.go.dev/database/sql#Rows.NextResultSet).

`File(path)` resolverá e armazenará o caminho absoluto na criação do builder, usando o diretório de trabalho daquele momento para caminhos relativos. O conteúdo será lido a cada execução. Serão exigidos extensão `.sql`, arquivo regular e conteúdo não vazio. Erros de resolução ou leitura serão retornados na execução, preservando a causa.

O conteúdo inteiro será enviado ao driver em uma chamada, sem divisão por `;`, substituição de variáveis ou interpretação de comandos. O arquivo conterá placeholders corretos para seu dialeto/driver; os argumentos serão enviados separadamente, na ordem fornecida.

Múltiplos comandos dependerão do driver e da configuração escolhida pela aplicação. A integração deverá comprovar execução de todos os comandos e propagação de erros antes de anunciar suporte. Não se presumirá rejeição uniforme de conteúdo não suportado: alguns drivers podem executar apenas parte dele. O driver MySQL possui opção específica e restrições de parâmetros para múltiplos comandos. [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql#multistatements).

Comandos de ferramentas cliente, como DELIMITER, SOURCE e comandos de psql, ficarão fora do escopo. O arquivo conterá SQL aceito pelo driver. [Comandos do cliente mysql](https://dev.mysql.com/doc/refman/8.0/en/mysql-commands.html).

Erros identificarão caminho e operação, sem imprimir automaticamente conteúdo ou argumentos. Não será prometida identificação da linha ou comando exato quando o driver não fornecer essa informação. Falhas fora de transação poderão deixar efeitos parciais; não haverá repetição automática. Nas operações transacionais, a aplicação continuará responsável pelo encerramento da transação.

Exemplo de comando sem leitura de linhas:

```go
file := client.File("sql/update_users.sql")
result, err := file.Exec()
if err != nil {
    return err
}
// A aplicação utiliza result.
```

Exemplo independente de arquivo PostgreSQL com `SELECT id, name FROM users WHERE id = $1`:

```go
file := client.File("sql/select_user.sql")
rows, err := file.Query(1)
if err != nil {
    return err
}
defer rows.Close()
// A aplicação consome e verifica Rows, como no exemplo de SELECT.
```

As alternativas transacionais são `file.ExecTx(tx, args...)` e `file.QueryTx(tx, args...)`.

## Migrações

```go
type Migration struct {
    Version          uint64
    Up               string
    Down             string
    NonTransactional bool
}
```

`Up` e `Down` são caminhos de arquivos `.sql`, compatíveis com o dialeto do cliente. `NonTransactional` é uma declaração do autor sobre o modo de execução do passo. O conteúdo e a correção dos scripts são responsabilidade do utilizador, que deve ser orientado a realizar backup antes de executar migrações.

### Versões e histórico

`Config.DbVersion` representará a versão alvo exigida pela aplicação. A versão efetivamente aplicada será obtida da tabela `migrations`. Banco na versão 2 com alvo 8 executará os UPs necessários até 8; a configuração não declarará que o banco já está nessa versão.

As versões usarão `uint64` na API, limitadas a `1..math.MaxInt64`. Esse é o limite comum do formato de controle do Krios. Não será feita detecção genérica do limite do driver nem armazenamento textual para suportar toda a faixa de `uint64`.

BIGINT do PostgreSQL e INTEGER do SQLite possuem faixa com sinal; a conversão padrão de Go também pode rejeitar valores acima de `MaxInt64`. [PostgreSQL](https://www.postgresql.org/docs/16/datatype-numeric.html), [SQLite](https://www.sqlite.org/datatype3.html), [Conversor Go](https://go.dev/src/database/sql/driver/types.go).

As versões serão únicas, positivas e ordenadas, sem exigência de continuidade. O Krios ordenará uma cópia do cadastro. Uma entrada `[2, 8, 6]` será executada como `[2, 6, 8]`; versões repetidas produzirão erro.

O cadastro deverá representar o histórico completo. As versões aplicadas formarão um prefixo da lista ordenada: para `[2, 6, 8]`, serão válidos `[]`, `[2]`, `[2, 6]` e `[2, 6, 8]`; `[2, 8]` será divergente. Versões desconhecidas, duplicadas ou fora dessa sequência lógica bloquearão a execução. Novas migrations não serão inseridas abaixo da maior versão já distribuída ou aplicada.

Versão zero será inválida como alvo e cadastro. Tabela inexistente ou vazia representará estado inicial sem migrations. Não haverá downgrade completo para zero.

`Migrate`/`MigrateTx` retornarão erro para cadastro vazio, alvo ausente ou histórico divergente. `New` aceitará `Migrations: nil` para uso somente do query builder, conforme a seção **Criação e validação**.

A tabela registrará somente as versões atualmente aplicadas. Downgrade removerá o registro correspondente; não haverá auditoria permanente de aplicações e reversões. Tabela inexistente será criada. Estrutura existente incompatível produzirá erro, sem alteração automática para apropriação da tabela.

| Coluna       | PostgreSQL               | MySQL                    | SQLite                    |
| ------------ | ------------------------ | ------------------------ | ------------------------- |
| `id`         | `BIGINT PRIMARY KEY`     | `BIGINT PRIMARY KEY`     | `INTEGER PRIMARY KEY`     |
| `version`    | `BIGINT NOT NULL UNIQUE` | `BIGINT NOT NULL UNIQUE` | `INTEGER NOT NULL UNIQUE` |
| `applied_at` | `TIMESTAMPTZ NOT NULL`   | `DATETIME(6) NOT NULL`   | `TEXT NOT NULL`           |

`id` terá o mesmo valor de `version`, sem sequência separada nem dependência de `LastInsertId`. Uma versão removida e reaplicada receberá o mesmo id e nova data.

A tabela usará a origem padrão da conexão, com `main` no SQLite. Não será imposto o schema `public` a todos os bancos. O horário de conclusão do passo será gerado pela aplicação em UTC: instante com fuso no PostgreSQL, representação UTC no MySQL e texto RFC3339Nano no SQLite. Os drivers de integração serão configurados para esses formatos.

As três colunas não detectarão alterações de conteúdo em scripts já aplicados nem toda falha parcial não registrada. Essas limitações integram o escopo aprovado na seção **Garantias e limites das migrações**.

### Aplicação e recuperação

O cadastro, o histórico e o plano completo serão validados antes do primeiro script. Todos os arquivos necessários à direção escolhida serão lidos previamente; arquivo UP/DOWN necessário ausente bloqueará o plano. Os conteúdos serão mantidos durante a chamada, sem releitura entre validação e execução.

UPs serão executados em ordem crescente; DOWNs, em ordem decrescente. O DOWN de uma versão desfará seu respectivo UP, retornando ao estado da versão cadastrada anterior, não ao número `version-1`. Com cadastro `[2, 6, 8]`, descer de 8 para 2 executará DOWN 8 e DOWN 6, preservando a versão 2.

Após UP bem-sucedido, o registro será inserido; após DOWN bem-sucedido, será removido. Alvo já aplicado com histórico válido retornará sucesso sem scripts. Downgrade seguirá o alvo informado, sem segunda confirmação durante a execução da biblioteca.

Em `Migrate`, a atomicidade será por migration declarada transacional, incluindo script e atualização do histórico na mesma transação interna. O Krios confirmará em sucesso e tentará rollback em falha. Migrations anteriores confirmadas permanecerão aplicadas; não haverá promessa de atomicidade do plano inteiro.

`Migration.NonTransactional bool`, com padrão `false`, declarará a necessidade de execução fora de transação. Para passos com `true`, o Krios reservará uma conexão do pool durante script e registro. Não analisará SQL para adivinhar sua capacidade transacional. Os scripts não conterão seu próprio BEGIN, COMMIT ou ROLLBACK.

`MigrateTx` verificará previamente se todos os passos necessários são declarados transacionais e rejeitará os demais antes de qualquer script. Histórico e scripts usarão somente a transação recebida; o Krios não criará transação interna nem executará commit/rollback. Caso a criação da tabela de controle exija DDL incompatível com a transação, será necessária preparação pela execução sem transação externa, sem fallback para o pool.

DDL com commit implícito do MySQL será declarado `NonTransactional`. Uma declaração incorreta poderá encerrar a transação; receber `*sql.Tx` não assegura rollback de SQL incompatível. [Commits implícitos do MySQL](https://dev.mysql.com/doc/refman/8.0/en/implicit-commit.html).

Scripts seguirão a política da seção **Execução de arquivos SQL**: envio integral ao driver configurado, com suporte a múltiplos comandos comprovado na integração. O timeout abrangerá a chamada completa. A execução parará no primeiro erro ou cancelamento, sem repetição automática.

Erros identificarão versão, direção, caminho e etapa: leitura, SQL, histórico, commit ou rollback. As causas serão preservadas. Se rollback também falhar, ambos os erros serão preservados, por exemplo com `errors.Join`. Erro de commit poderá deixar resultado incerto.

Falhas não transacionais ou resultados incertos exigirão verificação e reparo manual pela aplicação. Não haverá compensação automática nem DOWN baseado na suposição de que todo UP terminou. A ausência do registro não provará que o script não executou. Uma nova chamada não conseguirá detectar todos esses casos; a aplicação bloqueará novas execuções até verificar e reparar banco e histórico.

Haverá um único executor de migrations por banco, coordenado pela aplicação ou pelo deploy. A exclusão abrangerá leitura do histórico, scripts e registro; em `MigrateTx`, permanecerá até o commit/rollback externo. O Krios não fornecerá bloqueio distribuído automático. Uma chave única ou mutex local não será tratado como garantia contra execução duplicada entre processos.

### Garantias e limites das migrações

Foi aprovado o escopo simples das seções **Versões e histórico** e **Aplicação e recuperação**: histórico com três colunas, exclusão coordenada pela aplicação e recuperação manual de falhas parciais ou incertas.

Não fazem parte desse escopo:

- Checksums para detectar alterações em scripts já aplicados.
- Tabela adicional para registrar operação em andamento ou falha persistente.
- Bloqueio distribuído automático e gestão de locks por banco.
- Operação automática de reparo ou regularização do histórico.

Essas possibilidades foram avaliadas como ampliação futura e não são requisitos da implementação inicial. Sua eventual inclusão exigirá novo planejamento e aprovação.

A documentação deverá declarar as limitações aprovadas. O Krios verificará a estrutura e a sequência do histórico, sem prometer detecção de toda divergência semântica do schema ou execução concorrente segura por conta própria.

A regra de uso exclusivo da transação recebida continuará válida: `MigrateTx` não gravará marcadores ou diagnósticos em outra conexão para escapar do rollback.

### Exemplos de migração

Neste exemplo, `db` é um pool SQLite fornecido pela aplicação. Os arquivos devem conter SQL transacional compatível com o banco:

```go
ctx := context.Background()
client, err := krios.New(krios.Config{
    Db:        db,
    DbVersion: 2,
    Dialect:   krios.DialectSQLite3,
    Timeout:   30 * time.Second,
    Context:   ctx,
    Migrations: []krios.Migration{
        {
            Version: 1,
            Up:      "migrations/001_up.sql",
            Down:    "migrations/001_down.sql",
        },
        {
            Version: 2,
            Up:      "migrations/002_up.sql",
            Down:    "migrations/002_down.sql",
        },
    },
})
if err != nil {
    return err
}
if err := client.Migrate(); err != nil {
    return err
}
```

A execução normal acima pode ser substituída pela execução em transação externa, quando todos os passos e a preparação do histórico forem compatíveis:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

if err := client.MigrateTx(tx); err != nil {
    return err
}
if err := tx.Commit(); err != nil {
    return err
}
```

A aplicação mantém a exclusão entre executores até encerrar a transação. Para passos com DDL de commit implícito no MySQL, declarar `NonTransactional: true` no cadastro e usar `Migrate`, conforme as regras anteriores.
