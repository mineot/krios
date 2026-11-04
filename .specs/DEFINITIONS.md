# Definições

## Configurações

- Devemos validar se a conexão do banco esta valida e aberta, e usar o timeout;
- A versão do banco deve ser maior ou igual a 1, porem não tem limites;
- O dialeto é obrigatório;
- o Contexto é obrigatório.
- Não vamos contemplar tudo que existe nas instruções sql, começaremos com o básico, e conforme eu for vendo necessidade, eu irei adicionar no krios.
- Todos os resultados devem ser o padrão do `database/sql`;
- Se ocorrer um erro a nivel de banco, vamos retornar para o utilizador;
- Temos que considerar que o Utilizador, sabe fazer um SQL, portanto ele tem responsabilidade para montar algo funcional;

```go
type Config struct {
    Db         *sql.DB
    DbVersion  uint
    Dialect    Dialect
    timeout    time.Duration
    Context    context.Context
    Migrations []Migration
}

krios.DialectSQLite3    = "sqlite3"
krios.DialectPostgreSQL = "postgres"
krios.DialectMySQL      = "mysql"
```

```go
// Example
client := krios.New(Config{
    Db:         db,
    DbVersion:  1,
    Dialect:    krios.DialectSQLite3, // example
    timeout:    5 * time.Second,
    Context:    context.Background(),
    Migrations: nil,
})
```

## Critérios

- Deve aceitar todos os tipos que são aceitos pelo `database/sql`;
- Tipos disponíveis: `int`, `int8`, `int16`, `int32`, `int64`, `string`, `[]byte`, `time.Time`, `bool` (Pergunta para IA: existe outros?)
- O Dialeto deve ser verificado sempre, por isso deve-se usar o `client`;
- A ideia é que quando o query builder construir uma query e for passado algum critério, deve-se criar a query com placeholder numerados e substituido no momento da execução;

```go
client.Eq("id", 1) // id = $1 e no momento da execução deve-se passar o valor 1
client.Eq("name", "John Doe") // name = $1 e no momento da execução deve-se passar o valor "John Doe"
client.Eq("date", time.Now()) // date = $1 e no momento da execução deve-se passar o valor time.Now()
client.Like("name", "%Joe Due%") // name LIKE $1 e no momento da execução deve-se passar o valor %Joe Due%
client.In("id", []int{1, 2, 3}) // id IN ($1) e no momento da execução deve-se passar o slice, isso deve funcionar com todos os tipos
client.Between("id", 1, 2) // id BETWEEN $1 AND $2 e no momento da execução deve-se passar os valores 1 e 2, deve funcionar com todos os valores
```

```go
client.Eq("id", 1) //int, int8, int16, int32, int64
client.Eq("name", "John Due") //string, []byte
client.Eq("date", time.Now()) // time.Time
client.Eq("flag", true) // bool
```

Lista de Predicados existentes:

```go
client.Eq(...) // =
client.Ne(...) // !=
client.Lt(...) // <
client.Lte(...) // <=
client.Gt(...) // >
client.Gte(...) // >=
client.Like(...) // LIKE
client.NotLike(...) // NOT LIKE
client.IsNull(...) // IS NULL
client.IsNotNull(...) // IS NOT NULL
client.In(...) // IN
client.NotIn(...) // NOT IN
client.Between(...) // BETWEEN
client.NotBetween(...) // NOT BETWEEN
```

Operadores Lógicos:

```go
client.And(krios.Eq("id", 1), krios.Eq("name", "John Doe")) // id = $1 AND name = $2
client.Or(krios.Eq("id", 1), krios.Eq("name", "John Doe")) // id = $1 OR name = $2
client.Or(client.And(...), ...) // (expr and expr) or expr
client.And(client.Or(...), ...) // (expr or expr) and expr
```

## Select

- Lembrando, a descrição abaixo é apenas um exemplo, deve-se sempre respeitar o dialeto.

```go
selectQuery := client.Select()
selectQuery.From("users").As("u").Schema("public")
selectQuery.Distinct()
// SQL: SELECT DISTINCT * FROM public.users AS u

selectQuery := client.Select().From("users").Distinct()
// SQL: SELECT DISTINCT * FROM users

selectQuery := client.Select().From("users")
// SQL: SELECT * FROM users
```

```go
selectQuery := client.Select().From("users").As("u").Columns("id", "name")
// SQL: SELECT u.id, u.name FROM users

selectQuery := client.Select().From("users").As("u").Column("id").As("user_id")
// SQL: SELECT u.id As user_id FROM users AS u

selectQuery := client.Select().From("users").As("u").Count("*").As("total")
// SQL: SELECT COUNT(u.*) As total FROM users AS u

selectQuery := client.Select().From("users").Count("*")
// SQL: SELECT COUNT(*) FROM users

// Agregadores disponiveis: COUNT, MAX, MIN, AVG, SUM

selectQuery := client.Select().From("users").OrderBy("id").OrderBy("name").Asc().OrderBy("date").Desc()
// SQL: SELECT * FROM users ORDER BY id, name ASC, date DESC

selectQuery := client.Select().From("users").GroupBy("id", "name")
// SQL: SELECT * FROM users GROUP BY id, name
```

```go
subSelect := client.Select().From("others")
selectQuery := client.Select().From("users").As("u").SubSelect(subSelect).As("o")
// SQL: SELECT * FROM users AS u, (SELECT * FROM others) AS o

subSelect := client.Select().From("others").Count("*")
selectQuery := client.Select().From("users").As("u").ColumnSelect(subSelect).As("total")
// SQL: SELECT u.*, (SELECT COUNT(*) FROM others) AS total FROM users AS u

// a clausula on vai se comportar exatamente como o where, respeitando o dialeto
// teremos, Join/InnerJoin (mesma coisa), LeftJoin, RightJoin, pergunta para IA existe outros?
joinSelect := client.Join("others").As("o").On(...)
selectQuery := client.Select().From("users").As("u").Join(joinSelect)
// SQL: SELECT * FROM users AS u INNER JOIN "others" AS o ON ...
```

```go
// Teremos tbem o UnionAll()
query1 := client.Select().From("users")
query2 := client.Select().From("users")
selectQuery := client.Union(query2)
// SQL: SELECT * FROM users UNION SELECT * FROM users
```

```go
// O union e union all tbem podem ser passados no wrapper
mainQuery := client.Select().From("users")
wrapperQuery := client.Wrapper(mainQuery).As("w").Columns("id")
// SQL: SELECT w.id FROM (SELECT * FROM users) AS w
```

```go
// o On do join deve funcionar igual
// pergunta para IA: o Having funciona igual ao Where?
selectQuery := client.Select().From("users")
selectQuery.Where(client.Eq("id", 1), client.Eq("name", "John Doe"))
// SQL: SELECT u.id, u.name FROM users AS u WHERE u.id = $1 AND u.name = $2

selectQuery := client.Select().From("users")
selectQuery.Where(client.Or(client.Eq("id", 1), client.Eq("name", "John Doe")))
// SQL: SELECT u.id, u.name FROM users AS u WHERE u.id = $1 OR u.name = $2

// No momento da execução deve-se substituir o placeholder pelo valor real
```

```go
// valores respectivamente: offset e limit
// vamos calcular do 1 para o total, exemplo: primeira pagina 1 = offset 0 ultima pagina = 25 offset = 24
selectQuery := client.Select().From("users").Paginate(1, 10)
// SQL: SELECT * FROM users LIMIT 10 OFFSET 0

// Precisamos armazenar isso em memoria
// se a query for modificada, deve-se "zerar" a paginação
selectQuery.Next() // proxima pagina, se for a ultima não deve fazer nada
selectQuery.Prev() // anterior pagina, se for a primeira nao deve fazer nada
```

```go
// Retorno sem paginação
rows, _, err := selectQuery.Exec()
rows, _, err := selectQuery.ExecTx(tx) // sql.Tx

// Retorno com paginação
rows, page, err := selectQuery.Exec()
rows, page, err := selectQuery.ExecTx(tx) // sql.Tx

type Page struct {
    TotalRows       uint64
    TotalPages      uint64
    RowsPerPage     uint64
    CurrentPage     uint64
    LastPage        uint64
    HasNextPage     bool
    HasPreviousPage bool
}
```

## Insert

```go
// o valor deve-se acumular as linhas
insertQuery := client.Insert().Into("users").Columns("name", "email")
insertQuery.Values("name1", "email1@example.com")
insertQuery.Values("name2", "email2@example.com")
// SQL: INSERT INTO users(name, email) VALUES ($1, $2), ($3, $4)

result, err := insertQuery.Exec()
result, err := insertQuery.ExecTx(tx) // sql.Tx
```

## Update

```go
updateQuery := client.Update().From("users").Set("name", "name1").Set("email", "email1@example.com")
updateQuery.Where(client.Eq("id", 1)) // o where é obrigatório
// SQL: UPDATE users SET name = $1, email = $2 WHERE users.id = $3

result, err := updateQuery.Exec()
result, err := updateQuery.ExecTx(tx) // sql.Tx
```

## Delete

```go
deleteQuery := client.Delete().From("users")
deleteQuery.Where(client.Eq("id", 1)) // o where é obrigatório
// SQL: DELETE FROM users WHERE users.id = $1

result, err := deleteQuery.Exec()
result, err := deleteQuery.ExecTx(tx) // sql.Tx
```

## Execução de Arquivos `.sql`

```go
// o utilizador deve garantir o dialeto do arquivo, não vamos validar isso
// pode ser de qq tipo, select, insert, update, delete, procedures, funções e etc...
fileQuery := client.File("path/to/file.sql")
result, err := fileQuery.Exec()
result, err := fileQuery.ExecTx(tx) // sql.Tx
```

## Migração

- O Krios, vai considerar que a migração será executada no banco do dialeto;
- Só deve aceitar o path para um arquivo `.sql`;
- O Krios deve gerenciar uma tabela chamada `migrations`, que vai conter, version, execute date;
- Deve-se executar o up ou down de acordo a versão registrada em migrations, claro, sempre atualizado a tabela;

```go
type Migration struct {
    Version uint
    Up      string
    Down    string
}

client := krios.New(Config{
    Db:         db,
    DbVersion:  1,
    Dialect:    krios.DialectSQLite3, // example
    timeout:    5 * time.Second,
    Context:    context.Background(),
    Migrations: []krios.Migration{
        krios.Migration{ Version: 1, Up: "path/to/file_up.sql", Down: "path/to/file_down.sql" },
        krios.Migration{ Version: 2, Up: "path/to/file_up.sql", Down: "path/to/file_down.sql" },
    },
})
```

- O Krios assume nenhuma responsabilidade sobre a migração, se der erro iremos retornar para o utilizador;
- Portanto o utilizador deve ser encorajado a fazer backup do banco antes de executar a migração;

```go
client.Migrate()
client.MigrateTx(tx) // sql.Tx
```
