# Definição

Vamos criar uma biblioteca Go para manipulação de bancos de dados SQL, chamada Krios.

## Requisitos

- Usar `database/sql` e a biblioteca padrão do Go; o utilizador instala e registra o driver.
- Suportar PostgreSQL 16+, MySQL 8.0+ e SQLite 3.39+, detectando automaticamente as capacidades do banco conectado.
- Trabalhar com instâncias `DB` e `Tx`, sem conexão, configuração ou transação global.
- Executar SELECT, INSERT, UPDATE, DELETE e SQL direto por uma transação explícita, com `context.Context`.
- Disponibilizar builders imutáveis: cada método retorna uma nova versão, que deve ser reatribuída ou encadeada.
- Permitir SQL textual e arquivos acessíveis por `fs.FS`, incluindo `embed.FS`.
- Gerenciar versões do banco por migrações Up/Down.
- Separar identificadores, valores, operadores e subconsultas em uma árvore de expressões (AST).
- Parametrizar valores e escapar identificadores conforme o dialeto; não interpretar nomes de colunas como SQL.
- Retornar `ErrUnsupported` antes da execução de funcionalidades sem suporte no banco/versão.

Os exemplos são trechos de uso da API proposta. `ctx` é um `context.Context`; `db` e `tx` são instâncias abertas nas seções correspondentes. Trechos com `return err` pressupõem uma função que retorna `error`.

## Configurações

```go
cfg := krios.Config{
    Dialect: krios.Postgres, // Postgres ("pg"), MySQL ("mysql") ou SQLite ("sqlite")
    Driver:  "pgx",         // exemplo: driver registrado pelo utilizador
    DSN:     "postgres://user:password@localhost:5432/app?sslmode=disable",
    Timeout: 5 * time.Second,
}
```

`Dialect` define a geração de SQL; `Driver` identifica o driver registrado em `database/sql`; `DSN` contém os parâmetros de conexão no formato aceito pelo driver. Para SQLite, o caminho do arquivo integra o DSN.

A versão e as capacidades são detectadas automaticamente na inicialização, inclusive em `Wrap` e na conexão usada por migrações. Falha na detecção ou interpretação da versão retorna `ErrVersionDetection`, preservando a causa, e impede o uso da instância; não há fallback silencioso. Os builders usam as capacidades identificadas ao validar `Build` e executar comandos. Versões abaixo dos mínimos retornam `ErrUnsupported`.

`Timeout` limita a abertura/verificação da conexão: negativo retorna `ErrInvalidConfig`; zero mantém o prazo de `ctx`; positivo aplica `context.WithTimeout`, respeitando eventual prazo menor do contexto. Ping e detecção compartilham o mesmo prazo, sem reiniciá-lo entre operações. `sql.Open` não recebe contexto e não possui garantia de interrupção por esse prazo. O prazo de cada comando é definido pelo contexto da operação.

Dialeto desconhecido, driver vazio, timeout negativo, contexto nulo e pool nulo em `Wrap` retornam `ErrInvalidConfig`. O DSN é repassado sem alteração, inclusive se vazio; sua interpretação cabe ao driver.

Para migrações, a aplicação incorpora os arquivos em uma variável de pacote:

```go
import "embed"

//go:embed migrations/*.sql
var migrationFiles embed.FS
```

```go
migrationCfg := krios.MigrationConfig{
    TargetVersion: 1,
    Files:         migrationFiles, // fs.FS
    Migrations: []krios.Migration{
        {
            Version: 1,
            Up:      []string{"migrations/001_create_users.up.sql"},
            Down:    []string{"migrations/001_create_users.down.sql"},
        },
    },
    AllowNonAtomic: false,
}

if err := krios.Migrate(ctx, cfg, migrationCfg); err != nil {
    return err
}
```

Cada arquivo de migração contém uma instrução SQL; `Up` e `Down` podem listar vários arquivos, executados na ordem declarada. A biblioteca lê os arquivos com `fs.ReadFile`, sem dividir o conteúdo por `;` ou traduzir SQL entre dialetos.

A tabela `migrations` registra `version` (chave primária), `checksum` e `applied_at`. O checksum é SHA-256 da versão, caminhos e conteúdos Up/Down, com campos delimitados por comprimento.

- Versões devem ser positivas e únicas; `TargetVersion` é `int64`, não negativo.
- Destino zero desfaz todas as versões; outro destino deve existir no manifesto.
- O histórico aplicado deve corresponder a um prefixo do manifesto ordenado por versão.
- Versões pendentes até o destino executam Up em ordem crescente; versões aplicadas acima do destino executam Down em ordem decrescente.
- Versão desconhecida, checksum divergente ou Down necessário ausente provoca erro antes de executar scripts.
- Up concluído insere o registro; Down concluído remove o registro.

`Migrate` abre seu próprio pool, adquire exclusividade, valida o histórico, executa as etapas, libera a exclusividade e fecha o pool, inclusive em caso de erro. O bloqueio cobre toda a operação: lock de sessão PostgreSQL/MySQL em conexão fixada com `sql.Conn`; transação de escrita no SQLite.

No modo atômico, Krios executa as etapas e o histórico na mesma transação. O utilizador é responsável por garantir que as instruções sejam compatíveis com essa atomicidade; Krios não analisa SQL arbitrário para certificar essa garantia. Se o banco retornar erro, Krios tenta rollback e retorna o erro original, independentemente do resultado do rollback. Não há garantia de desfazer efeitos de instruções incompatíveis ou com commit implícito. Para scripts deliberadamente não atômicos, o utilizador deve selecionar `AllowNonAtomic: true`.

O modo não atômico exige `AllowNonAtomic: true`. Antes de cada etapa, persiste em `migration_state` a versão, direção, índice e indicador `dirty`. O estado só é limpo após confirmação da execução e atualização do histórico. Estado `dirty` bloqueia novas migrações até recuperação manual; instruções com resultado incerto não são repetidas automaticamente.

## Abrir o banco

```go
db, err := krios.Open(ctx, cfg)
if err != nil {
    return err
}
defer db.Close()
```

`Open(ctx, cfg) (*DB, error)` valida a configuração, chama `sql.Open` e verifica a conexão com `PingContext`. Depois do ping, detecta versão/capacidades. Se a inicialização falhar, fecha o pool criado e preserva a causa do erro.

Também será permitido usar um pool existente:

```go
db, err := krios.Wrap(ctx, existingSQLDB, krios.Postgres)
if err != nil {
    return err
}
defer db.Close()
```

`Wrap(ctx, pool, dialect)` recebe um `*sql.DB` e não assume sua propriedade. Verifica conectividade e versão/capacidades usando o contexto fornecido, sem criar timeout adicional ou reconfigurar o pool. Em falha, não fecha o pool externo. `Close` retorna `ErrActiveTransactions` informando a existência de transações ativas e não fecha nem invalida a instância nesse caso: todas devem terminar antes do fechamento. Essa regra vale tanto para `Open` quanto para `Wrap`. Quando não há transações ativas, o fechamento é idempotente e invalida a instância Krios; fecha o pool somente quando criado por `Open`. O utilizador continua responsável pelo pool fornecido a `Wrap`, inclusive por transações criadas diretamente nele, fora do controle do Krios.

`DB` pode ser compartilhado entre goroutines. Novas operações em uma instância fechada retornam `ErrClosed`.

## Contexto

O contexto controla cancelamento e prazos; a transação controla atomicidade. `Begin(ctx, opts) (*Tx, error)` recebe `*sql.TxOptions`; `nil` utiliza as opções padrão de `database/sql`.

```go
tx, err := db.Begin(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

insert := tx.Insert(krios.Table("users")).
    Columns("name").
    Values("John")

if _, err := insert.Exec(ctx); err != nil {
    return err
}

update := tx.Update(krios.Table("users")).
    Set("active", true).
    Where(krios.Eq("name", "John"))

if _, err := update.Exec(ctx); err != nil {
    return err
}

return tx.Commit()
```

O contexto de `Begin` define a vida da transação. Cada execução recebe um contexto para cancelamento/prazo local. A API inicial não permite uso concorrente de uma mesma `Tx`.

Após falha, o chamador deve abandonar a unidade de trabalho e executar rollback. Após `Commit` ou `Rollback`, novas execuções retornam `sql.ErrTxDone`; o erro do rollback adiado após commit é esperado.

Builders pertencem à transação que os criou. Seus métodos copiam as estruturas internas modificadas, preservando versões anteriores. Todos oferecem `Build() (string, []any, error)` para gerar SQL e argumentos sem executar.

## Cláusula Where

```go
where := krios.Where(
    krios.Eq("active", true),
    krios.Or(
        krios.Eq("id", 1),
        krios.Eq("name", "John"),
    ),
)

query := tx.Select(krios.Table("users")).Where(where)
```

`Where` agrupa predicados com AND implícito e retorna um predicado reutilizável. Chamadas sucessivas de `.Where(...)` também são combinadas por AND. Grupos explícitos preservam seus parênteses.

SELECT pode omitir WHERE. UPDATE e DELETE exigem uma condição não vazia e rejeitam predicados constantes verdadeiros após simplificação da AST, incluindo `NotIn` vazio. Não haverá opção de atualização/exclusão global nos builders iniciais; SQL direto permite essa operação explicitamente.

## Operadores

```go
predicates := []krios.Predicate{
    krios.Eq("id", 1),
    krios.Ne("id", 1),
    krios.Lt("id", 10),
    krios.Lte("id", 10),
    krios.Gt("id", 1),
    krios.Gte("id", 1),
    krios.In("id", 1, 2, 3),
    krios.NotIn("id", 1, 2, 3),
    krios.Like("name", "%John%"),
    krios.NotLike("name", "%John%"),
    krios.IsNull("deleted_at"),
    krios.IsNotNull("created_at"),
    krios.Between("id", 1, 10),
    krios.NotBetween("id", 1, 10),
}
```

- Comparações recebem valores `any` compatíveis com `database/sql`, incluindo números, strings, `time.Time`, `[]byte`, `sql.Null*` e `driver.Valuer`.
- Valores seguem para o driver como parâmetros, sem interpolação ou conversão textual de datas.
- Valores nulos são aceitos e enviados ao banco como NULL, inclusive `nil`, ponteiros nulos, `sql.Null*` inválidos e valores NULL produzidos por `driver.Valuer`. Krios não converte comparações para `IS NULL`/`IS NOT NULL`: `Eq` usa `=` e `Ne` usa `<>`, com o valor parametrizado. Para testar nulidade, o utilizador deve escolher `IsNull`/`IsNotNull`.
- Krios não corrige a semântica SQL escolhida pelo utilizador. Erros de conversão do driver ou de execução do banco são retornados; uma comparação com NULL pode não corresponder a linhas, sem produzir erro.
- `In("col")` representa falso; `NotIn("col")` representa verdadeiro. Listas não vazias aceitam itens NULL e preservam a semântica SQL de IN/NOT IN, sem reescrita para testes de nulidade.
- `Between` e `NotBetween` recebem exatamente dois limites; `Like` e `NotLike` recebem padrão textual.
- Comparar coluna com coluna exige expressões explícitas; uma string passada a `Eq` sempre é valor.

```go
joinCondition := krios.EqExpr(
    krios.Column("u.id"),
    krios.Column("p.user_id"),
)

subquery := tx.Select(krios.Table("orders").As("o")).
    Columns(krios.Column("o.user_id")).
    Where(krios.EqExpr(krios.Column("o.user_id"), krios.Column("u.id")))

exists := krios.Exists(subquery)
notExists := krios.NotExists(subquery)
```

`Exists` e `NotExists` recebem subconsultas, não nomes de colunas. As variantes `EqExpr`, `NeExpr`, `LtExpr`, `LteExpr`, `GtExpr` e `GteExpr` comparam duas expressões; `Value(v)` representa um parâmetro nessa API. Expressões e predicados pertencem à AST controlada pelo pacote.

## Operadores lógicos

AND é o padrão. `And` e `Or` permitem agrupamentos aninhados; grupos vazios, inclusive `Where()` e `On()`, retornam erro de validação ao compilar/executar.

```go
condition := krios.And(
    krios.Eq("active", true),
    krios.Or(
        krios.Eq("id", 1),
        krios.And(
            krios.Eq("name", "John"),
            krios.IsNull("deleted_at"),
        ),
    ),
)
```

### Agregação

- `krios.Column("col")`
- `krios.Count("*")` e `krios.Count("col")`
- `krios.Avg("col")`
- `krios.Sum("col")`
- `krios.Min("col")`
- `krios.Max("col")`

Expressões de seleção, incluindo `Column("name").As("user_name")`, aceitam `.As("alias")`. Filtros sobre agregações usam `Having`, com as mesmas regras de agrupamento de WHERE.

## Select

```go
// Alias e schema são opcionais e configurados explicitamente.
table := krios.Table("users").As("u").InSchema("public")
query := tx.Select(table)

query = query.Distinct()
query = query.All() // SELECT *; padrão quando Columns não foi informado

query = query.Columns(
    krios.Column("u.id"),
    krios.Column("u.name"),
)

query = query.OrderBy(
    krios.Asc("u.name"),
    krios.Desc("u.id"),
)

query = query.Where(
    krios.Eq("u.active", true),
    krios.Or(
        krios.Eq("u.id", 1),
        krios.Eq("u.name", "John"),
    ),
)
```

`All` e `Columns` de SELECT substituem a projeção anterior. `OrderBy` e `GroupBy` acumulam itens, respeitando a ordem das chamadas; `Having` acumula por AND. `Returning`, nos builders de escrita, também acumula colunas na ordem informada. `Schema` representa schema no PostgreSQL, database no MySQL e banco anexado no SQLite. Identificadores qualificados são escapados por componente; `*` é aceito somente nas posições apropriadas.

JOINs usam `TableRef` e `On`, que segue as regras de predicados de WHERE:

```go
orders := krios.Table("orders").As("o")
on := krios.On(krios.EqExpr(krios.Column("u.id"), krios.Column("o.user_id")))

// Alternativas independentes: escolher o JOIN desejado.
joined := query.Join(orders, on) // equivalente a InnerJoin
joined = query.InnerJoin(orders, on)
joined = query.LeftJoin(orders, on)
joined = query.RightJoin(orders, on)
joined = query.FullJoin(orders, on)
```

Cada JOIN é validado conforme o banco/versão; não existe emulação silenciosa. Agregações podem ser construídas separadamente:

```go
grouped := tx.Select(krios.Table("orders")).
    Columns(krios.Column("user_id"), krios.Sum("total").As("amount")).
    GroupBy("user_id").
    Having(krios.GtExpr(krios.Sum("total"), krios.Value(100)))
```

Paginação recebe offset e limite, nessa ordem; offset deve ser não negativo e limite positivo. Não representa número da página.

```go
paged := query.Paginate(0, 10)
rows, page, err := paged.ExecPage(ctx)
if err != nil {
    return err
}
defer rows.Close()
// page.Offset, page.Limit e page.Total são int64.
```

`ExecPage` exige paginação e executa a contagem antes da consulta. A consistência entre total e linhas depende do isolamento escolhido em `Begin`; Krios não altera esse isolamento implicitamente.

### Resultado

Os métodos têm retornos fixos:

| Método          | Retorno                    | Comportamento                                                                 |
| --------------- | -------------------------- | ----------------------------------------------------------------------------- |
| `Exec(ctx)`     | `(*sql.Rows, error)`       | Aplica paginação, se configurada; caso contrário, retorna todos os registros. |
| `ExecPage(ctx)` | `(*sql.Rows, Page, error)` | Retorna linhas e metadados; exige `Paginate`.                                 |
| `Count(ctx)`    | `(int64, error)`           | Conta os resultados sem ordenação/paginação.                                  |
| `First(ctx)`    | `(*sql.Row, error)`        | Exige ordenação, rejeita paginação prévia e aplica limite 1.                  |
| `Last(ctx)`     | `(*sql.Row, error)`        | Mesmas exigências de First, invertendo a ordenação.                           |

`Count` envolve a consulta em `SELECT COUNT(*) FROM (...) AS krios_count`, removendo ORDER BY/LIMIT/OFFSET e preservando WHERE, JOIN, DISTINCT, GROUP BY e HAVING.

`Last` inverte todas as direções e a posição dos NULLs, inclusive resolvendo o padrão do dialeto quando não explícito. O utilizador deve fornecer desempate único para obter ordem determinística. Ordenação explícita de NULLs usa `Asc/Desc(...).NullsFirst()` ou `.NullsLast()`; capacidades não suportadas retornam `ErrUnsupported`.

O resultado usa `Scan` nativo:

```go
rows, err := query.Exec(ctx)
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
}
if err := rows.Err(); err != nil {
    return err
}
if err := rows.Close(); err != nil {
    return err
}
```

Fechar as linhas e verificar erros antes de confirmar a transação. Em `First` e `Last`, o retorno `error` informa falhas de construção; erros de consulta e ausência de registro são retornados por `Row.Scan`, incluindo `sql.ErrNoRows`.

## Insert

```go
insert := tx.Insert(krios.Table("users").InSchema("public"))
insert = insert.Columns("name", "active")
insert = insert.Values("John", true)

result, err := insert.Exec(ctx)
if err != nil {
    return err
}
```

Schema é opcional, como em SELECT. `Columns` acumula colunas na ordem das chamadas. Cada `Values` acrescenta uma linha; todas as linhas devem ter a quantidade de valores correspondente ao total de colunas. Configuração inválida retorna `ErrInvalidQuery` antes da execução.

### Resultado

`Exec` retorna `sql.Result`. `RowsAffected()` informa a quantidade de linhas afetadas; `LastInsertId()` pode informar o ID gerado quando suportado pelo driver, e seu erro deve ser tratado. Krios não infere a coluna identificadora.

Quando o banco/versão suporta retorno de colunas, usar uma execução alternativa:

```go
rows, err := insert.Returning("id").Query(ctx)
if err != nil {
    return err
}
defer rows.Close()
// Consumir com Next/Scan e verificar Err.
```

`Query(ctx)` retorna `(*sql.Rows, error)` e exige `Returning`. `Exec` com `Returning` configurado retorna `ErrInvalidQuery`. Banco/versão sem suporte retorna `ErrUnsupported`.

## Update

```go
update := tx.Update(krios.Table("users").InSchema("public"))
update = update.Set("name", "John")
update = update.Set("active", true)
update = update.Where(krios.Eq("id", 1))

result, err := update.Exec(ctx)
if err != nil {
    return err
}
```

É obrigatório informar ao menos uma atribuição e um WHERE válido. `Set` repetido para a mesma coluna substitui o valor anterior. WHERE ausente retorna `ErrMissingWhere`; grupos vazios, como `Where(And())`, e predicados inválidos retornam `ErrInvalidQuery`. A proteção contra condição constante verdadeira permanece. Se ocorrerem simultaneamente condições de `ErrMissingWhere` e `ErrInvalidQuery`, prevalece `ErrMissingWhere`.

### Resultado

`Exec` retorna `sql.Result`; usar `RowsAffected()`. Não existe ID do último registro alterado. Para obter IDs ou outras colunas, usar `Returning(...).Query(ctx)`, com o mesmo contrato de INSERT e sujeito ao suporte do banco/versão.

## Delete

```go
deletion := tx.Delete(krios.Table("users").InSchema("public"))
deletion = deletion.Where(krios.Eq("id", 1))

result, err := deletion.Exec(ctx)
if err != nil {
    return err
}
```

WHERE é obrigatório e segue as mesmas validações de UPDATE.

### Resultado

`Exec` retorna `sql.Result`, permitindo verificar `RowsAffected()`. Quando o resultado não for necessário, usar `_, err := deletion.Exec(ctx)`. Retorno de colunas usa `Returning(...).Query(ctx)`, sujeito às mesmas regras de suporte.

## Execução de queries embed

A aplicação incorpora os arquivos e fornece um `fs.FS`; o nome do arquivo sozinho não permite acessar o conteúdo incorporado pelo utilizador.

```go
import "embed"

//go:embed queries/*.sql
var queryFiles embed.FS
```

SQL direto e arquivos usam placeholders nativos: `$1`, `$2`, ... no PostgreSQL; `?` no MySQL/SQLite. Builders geram esses placeholders automaticamente, com numeração compartilhada entre consulta e subconsultas no PostgreSQL. Arquivos e SQL direto não são reescritos.

### Select

```go
// queries/users_by_name.sql, neste exemplo PostgreSQL:
// SELECT id, name FROM users WHERE name = $1 ORDER BY id
rows, err := tx.QueryFile(ctx, queryFiles, "queries/users_by_name.sql", "John")
if err != nil {
    return err
}
defer rows.Close()
```

Também é permitido executar SQL textual diretamente:

```go
rows, err := tx.Query(ctx, "SELECT id, name FROM users WHERE name = $1", "John")
if err != nil {
    return err
}
defer rows.Close()
```

### Resultado

- `Query` e `QueryFile` retornam `(*sql.Rows, error)`; consumir com `Next/Scan`, verificar `Err` e fechar as linhas.
- Não há detecção automática de paginação, tipos de comando ou esquema do resultado.
- Paginação em SQL direto exige LIMIT/OFFSET no SQL e consulta de contagem explícita da aplicação.

### Insert, Delete e Update

```go
// queries/update_user.sql, neste exemplo PostgreSQL:
// UPDATE users SET name = $1 WHERE id = $2
result, err := tx.ExecFile(ctx, queryFiles, "queries/update_user.sql", "John", 1)
if err != nil {
    return err
}
```

```go
result, err := tx.Exec(ctx, "DELETE FROM users WHERE id = $1", 1)
if err != nil {
    return err
}
```

Os métodos aceitam `args ...any`; valores são enviados separadamente do texto SQL. A validação obrigatória de WHERE pertence aos builders; SQL direto é responsabilidade do utilizador. Arquivos não são divididos por `;`; múltiplas instruções dependem do driver.

### Resultado

- `Exec` e `ExecFile` retornam `(sql.Result, error)` para INSERT, UPDATE e DELETE.
- Usar `RowsAffected()` e, para INSERT quando suportado, `LastInsertId()`.
- SQL que retorna colunas, incluindo escritas com `RETURNING`, deve usar `Query` ou `QueryFile`.
- Não há inferência automática de IDs; resultados dispensáveis podem ser ignorados com `_`.

## Erros e validação

- Erros sentinela: `ErrInvalidConfig`, `ErrInvalidQuery`, `ErrMissingWhere`, `ErrUnsupported`, `ErrClosed`, `ErrMigrationDirty`, `ErrActiveTransactions` e `ErrVersionDetection`.
- Preservar causas com `%w` e permitir `errors.Is/As`, incluindo erros de contexto e `sql.ErrTxDone`.
- Mensagens produzidas pela biblioteca e logs não devem incluir DSN nem valores de parâmetros; erros do driver preservados podem conter detalhes fornecidos pelo próprio driver.
- Validar compilação por dialeto: placeholders, identificadores, argumentos, grupos, NULL, IN vazio e comparação entre colunas.
- Validar integração nos três bancos: commit/rollback, cancelamento, paginação, retornos, concorrência e falhas de migração.
- Manter drivers nos testes de integração e executar `go test -race`.

### Momento e prioridade dos erros

| Condição | Retorno / momento |
| --- | --- |
| Configuração inválida, incluindo timeout negativo | `ErrInvalidConfig` na inicialização. |
| Detecção automática falha ou versão não pode ser interpretada | `ErrVersionDetection`, preservando a causa; nenhuma instância utilizável. |
| Versão inferior à mínima | `ErrUnsupported` na inicialização. |
| Fechar instância com transações Krios ativas | `ErrActiveTransactions`; instância permanece aberta. |
| Operação em `DB` fechado / execução em `Tx` encerrada | `ErrClosed` / `sql.ErrTxDone`. |
| UPDATE/DELETE sem WHERE ou com condição constante verdadeira | `ErrMissingWhere` na validação do builder. |
| Grupo/predicado vazio ou inválido, coluna vazia, quantidade de valores inválida | `ErrInvalidQuery` em `Build` ou antes da execução. |
| `ErrMissingWhere` e `ErrInvalidQuery` simultâneos | Prevalece `ErrMissingWhere`. |
| Recurso de builder sem suporte | `ErrUnsupported` em `Build` ou antes da execução. |
| Escrita com `Returning` executada por `Exec`, ou `Query` sem `Returning` | `ErrInvalidQuery` antes da execução. |
| Falha de consulta em `First/Last`, incluindo ausência de registro | Erro em `Scan`; ausência retorna `sql.ErrNoRows`. |
| Falha durante iteração de `Rows` | Erro em `Scan` ou `Rows.Err`; o utilizador fecha as linhas. |
| Falha do driver/banco ou cancelamento | Preservar a causa; não corrigir o SQL escolhido pelo utilizador. |
| Migração não atômica com estado `dirty` | `ErrMigrationDirty`; requer recuperação manual. |

## Compatibilidade por banco

Mínimos escolhidos pelo utilizador: PostgreSQL 16, MySQL 8.0 e SQLite 3.39. A matriz descreve esses mínimos; a detecção automática deve considerar os recursos da versão efetivamente conectada, sem presumir recursos futuros.

| Recurso | PostgreSQL 16 | MySQL 8.0 | SQLite 3.39 |
| --- | --- | --- | --- |
| INNER / LEFT / RIGHT JOIN | Sim | Sim | Sim |
| FULL JOIN | Sim | Não nativo | Sim |
| `NULLS FIRST/LAST` explícito | Sim | Não nativo | Sim |
| INSERT/UPDATE/DELETE com `RETURNING` | Sim | Não nativo | Sim, com restrições do SQLite |
| Qualificação de tabela | Schema | Database | Banco principal/anexado |
| Placeholders dos builders | `$1`, `$2`, ... | `?` | `?` |

JOINs e ordenação: [PostgreSQL SELECT](https://www.postgresql.org/docs/16/sql-select.html), [MySQL JOIN](https://dev.mysql.com/doc/refman/8.0/en/join.html), [MySQL SELECT](https://dev.mysql.com/doc/refman/8.0/en/select.html), [SQLite 3.39](https://www.sqlite.org/releaselog/3_39_0.html) e [SQLite SELECT](https://www.sqlite.org/lang_select.html).

Retorno de escritas: [PostgreSQL INSERT](https://www.postgresql.org/docs/16/sql-insert.html), [UPDATE](https://www.postgresql.org/docs/16/sql-update.html) e [DELETE](https://www.postgresql.org/docs/16/sql-delete.html); sintaxes MySQL [INSERT](https://dev.mysql.com/doc/refman/8.0/en/insert.html), [UPDATE](https://dev.mysql.com/doc/refman/8.0/en/update.html) e [DELETE](https://dev.mysql.com/doc/refman/8.0/en/delete.html); [SQLite RETURNING](https://www.sqlite.org/lang_returning.html). No SQLite, UPDATE/DELETE de tabelas virtuais não suportam RETURNING; a ordem das linhas retornadas não é garantida. Limitações dependentes do objeto podem ser informadas pelo banco na execução.

`Last` inverte a ordenação sem emular recursos indisponíveis. No MySQL, inverter ASC/DESC também inverte a posição padrão dos NULLs; solicitar `NullsFirst/NullsLast` explicitamente não ganha emulação automática.

Atomicidade não é uma capacidade universal de qualquer script: depende das instruções e do banco. DDL MySQL pode realizar commit implícito; SQLite permite transações explícitas, com as restrições documentadas. O utilizador deve escolher scripts compatíveis com o modo selecionado. Fontes: [MySQL commits implícitos](https://dev.mysql.com/doc/refman/8.0/en/implicit-commit.html) e [SQLite transações](https://www.sqlite.org/lang_transaction.html).

## Catálogo dos contratos públicos

Assinaturas documentais; não representam implementação. Os nomes `DB`, `Tx` e builders referem-se às instâncias descritas neste documento. A representação interna dos tipos pertence à implementação das etapas seguintes.

```go
type Dialect string // Postgres = "pg"; MySQL = "mysql"; SQLite = "sqlite"
type Config struct {
    Dialect Dialect
    Driver  string
    DSN     string
    Timeout time.Duration
}
type Page struct { Offset, Limit, Total int64 }
type Migration struct {
    Version int64
    Up      []string
    Down    []string
}
type MigrationConfig struct {
    TargetVersion  int64
    Files          fs.FS
    Migrations     []Migration
    AllowNonAtomic bool
}

func Open(ctx context.Context, cfg Config) (*DB, error)
func Wrap(ctx context.Context, pool *sql.DB, dialect Dialect) (*DB, error)
func (db *DB) Close() error
func (db *DB) Begin(ctx context.Context, opts *sql.TxOptions) (*Tx, error)
func (tx *Tx) Commit() error
func (tx *Tx) Rollback() error
func Migrate(ctx context.Context, cfg Config, migrations MigrationConfig) error

func (tx *Tx) Query(ctx context.Context, sqlText string, args ...any) (*sql.Rows, error)
func (tx *Tx) Exec(ctx context.Context, sqlText string, args ...any) (sql.Result, error)
func (tx *Tx) QueryFile(ctx context.Context, files fs.FS, path string, args ...any) (*sql.Rows, error)
func (tx *Tx) ExecFile(ctx context.Context, files fs.FS, path string, args ...any) (sql.Result, error)

func (tx *Tx) Select(table TableRef) SelectBuilder
func (tx *Tx) Insert(table TableRef) InsertBuilder
func (tx *Tx) Update(table TableRef) UpdateBuilder
func (tx *Tx) Delete(table TableRef) DeleteBuilder

func (q SelectBuilder) Build() (string, []any, error)
func (q SelectBuilder) Exec(ctx context.Context) (*sql.Rows, error)
func (q SelectBuilder) ExecPage(ctx context.Context) (*sql.Rows, Page, error)
func (q SelectBuilder) Count(ctx context.Context) (int64, error)
func (q SelectBuilder) First(ctx context.Context) (*sql.Row, error)
func (q SelectBuilder) Last(ctx context.Context) (*sql.Row, error)
```

Cada builder de escrita oferece `Build() (string, []any, error)`, `Exec(ctx context.Context) (sql.Result, error)` e `Query(ctx context.Context) (*sql.Rows, error)`. Todos os modificadores retornam o mesmo tipo de builder, com estado independente.

| Construção / modificador | Argumentos | Retorno e composição |
| --- | --- | --- |
| `Table` | Nome `string` | `TableRef`; aceita `As(alias string)` e `InSchema(schema string)`, retornando `TableRef`. |
| `Column` | Nome `string` | Expressão `Expr` com `As(alias string)`, retornando expressão com alias. |
| `Count`, `Avg`, `Sum`, `Min`, `Max` | Coluna `string`; `Count` também aceita `"*"` | Expressão de agregação com `As(alias string)`. |
| `Value` | `any` | Expressão de parâmetro. |
| `Eq`, `Ne`, `Lt`, `Lte`, `Gt`, `Gte` | Coluna `string`, valor `any` | `Predicate`. |
| Variantes `EqExpr`, `NeExpr`, `LtExpr`, `LteExpr`, `GtExpr`, `GteExpr` | Duas expressões `Expr` | `Predicate`. |
| `In`, `NotIn` | Coluna `string`, valores `...any` | `Predicate`. |
| `Like`, `NotLike` | Coluna e padrão `string` | `Predicate`. |
| `Between`, `NotBetween` | Coluna `string`, dois limites `any` | `Predicate`. |
| `IsNull`, `IsNotNull` | Coluna `string` | `Predicate`. |
| `Exists`, `NotExists` | `SelectBuilder` | `Predicate` de subconsulta. |
| `And`, `Or`, `Where`, `On` | `...Predicate` | `Predicate` agrupado; WHERE/ON usam AND. |
| `Asc`, `Desc` | Coluna `string` | Termo de ordenação; `NullsFirst()` e `NullsLast()` retornam o termo configurado. |
| SELECT `Distinct`, `All` | Sem argumentos | `SelectBuilder`. |
| SELECT `Columns` | `...Expr` | `SelectBuilder`; substitui projeção. |
| SELECT `Where`, `Having` | `...Predicate` | `SelectBuilder`; acumula por AND. |
| SELECT `OrderBy` | Termos retornados por `Asc/Desc`, variádicos | `SelectBuilder`; acumula em ordem. |
| SELECT `GroupBy` | Colunas `...string` | `SelectBuilder`; acumula em ordem. |
| SELECT `Paginate` | Offset e limite `int64` | `SelectBuilder` com paginação. |
| SELECT `Join`, `InnerJoin`, `LeftJoin`, `RightJoin`, `FullJoin` | `TableRef`, condição `Predicate` produzida por `On` | `SelectBuilder` com JOIN. |
| INSERT `Columns` | `...string` | `InsertBuilder`; acumula em ordem. |
| INSERT `Values` | `...any` | `InsertBuilder`; acrescenta uma linha. |
| UPDATE `Set` | Coluna `string`, valor `any` | `UpdateBuilder`; substitui valor da mesma coluna. |
| UPDATE/DELETE `Where` | `...Predicate` | Respectivo builder; acumula por AND. |
| INSERT/UPDATE/DELETE `Returning` | Colunas `...string` | Respectivo builder; acumula em ordem. |

O catálogo descreve os tipos de entrada e composição sem impor nomes novos aos tipos concretos de expressão/ordenação. `Column(...).As(...)` deve continuar aceito por `Columns`, mantendo o contrato de expressão.
