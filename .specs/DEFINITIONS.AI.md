# Definições do Projeto Krios

> Versão revisada e anotada de [DEFINITIONS.md](DEFINITIONS.md). O original permanece inalterado. A estrutura, a ordem das tarefas e as funcionalidades foram preservadas, com correções e anotações baseadas em [REPORTS.md](REPORTS.md).
>
> **Correção:** ajuste de erro técnico, inconsistência ou exemplo. **Proposta IA:** solução sugerida para revisão, ainda não aprovada como contrato de implementação. **Decisão pendente:** escolha que precisa ser feita pelo desenvolvedor. As referências R01–R64 identificam os achados do relatório; não são números de tarefas.

- É uma biblioteca para criação de queries e gerenciamento de alterações versionadas no banco de dados, denominadas migrations.
- A biblioteca deve ser distribuída pelo GitHub. Seu módulo já está definido como `github.com/mineot/krios`.
- Deve atender, neste momento, aos bancos SQLite3, PostgreSQL e MySQL. Porém, deve permitir implementar suporte a outros bancos no futuro.
- A criação da conexão, a escolha e configuração dos drivers e o fornecimento de contexto são responsabilidades do utilizador. O Krios deve receber conexão/executor e dialeto em uma struct pública `krios.Config`, criar uma instância com `client := krios.New(config)` e propagar essas informações internamente às operações dessa instância.

> **Definição do desenvolvedor — R12 e R14:** conexão e dialeto pertencem a `krios.Config`; as consultas são criadas pela instância, como `client.Select()`. Quando receber `*sql.Tx`, o Krios deve executar nessa transação, sem substituí-la pelo pool ou por outra conexão.
>
> **Decisões pendentes — R09–R15, R23 e R60:** aprovar a declaração dos campos e da interface de execução proposta abaixo, as versões mínimas dos bancos e os drivers homologados. Esclarecer se a extensão de dialetos será feita apenas dentro do projeto ou também por terceiros. A API pública deve continuar acessível por `krios.go`, sem exigir que consumidores importem `internal`.

## Tarefas

As tarefas devem ter o mínimo de atividades possível. É preferível ter muitas tarefas pequenas, cada uma com escopo bem definido.

Seguiremos esta ordem:

- Tarefas de desenvolvimento: focar no desenvolvimento do código.
- Tarefas de testes: criar testes unitários de geração de SQL e testes de integração para cada banco. Usar Docker para os bancos que necessitam de servidor e, se possível, GitHub Actions. SQLite é embutido; Docker pode padronizar seu ambiente de testes, mas não é necessário um servidor SQLite.
- Tarefas de documentação: escrever o `README.md`, criar um `MANUAL.md` e melhorar a documentação do código. Por fim, traduzir as specs e `AGENTS.md` para en-US.
- Tarefas de publicação: publicar a biblioteca no GitHub e disponibilizá-la para uso em outros projetos Go.

> **Correção — R58:** testes que executam operações em bancos reais são testes de integração; não devem ser classificados apenas como unitários.
>
> **Decisões pendentes — R59–R62 e R64:** manter a ordem acima e definir se verificações de compilação e exemplos mínimos poderão ocorrer durante o desenvolvimento. Cada tarefa deverá ter critérios de aceitação, incluindo casos inválidos relevantes. Definir matriz de versões/drivers, isolamento dos testes, licença, versionamento, tags e critérios de publicação. O `go.mod` atual exige Go 1.26.0; confirmar se esse será o mínimo desejado. Definir a localização de manual, licença e CI de acordo com as restrições de `AGENTS.md`, sem presumir autorização para novos arquivos na raiz.

## Escopo

### Receber a conexão com o banco de dados

Conexão e dialeto devem ser fornecidos juntos em `krios.Config`, substituindo os setters globais. Cada instância deve usar a sua configuração em todas as consultas, comandos e operações auxiliares.

**Proposta IA — R09–R14:** representar o executor por uma interface comum aos métodos de `*sql.DB`, `*sql.Conn` e `*sql.Tx`. A declaração abaixo propõe os nomes dos campos e da interface; não é implementação:

```go
// Declarações propostas para a API pública do pacote krios.
// context e sql correspondem a context e database/sql.
type Executor interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type Config struct {
    Connection Executor
    Dialect    Dialect
}
```

Os três tipos atendem aos métodos dessa interface. Não é necessário copiar a conexão nem extrair a conexão interna de uma transação. `PingContext`, `BeginTx`, `Commit`, `Rollback` e `Close` não fazem parte do contrato comum de execução. [API database/sql](https://pkg.go.dev/database/sql).

```go
// db é um *sql.DB criado e configurado pela aplicação.
config := krios.Config{
    Connection: db,
    Dialect:    krios.DialectPostgreSQL,
}

// Forma de criação escolhida pelo desenvolvedor.
client := krios.New(config)
```

| Tipo fornecido | O que representa | Onde a operação é encaminhada |
| --- | --- | --- |
| `*sql.DB` | Pool de conexões. | O pool seleciona uma conexão disponível para executar. Chamadas distintas podem usar conexões diferentes. |
| `*sql.Conn` | Uma conexão reservada do pool, obtida com `db.Conn(ctx)`. | Os métodos usam essa conexão específica. Reservá-la não inicia uma transação. |
| `*sql.Tx` | Uma transação iniciada com `BeginTx`, vinculada a uma conexão. | Os métodos executam dentro dessa transação. |

O Krios gera o SQL e os argumentos e chama o executor configurado. `database/sql` coordena o acesso através do driver; o motor do banco executa o SQL. PostgreSQL/MySQL normalmente executam no servidor, enquanto SQLite executa seu motor no processo da aplicação. A instância Krios não precisa obter um `*sql.Conn` para poder executar por `*sql.DB`. [Gerenciamento de conexões](https://go.dev/doc/database/manage-connections).

Uso com uma transação criada pela aplicação:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

txConfig := krios.Config{
    Connection: tx,
    Dialect:    krios.DialectPostgreSQL,
}
txClient := krios.New(txConfig)

insertQuery := txClient.Insert()
insertQuery.Into("users").Columns("name", "email")
insertQuery.Values("name1", "email1@example.com")
if _, err := insertQuery.Exec(ctx); err != nil {
    return err
}

// A aplicação confirma a transação.
return tx.Commit()
```

**Regra de execução com `*sql.Tx`:** SELECT, escritas, contagens de paginação e operações de migrations executadas por essa instância devem usar a transação recebida. Não fazer fallback para `*sql.DB`, abrir outra transação ou mudar de executor para contornar uma falha. Uma transação encerrada não pode ser reutilizada; o erro deve ser propagado. [Transações em Go](https://go.dev/doc/database/execute-transactions).

> **Proposta IA — R11 e R14:** a aplicação permanece responsável por `Commit`/`Rollback` da transação e pelo fechamento dos recursos que forneceu. O Krios não deve finalizar a transação do consumidor, fechar seu pool nem devolver sua conexão reservada por iniciativa própria. A interface proposta permite executar sem conceder ao código comum esses controles de ciclo de vida.
>
> **Validação — R09–R10 e R20:** `*sql.DB` e `*sql.Conn` oferecem `PingContext`; `*sql.Tx` não. Não exigir esse método na interface comum nem consultar o pool para validar uma transação. A verificação de comunicação não garante disponibilidade futura. Configuração ausente, dialeto inválido e executor nulo, inclusive ponteiro nulo dentro de interface, precisam de erro definido. Não é possível garantir genericamente, sem operação, que qualquer executor continua utilizável.
>
> **Decisões pendentes — R08–R13 e R20:** com a forma escolhida `client := krios.New(config)`, definir onde serão retornados erros de configuração e quando ocorrerá validação com contexto. A proposta é validar antes da execução em `Exec(ctx)` ou oferecer validação explícita; nenhuma dessas opções foi escolhida aqui. O contexto permanece por operação na proposta atual. Definir se a configuração será copiada na construção e como evitar que alterações externas mudem executor/dialeto de consultas já criadas. Instâncias diferentes não devem depender de setters ou estado global de conexão/dialeto.

### Identificação do Dialeto

O dialeto é informado no campo `Dialect` de `krios.Config` e propagado pela instância. Os exemplos abaixo representam alternativas, cada uma com seu executor correspondente:

```go
sqliteConfig := krios.Config{
    Connection: sqliteDB,
    Dialect:    krios.DialectSQLite3,
}
sqliteClient := krios.New(sqliteConfig)
```

```go
postgresConfig := krios.Config{
    Connection: postgresDB,
    Dialect:    krios.DialectPostgreSQL,
}
postgresClient := krios.New(postgresConfig)
```

```go
mysqlConfig := krios.Config{
    Connection: mysqlDB,
    Dialect:    krios.DialectMySQL,
}
mysqlClient := krios.New(mysqlConfig)
```

> **Decisões pendentes — R08, R12, R20, R22–R23:** definir o comportamento de dialeto ausente, inválido ou incompatível com o executor, e onde o erro será retornado. Recursos não suportados não podem ser silenciosamente convertidos em operações de significado diferente. A proposta é retornar erro explícito; qualquer emulação depende de especificação própria. Suporte depende também da versão do banco, não apenas do nome do dialeto.

### Query Builder

- O método `As` define aliases e é opcional.
- O método `Schema` define o qualificador da tabela e é opcional. Deve respeitar o significado desse qualificador no banco escolhido.
- A criação de qualquer query deve respeitar o dialeto selecionado e suas capacidades.
- Valores devem ser enviados como argumentos separados do SQL, usando placeholders compatíveis com o banco e o driver. Os exemplos não autorizam concatenação de valores no texto executável.
- Tabelas, schemas, colunas e aliases precisam de tratamento de identificadores, separado da parametrização de valores. Um nome qualificado deve ter seus componentes tratados individualmente; `*` precisa de tratamento próprio.
- A ordem dos argumentos deve corresponder à ordem dos placeholders da consulta completa, incluindo joins, grupos lógicos, listas e múltiplas linhas de insert.
- A ordem de registro deve ser preservada dentro de cada cláusula, inclusive quando seus itens vêm de tabelas diferentes. A ordem das cláusulas SQL continua seguindo a gramática do banco.

> **Correções — R16–R18, R21 e R26:** PostgreSQL possui schemas; no MySQL, schema corresponde a database; no SQLite, os qualificadores incluem `main`, `temp` e nomes de bancos anexados. `public` não é um padrão universal nem deve aparecer sem configuração explícita ou regra de herança aprovada.
>
> **Decisões pendentes — R08, R13, R19–R20, R27 e R63:** definir assinaturas e tipos retornados pelo encadeamento; mutabilidade; repetição de métodos; cópia ou retenção de slices; uso concorrente; reutilização de critérios; aliases repetidos; self-joins; alteração de alias e referências entre consultas. Os exemplos do original pressupõem que os métodos modificam a consulta, pois vários retornos são ignorados. Essa pressuposição precisa ser formalizada antes da implementação.
>
> Avaliar uma operação para obter SQL, argumentos e erro sem executar. Definir momento de validação e erros reconhecíveis, preservando a causa do driver. Não fazer retry automático de escritas sem política de idempotência. Expressões SQL livres, CTEs, unions, funções de janela, upserts e outros recursos não exemplificados precisam de delimitação explícita de escopo; sua inclusão não é presumida nesta revisão.

**Convenções dos exemplos — R01–R08 e R10–R11:**

- Cada bloco de uso representa um exemplo independente, salvo quando o texto disser que há continuação. `client` é uma instância criada por `krios.New(config)`, configurada para PostgreSQL nos exemplos do query builder. A preparação comum abaixo deve ser repetida no início de cada exemplo de seleção que use `selectQuery` e `selectFrom`.
- Os exemplos são fragmentos de uma API em definição, não programas compiláveis contra a implementação atual. `ctx` é um `context.Context` fornecido pela aplicação. Imports, tratamento repetitivo de erros e consumo completo das linhas são omitidos nos fragmentos posteriores ao exemplo de `Select`.
- **Proposta IA:** usar `Exec(ctx)` para propagar contexto a toda operação, inclusive contagens auxiliares. Para SELECT, retornar `(*sql.Rows, *krios.Page, error)`; para insert, update e delete sem linhas retornadas, `(sql.Result, error)`.
- **Proposta IA:** sem paginação, retornar `page == nil`; em erro de execução da seleção ou de sua contagem, não devolver resultados de sucesso parcial, fechando recursos intermediários. Aprovar essas regras antes de implementar.
- Nos SELECTs sem uso dos metadados, `_` descarta o segundo retorno. Nunca usar `nil` como nome de variável para esse fim.
- Os SQLs do query builder usam PostgreSQL como referência, com `$1`, `$2` etc. Mostram a forma equivalente da consulta, sem fixar a política final de aspas dos identificadores. Nos demais dialetos, adaptar qualificadores e placeholders.

Preparação comum das seleções:

```go
selectQuery := client.Select()
selectFrom := selectQuery.From("users").As("u").Schema("public")
```

#### Select

```go
selectQuery := client.Select()
selectQuery.From("users").As("u").Schema("public")

// SQL: SELECT * FROM public.users AS u
rows, _, err := selectQuery.Exec(ctx)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    // Ler os campos com rows.Scan, usando destinos compatíveis
    // com as colunas da tabela, e tratar o erro de Scan.
}
if err := rows.Err(); err != nil {
    return err
}
```

Exemplo independente com `Distinct`:

```go
selectQuery := client.Select()
selectQuery.From("users").As("u").Schema("public")
selectQuery.Distinct()

// SQL: SELECT DISTINCT * FROM public.users AS u
rows, _, err := selectQuery.Exec(ctx)
// Tratar err e consumir/fechar rows como no exemplo anterior.
```

> **Correções — R01–R02, R06 e R11:** SELECT retorna linhas, não `sql.Result`. Em exemplos sequenciais no mesmo bloco, usar `=` quando não houver variável nova e encerrar o resultado anterior antes de substituir sua referência. O consumidor deve tratar tanto o erro inicial quanto erros de leitura, e fechar os resultados que receber.

#### Colunas

Aplicar sobre uma nova preparação comum:

```go
selectFrom.Column("id").As("user_id")
selectFrom.Column("name").As("user_name")
selectFrom.Column("age").As("user_age")

// SQL:
// SELECT u.id AS user_id, u.name AS user_name, u.age AS user_age
// FROM public.users AS u
rows, _, err := selectQuery.Exec(ctx)
```

> **Anotação — R08 e R27:** a primeira projeção explícita substitui o `*` implícito no exemplo. Definir formalmente a combinação com `Column("*")`, nomes já qualificados e aliases de saída; não confundir alias de coluna com uma coluna da tabela de origem.

#### Agregadores

Aplicar sobre uma nova preparação comum:

```go
// Count também deve aceitar "*", gerando COUNT(*).
selectFrom.Count("col1").As("count")
selectFrom.Max("col2").As("max")
selectFrom.Min("col3").As("min")
selectFrom.Avg("col4").As("avg")
selectFrom.Sum("col5").As("sum")

// SQL:
// SELECT COUNT(u.col1) AS count, MAX(u.col2) AS max,
// MIN(u.col3) AS min, AVG(u.col4) AS avg, SUM(u.col5) AS sum
// FROM public.users AS u
rows, _, err := selectQuery.Exec(ctx)
```

> **Correções — R05 e R25:** `COUNT(*)` conta linhas; `COUNT(coluna)` conta valores não nulos. Não qualificar automaticamente o asterisco de `Count("*")`. Outros agregadores podem retornar `NULL` quando não há valores. O consumidor precisa usar destinos de leitura compatíveis com os tipos e a nulabilidade dos resultados.
>
> **Decisão pendente — R25 e R63:** delimitar entradas válidas de cada agregador e esclarecer o suporte a `COUNT(DISTINCT ...)` e `HAVING`.

#### GroupBy

Aplicar sobre uma nova preparação comum:

```go
selectFrom.Column("col1")
selectFrom.Column("col2")
selectFrom.GroupBy("col1")
selectFrom.GroupBy("col2")

// SQL:
// SELECT u.col1, u.col2 FROM public.users AS u
// GROUP BY u.col1, u.col2
rows, _, err := selectQuery.Exec(ctx)
```

> **Correção — R24:** substituir o `SELECT *` do exemplo por colunas compatíveis com o agrupamento. Colunas não agrupadas exigem agregação ou uma dependência funcional reconhecida pelo banco; não se pode assumir que a consulta original seja válida em todos os dialetos. Definir o limite da validação do Krios, que não conhece necessariamente as restrições da tabela.

#### OrderBy

Aplicar sobre uma nova preparação comum:

```go
selectFrom.OrderBy("col1")
selectFrom.OrderBy("col2").Desc()

// SQL:
// SELECT * FROM public.users AS u ORDER BY u.col1, u.col2 DESC
rows, _, err := selectQuery.Exec(ctx)
```

> **Anotação — R08, R26–R27:** `Desc` afeta o item de ordenação ao qual está encadeado. Preservar a ordem de registro entre itens de diferentes origens. Definir como ordenar por aliases de projeção sem tratá-los indevidamente como nomes de colunas da tabela.

#### Critérios

Devem receber strings, tipos numéricos e `time.Time`, dentro dos limites suportados pelo banco e pelo driver. Outros tipos precisam de um contrato explícito; aceitar `any` não implica suporte a qualquer valor Go.

Aplicar sobre uma nova preparação comum:

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(krios.Eq("id", 1))
selectQuery.Criteria(criteriaFrom)

// SQL: SELECT * FROM public.users AS u WHERE u.id = $1
// Argumentos: [1]
// O operador lógico padrão entre critérios adicionados é AND.
rows, _, err := selectQuery.Exec(ctx)
```

Catálogo dos demais operadores, como alternativas independentes. Não adicionar todos à mesma consulta, pois algumas condições se contradizem:

```go
// Cada linha demonstra uma alternativa de Append:
// criteriaFrom.Append(krios.Ne("id", 1))
// criteriaFrom.Append(krios.Lt("id", 1))
// criteriaFrom.Append(krios.Lte("id", 1))
// criteriaFrom.Append(krios.Gt("id", 1))
// criteriaFrom.Append(krios.Gte("id", 1))
// criteriaFrom.Append(krios.Like("name", "%ana%"))
// criteriaFrom.Append(krios.NotLike("name", "%ana%"))
// criteriaFrom.Append(krios.In("id", []int{1, 2, 3}))
// criteriaFrom.Append(krios.NotIn("id", []int{1, 2, 3}))
// criteriaFrom.Append(krios.IsNull("email"))
// criteriaFrom.Append(krios.IsNotNull("email"))
// criteriaFrom.Append(krios.Between("id", 1, 2))
// criteriaFrom.Append(krios.NotBetween("id", 1, 2))
```

`Exists` e `NotExists` permanecem como funcionalidades previstas, mas precisam receber uma subconsulta, e não somente um nome de coluna:

```go
// Proposta de uso; o tipo de subconsulta e a correlação ainda precisam ser definidos:
// criteriaFrom.Append(krios.Exists(subquery))
// criteriaFrom.Append(krios.NotExists(subquery))
```

> **Correções — R05, R28 e R32:** `EXISTS` verifica se uma subconsulta retorna linhas; não verifica existência de coluna ou valor não nulo. Os exemplos de `LIKE` usam uma coluna textual. Sensibilidade a maiúsculas, acentos e escape dos curingas dependem do banco e da configuração.
>
> **Decisões pendentes — R15, R27–R28 e R30–R33:** definir suporte a booleanos, bytes, ponteiros, valores nulos e tipos com `driver.Valuer`; limites numéricos; precisão e fuso de datas; coleções aceitas por `IN`; expansão de parâmetros; listas vazias ou nulas; referências entre origens e subconsultas correlacionadas. `[]byte` não deve ser confundido automaticamente com uma lista de valores.
>
> `Eq("campo", nil)` precisa ser rejeitado ou normalizado conforme regra aprovada; `= NULL` não substitui `IS NULL`. Nulos em `NOT IN` têm semântica própria e não podem ser removidos silenciosamente. `IN ()` não é portável. A proposta conservadora é rejeitar listas vazias até haver uma política explícita, especialmente para não transformar `NOT IN` vazio em filtro universal de escrita.

#### Operadores Lógicos

Cada bloco abaixo usa uma nova preparação comum e um novo objeto de critérios.

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(
    krios.And(
        krios.Gte("age", 18),
        krios.IsNotNull("email"),
    ),
)
selectQuery.Criteria(criteriaFrom)

// SQL:
// SELECT * FROM public.users AS u
// WHERE (u.age >= $1 AND u.email IS NOT NULL)
// Argumentos: [18]
rows, _, err := selectQuery.Exec(ctx)
```

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(
    krios.Or(
        krios.Eq("id", 1),
        krios.Eq("id", 2),
    ),
)
selectQuery.Criteria(criteriaFrom)

// SQL: SELECT * FROM public.users AS u WHERE (u.id = $1 OR u.id = $2)
// Argumentos: [1, 2]
rows, _, err := selectQuery.Exec(ctx)
```

Também deve ser possível aninhar `And` em `Or`, e vice-versa:

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(
    krios.Or(
        krios.Eq("id", 1),
        krios.And(
            krios.Gte("age", 18),
            krios.IsNotNull("email"),
        ),
    ),
)
selectQuery.Criteria(criteriaFrom)

// SQL:
// SELECT * FROM public.users AS u
// WHERE (u.id = $1 OR (u.age >= $2 AND u.email IS NOT NULL))
// Argumentos: [1, 18]
rows, _, err := selectQuery.Exec(ctx)
```

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(
    krios.And(
        krios.Or(krios.Eq("id", 1), krios.Eq("id", 2)),
        krios.Gte("age", 18),
    ),
)
selectQuery.Criteria(criteriaFrom)

// SQL:
// SELECT * FROM public.users AS u
// WHERE ((u.id = $1 OR u.id = $2) AND u.age >= $3)
// Argumentos: [1, 2, 18]
rows, _, err := selectQuery.Exec(ctx)
```

> **Correção — R29:** a geração deve preservar a árvore lógica solicitada. Não depender apenas da precedência de `AND` sobre `OR`, pois o agrupamento inverso altera o resultado sem os parênteses necessários.
>
> **Decisões pendentes — R08, R29 e R33:** definir grupos vazios, critérios nulos, grupos com um elemento e chamadas repetidas de `Criteria`. Não descartar silenciosamente partes inválidas do filtro.

#### Joins

Estão previstos `InnerJoin`, `LeftJoin`, `RightJoin`, `FullJoin` e `CrossJoin`, conforme o suporte do dialeto e da versão do banco. A intenção original de oferecer `CrossApply` e outros joins precisa de delimitação:

- PostgreSQL 18 e MySQL 8.4 suportam `INNER`, `LEFT`, `RIGHT` e `CROSS JOIN`; MySQL 8.4 não oferece `FULL OUTER JOIN` nativo.
- SQLite suporta `RIGHT` e `FULL OUTER JOIN` a partir da versão 3.39.0.
- `CROSS APPLY` literal não é uma sintaxe nativa dos três bancos. Uma eventual abstração baseada em `LATERAL` depende de API e semântica próprias, onde houver suporte.

É possível compartilhar a representação dos critérios e operadores lógicos entre `ON` e `WHERE`, desde que a origem dos operandos e a cláusula de destino sejam explícitas. Mover um filtro entre essas cláusulas pode alterar o resultado de outer joins.

Aplicar sobre uma nova preparação comum:

```go
// Interpretação proposta para o exemplo original:
// Join representa INNER JOIN; a coluna da esquerda pertence ao join.
selectJoin1 := selectQuery.Join("seniors").As("s").Schema("public").On(
    []krios.Ons{
        krios.On().Equal("user_id", selectFrom, "id"),
        krios.On().Equal("user_id", nil, 1),
    },
)

// A partir de selectJoin1, devem estar disponíveis os recursos
// aplicáveis à origem: colunas, agregadores, GroupBy e OrderBy.

// SQL:
// SELECT * FROM public.users AS u
// INNER JOIN public.seniors AS s ON s.user_id = u.id AND s.user_id = $1
// Argumentos: [1]
rows, _, err := selectQuery.Exec(ctx)
```

> **Correções — R21–R22, R26 e R34–R35:** o schema do join foi declarado explicitamente, sem presumir herança. Sob a interpretação indicada, a segunda condição se refere a `s.user_id`, não a `u.id`. Preservar a ordem de registro de colunas, agrupamentos e ordenações entre todas as origens.
>
> **Decisões pendentes — R22, R27 e R34–R35:** aprovar o tipo padrão de `Join`, o contrato de `Ons`/`Equal`, o significado de `nil` como marcador de literal e a API compartilhada de expressões. Definir requisitos de condição por tipo de join; não aplicar indiscriminadamente as regras de `ON` a `CROSS JOIN`. Emulação de recursos ausentes e suporte a outros joins não estão aprovados por esta anotação.

#### Paginação

O primeiro argumento de `Paginate` representa a página, e o segundo, a quantidade de linhas por página.

**Decisão pendente — R36:** definir se a primeira página é zero ou um. **Proposta IA:** usar base um; o exemplo abaixo está condicionado à aprovação dessa convenção. Em base um, `offset = (pagina - 1) * tamanho`; em base zero, `offset = pagina * tamanho`.

Aplicar sobre uma nova preparação comum:

```go
// Neste exemplo, id é uma chave única, usada para ordenação determinística.
selectFrom.OrderBy("id")
selectQuery.Paginate(1, 10)

// SQL equivalente, se aprovada a base um:
// SELECT * FROM public.users AS u ORDER BY u.id LIMIT 10 OFFSET 0
// Na convenção de base zero, Paginate(1, 10) corresponderia a OFFSET 10.
rows, page, err := selectQuery.Exec(ctx)
```

A biblioteca deve disponibilizar publicamente a estrutura `Page`. Sua declaração pertence à API da biblioteca; o consumidor recebe uma variável chamada `page`.

```go
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

Devem existir dois auxiliares para reaproveitar a consulta:

```go
selectQuery.NextPage()     // Se já estiver na última, permanecer nela.
selectQuery.PreviousPage() // Se já estiver na primeira, permanecer nela.
```

> **Correções — R03, R07 e R36–R40:** retornar metadados preenchidos quando a paginação for executada com sucesso. A contagem precisa considerar os efeitos de joins, `DISTINCT`, agrupamentos e agregações. Não basta substituir qualquer projeção por `COUNT(*)`. Paginação sem ordenação determinística não garante páginas estáveis; mesmo com ordenação, mudanças concorrentes podem deslocar linhas entre páginas por offset.
>
> **Propostas IA — R38–R39:** interpretar `TotalRows` como quantidade de linhas do resultado antes de aplicar a paginação. Planejar contagem e leitura para não solicitar uma segunda conexão enquanto um resultado retém a única conexão disponível do pool, nem iniciar comandos incompatíveis com resultados ainda abertos na mesma conexão/transação. Usar o contexto da operação em ambas e documentar o custo da contagem. Se a instância recebeu `*sql.Tx`, contagem e leitura devem usar essa mesma transação, respeitando seu isolamento; não criar uma transação auxiliar nem retornar ao pool.
>
> **Decisões pendentes — R07 e R36–R40:** aprovar base, significado de `TotalRows` e `LastPage`, resultados vazios, tamanho zero, entradas negativas, página além do fim, overflow e conversão de `uint64`. Definir consistência entre contagem e leitura e eventual transação/isolamento. Definir o comportamento dos auxiliares antes da primeira execução e após mudança de filtros ou dados; esclarecer se apenas alteram estado ou executam consultas. O Krios não pode conhecer a última página atual sem informação suficiente.

#### Insert

```go
insertQuery := client.Insert()
insertQuery.Into("users").Columns("name", "email")
insertQuery.Values("name1", "email1@example.com")
insertQuery.Values("name2", "email2@example.com") // Acumula as linhas.

// SQL PostgreSQL, sem schema explicitamente configurado:
// INSERT INTO users(name, email) VALUES ($1, $2), ($3, $4)
// Argumentos: ["name1", "email1@example.com", "name2", "email2@example.com"]
result, err := insertQuery.Exec(ctx)
if err != nil {
    return err
}

// Consumir result conforme necessário.
// Antes de montar outro lote no mesmo builder:
insertQuery.ClearValues()
```

O retorno de insert, update e delete sem linhas retornadas deve usar `sql.Result`, respeitando as limitações do driver. SELECT usa `*sql.Rows`. `LastInsertId` e `RowsAffected` podem retornar erro; `sql.Result` não fornece uma lista universal de IDs para inserts em lote.

> **Correções — R05–R06, R16, R21, R42 e R45:** corrigidos o nome `insertQuery`, os valores ilustrativos e o SQL parametrizado. O prefixo `public` foi removido porque não era configurado. `defer insertQuery.ClearValues()` é possível, mas limpa apenas na saída da função; não limpa entre execuções na mesma função. Por isso, o exemplo usa limpeza explícita após sucesso.
>
> **Decisões pendentes — R08, R13 e R41–R45:** definir validação de tabela, colunas, duplicatas, quantidade de valores, lote vazio e limites de parâmetros/tamanho. Decidir entre erro e divisão de lotes grandes; se houver divisão, definir atomicidade e resultado parcial. Especificar retenção de valores após execução e falha, cópia de argumentos e alcance de `ClearValues`, preservando configuração necessária à reutilização. Definir `DEFAULT`, nulos, expressões e eventual recuperação de dados gerados. Uma string de valor não deve ser interpretada automaticamente como SQL.

#### Update

**Proposta IA — R33:** para tornar explícita a origem dos critérios, o exemplo considera que `Table` retorna uma referência aceita por `NewCriteria`. Os tipos e essa assinatura precisam de aprovação.

```go
updateQuery := client.Update()
updateTable := updateQuery.Table("users")
updateQuery.Set("name", "name1")
updateQuery.Set("email", "email1@example.com")

criteriaUpdate := krios.NewCriteria(updateTable)
criteriaUpdate.Append(krios.Eq("id", 1))
updateQuery.Criteria(criteriaUpdate) // Pelo menos um critério efetivo é obrigatório.

// SQL equivalente PostgreSQL, sem schema explicitamente configurado:
// UPDATE users SET name = $1, email = $2 WHERE users.id = $3
// Argumentos: ["name1", "email1@example.com", 1]
result, err := updateQuery.Exec(ctx)
```

> **Correções e decisões pendentes — R17, R20–R21, R33 e R43–R44:** rejeitar escrita sem filtro efetivo; um objeto de critérios não vazio, sozinho, não comprova proteção. Não eliminar silenciosamente filtros inválidos. Definir update sem atribuições, repetição de `Set`, referências a colunas e expressões. Mesmo um filtro válido pode atingir todas as linhas; não prometer que a presença de `WHERE` garante seletividade. A política para predicados constantes verdadeiros precisa ser aprovada.

#### Delete

**Proposta IA — R33:** usar uma referência de tabela como origem dos critérios, com a mesma aprovação de contrato exigida no update.

```go
deleteQuery := client.Delete()
deleteTable := deleteQuery.Table("users")

criteriaDelete := krios.NewCriteria(deleteTable)
criteriaDelete.Append(krios.Eq("id", 1))
deleteQuery.Criteria(criteriaDelete) // Pelo menos um critério efetivo é obrigatório.

// SQL equivalente PostgreSQL, sem schema explicitamente configurado:
// DELETE FROM users WHERE users.id = $1
// Argumentos: [1]
result, err := deleteQuery.Exec(ctx)
```

> **Correções e decisões pendentes — R20–R21, R33 e R44:** aplicar as mesmas regras de validade de filtro do update. Critérios vazios, grupos vazios, referências de outra consulta e listas que possam virar condições universais precisam de política explícita. A criação de um mecanismo para escrita sem restrição não é presumida nesta versão.

### Migrations

O utilizador deve informar o dialeto de cada conjunto de migrations. Essa identificação descreve os scripts e permite preparar conjuntos para bancos distintos; não configura um executor global nem traduz SQL arbitrário. A execução usa exclusivamente o executor e o dialeto da `Config` da instância, que devem ser compatíveis com o conjunto selecionado.

Nos exemplos desta seção, `sqliteConfig` e `sqliteClient` são os definidos em Identificação do Dialeto, com um executor SQLite fornecido pela aplicação. O construtor de conjuntos mantém o dialeto como metadado; a entrada de conexão e dialeto para execução fica em `Config`.

**Proposta IA — R04 e R46:** usar argumentos posicionais para o cadastro de SQL inline, mantendo a intenção original. A assinatura proposta de `Append` recebe nome, SQL de subida e SQL de descida; sua validação e forma de retornar erros ainda precisam ser aprovadas. A versão imutável de cada migration também permanece pendente, conforme anotado adiante.

```go
// Este exemplo contém SQL específico de SQLite.
// Para outros bancos, fornecer scripts compatíveis com o respectivo dialeto.
migrations := krios.NewMigrations(sqliteConfig.Dialect)

migrations.Append(
    "create_users",
    `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
            name TEXT NOT NULL,
            email TEXT NOT NULL
        )
    `,
    `
        DROP TABLE users
    `,
)
```

Alternativa com arquivos embutidos, preservando a intenção de fornecer ao Krios os arquivos e os nomes dos scripts de subida e descida:

```go
// No pacote da aplicação consumidora:
import "embed"

// Os arquivos precisam existir no momento da compilação da aplicação.
//go:embed migrations/sqlite/*.sql
var migrationFiles embed.FS
```

```go
// Exemplo independente do cadastro inline.
migrations := krios.NewMigrations(sqliteConfig.Dialect)

// Proposta IA: método distinto, aceitando fs.FS como fonte de arquivos.
// Nome e assinatura de AppendFiles ainda dependem de aprovação.
migrations.AppendFiles(
    "create_users",
    migrationFiles,
    "migrations/sqlite/create_users.up.sql",
    "migrations/sqlite/create_users.down.sql",
)
```

> **Correções e proposta — R04 e R56:** Go não possui argumentos nomeados em chamadas nem sobrecarga de funções por assinatura. Por isso, foram removidos `up:`, `down:` e `EmdedFiles:` das chamadas. O nome `AppendFiles` é uma proposta para diferenciar as duas formas, não um recurso já implementado. `embed.FS` pode ser recebido através de `fs.FS`. O consumidor embute os arquivos; o Krios os lê conforme o contrato aprovado.

Então, chamar no início da aplicação:

```go
// Proposta IA: acrescentar contexto e interpretar o número como versão alvo.
// 1 é o estado desejado; a versão já aplicada deve ser lida do banco.
// sqliteClient recebe executor e dialeto através de sqliteConfig.
// A identidade e a numeração das migrations ainda precisam ser definidas.
// A assinatura do método de execução continua sendo uma proposta.
err := sqliteClient.RunMigrations(ctx, 1, migrations)
if err != nil {
    return err
}
```

Deve ser possível fornecer várias listas, conforme a intenção original. **Decisão pendente — R48:** definir se elas representam partes de uma sequência, módulos independentes ou alternativas para dialetos diferentes. A assinatura variádica, seleção, ordenação e resolução de conflitos dependem dessa decisão; o `...` isolado do exemplo original não era sintaxe Go válida.

O Krios deve gerenciar uma tabela chamada `migrations`, consultar o histórico aplicado e executar os passos necessários para atingir a versão alvo, respeitando as políticas aprovadas de subida, descida e recuperação.

Se a instância receber `*sql.Tx`, tanto os scripts compatíveis quanto a leitura/escrita do histórico devem usar essa transação. A chamada não representa commit da transação do consumidor. **Proposta IA — R14 e R51–R52:** rejeitar antes da execução os planos conhecidos como incompatíveis com a transação recebida, em vez de executá-los por fora dela. Não prometer atomicidade para DDL com commit implícito. A política de coordenação, locks e operações não transacionais continua pendente e deve respeitar a impossibilidade de trocar silenciosamente o executor.

> **Correções — R46–R49:** versão alvo e versão já aplicada são conceitos distintos. Scripts de subida e descida devem ser compatíveis com o dialeto da conexão. `down` não garante restauração de dados; por exemplo, `DROP TABLE` elimina o conteúdo da tabela. Um binário antigo não deve provocar downgrade destrutivo por uma interpretação não documentada da versão.

**Anotações necessárias para fechar o contrato das migrations:**

- **Identidade e ordem — R47–R48:** definir versão ou identificador imutável de cada migration, nomes duplicados, lacunas, versão zero, alvo inexistente, ordem de subida e ordem inversa de descida. Não presumir que inserir um item no meio da lista possa renumerar com segurança um histórico aplicado. Definir dependências e namespaces se houver vários conjuntos.
- **Descida — R49:** aprovar a política de downgrade, configuração/autorização necessária, migrations irreversíveis e comportamento quando o banco estiver em versão superior à da aplicação. A intenção original de suportar `down` permanece, mas sua execução automática precisa desse contrato.
- **Concorrência — R50:** impedir que várias instâncias apliquem simultaneamente a mesma migration. Definir coordenação pelo banco ou garantia operacional de executor único, cobrindo leitura, decisão, execução e registro. Mutex local não coordena processos. Prever timeout, perda de conexão e afinidade de sessão quando houver lock de sessão.
- **Atomicidade — R51:** não prometer rollback transacional uniforme para DDL. MySQL possui comandos com commit implícito; PostgreSQL possui operações com restrições próprias, como `CREATE INDEX CONCURRENTLY`. Definir a política por banco e por tipo de migration, incluindo operações não transacionais.
- **Falhas parciais — R52:** definir estados em execução, concluído e falho, ou representação equivalente aprovada. Não registrar sucesso antes de concluir nem repetir automaticamente uma migration parcialmente aplicada. Prever falha ao registrar o histórico após sucesso do SQL e um procedimento explícito de reparo.
- **Integridade — R53:** avaliar checksum e imutabilidade dos scripts já aplicados. Definir reação a conteúdo alterado, histórico desconhecido e arquivos removidos. Checksum detecta mudança de script, não toda alteração manual de schema.
- **Tabela de controle — R54:** aprovar colunas, chaves, schema, identificação de conjunto e evolução da tabela `migrations`. Definir como detectar colisão com tabela preexistente da aplicação; não assumir propriedade apenas pelo nome.
- **Múltiplos comandos — R55:** definir suporte a um comando, lista estruturada ou script completo por migration. Não separar SQL apenas por ponto e vírgula, pois strings e corpos de funções/triggers podem contê-lo. Compatibilidade e opções do driver precisam ser documentadas; não ativar opções no executor do consumidor silenciosamente.
- **Arquivos — R56:** definir caminhos, leitura, validação antecipada, arquivos ausentes, prioridade entre conteúdo inline e arquivo e retorno de erros de cadastro. A ordem alfabética dos nomes não é uma regra suficiente de versionamento sem convenção aprovada.
- **Contexto, permissões e diagnóstico — R10 e R57:** propagar o contexto recebido, documentar permissões necessárias e identificar nome/versão, direção e etapa em falhas, preservando o erro original. Definir cancelamento, interrupção do processo e inspeção prévia do que será executado. Logs não devem expor credenciais ou valores sensíveis por padrão.

> **Anotação final — R64:** esta versão corrige os exemplos e torna explícitas as limitações e escolhas antes implícitas. As propostas e decisões pendentes precisam ser revisadas pelo desenvolvedor antes de se tornarem tarefas de implementação. O planejamento e as perguntas formais continuarão em `PLANNING.md`, nos arquivos numerados e em `QUESTIONS.md`, conforme as regras do projeto.
