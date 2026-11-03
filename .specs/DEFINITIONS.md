# Definições do Projeto Krios

> Este documento é a definição oficial do projeto Krios. Ele reúne o contrato aprovado e os detalhamentos pendentes que precisam de validação antes da implementação.
>
> **Definição aprovada:** decisão confirmada pelo desenvolvedor. **Nota técnica:** esclarecimento sobre comportamento ou exemplos. **Detalhamento pendente:** aspecto ainda não definido; não autoriza a implementação a escolher silenciosamente uma política.

- É uma biblioteca para criação de queries e gerenciamento de alterações versionadas no banco de dados, denominadas migrations.
- A biblioteca deve ser distribuída pelo GitHub. Seu módulo já está definido como `github.com/mineot/krios`.
- Deve atender, neste momento, aos bancos SQLite3, PostgreSQL e MySQL. Porém, deve permitir implementar suporte a outros bancos no futuro.
- A criação da conexão, a escolha e configuração dos drivers e o fornecimento de contexto são responsabilidades do utilizador. O Krios deve receber um pool `*sql.DB`, o dialeto e um `context.Context` em uma struct pública `krios.Config`, criar uma instância com `client, err := krios.New(config)` e propagar essas informações internamente. Somente a eventual transação é fornecida por execução.

> **Definição aprovada:** `Config.Connection` recebe apenas `*sql.DB`, e `Config.Context` recebe o contexto usado pelas operações da instância. As consultas são criadas pela instância, como `client.Select()`. `Exec()` usa o pool configurado; `ExecTx(tx)` usa exclusivamente a transação fornecida naquela chamada. Ambas usam `Config.Context`, sem argumento de contexto por execução. A transação não fica armazenada no cliente nem no builder. Migrations seguem o mesmo padrão com `RunMigrations` e `RunMigrationsTx`.
>
> **Definições aprovadas:** validar a configuração na criação do cliente e retornar erro para configuração inválida. Novos dialetos serão implementados somente dentro do Krios, sem extensão por terceiros nesta versão. Recursos não suportados retornam erro, sem emulação. A API pública continua acessível por `krios.go`, sem exigir que consumidores importem `internal`.

## Tarefas

As tarefas devem ter o mínimo de atividades possível. É preferível ter muitas tarefas pequenas, cada uma com escopo bem definido.

Seguiremos esta ordem:

- Tarefas de desenvolvimento: focar no desenvolvimento do código.
- Tarefas de testes: criar testes unitários de geração de SQL e testes de integração para cada banco. Usar Docker para os bancos que necessitam de servidor e GitHub Actions para CI. SQLite é embutido; Docker pode padronizar seu ambiente de testes, mas não é necessário um servidor SQLite.
- Tarefas de documentação: escrever o `README.md`, criar um `MANUAL.md` e melhorar a documentação do código. Por fim, traduzir as specs e `AGENTS.md` para en-US.
- Tarefas de publicação: publicar a biblioteca no GitHub e disponibilizá-la para uso em outros projetos Go.

> **Nota técnica:** testes que executam operações em bancos reais são testes de integração; não devem ser classificados apenas como unitários.

**Desenvolvimento, testes e publicação:**

- Preservar a sequência desenvolvimento, testes, documentação e publicação. Durante o desenvolvimento, permitir `gofmt`, compilação, `go vet` e exemplos mínimos; a suíte completa será criada na fase de testes.
- Usar Go 1.26+ (`go 1.26.0` no módulo), PostgreSQL 16+, MySQL 8.0+ e SQLite 3.39+ como referências mínimas da matriz. Testar esses mínimos e versões adicionais explicitamente fixadas.
- Escolher e fixar drivers e versões na tarefa de integração. A aplicação continua responsável pelo pool e pelo registro do driver.
- Isolar bancos/arquivos entre testes. Cobrir SQL por dialeto, entradas inválidas, transações, cancelamento, paginação, UP/DOWN, falhas parciais e concorrência de builders, incluindo detector de races. Cada tarefa deve registrar exemplos aceitos, erros esperados e verificações necessárias.
- Usar GitHub Actions em `.github/workflows`, com formatação, compilação, `go vet` e os testes disponíveis em cada etapa. Integrações usam serviços isolados de PostgreSQL/MySQL e arquivos temporários de SQLite, com versões fixadas.
- Licença Apache-2.0 em `LICENSE`; `MANUAL.md` ao lado do `README.md`, que o referencia. Essas localizações e `.github/workflows` são exceções aprovadas às restrições de organização; deverão ser refletidas em `AGENTS.md` na tarefa correspondente. A implementação permanece em `internal`, com API pública em `krios.go`.
- Usar versionamento semântico: tags `v0.x.y` durante a validação da API e `v1.0.0` quando o contrato estiver estável. Releases exigem as verificações aplicáveis aprovadas e documentação atualizada.

## Escopo

### Receber a conexão com o banco de dados

O pool de conexões, o dialeto e o contexto devem ser fornecidos juntos em `krios.Config`, substituindo os setters globais. O contexto da configuração é usado tanto nas execuções normais quanto nas transacionais; uma execução transacional recebe apenas seu `*sql.Tx` separadamente.

**Definição aprovada:** a declaração pública de configuração deve ser:

```go
// Declaração da API do pacote krios.
// sql corresponde a database/sql; context corresponde ao pacote context.
type Config struct {
    Connection *sql.DB
    Dialect    Dialect
    Context    context.Context
}
```

`Config` não aceita `*sql.Tx` nem `*sql.Conn`. O suporte público a conexão reservada fica fora desta definição inicial. A interface pública `Executor` anteriormente proposta para o campo `Connection` foi removida; uma abstração interna de execução pode compartilhar os métodos necessários de `*sql.DB` e `*sql.Tx`, sem ampliar os tipos aceitos pela configuração.

```go
// db é um *sql.DB criado e configurado pela aplicação.
config := krios.Config{
    Connection: db,
    Dialect:    krios.DialectPostgreSQL,
    Context:    context.Background(),
}

// Forma de criação escolhida pelo desenvolvedor.
client, err := krios.New(config)

if err != nil {
    return err
}
```

**Contexto da instância:** a aplicação pode fornecer um contexto próprio, com cancelamento ou prazo, no campo `Context`. `context.Background()` é a opção explícita quando não houver cancelamento ou prazo definidos pela aplicação. O Krios deve propagar o contexto configurado às chamadas internas de banco, incluindo contagens de paginação e operações auxiliares de migrations. As chamadas públicas de execução não recebem outro contexto nem oferecem substituição por chamada nesta definição.

**Definição aprovada:** a instância deve disponibilizar o método público `Context() context.Context`, acessível como `client.Context()`. Ele retorna o mesmo contexto recebido em `Config.Context` e utilizado nas operações do cliente, preservando valores, prazo e cancelamento. O método não cria nem substitui o contexto e não reinicia seu prazo. Seu nome público é `Context()`, sem o prefixo `Get`.

```go
// Obtém o contexto usado pelo cliente para outras operações da aplicação.
ctx := client.Context()
```

O contexto mantém seu prazo e estado de cancelamento durante a vida da instância; seu prazo não é reiniciado a cada execução. Se for cancelado ou expirar, as próximas operações continuam recebendo esse mesmo contexto. O Krios não deve substituí-lo silenciosamente por `context.Background()` para continuar executando. Para usar outro contexto, a aplicação pode criar outra instância com o mesmo pool e dialeto, fornecendo o novo contexto. A aplicação permanece responsável por chamar a função de cancelamento dos contextos que criar.

| Tipo        | O que representa                       | Uso no Krios                                                                 |
| ----------- | -------------------------------------- | ---------------------------------------------------------------------------- |
| `*sql.DB`   | Pool de conexões.                      | Recebido em `Config.Connection`; usado por `Exec()` e `RunMigrations`.       |
| `*sql.Conn` | Uma conexão reservada do pool.         | Não é aceito como entrada pública nesta definição inicial.                   |
| `*sql.Tx`   | Uma transação vinculada a uma conexão. | Recebido apenas por `ExecTx(tx)` ou `RunMigrationsTx`, para aquela execução. |

O Krios gera o SQL e os argumentos e chama o executor selecionado para a operação: o pool da configuração ou a transação recebida na chamada. `database/sql` coordena o acesso através do driver; o motor do banco executa o SQL. PostgreSQL/MySQL normalmente executam no servidor, enquanto SQLite executa seu motor no processo da aplicação. A instância Krios não precisa obter um `*sql.Conn` para poder executar por `*sql.DB`. [Gerenciamento de conexões](https://go.dev/doc/database/manage-connections).

Uso com uma transação criada pela aplicação, mantendo o mesmo `client` configurado com `db`. A chamada a `BeginTx` pertence a `database/sql` e continua exigindo contexto; no exemplo, a aplicação obtém esse contexto por `client.Context()`, sem precisar manter acesso à variável de configuração:

```go
tx, err := db.BeginTx(client.Context(), nil)

if err != nil {
    return err
}

defer tx.Rollback()

insertQuery := client.Insert()
insertQuery.Into("users").Columns("name", "email")
insertQuery.Values("name1", "email1@example.com")

if _, err := insertQuery.ExecTx(tx); err != nil {
    return err
}

// A aplicação confirma a transação.
return tx.Commit()
```

**Regras aprovadas de execução:**

- `Exec()` seleciona o pool de `Config.Connection`. `ExecTx(tx)` seleciona exclusivamente `tx`, inclusive para consultas auxiliares, como a contagem da paginação.
- O contexto vem de `Config.Context`; somente a transação é argumento da chamada transacional. O executor selecionado é propagado internamente junto do contexto configurado durante a operação; a escolha de executor não altera `Config` nem fica retida no cliente ou no builder. O mesmo cliente pode participar de diferentes transações em chamadas distintas, usando o contexto da instância.
- Depois de uma chamada a `ExecTx`, uma futura chamada a `Exec` continua usando o pool. A reutilização dos valores e filtros do builder segue seu contrato próprio; selecionar outro executor não os limpa. O compartilhamento de builders segue as regras de concorrência da seção Query Builder.
- `Config.Context` controla cancelamento e prazo das operações do Krios; `*sql.Tx` define a transação. Não usar valores do contexto para selecionar implicitamente uma transação. Receber `tx` não substitui o contexto configurado nem modifica o contexto utilizado pela aplicação ao iniciar a transação.
- `ExecTx(nil)` deve retornar erro. Transação encerrada também deve produzir erro, preservando a causa. Não fazer fallback para o pool, abrir outra transação nem repetir a operação por fora dela para contornar falhas.
- A aplicação cria a transação e permanece responsável por `Commit` e `Rollback`. O Krios não inicia nem finaliza a transação recebida, e não fecha o pool fornecido pelo consumidor. [Transações em Go](https://go.dev/doc/database/execute-transactions).

Os resultados de SELECT ainda precisam ser consumidos e fechados pela aplicação antes de encerrar a transação. Não reter `tx` no builder não muda o ciclo de vida dos resultados retornados pelo driver.

> **Compatibilidade:** a aplicação deve fornecer uma transação do banco pretendido e compatível com o dialeto do cliente. O dialeto continua vindo da configuração; `ExecTx` não o altera.
>
> **Validação:** `*sql.DB` oferece `PingContext`; `*sql.Tx` não. Qualquer verificação de comunicação feita pelo Krios deve usar `Config.Context`. Não consultar o pool para validar uma execução transacional. A verificação de comunicação não garante disponibilidade futura. Configuração ausente, dialeto inválido e `Config.Connection == nil` geram erro de configuração em `New`. Não é possível garantir, sem operação, que uma transação continua utilizável.

**Criação e configuração:**

- `New(config)` retorna cliente e erro. Validar os campos antes de disponibilizar o cliente; configuração inválida retorna cliente nulo e erro. A aplicação deve tratar o erro antes de construir ou executar consultas.
- Rejeitar `Connection == nil`, `Context == nil` e dialeto ausente/inválido. Não escolher contexto padrão implicitamente. A aplicação pode fornecer `Background`, contexto cancelável ou contexto com prazo.
- Copiar a struct `Config` na criação, sem setters posteriores. A cópia mantém as referências do pool e do contexto: alterar a struct original não muda o cliente; fechar o pool ou cancelar o contexto continua afetando suas operações.
- Não manter configuração global. Cada builder usa a configuração da sua instância. Validações locais precedem acesso ao banco; uma chamada transacional nunca consulta o pool para validar sua execução.
- **Detalhamento pendente:** definir se haverá verificação de comunicação/detecção de versão na criação e como verificar compatibilidade real entre dialeto e banco. Validar campos não comprova disponibilidade do banco; nenhuma política de `PingContext` automático foi aprovada.


### Identificação do Dialeto

O dialeto é informado no campo `Dialect` de `krios.Config` e propagado pela instância, inclusive em chamadas transacionais. Os exemplos abaixo representam alternativas, cada uma com seu pool `*sql.DB` correspondente:

```go
sqliteConfig := krios.Config{
    Connection: sqliteDB,
    Dialect:    krios.DialectSQLite3,
    Context:    context.Background(),
}

sqliteClient, err := krios.New(sqliteConfig)

if err != nil {
    return err
}
```

```go
postgresConfig := krios.Config{
    Connection: postgresDB,
    Dialect:    krios.DialectPostgreSQL,
    Context:    context.Background(),
}

postgresClient, err := krios.New(postgresConfig)

if err != nil {
    return err
}
```

```go
mysqlConfig := krios.Config{
    Connection: mysqlDB,
    Dialect:    krios.DialectMySQL,
    Context:    context.Background(),
}

mysqlClient, err := krios.New(mysqlConfig)

if err != nil {
    return err
}
```

> **Definição aprovada:** dialeto ausente/inválido gera erro de configuração em `New`. Recursos sem suporte geram erro de incompatibilidade, sem emulação ou mudança silenciosa de semântica. O dialeto é fornecido pelo utilizador e não muda ao receber uma transação. As capacidades dependem também da versão do banco; a estratégia de verificação dessa versão ainda exige detalhamento.

### Query Builder

- O método `As` define aliases e é opcional.
- O método `Schema` define o qualificador da tabela e é opcional. Deve respeitar o significado desse qualificador no banco escolhido.
- A criação de qualquer query deve respeitar o dialeto selecionado e suas capacidades.
- Valores devem ser enviados como argumentos separados do SQL, usando placeholders compatíveis com o banco e o driver. Os exemplos não autorizam concatenação de valores no texto executável.
- Tabelas, schemas, colunas e aliases precisam de tratamento de identificadores, separado da parametrização de valores. Um nome qualificado deve ter seus componentes tratados individualmente; `*` precisa de tratamento próprio.
- A ordem dos argumentos deve corresponder à ordem dos placeholders da consulta completa, incluindo joins, grupos lógicos, listas e múltiplas linhas de insert.
- A ordem de registro deve ser preservada dentro de cada cláusula, inclusive quando seus itens vêm de tabelas diferentes. A ordem das cláusulas SQL continua seguindo a gramática do banco.

> **Notas técnicas:** PostgreSQL possui schemas; no MySQL, schema corresponde a database; no SQLite, os qualificadores incluem `main`, `temp` e nomes de bancos anexados. `public` não é um padrão universal nem deve aparecer sem configuração explícita ou regra de herança aprovada.

**Encadeamento, repetição e referências:**

- Os métodos modificam o builder existente. `From` retorna uma referência pública de tabela, `Column` uma referência de coluna e `OrderBy` um item de ordenação; `As`, `Schema` e `Desc` retornam o próprio objeto correspondente.
- SELECT, UPDATE e DELETE usam `From`; INSERT mantém `Into`. A referência retornada por `From` pode ser usada por `NewCriteria`.
- `From`, `Into`, `Schema` e `As` substituem a definição anterior. `Criteria` acumula filtros com AND por padrão, aceitando composição explícita com `krios.And`/`krios.Or`. Adições de colunas, joins, agrupamentos, ordenações e linhas de valores acumulam na ordem de registro; `Columns(...)` do INSERT substitui sua lista completa. `Set` substitui o valor já atribuído à mesma coluna.
- Rejeitar aliases de tabela repetidos no mesmo escopo; self-joins precisam de referências distintas e aliases diferentes. Critérios se vinculam à referência da tabela, acompanhando suas alterações de alias. Substituir `From` invalida as referências antigas: não redirecionar filtros silenciosamente.
- Para reutilizar critérios em outra consulta, exigir associação explícita à tabela de destino. Referências entre consultas são inválidas fora dessa associação ou da correlação explícita de subconsultas. Os tipos públicos e a assinatura dessa associação ainda precisam de detalhamento.

**Reutilização e concorrência:**

- Builders e critérios podem ser compartilhados entre goroutines, com proteção interna de acesso. Cada execução usa uma cópia consistente do estado da consulta; alterações posteriores não modificam a execução em andamento.
- Copiar slices, bytes e critérios recebidos. Objetos personalizados recebidos por referência não podem ser modificados externamente durante o uso; não prometer cópia profunda arbitrária de valores Go.
- Disponibilizar `Clone()` para variações independentes. Uma sequência de chamadas de construção não é indivisível: goroutines que precisam de filtros ou páginas diferentes devem usar clones. A proteção interna não torna a ordem de alterações concorrentes determinística.
- Copiar critérios não transfere automaticamente sua origem para outra consulta; aplicar as regras de associação acima. A transação continua restrita a cada execução, sem ser armazenada ou clonada.

**Construção, erros e escopo:**

- Disponibilizar `Build() (string, []any, error)` nos builders para obter SQL e parâmetros sem acessar o banco. `Exec` e `ExecTx` compartilham a validação e a geração de SQL com `Build`.
- Validar configuração em `New` e estrutura da consulta em `Build`/`Exec`/`ExecTx`, antes de enviar comandos ao banco. Erros de configuração, construção, incompatibilidade e execução devem ser distinguíveis por `errors.Is`/`errors.As`, preservando a causa original do driver. Os identificadores públicos dessas categorias ainda serão detalhados.
- Não usar panic para entradas inválidas nem iniciar repetição automática de escritas por iniciativa do Krios.
- O escopo inicial inclui os recursos exemplificados e as subconsultas descritas abaixo. SQL livre no builder, CTEs, unions, funções de janela e upserts ficam para uma etapa posterior. Scripts SQL das migrations permanecem permitidos.

```go
selectQuery := client.Select()
selectQuery.From("users")
sqlText, args, err := selectQuery.Build()
if err != nil {
    return err
}
// Inspecionar sqlText e args sem executar a consulta.
```

**Convenções dos exemplos:**

- Cada bloco de uso representa um exemplo independente, salvo quando o texto disser que há continuação. `client` é uma instância criada por `krios.New(config)`, configurada para PostgreSQL nos exemplos do query builder. A preparação comum abaixo deve ser repetida no início de cada exemplo de seleção que use `selectQuery` e `selectFrom`.
- Os exemplos são fragmentos de uma API em definição, não programas compiláveis contra a implementação atual. O contexto é fornecido pela aplicação em `Config.Context`. Imports, tratamento repetitivo de erros e consumo completo das linhas são omitidos nos fragmentos posteriores ao exemplo de `Select`.
- **Definição aprovada:** SELECT, insert, update e delete oferecem `Exec()` e `ExecTx(tx)`, com os mesmos retornos por tipo de query. Ambas as formas usam e propagam `Config.Context` a toda a operação, inclusive contagens auxiliares; a transação só vale para a chamada em que foi recebida.
- **Definição aprovada:** SELECT retorna `(*sql.Rows, *krios.Page, error)`; INSERT, UPDATE e DELETE retornam `(sql.Result, error)`.
- **Definição aprovada:** sem paginação e com sucesso, retornar `(rows, nil, nil)`; com paginação, retornar os metadados. Falha na preparação, contagem ou abertura da seleção retorna `(nil, nil, err)`, fechando recursos intermediários. Falhas posteriores de leitura continuam sendo verificadas pela aplicação em `Scan` e `rows.Err()`; não é possível alterar retroativamente o retorno de `Exec`.
- Nos SELECTs sem uso dos metadados, `_` descarta o segundo retorno. Nunca usar `nil` como nome de variável para esse fim.
- Os SQLs do query builder usam PostgreSQL como referência, com `$1`, `$2` etc. Mostram a forma equivalente da consulta, sem fixar a política final de aspas dos identificadores. Nos demais dialetos, adaptar qualificadores e placeholders.

As chamadas abaixo são alternativas de execução para uma query já construída; não devem ser executadas em sequência apenas para demonstrar as duas formas, pois isso repetiria a operação:

| Query  | Execução pelo pool                      | Execução na transação recebida              |
| ------ | --------------------------------------- | ------------------------------------------- |
| SELECT | `rows, page, err := selectQuery.Exec()` | `rows, page, err := selectQuery.ExecTx(tx)` |
| INSERT | `result, err := insertQuery.Exec()`     | `result, err := insertQuery.ExecTx(tx)`     |
| UPDATE | `result, err := updateQuery.Exec()`     | `result, err := updateQuery.ExecTx(tx)`     |
| DELETE | `result, err := deleteQuery.Exec()`     | `result, err := deleteQuery.ExecTx(tx)`     |

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
rows, _, err := selectQuery.Exec()

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
rows, _, err := selectQuery.Exec()
// Tratar err e consumir/fechar rows como no exemplo anterior.
```

> **Notas técnicas:** SELECT retorna linhas, não `sql.Result`. Em exemplos sequenciais no mesmo bloco, usar `=` quando não houver variável nova e encerrar o resultado anterior antes de substituir sua referência. O consumidor deve tratar tanto o erro inicial quanto erros de leitura, e fechar os resultados que receber.

#### Colunas

Aplicar sobre uma nova preparação comum:

```go
selectFrom.Column("id").As("user_id")
selectFrom.Column("name").As("user_name")
selectFrom.Column("age").As("user_age")

// SQL:
// SELECT u.id AS user_id, u.name AS user_name, u.age AS user_age
// FROM public.users AS u
rows, _, err := selectQuery.Exec()
```

> **Definição aprovada:** a primeira projeção explícita remove o `*` automático. Permitir `Column("*")` junto de outras projeções, preservando a ordem; na referência de tabela, o asterisco representa `alias.*` (ou o qualificador da tabela sem alias). Nomes já qualificados precisam corresponder a uma origem registrada. Rejeitar aliases explícitos de saída duplicados; não confundir alias de saída com coluna da origem.

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
rows, _, err := selectQuery.Exec()
```

> **Notas técnicas:** `COUNT(*)` conta linhas; `COUNT(coluna)` conta valores não nulos. Não qualificar automaticamente o asterisco de `Count("*")`. Outros agregadores podem retornar `NULL` quando não há valores. O consumidor precisa usar destinos de leitura compatíveis com os tipos e a nulabilidade dos resultados.
>
> **Definição aprovada:** aceitar `Count("*")` e agregadores sobre uma coluna válida. Rejeitar `*` nos demais agregadores. `COUNT(DISTINCT ...)` e `HAVING` ficam para uma próxima etapa.

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
rows, _, err := selectQuery.Exec()
```

> **Definição aprovada:** quando houver agrupamento ou agregação, cada projeção não agregada deve constar em `GroupBy`. O Krios não infere dependências funcionais pelo schema para dispensar essa exigência. Não aceitar um `SELECT *` incompatível com essa regra.

#### OrderBy

Aplicar sobre uma nova preparação comum:

```go
selectFrom.OrderBy("col1")
selectFrom.OrderBy("col2").Desc()

// SQL:
// SELECT * FROM public.users AS u ORDER BY u.col1, u.col2 DESC
rows, _, err := selectQuery.Exec()
```

> **Definição aprovada:** `Desc` afeta somente o item de ordenação ao qual está encadeado; preservar a ordem dos itens entre todas as origens. Oferecer `OrderByAlias("nome")` para ordenar por um alias explícito da projeção, sem qualificá-lo como coluna da tabela. A referência de alias deve existir e ser inequívoca.

```go
selectQuery := client.Select()
selectFrom := selectQuery.From("users").As("u")
selectFrom.Column("name").As("user_name")
selectQuery.OrderByAlias("user_name").Desc()
// SELECT u.name AS user_name FROM users AS u ORDER BY user_name DESC
```

#### Critérios

Aceitar strings, números, `time.Time`, booleanos, bytes, ponteiros, nulos e tipos que implementam `driver.Valuer`, respeitando a conversão de `database/sql` e as capacidades do driver/banco. Não converter números silenciosamente com perda de precisão nem alterar o fuso de datas por iniciativa do Krios. Valores não representáveis retornam erro. Isso não autoriza qualquer struct ou mapa sem conversão. Nulos em escritas e nulos em comparações seguem regras distintas. [Conversão de parâmetros](https://pkg.go.dev/database/sql/driver#DefaultParameterConverter).

Aplicar sobre uma nova preparação comum:

```go
criteriaFrom := krios.NewCriteria(selectFrom)
criteriaFrom.Append(krios.Eq("id", 1))
selectQuery.Criteria(criteriaFrom)

// SQL: SELECT * FROM public.users AS u WHERE u.id = $1
// Argumentos: [1]
// O operador lógico padrão entre critérios adicionados é AND.
rows, _, err := selectQuery.Exec()
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

`Exists` e `NotExists` recebem um builder SELECT, copiado ao ser incorporado. A consulta externa reúne os parâmetros da consulta completa, ajustando os placeholders na ordem do SQL. Exigir o mesmo dialeto. Subconsultas correlacionadas usam referências explícitas a colunas da consulta externa; rejeitar referências fora desse vínculo e nunca interpretar strings de valor como SQL. A assinatura da referência externa ainda precisa ser detalhada.

```go
// subquery é um builder SELECT compatível. Chamadas alternativas:
// criteriaFrom.Append(krios.Exists(subquery))
// criteriaFrom.Append(krios.NotExists(subquery))
```

> **Notas técnicas:** `EXISTS` verifica se uma subconsulta retorna linhas; não verifica existência de coluna ou valor não nulo. Os exemplos de `LIKE` usam uma coluna textual. Sensibilidade a maiúsculas, acentos e escape dos curingas dependem do banco e da configuração.
>
> **Definições aprovadas:** listas vazias ou nulas em `IN`/`NOT IN` geram erro, sem gerar condição constante nem remover filtros. Valores continuam parametrizados; `[]byte` representa valor binário e não deve ser confundido automaticamente com uma lista. As referências de origem e as subconsultas obedecem às regras de escopo acima.
>
> **Definição aprovada:** `Eq("campo", nil)` e demais comparações de valor com nulo retornam erro; exigir `IsNull`/`IsNotNull`, sem normalização silenciosa. Nulos permanecem aceitos como valores de escrita.
>
> **Detalhamento pendente:** fechar os tipos de coleções aceitos por `IN`/`NOT IN`, sua expansão e o tratamento de elementos nulos dentro de listas não vazias. Não confundir lista nula com elemento nulo nem inferir que a rejeição de listas vazias resolve essas escolhas. Não remover elementos nulos silenciosamente.

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
rows, _, err := selectQuery.Exec()
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
rows, _, err := selectQuery.Exec()
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
rows, _, err := selectQuery.Exec()
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
rows, _, err := selectQuery.Exec()
```

> **Nota técnica:** a geração deve preservar a árvore lógica solicitada. Não depender apenas da precedência de `AND` sobre `OR`, pois o agrupamento inverso altera o resultado sem os parênteses necessários.
>
> **Definições aprovadas:** rejeitar grupos explícitos `And`/`Or` vazios ou com apenas um elemento, assim como critérios nulos. Um critério simples pode ser anexado diretamente, sem envolvê-lo em grupo unitário, como nos exemplos. Chamadas repetidas de `Criteria` acumulam com AND; aceitar composição explícita com AND/OR e preservar os parênteses. Não descartar filtros inválidos silenciosamente.

#### Joins

**Definição aprovada:** `Join` significa INNER JOIN. Oferecer `InnerJoin`, `LeftJoin`, `RightJoin`, `FullJoin` e `CrossJoin`, conforme as capacidades do banco. INNER, LEFT, RIGHT e FULL exigem condição válida; CROSS não aceita `On`. Recursos sem suporte retornam erro, sem emulação. `CrossApply` e outros joins ficam para uma etapa posterior.

`On` usa os mesmos operadores de critérios e composição lógica disponíveis em `WHERE`, distinguindo explicitamente referência de coluna e valor parametrizado. A API anterior com `Ons`, `On().Equal(...)` e `nil` como marcador de literal foi substituída. `nil` não identifica coluna nem literal nesse contrato. O tipo público da referência de coluna ainda será detalhado; não confundir essa referência com o método `Column` que adiciona projeções.

A origem de cada operando e a cláusula de destino devem ser explícitas. Mover condições entre `ON` e `WHERE` pode alterar o resultado de outer joins. Joins oferecem os recursos aplicáveis à origem, como projeções, agregadores, agrupamento e ordenação. Schemas são explícitos, sem herança automática do `From`.

Exemplo do SQL que a composição deve produzir:

```sql
SELECT * FROM public.users AS u
INNER JOIN public.seniors AS s
    ON s.user_id = u.id AND s.user_id = $1
```

O parâmetro é `[1]`. `s.user_id = u.id` compara duas referências de coluna; `s.user_id = $1` compara uma coluna a um valor. A forma Go completa será registrada quando os tipos de operandos forem definidos, sem reutilizar o marcador `nil` rejeitado.

#### Paginação

O primeiro argumento de `Paginate` representa a página, e o segundo, a quantidade de linhas por página.

**Definição aprovada:** paginação usa base um. A primeira página é 1; `offset = (pagina - 1) * tamanho`. Página e tamanho devem ser positivos, com cálculo e conversões verificados contra overflow e limites representáveis pelo banco.

Aplicar sobre uma nova preparação comum:

```go
// Neste exemplo, id é uma chave única, usada para ordenação determinística.
selectFrom.OrderBy("id")
selectQuery.Paginate(1, 10)

// SQL equivalente:
// SELECT * FROM public.users AS u ORDER BY u.id LIMIT 10 OFFSET 0
rows, page, err := selectQuery.Exec()
```

Para executar a mesma consulta paginada em uma transação, substituir a chamada acima por:

```go
// tx é uma transação válida criada pela aplicação.
// Tanto a contagem quanto a leitura da página usam tx e Config.Context.
rows, page, err := selectQuery.ExecTx(tx)
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

Os auxiliares executam a consulta e retornam `(*sql.Rows, *krios.Page, error)`, como `Exec`. As chamadas abaixo são alternativas, depois de uma execução paginada bem-sucedida e do fechamento das linhas anteriores:

```go
rows, page, err := selectQuery.NextPage() // Usa o pool.
// Alternativas:
// rows, page, err := selectQuery.PreviousPage()
// rows, page, err := selectQuery.NextPageTx(tx)
// rows, page, err := selectQuery.PreviousPageTx(tx)
```

> **Notas técnicas:** retornar metadados preenchidos quando a paginação for executada com sucesso. A contagem precisa considerar os efeitos de joins, `DISTINCT`, agrupamentos e agregações. Não basta substituir qualquer projeção por `COUNT(*)`. Paginação sem ordenação determinística não garante páginas estáveis; mesmo com ordenação, mudanças concorrentes podem deslocar linhas entre páginas por offset.
>
> **Definições aprovadas:** contar as linhas do resultado completo antes de abrir as linhas da página, considerando joins, `DISTINCT` e agrupamentos. Contagem e leitura pelo pool podem observar alterações concorrentes. Em `ExecTx`, ambas usam a transação recebida e dependem do isolamento escolhido pela aplicação; a existência de uma transação, sozinha, não garante visão estável. Documentar o custo da contagem.
>
> **Definição aprovada:** contagem e leitura usam `Config.Context` e o executor selecionado para aquela chamada. Em `ExecTx(tx)`, ambas devem usar a transação fornecida, respeitando seu isolamento; não criar uma transação auxiliar nem retornar ao pool. Em `Exec()`, usar o pool configurado. O cliente e o builder não retêm a transação para a próxima página; cada nova execução transacional precisa receber `tx` explicitamente, enquanto o contexto continua vindo da configuração.

**Metadados e limites:**

- `TotalRows`: quantidade de linhas do resultado antes da paginação. `RowsPerPage`: tamanho solicitado. `TotalPages`: divisão de `TotalRows` por `RowsPerPage`, arredondada para cima sem overflow. `LastPage` é igual a `TotalPages`; `CurrentPage` preserva a página solicitada.
- Com resultado vazio, `TotalRows`, `TotalPages` e `LastPage` são zero, e os dois indicadores de navegação são falsos. Solicitar a página 1 continua válido, com linhas vazias.
- Com resultado não vazio, `HasNextPage` é `CurrentPage < LastPage` e `HasPreviousPage` é `CurrentPage > 1`.
- Solicitação de página além do fim retorna linhas vazias e mantém a página solicitada; não ajustar silenciosamente. Rejeitar página/tamanho zero, negativos, overflow e conversões que não caibam no banco. Os campos `uint64` de `Page` não dispensam essas validações; o tipo dos argumentos de `Paginate` ainda será formalizado.

**Navegação com execução:**

- Registrar a revisão da consulta executada com sucesso. `NextPage`/`PreviousPage` exigem execução paginada anterior e nenhuma alteração posterior na construção; caso contrário, retornar erro e exigir novo `Exec` ou `ExecTx`.
- Preservar filtros, projeções, ordenação e tamanho, mudando somente a página/offset. Atualizar a posição somente após sucesso; nos limites conhecidos, permanecer na página atual. Não armazenar `*sql.Rows` para devolver um resultado já consumido.
- Refazer a contagem em cada chamada, pois os dados podem mudar; uma revisão de builder não detecta mudanças externas no banco. As regras de consistência entre contagem e seleção continuam aplicáveis.
- `NextPage()`/`PreviousPage()` usam o pool. `NextPageTx(tx)`/`PreviousPageTx(tx)` usam somente a transação fornecida nessa chamada, sem reter ou reutilizar implicitamente a anterior. Todas usam `Config.Context`.
- A aplicação deve fechar as linhas anteriores antes de navegar. Navegações independentes em goroutines usam clones; mudanças concorrentes no builder não podem ser confundidas com a revisão que terminou de executar.
- **Detalhamento pendente:** formalizar a cópia do estado de navegação em `Clone` e o comportamento nos limites quando a nova contagem divergir da anterior. Preservar as regras aprovadas de não ajustar uma página solicitada silenciosamente e de não avançar a posição em falha.

#### Insert

```go
insertQuery := client.Insert()
insertQuery.Into("users").Columns("name", "email")
insertQuery.Values("name1", "email1@example.com")
insertQuery.Values("name2", "email2@example.com") // Acumula as linhas.

// SQL PostgreSQL, sem schema explicitamente configurado:
// INSERT INTO users(name, email) VALUES ($1, $2), ($3, $4)
// Argumentos: ["name1", "email1@example.com", "name2", "email2@example.com"]
result, err := insertQuery.Exec()

if err != nil {
    return err
}

// Consumir result conforme necessário.
// Antes de montar outro lote no mesmo builder:
insertQuery.ClearValues()
```

O retorno de insert, update e delete sem linhas retornadas deve usar `sql.Result`, respeitando as limitações do driver. SELECT usa `*sql.Rows`. `LastInsertId` e `RowsAffected` podem retornar erro; `sql.Result` não fornece uma lista universal de IDs para inserts em lote.

> **Notas técnicas:** corrigidos o nome `insertQuery`, os valores ilustrativos e o SQL parametrizado. O prefixo `public` foi removido porque não era configurado. `defer insertQuery.ClearValues()` é possível, mas limpa apenas na saída da função; não limpa entre execuções na mesma função. Por isso, o exemplo usa limpeza explícita após sucesso.

**Definições aprovadas:**

- Exigir tabela e colunas válidas, sem colunas duplicadas, pelo menos uma linha e a mesma quantidade de valores e colunas em cada linha. Não descartar valores ou linhas inválidos.
- Validar limites conhecidos de parâmetros e tamanho antes da execução. Limites dependentes da configuração do servidor podem resultar em erro do driver. Lotes grandes retornam erro, sem divisão automática; a aplicação pode dividir e executar as partes em uma transação própria.
- Manter valores após sucesso ou falha. `ClearValues` remove todas as linhas, preservando destino e colunas; executar sem adicionar novas linhas depois dessa limpeza gera erro. Aplicar as regras de cópia e concorrência da seção Query Builder.
- Aceitar `nil` como SQL NULL e valores parametrizados. Usar defaults omitindo a coluna no INSERT. Marcador `DEFAULT`, expressões, atribuições entre colunas e `RETURNING` ficam para uma etapa posterior. Strings de valor nunca são interpretadas automaticamente como SQL.
- Manter `sql.Result` nas escritas, sem prometer IDs gerados quando o driver não os disponibilizar.

#### Update

**Definição aprovada:** UPDATE usa `From`, como SELECT. A referência retornada é aceita por `NewCriteria`; `Table` não faz parte desse encadeamento.

```go
updateQuery := client.Update()
updateFrom := updateQuery.From("users")
updateQuery.Set("name", "name1")
updateQuery.Set("email", "email1@example.com")

criteriaUpdate := krios.NewCriteria(updateFrom)
criteriaUpdate.Append(krios.Eq("id", 1))
updateQuery.Criteria(criteriaUpdate) // Pelo menos um critério efetivo é obrigatório.

// SQL equivalente PostgreSQL, sem schema explicitamente configurado:
// UPDATE users SET name = $1, email = $2 WHERE users.id = $3
// Argumentos: ["name1", "email1@example.com", 1]
result, err := updateQuery.Exec()
```

> **Definição aprovada:** rejeitar UPDATE sem atribuições e UPDATE/DELETE sem filtro efetivo. `Set` repetido para a mesma coluna substitui o valor anterior. Rejeitar critérios inválidos e condições constantes reconhecidas como sempre verdadeiras; não oferecer escrita irrestrita nesta versão. Um filtro válido ainda pode atingir todas as linhas existentes: a proteção não garante seletividade. Expressões e atribuições entre colunas seguem o adiamento do item de valores especiais.

#### Delete

**Definição aprovada:** DELETE usa `From`, retornando uma referência aceita por `NewCriteria`.

```go
deleteQuery := client.Delete()
deleteFrom := deleteQuery.From("users")

criteriaDelete := krios.NewCriteria(deleteFrom)
criteriaDelete.Append(krios.Eq("id", 1))
deleteQuery.Criteria(criteriaDelete) // Pelo menos um critério efetivo é obrigatório.

// SQL equivalente PostgreSQL, sem schema explicitamente configurado:
// DELETE FROM users WHERE users.id = $1
// Argumentos: [1]
result, err := deleteQuery.Exec()
```

> **Definição aprovada:** DELETE segue as mesmas regras de validade e proteção do UPDATE. Critérios vazios/nulos, grupos inválidos, referências indevidas e listas vazias geram erro, sem remover filtros nem convertê-los em condições universais. Não disponibilizar execução sem filtro efetivo.

### Migrations

O utilizador deve informar o dialeto de cada conjunto de migrations. Essa identificação descreve os scripts e permite preparar conjuntos para bancos distintos; não configura um executor global nem traduz SQL arbitrário. O dialeto da execução vem da `Config` da instância e deve ser compatível com o conjunto selecionado. `RunMigrations` usa o pool configurado; `RunMigrationsTx` usa exclusivamente a transação recebida para aquela chamada. Ambas as formas usam `Config.Context`, sem argumento de contexto nas chamadas.

Nos exemplos desta seção, `sqliteConfig` e `sqliteClient` são os definidos em Identificação do Dialeto, com um pool SQLite fornecido pela aplicação. O construtor de conjuntos mantém o dialeto como metadado; o pool padrão, o dialeto de execução e o contexto ficam em `Config`. A eventual transação é fornecida diretamente a `RunMigrationsTx`.

**Definição aprovada:** `Append` e `AppendFiles` devem receber explicitamente a versão numérica e a ordem de execução da migration. Uma mesma versão pode agrupar várias migrations. Cada cadastro representa um par de scripts de subida e descida, associado à mesma versão e ordem. Esses valores não são inferidos da posição na lista nem do nome dos arquivos.

O cadastro inline é `Append(version, order, name, up, down) error`; por arquivos, `AppendFiles(version, order, name, files, upFile, downFile) error`. Versão e ordem são `int64`, positivos, nessa sequência, seguidos do nome; a fonte de arquivos é `fs.FS`. Ambos retornam erro de cadastro. Erros também invalidam o conjunto para execução, evitando executar uma lista incompleta caso o consumidor ignore o retorno.

**Tipos, limites e alvo:** versão e ordem começam em 1 e podem ter lacunas, dentro do limite de `int64`. Nomes devem ser únicos no histórico. A versão alvo também é `int64`: alvo negativo gera erro; alvo 0 desfaz todas as migrations aplicadas; alvo positivo precisa estar cadastrado. A escolha do alvo não dispensa validação do histórico e do plano completo antes de qualquer script.

Os exemplos representam a API documentada, não uma implementação já disponível. Os nomes dos tipos públicos de conjuntos e o modo de corrigir/recriar um conjunto invalidado ainda serão formalizados.

**Regras aprovadas de ordenação:**

- No UP, executar as versões em ordem crescente e, dentro de cada versão, as migrations em ordem crescente de `order`.
- No DOWN, executar as versões em ordem decrescente e, dentro de cada versão, as migrations em ordem decrescente de `order`.
- A versão tem prioridade sobre a ordem: ordenar primeiro por `version` e depois por `order`. A posição das chamadas de cadastro não determina a sequência de execução.
- A ordem pode se repetir em versões diferentes. Dentro da mesma versão e do mesmo histórico, ordens duplicadas devem gerar erro antes da execução de qualquer script do plano, inclusive quando vêm de listas distintas que compõem esse histórico. A validação deve considerar em conjunto os cadastros inline e por arquivos.
- São permitidos intervalos entre ordens, como `10`, `20` e `30`; não é necessário usar números consecutivos. Lacunas entre versões também são permitidas, respeitando a exigência de que o alvo positivo exista.
- Executar DOWN somente para migrations efetivamente aplicadas e pertencentes às versões que precisam ser desfeitas para atingir o alvo. As migrations da versão alvo permanecem aplicadas.
- Não alterar a ordem nem renumerar a versão de migrations já aplicadas.

Nos exemplos abaixo, `create_users` e `create_roles` pertencem à versão 1, com ordens 10 e 20. `alter_users` pertence à versão 2, com ordem 10. A repetição da ordem 10 é válida porque ocorre em versões diferentes.

```go
// Este exemplo contém SQL específico de SQLite.
// Para outros bancos, fornecer scripts compatíveis com o respectivo dialeto.
migrations := krios.NewMigrations(sqliteConfig.Dialect)

if err := migrations.Append(
    1,
    10,
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
); err != nil {
    return err
}

// Outra migration da mesma versão, executada depois de create_users.
if err := migrations.Append(
    1,
    20,
    "create_roles",
    `
        CREATE TABLE roles (
            id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
            name TEXT NOT NULL
        )
    `,
    "DROP TABLE roles",
); err != nil {
    return err
}

// Exemplo de alteração posterior: adicionar um índice à tabela users.
if err := migrations.Append(
    2,
    10,
    "alter_users",
    "CREATE INDEX idx_users_email ON users(email)",
    "DROP INDEX idx_users_email",
); err != nil {
    return err
}
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

// A fonte de arquivos é fs.FS, conforme aprovado.
// Cada par de arquivos é registrado com versão e ordem explícitas.
if err := migrations.AppendFiles(
    1,
    10,
    "create_users",
    migrationFiles,
    "migrations/sqlite/create_users.up.sql",
    "migrations/sqlite/create_users.down.sql",
); err != nil {
    return err
}

if err := migrations.AppendFiles(
    1,
    20,
    "create_roles",
    migrationFiles,
    "migrations/sqlite/create_roles.up.sql",
    "migrations/sqlite/create_roles.down.sql",
); err != nil {
    return err
}

if err := migrations.AppendFiles(
    2,
    10,
    "alter_users",
    migrationFiles,
    "migrations/sqlite/alter_users.up.sql",
    "migrations/sqlite/alter_users.down.sql",
); err != nil {
    return err
}
```

> **Definição aprovada:** `Append` e `AppendFiles` distinguem as duas formas de cadastro, com versão e ordem explícitas e retorno de erro. `AppendFiles` recebe `fs.FS`; `embed.FS` atende a essa interface. O consumidor embute os arquivos; o Krios os lê. Não há argumentos nomeados nem sobrecarga de funções por assinatura na API Go.

Então, chamar no início da aplicação:

```go
// Execução normal: contexto e pool configurados no cliente.
// O número informado representa a versão alvo.
// 1 é o estado desejado; a versão já aplicada deve ser lida do banco.
// sqliteClient recebe pool, dialeto e contexto através de sqliteConfig.
// Versão 1: create_users (ordem 10) e create_roles (ordem 20).
// Versão 2: alter_users (ordem 10).
if err := sqliteClient.RunMigrations(1, migrations); err != nil {
    return err
}
```

Em um banco sem migrations aplicadas, o alvo 1 executa `create_users` e depois `create_roles`; o alvo 2 executa `create_users`, `create_roles` e depois `alter_users`. Se a versão 1 já estiver integralmente aplicada, avançar ao alvo 2 executa apenas `alter_users`. Voltar da versão 2 para a versão 1 executa somente o DOWN de `alter_users`, preservando as duas migrations da versão 1, seguindo o alvo informado, sem segunda autorização.

Se uma descida incluir também a versão 1, dentro dela o DOWN de `create_roles` deve ocorrer antes do DOWN de `create_users`, desde que essas migrations tenham sido aplicadas. A versão agrupa migrations, a ordem define sua sequência dentro do grupo e o número passado a `RunMigrations` indica a versão alvo do banco. A ordenação não garante, por si só, atomicidade nem recuperação automática em caso de falha parcial.

Alternativamente, para um plano de migrations compatível com execução transacional, a aplicação cria e fornece a transação. Este exemplo substitui a chamada normal anterior:

```go
tx, err := sqliteDB.BeginTx(sqliteClient.Context(), nil)

if err != nil {
    return err
}

defer tx.Rollback()

if err := sqliteClient.RunMigrationsTx(tx, 1, migrations); err != nil {
    return err
}

return tx.Commit()
```

**Múltiplos conjuntos:** `RunMigrations(target, conjuntos...)` e `RunMigrationsTx(tx, target, conjuntos...)` retornam `error` e recebem partes de um único histórico. Todas as listas devem ter o dialeto do cliente; a aplicação escolhe quais fornecer. O Krios reúne os cadastros, valida nomes e ordens duplicados e ordena por versão/ordem. Listas omitidas não autorizam ignorar migrations já presentes no histórico.

Não oferecer módulos independentes, namespaces ou resolução automática de dependências nesta etapa. A ordem cadastrada deve respeitar as dependências. A validação considera todos os conjuntos selecionados antes de iniciar qualquer script.

O Krios deve gerenciar uma tabela chamada `migrations`, consultar o histórico aplicado e executar os passos necessários para atingir a versão alvo, respeitando as políticas aprovadas de subida, descida e recuperação.

**Definição aprovada:** em `RunMigrationsTx(tx, ...)`, tanto os scripts compatíveis quanto a leitura/escrita do histórico devem usar a transação fornecida à chamada e o contexto de `Config.Context`. A aplicação continua responsável por `Commit`/`Rollback`; o sucesso do método não representa commit. Transação nula ou encerrada deve resultar em erro, sem fallback para o pool. O método não altera a configuração nem armazena a transação no cliente ou no conjunto de migrations. Em uma chamada posterior a `RunMigrations`, o executor padrão continua sendo o pool, com o mesmo contexto configurado.

**Atomicidade:**

- Em `RunMigrations`, usar transação por migration quando o banco e todos os comandos permitirem, incluindo a atualização do histórico. O Krios gerencia somente essas transações internas; migrations anteriores já concluídas permanecem aplicadas se uma posterior falhar. Não prometer atomicidade de uma versão inteira ou do plano completo nesse modo.
- Fora dessas condições, executar com controle de falha parcial, sem prometer rollback. Scripts não podem iniciar, confirmar ou desfazer transações por conta própria.
- Em `RunMigrationsTx`, usar exclusivamente a transação recebida. Aceitar somente planos cuja compatibilidade transacional esteja estabelecida e rejeitar incompatibilidade ou incerteza antes dos scripts. Não tentar contornar restrições usando o pool ou outra transação.
- O sucesso de `RunMigrationsTx` depende do commit posterior da aplicação. DDL do MySQL com commit implícito fica excluído desse modo; operações não transacionais de outros bancos também devem ser rejeitadas. [Comandos com commit implícito no MySQL](https://dev.mysql.com/doc/refman/8.0/en/implicit-commit.html).
- A classificação dos comandos e a forma de estabelecer compatibilidade de scripts completos ainda precisam ser detalhadas por dialeto; não presumir que qualquer SQL recebido é transacional.

> **Notas técnicas:** versão alvo e estado aplicado são conceitos distintos. Os scripts precisam ser compatíveis com o banco. Um DOWN descreve a reversão de schema prevista pelo autor, mas não garante restauração de dados: por exemplo, desfazer uma tabela com `DROP TABLE` elimina seu conteúdo. A execução segue a política explícita de alvo abaixo.

**Histórico, versões aplicadas e downgrade:**

- A identidade usa `(versão, ordem)`, com nome único no histórico. Comparar os registros individuais e seu estado; a maior versão encontrada, isoladamente, não comprova que uma versão está integralmente aplicada.
- Permitir várias migrations em uma versão antes da primeira aplicação. Rejeitar scripts novos em versões já aplicadas, inclusive quando existirem versões posteriores. Novas mudanças devem usar uma versão superior à maior já distribuída/aplicada; não alterar versão, ordem, nome ou conteúdo de cadastros aplicados.
- Alvo maior exige UP; alvo menor exige DOWN. Alvo igual ao estado integralmente aplicado valida o histórico e não executa scripts. Estado incompleto ou incerto segue as regras de recuperação abaixo, sem ser confundido com uma versão concluída.
- Não exigir segunda autorização para downgrade. Antes de executar qualquer DOWN, verificar se todos os passos necessários existem e estão definidos como reversíveis. Ausência de passo ou de reversão bloqueia o plano antes do primeiro script.
- Banco mais novo que a aplicação só pode descer quando os scripts correspondentes estiverem disponíveis. Histórico desconhecido bloqueia a execução; não ignorar versões desconhecidas para prosseguir.
- Guardar checksums SHA-256 dos conteúdos UP e DOWN. Conteúdo alterado, migration desconhecida ou arquivo necessário removido gera erro e bloqueia a execução. Não atualizar checksums automaticamente para ocultar divergências; correções usam uma nova versão. Checksum verifica scripts, não mudanças manuais no schema.

**Execução exclusiva:** não permitir migrations concorrentes sobre o mesmo histórico. A exclusividade deve cobrir leitura do histórico, decisão, execução e registro; no modo com transação externa, considerar também o encerramento pela aplicação. O requisito de executor único está aprovado; a escolha entre coordenação pelo banco e garantia operacional da aplicação permanece pendente. O mecanismo, timeout, perda de conexão e eventual afinidade de sessão permanecem como detalhamento pendente; um mutex local não coordena processos diferentes.

**Falhas e recuperação:**

- Em execução não transacional, registrar estado, direção e início antes do SQL. Marcar conclusão somente depois de concluir o SQL e atualizar o histórico. Se não for possível registrar o início, não iniciar o script.
- Estado pendente/falho bloqueia novas migrations até reparo manual. Não repetir automaticamente uma migration parcial. Se o SQL concluir, mas a atualização do histórico falhar, tratar como estado incerto e exigir verificação/reparo antes de continuar.
- Em execução transacional, o histórico acompanha commit/rollback. Não prometer que um registro de falha desfeito pelo rollback será preservado. Em `RunMigrationsTx`, não gravar diagnóstico ou histórico por outra conexão para escapar dessa regra.
- Respeitar `Config.Context`, parar no primeiro erro/cancelamento e não iniciar o próximo script. Ao reiniciar após interrupção, verificar o histórico e bloquear estados incompletos/incertos.
- Ainda será detalhado o procedimento de reparo manual, inclusive para versão parcialmente concluída, resultado incerto de commit e retomada após rollback confirmado. O procedimento não pode transformar erro em sucesso silenciosamente.

**Tabela de controle:** usar `migrations` no schema/database padrão da conexão, com identificação do formato Krios e validação da estrutura antes de reutilizá-la. Registrar versão, ordem, nome, checksums UP/DOWN, estado, direção e horários; chave por `(versão, ordem)` e nome único. Tabela incompatível ou formato desconhecido gera erro; não assumir propriedade somente pelo nome nem alterar automaticamente uma tabela da aplicação. A evolução do formato da tabela tem versionamento próprio. Os nomes/tipos SQL das colunas, a identificação do formato e a representação de registros revertidos serão detalhados antes da implementação.

**Scripts e arquivos:**

- Uma migration aceita vários comandos; um único UP pode, por exemplo, criar mais de uma tabela. Isso não altera as regras de atomicidade, interrupção no primeiro erro e recuperação.
- Não separar scripts apenas por ponto e vírgula: strings e corpos de funções/triggers podem contê-lo. A estratégia de execução e os requisitos dos drivers para múltiplos comandos precisam ser especificados e testados; não modificar silenciosamente opções da conexão fornecida pela aplicação.
- `AppendFiles` recebe `fs.FS` e os caminhos UP/DOWN, enquanto `Append` recebe os conteúdos inline. Ambos usam versão, ordem e nome explícitos e participam da mesma validação de duplicatas. Não inferir versão/ordem do nome do arquivo ou da posição do cadastro.
- Formalizar os caminhos aceitos, momento da leitura, validação antecipada e comportamento de fonte/arquivo ausente. A aprovação de `fs.FS` não escolheu esses detalhes. Não estabelecer prioridade silenciosa entre cadastros inline e por arquivo com a mesma identidade.

**Inspeção e diagnóstico:** oferecer inspeção do plano que consulta o histórico sem executar scripts nem criar a tabela de controle. Documentar as permissões necessárias para o histórico e os scripts. Erros identificam versão, ordem, nome, direção e etapa, preservando a causa original. Não expor credenciais nem SQL sensível automaticamente. O nome/retorno da operação de inspeção e seus equivalentes com transação ainda serão formalizados; qualquer operação transacional deve respeitar o executor recebido.

## Detalhamentos ainda necessários

Os requisitos aprovados estão definidos nas seções correspondentes. Os seguintes aspectos ainda precisam ser especificados:

- Verificação de comunicação e versão do banco, compatibilidade real com o dialeto e escolha/fixação dos drivers de teste.
- Nomes e assinaturas dos tipos públicos de referências, associação de critérios a outra origem, operandos de `On`, correlação de subconsultas e categorias públicas de erro.
- Coleções aceitas por `IN`/`NOT IN`, expansão e elementos nulos; tipo dos argumentos de paginação, cópia do estado de navegação e comportamento nos limites quando os dados mudarem.
- Mecanismo de exclusividade das migrations, classificação transacional de scripts, execução de múltiplos comandos e requisitos de drivers.
- Formato físico/evolução da tabela de controle, reversões, reparo de falhas e versões parciais; leitura/validação de arquivos, recuperação de conjuntos inválidos e assinatura de inspeção do plano.

Esses pontos devem ser resolvidos na especificação das tarefas, sem reabrir as decisões já aprovadas nem presumir novas políticas. O planejamento e as perguntas formais continuam em `PLANNING.md`, nos arquivos numerados e em `QUESTIONS.md`, conforme as regras do projeto. Este documento especifica o contrato da biblioteca.
