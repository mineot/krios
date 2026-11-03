# Definições do Projeto Krios

- É uma biblioteca para criação de queries, e gerenciamento de banco de dados, denominado migrations.
- Essa biblioteca deve ser distribuida pelo github, a sua definição já esta no modulo.
- Deve atender, nesse momento, os bancos SQLite3, PostgreSQL e MySQL. Porem deve estar aberta para implementar mais bancos no futuro.
- A conexão com o banco de dados, bem como a definição dos drivers e contexto, devem ser definidar pelo utilizador da biblioteca, não fica a cargo do Krios cuidar disso. Mas o Krios deve receber uma conexão valida e executar a partir dela.

## Tarefas

As tarefas devem ter o minimo de atividades possivel, prefiro ter muitas tarefas fazendo pouca coisa, ou seja, cada tarefa deve conter um escopo bem definido.

Seguiremos essa ordem:

- Tarefas de desenvolvimento: focar apenas no desenvolvimento do código
- Tarefas de testes: focar apenas em testes unitários, precisamos criar testes para cada banco, usando dockers, e se possivel usar o github actions
- Tarefas de documentação: Escrever o Readme.md, criar um MANUAL.md, e melhorar a documentação de codigo. Por fim, traduzir as specs e agents.md para en-us.
- Tarefas de Publicação: Publicar a biblioteca no github e disponibilizar para ser usada em outros projetos golang.

## Escopo

### Receber a conexão com o banco de dados

Aqui precisamos verificar se a conexão esta aberta e valida, se não retornar um erro.

```go
err := krios.SetConnection(conn)

if err != nil {
    return err
}
```

### Identificação do Dialeto

```go
krios.SetDialect(krios.DialectSQLite3)
krios.SetDialect(krios.DialectPostgreSQL)
krios.SetDialect(krios.DialectMySQL)
```

### Query Builder

- O método `As` é usado para definir os alias. É opcional.
- O método `Schema` é usado para definir o schema, seu uso deve ser de acordo com o banco de dados escolhido. É opcional.
- A criação da query, de qualquer tipo, deve sempre respeitar o dialeto do banco de dados escolhido.

#### Select

```go
selectQuery := krios.Select()
selectFrom := selectQuery.From("users").As("u").Schema("public")

// executa: "SELECT * FROM public.users AS u"
result, nil, err := selectQuery.Exec()

// ---

selectQuery.Distinct()

// executa: "SELECT DISTINCT * FROM public.users AS u"
result, nil, err := selectQuery.Exec()
```

#### Colunas

```go
selectFrom.Column("id").As("user_id")
selectFrom.Column("name").As("user_name")
selectFrom.Column("age").As("user_age")

// executa:
// "SELECT u.id AS user_id, u.name AS user_name, u.age AS user_age FROM public.users AS u"
result, nil, err := selectQuery.Exec()
```

#### Agregadores

```go
// no caso do count pode ser *
selectFrom.Count("col1").As("count")
selectFrom.Max("col2").As("max")
selectFrom.Min("col3").As("min")
selectFrom.Avg("col4").As("agv")
selectFrom.Sum("col5").As("sum")

// executa:
// "SELECT COUNT(u.col1) AS count, MAX(u.col2) AS max, MIN(u.col3) AS min, AVG(u.col4) AS agv, SUM(u.col5) AS sum FROM public.users AS u"
result, nil, err := selectQuery.Exec()
```

#### GroupBy

```go
selectFrom.GroupBy("col1")
selectFrom.GroupBy("col2")

// executa:
// "SELECT * FROM public.users AS u GROUP BY u.col1, u.col2"
result, nil, err := selectQuery.Exec()
```

#### OrderBy

```go
selectFrom.OrderBy("col1")
selectFrom.OrderBy("col2").Desc()

// executa:
// "SELECT * FROM public.users AS u ORDER BY u.col1, u.col2 DESC"
result, nil, err := selectQuery.Exec()
```

#### Critérias

Deve receber os tipos string e numericos e outros, mas tbem o time.Time

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(krios.Eq("id", 1))
criteriaFrom.Append(krios.Ne("id", 1))
criteriaFrom.Append(krios.Lt("id", 1))
criteriaFrom.Append(krios.Lte("id", 1))
criteriaFrom.Append(krios.Gt("id", 1))
criteriaFrom.Append(krios.Gte("id", 1))
criteriaFrom.Append(krios.Like("id", "%1%"))
criteriaFrom.Append(krios.NotLike("id", "%1%"))
criteriaFrom.Append(krios.In("id", []int{1, 2, 3}))
criteriaFrom.Append(krios.NotIn("id", []int{1, 2, 3}))
criteriaFrom.Append(krios.IsNull("id"))
criteriaFrom.Append(krios.IsNotNull("id"))
criteriaFrom.Append(krios.Between("id", 1, 2))
criteriaFrom.Append(krios.NotBetween("id", 1, 2))
criteriaFrom.Append(krios.Exists("id"))
criteriaFrom.Append(krios.NotExists("id"))

selectQuery.Criteria(criteriaFrom)

// executa:
// "SELECT * FROM public.users AS u WHERE u.id = 1 ..."
// O padrão do operador lógico é o And
result, nil, err := selectQuery.Exec()
```

#### Operadores Lógicos

```go
criteriaFrom.Append(
    krios.And(
        krios.Eq("id", 1),
        ...
    ),
)

selectQuery.Criteria(criteriaFrom)

// executa:
// "SELECT * FROM public.users AS u WHERE u.id = 1 AND ..."
result, nil, err := selectQuery.Exec()

criteriaFrom.Append(
    krios.Or(
        krios.Eq("id", 1),
        ...
    ),
)

selectQuery.Criteria(criteriaFrom)

// executa:
// "SELECT * FROM public.users AS u WHERE u.id = 1 OR ..."
result, nil, err := selectQuery.Exec()

// podemos ter ainda isso, e vise-versa
criteriaFrom.Append(
    krios.Or(
        krios.Eq("id", 1),
        krios.And(
            krios.Eq("id", 1),
            ...
        ),
    ),
)

selectQuery.Criteria(criteriaFrom)

// executa:
// "SELECT * FROM public.users AS u WHERE u.id = 1 OR u.id = 1 AND ..."
result, nil, err := selectQuery.Exec()
```

#### Joins

```go
// Existirá um InnerJoin, LeftJoin, RightJoin, FullJoin, CrossJoin, e CrossApply e todos os possivel dependendo do dialeto
// Precisamos ver se nos ons é possivel usar o mesmo do criteria, inclusive os operadores lógicos
selectJoin1 := selectQuery.Join("seniors").As("s").On(
    []krios.Ons{
        krios.On().Equal("user_id", selectFrom, "id"),
        krios.On().Equal("user_id", nil, 1),
    },
)

// desse ponto em diante, é similar ao from, tendo todos os recursos dele
// aqui a entrada, qualquer uma delas, columns, group by, order by, etc, devem respeitar a ordem de registro

// executa:
// "SELECT * FROM public.users AS u INNER JOIN public.seniors AS s ON u.id = s.user_id AND u.id = 1"
result, nil, err := selectQuery.Exec()
```

#### Paginação

```go
// Primeiro numero é a pagina, o segundo o total por pagina
selectQuery.Paginate(1, 10)

// teremos a seguinte struct que deve ficar dentro do Krios

type Page struct {
    TotalRows uint64
    TotalPages uint64
    RowsPerPage uint64
    CurrentPage uint64
    LastPage uint64
    HasNextPage bool
    HasPreviousPage bool
}

// executa:
// "SELECT * FROM public.users AS u LIMIT 10 OFFSET 10"
// Deve retornar o struct Page preenchido
result, Page, err := selectQuery.Exec()

// Teremos dois auxiliares, para reaproveitar a consulta, sem precisar modificar mais nada
selectQuery.NextPage() // se já for a ultima, então retorna a ultima
selectQuery.PreviousPage() // se já for a primeira, retorna a primeira
```

#### Insert

```go
insertQuery := krios.Insert()
insertQuery.Into("users").Columns("name", "email")
inertQuery.Values("name1", "email1")
inertQuery.Values("name2", "email2") // deve acumular as linhas

defer insertQuery.ClearValues() // se for possivel limpar os values para reaproveitar esse comando

// executa:
// "INSERT INTO public.users(name, email) VALUES ('name1', 'email1'), ('name2', 'email2')"
result, err := insertQuery.Exec()

// o resultado deve ser o padrão da lib database/sql do go
// o mesmo se aplicar para o select, update e delete
```

#### Update

```go
updateQuery := krios.Update()
updateQuery.Table("users")
updateQuery.Set("name", "name1")
updateQuery.Set("email", "name2")
updateQuery.Criteria(...) // pelo menos um critério é obrigatório

// executa:
// "UPDATE public.users SET name = 'name1', email = 'name2' WHERE ..."
result, err := updateQuery.Exec()
```

#### Delete

```go
deleteQuery := krios.Delete()
deleteQuery.Table("users")
deleteQuery.Criteria(...) // pelo menos um critério é obrigatório

// executa:
// "DELETE FROM public.users WHERE ..."
result, err := deleteQuery.Exec()
```

### Migrations

```go
migrations := krios.NewMigrations(dialect) // deve-se informar qual o dialeto se deve usar, permitindo criar scripts para todos os bancos de dados

migrations.Append(
    "create_users",
    up: `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
            name TEXT NOT NULL,
            email TEXT NOT NULL
        )
    `,
    down: `
        DROP TABLE users
    `,
)

// alternativa

migrations.Append(
    "create_users",
    EmdedFiles: <definir como passar uma lista de arquivos .sql>,
    up: "nome do arquivo .sql de up",
    down: up: "nome do arquivo .sql de down",
)

// Então chamar no inicio da aplicação

// 1 é a versão atual
// conn é a conexão com o banco
// migrations é a lista de migrations, pode ser passado várias aqui
err := krios.RunMigrations(1, conn, migrations, ...)

if err != nil {
    panic(err)
}

// o Krios deve gerenciar uma tabela chamada migrations, que deve ser atualizada
// deve-se verificar qual a ultima versão e fazer o up ou down respectivo
```
