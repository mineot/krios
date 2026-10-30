# Definição

Vamos criar uma biblioteca golang para manipulação de banco de dados SQL.

## Requisitos

- Usar a biblioteca nativa do go para manipular o banco de dados
- Teremos suporte para 3 bancos, MySql, Postgres e Sqlite (inicialmente)
- A biblioteca de manipulação do banco fica por responsabilidade do utilizador
- Deve ter gerencimando de controle de contexto, que deve ser iniciada, no que cada comando sql deve ser executado pelo contexto
- Precisamos ter manipuladores de Select, Insert, Delete e Update
- Tbem será permitido executar queries diretamente por arquivo, via "embed" do go
- Teremos gerenciadores de migração, permitindo assim controlar as versões do banco
- Precisamos gerencias o dialeto do banco para executar as queries da forma correta, isso nas funções de Select, Insert, Delete e Update

## Configurações

```go
krios.Config({
    Type:       "pg ou mysql ou sqlite",
    Path:       "...",
    "Conexão":  "..."
    Timeout:    5 * time.Second
})
```

O Type vai ser usado exclusivamente para definir o dialeto que iremos usar. Para o Path informaremos o cominho do banco de dados caso escolhemos o sqlite3, já para postgres ou mysql, vamos precisar definir as demais variaveis que são necessárias para efetuar essa conexão. O campo Timeout vai ser usado pra aguardar a conexão com o banco, se esse campo for negativo, deve ser retornado um erro.

```go
krios.MigrationConfig({
    CurrentVersion: 1,
    Files: []krios.MigrationFile{
        {
            Version: 1,
            Up: "embed sql file",
            Down: "embed sql file",
        },
        ...
    }
})
```

Sobre as migrações, vamos registrar no banco de dados uma tabela chamada `migrations` que vai registrar as versões executadas. Se a versão no arquivo for superior ao da configuração deve-se fazer o downgrade e se for inferior ou inexistete deve-se fazer o up. A migração fará tudo, abrir o banco, executar as instruções e fechar o banco. Se existir algum erro, um rollback deve ser executado, na seção Contexto que será explicado depois. Para executar o utilizador deve chamar a instrução `krios.Migrate()`.

Se for possivel passar o nome do arquivo e o `Krios` carregar sozinho o arquivo via embed.

## Abrir o banco

É assim que imagino que deve funcionar o abrir e fechar o banco de dados

```go
krios.Open()

if err != nil {
    return err
}

defer krios.Close()
```

## Contexto

É assim que imagino o gerencimando de contexto. Quando iniciado, tudo que foi executado no banco deve ser executado dentro do contexto.

```go
krios.Begin(context.Context)

r1, e1 := exec_sql

if e1 != nil {
    krios.Rollback()
    return e1
}

r2, e2 := exec_sql

if e2 != nil {
    krios.Rollback()
    return e1
}

...

krios.Commit()
```

## Clausula Where

```go
where := krios.Where(
    krios.Eq("id", 1),
    krios.Eq("name", "John"),
    krios.Or(
        krios.Eq("id", 1),
        krios.Eq("name", "John"),
    ),
)
```

## Operadores

Precisamos criar um metodo para cada dos operadores, respeitando o tipo, exemplo:

```go
krios.Eq("id", 1),
krios.Ne("id", 1),
krios.Lt("id", 1),
krios.Lte("id", 1),
krios.Gt("id", 1),
krios.Gte("id", 1),
krios.In("id", 1, 2, 3),
krios.NotIn("id", 1, 2, 3),
krios.Like("id", "%1%"),
krios.NotLike("id", "%1%"),
krios.IsNull("id"),
krios.IsNotNull("id"),
krios.Exists("id"),
krios.NotExists("id"),
krios.Between("id", 1, 2),
krios.NotBetween("id", 1, 2),
```

- Os itens que precisam ter valores, devem aceitar qualquer tipo de dado, o `Eq` deve aceitar numeros textos e por ai vai.
- Se for possivel, eu quero poder passar o tipo time.Time para gerenciar datas.

## Operadores logicos

Vamos ter um metodo para cada tipo, exemplo:

```go
krios.And(...)
krios.Or(...)
```

O And será o padrão, se não for informado usando nenhum. Porem se informado, tudo dentro dele deve seguir o operador, poremos ter isso:

```go
krios.And(
    krios.Eq("id", 1),
    krios.Eq("name", "John"),
    krios.Or(
        krios.Eq("id", 1),
        krios.Eq("name", "John"),
        krios.And(
            krios.Eq("id", 1),
            krios.Eq("name", "John"),
        ),
    ),
)
```

### Agregação

- `krios.Column()`
- `krios.Avg()`
- `krios.Sum()`
- `krios.Min()`
- `krios.Max()`

## Select

```go
// o alias não é obrigatório, mas pode ser usado nos agregadores
// o schema tbem não é obrigatorio
query := krios.Select("table", "alias", "schema")

query = query.Distinct()

query = query.All() // Lista todas as colunas `*`

query = query.Columns(
    krios.Column("col1"),
    krios.Max("col2"),
    ...
)

query = query.OrderBy(
    krios.Asc("col1"),
    krios.Desc("col2"),
)

query = query.GroupBy("col1", "col2")

query = query.Where(
    krios.Eq("id", 1),
    krios.Eq("name", "John"),
    krios.Or(
        krios.Eq("id", 1),
        krios.Eq("name", "John"),
    )
    // seguir as regras da seção acima Clausula Where
)

// não é obrigatória
// caso não informado trazer tudo
query = query.Paginate(1, 10) // offset e limit consecutivamente

// o alias tbem não é obrigatório
// o On seguirá a mesma regra do Where
query = query.Join("table2", "alias", "schema", krios.On(...))
query = query.InnerJoin("table2", "alias", "schema", krios.On(...))
query = query.LeftJoin("table2", "alias", "schema", krios.On(...))
query = query.RigthJoin("table2", "alias", "schema", krios.On(...))
query = query.OuterJoin("table2", "alias", "schema", krios.On(...))

result, err = query.Count() // select count(*) from table ...
result, err = query.First() // listar o primeiro registro apenas
result, err = query.Last() // listar o ultimo registro apenas
result, page, err = query.Exec() // executar a consulta completa
result, nil, err = query.Exec() // se não informado o paginate, page deve retornar nil e trazer todos os dados
```

### Resultado

O resultado deve retornar usando o padrão da biblioteca nativa do go, sendo permitido usar o `Scan`

## Insert

```go
// o schema é similar ao do select
insert := krios.Insert("table", "schema")

insert.Columns("col1", "col2", ...)
insert.Values("val1", 1, ...)

result, err = insert.Exec()
```

### Resultado

No resultado deve-se retornar o ID do ultimo do registro inserido

## Update

```go
// o schema é similar ao do select
update := krios.Update("table", "schema")

update.Set("col1", "val1")
update.Set("col2", 1)
// o where é obrigatorio, se não informado deve retornar um erro
update.Where(....)

result, err = update.Exec()
```

### Resultado

Similar ao insert, deve-se retornar o ID do ultimo do registro alterado

## Delete

```go
// o schema é similar ao do select
delete := krios.Delete("table", "schema")

// o where é obrigatorio, se não informado deve retornar um erro
delete.Where(....)

err = delete.Exec()
```

### Resultado

Não precisa de retorno

## Execução de queries embed

### Select

```go
// se for possivel, deve-se informar apenas o arquivo, e o krios deve buscar via embed
query := krios.Query("informar o sql")

result, page, err = query.Exec()
result, nil, err = query.Exec()
```

### Resultado

- O resultado deve ser possivel usar o `Scan`
- se possivel identificar que existe paginação, vamos retornar o page normal

### Insert, Delete e Update

```go
// se for possivel, deve-se informar apenas o arquivo, e o krios deve buscar via embed
query := krios.Query("informar o sql")

result, err = query.Exec()
```

### Resultado

- Se possivel, precisamos identificar o id, para insert, update
- no delete podemos retornar nil
