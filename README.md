# Krios

Biblioteca Go em desenvolvimento para acesso a SQLite3, MySQL e PostgreSQL por `database/sql`.

Implementados: configuração em código, abertura/fechamento, SQL parametrizado, transações explícitas, mapeamento de entidades por reflexão sobre structs (`krios.EntityOf`, `Row.Scan` para um registro e `Rows.All` para múltiplos), consultas `SELECT` encadeadas com seleção de colunas, filtros, junções, agregações, ordenação e paginação (`DB.Select`/`Tx.Select`, `Only`, `Where`, `Join`, `Fields`, `GroupBy`, `OrderBy`, `Limit`/`Offset`, `Count`, `Paginate`, `Build`), escritas encadeadas (`DB.Insert`/`Update`/`Delete` e equivalentes em `Tx`) e execução de SQL incorporado (`krios.NewSQLFile` com `DB.File`/`Tx.File`). Migrações versionadas estão disponíveis (`Config.Migrations` e `DB.RunMigrations`; a preparação consulta o histórico sem executar DDL e o plano pode ser inspecionado). A integração automatizada real usa SQLite; MySQL/PostgreSQL têm testes locais de regras/DSN/geração de SQL e aguardam integração manual.

## Desenvolvimento local

- Go mínimo: 1.26.0; matriz prevista: linhas 1.26 e 1.27.
- `make` é opcional. A primeira execução baixa as dependências de teste fixadas em `go.mod`/`go.sum`.
- Testes normais usam diretórios temporários; não acessam `dev.db` nem exigem servidores externos.

| Comando | Finalidade |
| --- | --- |
| `make fmt` | Formatar o código |
| `make fmt-check` | Rejeitar arquivos fora do padrão, sem alterá-los |
| `make build` | `go build ./...` |
| `make vet` | `go vet ./...` |
| `make test` | `go test ./...` |
| `make check` ou `make` | Formatação, compilação, análise estática e testes |
| `make race` | `CGO_ENABLED=1 go test -race -count=1 ./...` |
| `make dev-db` | Criar/abrir o banco local, preservando dados existentes |

Para selecionar outra instalação: `make check GO=/caminho/go/bin/go`. O alvo `race` exige compilador C no Linux, embora o driver SQLite continue sendo escrito em Go. Os testes com detector exercitam somente os caminhos executados, não provam ausência de todas as corridas possíveis. Ver [documentação do detector](https://go.dev/doc/articles/race_detector).

## Uso da base implementada

O consumidor importa seu driver e preenche `Config` em código:

```go
import (
    "context"
    "github.com/mineot/krios"
    _ "modernc.org/sqlite"
)

// Dentro de uma função que retorna error:
db, err := krios.Open(context.Background(), krios.Config{
    Type: krios.DBTypeSQLite,
    Driver: "sqlite",
    SQLite: &krios.SQLiteConfig{DBPath: "app.db"},
})
if err != nil { return err }
defer db.Close()
```

O nome de registro e o formato do DSN devem ser compatíveis com o driver escolhido. MySQL usa o formato de `go-sql-driver/mysql`; PostgreSQL usa URL compatível com `pgx`. Configuração e validações estão em [E02-P01](.specs/E02-P01_decisions.md). Portas explícitas devem estar em 1–65535; zero aplica o padrão. Timeout zero aplica 10 segundos.

`Open` inclui preparação e ping no orçamento temporal. Interromper uma chamada bloqueante depende do driver; o Krios não simula cancelamento abandonando recursos em goroutines.

Os [exemplos executáveis](tests/examples_test.go) demonstram abertura, consulta, `Rows.Err`, transações, commit bloqueado e rollback após erro de leitura. Execute `go test -run '^Example' -v ./tests`. Cada exemplo cria seu próprio banco temporário.

## Entidades e mapeamento

Estruturas Go descrevem tabelas e colunas; tags `krios` nomeiam as colunas:

```go
type User struct {
    ID   int64   `krios:"id,pk"`
    Name string  `krios:"name"`
    Bio  *string `krios:"bio"`
}

func (User) TableName() string { return "users" }
```

- Nome da tabela: `snake_case` do tipo, sobrescrito por um método `TableName() string` (por valor ou ponteiro).
- Campo sem tag usa `snake_case` do nome (`UserID` → `user_id`); `krios:"-"` ignora o campo; `pk` marca a chave primária (pode ser composta).
- `krios.EntityOf[User]()` devolve o descritor `*krios.Entity` (`Table`, `Columns`, `PrimaryKeys`, `Type`), memorizado por tipo. Entidade inválida retorna `ErrInvalidEntity`.
- `Row.Scan(&user)` preenche um registro na ordem de declaração dos campos; `sql.ErrNoRows` é preservado.
- `Rows.All(&users)` preenche `*[]T` casando colunas por nome (exato e depois sem diferenciar maiúsculas/minúsculas). Colunas sem campo são descartadas, campos sem coluna permanecem zero e o iterador é sempre fechado; o slice só é substituído em caso de sucesso.
- `NULL` é aceito em `*T` e `sql.Null*`; `NULL` em campo não anulável e conversões incompatíveis retornam erro contextualizado com a tabela, preservando a causa (`errors.Is`/`errors.As`). Ler um registro único por `Row.Scan` é posicional; para nomes/aliases e subconjuntos de colunas use `Rows.All`.
- `Rows.Columns()` expõe os nomes do resultset corrente.

O exemplo executável `Example_structMapping` em [tests/examples_test.go](tests/examples_test.go) percorre os dois modos de leitura. As decisões de sintaxe e correspondência estão em [E04-P02](.specs/E04-P02_decisions.md).

## Validação e formatação por campo

Um campo participa de validação e formatação implementando as interfaces no **tipo do campo** (receptor por valor ou ponteiro):

```go
type Email string

func (e Email) Validate() error { return /* nil quando válido */ }
func (e Email) Format() (string, error) { return string(e), nil }
```

- `Entity.Validate(record)` e `Entity.ValidateAll(records)` validam campos que implementam `krios.Validator`, na ordem de declaração, parando no primeiro erro. Aceitam `T` ou `*T` (e `[]T` ou `*[]T`); campo ponteiro nulo é pulado.
- `Entity.Format(record)` devolve `map[string]string` (coluna → texto) com **todas** as colunas. Campos sem `Formatter` usam `fmt.Sprint`; ponteiro nulo vira texto vazio.
- O texto formatado é só para apresentação; operações de banco usam o valor original do campo.
- Na leitura, a execução é explícita: chame `Validate`/`Format` quando precisar. Na **escrita**, `SetValues` executa `Entity.Validate` **automaticamente** e falha rápido com `ErrValidation`, sem chamada do consumidor e sem opção de desligar (E06 §8).
- Falhas retornam `ErrValidation` (campo reprovado) ou `ErrFormat` (falha de formatação), contextualizadas com tabela e coluna e preservando a causa (`errors.Is`/`errors.As`). Destino inválido retorna `ErrInvalidEntity`.

Campos primitivos (`string`, `int`, …) não têm validação/formatação sem um tipo nomeado. O exemplo `Example_validationFormatting` demonstra o fluxo; as decisões estão em [E04-P03](.specs/E04-P03_decisions.md).

O exemplo `Example_mappingValidation` reúne o fluxo completo de E04 (leitura de vários registros, `sql.Null*` e registro único) e as decisões de verificação estão em [E04-P04](.specs/E04-P04_decisions.md).

## Consultas encadeadas

`DB.Select(ent)` e `Tx.Select(ent)` criam o construtor; a execução usa o mesmo contrato e as mesmas proteções de `ExecContext`/`QueryContext`.

```go
users, _ := krios.EntityOf[User]()
tasks, _ := krios.EntityOf[Task]()

var found []User
err := db.Select(users).
    Only("id", "name").
    Join(tasks, krios.LeftJoin, tasks.Col("owner_id").Eq(users.Col("id"))).
    Where(krios.And(users.Col("name").Like(krios.StartsWith, "a"), tasks.Col("id").IsNull())).
    All(ctx, &found)
```

- `Only(names...)` restringe as colunas da **entidade base** (validada em `Build()`); sem seleção explícita, todas as colunas mapeadas são usadas na ordem de declaração dos campos. Duplicatas são erro.
- `Where(conditions...)` combina condições com `AND` e pode ser chamado várias vezes. `And`/`Or` agrupam e aninham condições; grupo vazio é erro.
- `Join(ent, kind, on)` aceita `krios.InnerJoin` e `krios.LeftJoin`; a condição `on` usa os mesmos comparadores, inclusive coluna × coluna. Toda entidade citada em `Only`/`Where`/`Join` precisa participar da consulta; tabela repetida é erro.
- Comparadores: `Eq`, `Ne`, `Lt`, `Lte`, `Gt`, `Gte`, `IsNull`, `IsNotNull`, `In`, `Like(krios.Contains|StartsWith|EndsWith, texto)`. `LIKE` escapa `!`, `%` e `_` no valor e sempre emite `ESCAPE '!'`; a diferenciação de maiúsculas segue a collation do banco. `In()` vazio renderiza `FALSE`. Operando `nil` é erro: use `IsNull`/`IsNotNull`.
- Valores Go viram parâmetros; `krios.Column` é renderizado como identificador qualificado, o que cobre comparações entre colunas.
- `Build()` retorna SQL e argumentos sem executar, com parâmetros na ordem textual (junções antes do `WHERE`). Falhas de construção retornam `ErrInvalidQuery` antes de chegar ao driver.
- `Rows`/`Row` executam e devolvem os wrappers de E03 (`Close`, `Err`, `Scan`); `All` mapeia por nome e `One` usa o mapeamento posicional de um registro.

As decisões estão em [E05-P01](.specs/E05-P01_decisions.md); `Example_selectQueries` demonstra `Build`, filtros e execução.

## Agregações, ordenação e paginação

Agregações são expressões selecionáveis, com alias para mapeamento por nome; `krios.Count()` representa `COUNT(*)`.

```go
type ownerTotal struct {
    OwnerID int64 `krios:"owner_id"`
    Total   int64 `krios:"total"`
}

var stats []ownerTotal

err := db.Select(tasks).
    Fields(tasks.Col("owner_id"), krios.Count().As("total")).
    GroupBy(tasks.Col("owner_id")).
    OrderBy(tasks.Col("owner_id").Asc()).
    All(ctx, &stats)

var page []Task
info, err := db.Select(tasks).
    Where(tasks.Col("owner_id").Eq(int64(1))).
    OrderBy(tasks.Col("id").Desc()).
    Paginate(ctx, 20, 0, &page)
// info: Total, Limit, Offset, Page, Pages, HasNext, HasPrev
```

- `Fields(exprs...)` acrescenta colunas ou agregações (`Count`, `Sum`, `Max`, `Min`, com `As(alias)`). Sem `Only`, a seleção contém apenas `Fields`; com `Only`, as colunas vêm primeiro. Alias vazio (só espaços), repetido ou que colida com uma coluna selecionada é erro.
- `Distinct()`, `GroupBy(cols...)` e `OrderBy(criteria...)` acumulam na ordem das chamadas. Critérios usam `coluna.Asc()/Desc()` ou `agregacao.Asc()/Desc()`. Toda entidade citada precisa participar da consulta.
- `Limit(n)` e `Offset(n)` são setters (a última chamada vence). `Limit(0)` retorna zero linhas; `Offset > 0` exige `Limit`; valores negativos são erro. Os argumentos seguem o `WHERE` na ordem textual.
- `Count(ctx)` ignora `ORDER BY`, `LIMIT` e `OFFSET` e conta com `SELECT COUNT(*)` sobre a consulta como tabela derivada, preservando `DISTINCT` e `GROUP BY`.
- `Paginate(ctx, limit, offset, dest)` valida `limit > 0` e `offset >= 0`, não muta o construtor e retorna total e estatísticas (`Page` base 1, `Pages`, `HasNext`, `HasPrev`). Em `*DB` contagem e página são consultas independentes; em `*Tx` compartilham a transação.
- `SUM`/`MAX`/`MIN` podem retornar `NULL` e o tipo nativo do driver (use `*T`/`sql.Null*`). `DISTINCT` com `ORDER BY` fora da seleção é rejeitado pelo PostgreSQL; a seleção deve ser coerente com o agrupamento.

As decisões estão em [E05-P03](.specs/E05-P03_decisions.md); `Example_aggregationsAndPagination` demonstra agrupamento e paginação. O pacote [examples/](examples/) (KriosTasks) usa o builder em todas as leituras e cobre junção, agrupamento e paginação em `examples/store_test.go`.

## SQL incorporado

O consumidor incorpora arquivos `.sql` com `go:embed` e executa o conteúdo sem tradução de sintaxe, fora do mecanismo de migrações:

```go
//go:embed queries/*.sql
var queries embed.FS

byEmail, err := krios.NewSQLFile(queries, "queries/users_by_email.sql")
if err != nil { return err }

var users []User
err = db.File(byEmail).All(ctx, &users, email)   // leitura mapeada

res, err := db.File(insertUser).Exec(ctx, name, email)   // escrita
```

- `krios.NewSQLFile(fsys, path)` carrega o recurso uma vez e devolve um objeto imutável (`Name()`, `Text()`), reutilizável entre conexões, transações e bancos. FS `nil`, caminho inválido/ausente, diretório ou arquivo em branco falham com `ErrFileNotFound` antes de tocar o banco.
- `db.File(f)` executa fora de transação; `tx.File(f)` executa na transação, com os mesmos guards (`ErrClosed`, `ErrTxDone`), serialização e bloqueio de commit de E03.
- Métodos: `Exec(ctx, args...)` devolve `Result`; `Query`/`QueryRow` devolvem `Rows`/`Row` nativos (com `Close`/`Err` pelo consumidor); `One(ctx, dest, args...)` preenche `*T`; `All(ctx, dest, args...)` preenche `*[]T` pelo mapeamento de E04.
- Os parâmetros são passados separadamente, com os marcadores do driver configurado (`?` em SQLite/MySQL, `$1..$n` em PostgreSQL). A biblioteca não divide múltiplos statements nem renumera parâmetros; use um arquivo por conjunto de arquivos compatível com cada banco.

`Example_sqlFile` demonstra incorporação, leitura mapeada, escrita parametrizada e execução em transação. As decisões estão em [E07-P01](.specs/E07-P01_decisions.md).

## Erros e propriedade dos resultados

- Tratar erros com `errors.Is`/`errors.As`. Erros do driver são preservados.
- `QueryRowContext(...).Scan(...)` preserva cancelamento, prazo, `ErrClosed` e `ErrTxDone`. `sql.ErrNoRows` significa consulta sem linha.
- Após `QueryContext`, registrar `defer rows.Close()`, tratar `Scan` e verificar `rows.Err()` depois da iteração. O erro continua disponível após fechar.
- `Result.RowsAffected()` e `LastInsertId()` retornam valor e disponibilidade; zero com `true` é válido. `false` inclui erro do driver ao obter essa informação, cujo detalhe não é exposto pela interface.
- Wrappers de acesso e resultados são obtidos por `Open`, `Begin` e consultas; seus valores zero não representam recursos prontos para uso.
- `ErrInvalidEntity` indica struct/tag/`TableName` inválidos na derivação ou destino de mapeamento malformado. Erros de mapeamento preservam a causa do driver.
- `ErrValidation` e `ErrFormat` indicam campo reprovado por `Validate()` ou falha em `Format()`, com tabela e coluna no contexto.
- `ErrInvalidQuery` indica construção inválida de consulta (coluna/entidade desconhecida, operando `nil`, junção inválida, grupo vazio, alias de saída inválido, limite/deslocamento inválido); é detectado em `Build()` e antes da execução.
- `ErrFileNotFound` indica recurso SQL incorporado que não pôde ser carregado (FS `nil`, caminho inválido/ausente, diretório ou arquivo em branco), detectado em `NewSQLFile` antes de qualquer acesso ao banco.

## Transações e concorrência

Após `Begin`, registrar `defer tx.Rollback(ctx)` para limpar também retornos antecipados. `Rollback(ctx)` tenta a limpeza mesmo com contexto cancelado; não é interrompido pelo contexto. Depois da finalização, novas chamadas retornam `ErrTxDone`.

`Commit(ctx)` rejeita contexto cancelado antes da tentativa nativa quando não há erro registrado anterior. A transação memoriza o primeiro erro observado — falha imediata de `ExecContext`/`QueryContext` ou erro tardio de leitura (`Row.Scan`, `Rows.Scan`, iteração e fechamento) — e, nesse caso, o commit é bloqueado e tenta rollback mesmo com contexto cancelado, preservando o primeiro erro e eventual erro de limpeza. `sql.ErrNoRows` não bloqueia o commit, para preservar o padrão de verificação de existência. O rollback explícito no primeiro erro continua sendo o comportamento recomendado; o bloqueio do commit é uma proteção adicional.

O contexto passado a `Begin` controla toda a vida da transação e seu cancelamento provoca reversão nativa. Falha de finalização torna o wrapper inutilizável e pode deixar o estado remoto incerto; não existe repetição automática de escrita.

`DB` permite uso concorrente. Chamadas na mesma `Tx` são serializadas, mas isso não coordena a leitura dos resultados. Use um consumidor por iterador, consuma/feche resultados antes de outra operação na mesma transação e aguarde os consumidores antes de finalizar. Não há ordem garantida entre goroutines.

## Organização

| Arquivo | Responsabilidade |
| --- | --- |
| `krios.go` | Fachada pública: aliases de tipos, constantes, erros e delegação de `Open`, `NewSQLFile`, `EntityOf`, `And`, `Or` e `Count` |
| `internal/errs/errors.go` | Erros sentinela compartilhados e normalização de erros nativos |
| `internal/config/config.go`, `dsn.go` | Configuração, validações e DSNs |
| `internal/dialect/dialect.go`, `migration.go` | Interface `Dialect`, seleção por `Config.Type` e capacidades de migração |
| `internal/entity/entity.go`, `validation.go`, `expression.go`, `aggregate.go` | Entidades por reflexão, validação/formatação, AST de condições e agregações/ordenação |
| `internal/query/query.go`, `pagination.go` | Construtor `Select` (seleção, filtros, junções, agrupamento/ordem/limite), `Build`, execução, `Count` e `Paginate` |
| `internal/access/db.go` | Conexão (`Open`, `Close`), dialeto retido e `Select` |
| `internal/transaction/tx.go` | Transação (`Begin`, `Commit`, `Rollback`), dialeto retido e `Select` |
| `internal/execute/execution.go` | Delegação nativa compartilhada (contexto, chamada, wrapper) |
| `internal/scan/rows.go`, `mapping.go` | Recursos de leitura (`Row`/`Rows`, `All`) e mapeamento para `*T`/`*[]T` |
| `internal/sqlfile/sqlfile.go` | Recurso SQL incorporado (`SQLFile`) e runner de execução (`File`) |
| `internal/result/result.go` | Resultado de escrita com flags de disponibilidade |
| `internal/testutil/driver.go` | Driver simulador com hooks e contadores usado pelos testes |
| `internal/**/*_test.go` | Testes internos por pacote |
| `tests/` | Integração SQLite e exemplos de consumo externo |
| `examples/` | Projeto consumidor de exemplo (KriosTasks) com leituras pelo builder de E05 |
| `cmd/krios/main.go` | Placeholder que imprime uma saudação; ainda não é exemplo funcional de acesso |
| `.specs/` | Requisitos, decisões, melhorias e roteiro manual |

Drivers são importados somente nos testes do repositório; não entram na compilação normal da biblioteca nem do placeholder. `DB`/`Tx` não retêm a configuração completa sem uso. O pacote público permanece na raiz e delega à implementação nos pacotes internos sob `internal`, que não são importados pelo consumidor. Os aliases preservam os tipos e métodos disponíveis pelo import `github.com/mineot/krios`; a fachada não duplica a lógica.

`scan` não depende de `transaction`: como Go não permite que métodos cruzem pacotes, `Row`/`Rows` guardam um recorder `func(error)` injetado por `transaction` em `QueryContext`/`QueryRowContext`. Assim o erro tardio de leitura bloqueia o commit sem criar o ciclo `scan ⇄ transaction`.

## SQLite local

Os testes usam `modernc.org/sqlite v1.60.1`, fixado no módulo. A seleção original e aceitação do SQLite incorporado foram registradas em E01-P03 em 18/10/2026. Atualizações são deliberadas; não há exigência de coincidir com a versão upstream mais recente.

`make dev-db` habilita um auxiliar de teste para criar/abrir `dev.db`; esse auxiliar é ignorado em testes normais e rejeita execução com `CI` definido. O banco e seus arquivos auxiliares estão no `.gitignore`. Não executar esse comando para validar uma alteração: use os bancos temporários da suíte.

## Integração contínua e evidências

O workflow possui jobs `check` (sem CGO) e `race` (CGO habilitado e compilador C), ambos nas linhas Go 1.26/1.27. Usa `ubuntu-latest`, permissões de leitura, timeout de 15 minutos por job e cancelamento de execuções antigas da mesma referência. Não publica artefatos nem utiliza cache persistente do Actions.

Os jobs executam somente quando o repositório é público, preservando a condição de gratuidade adotada pelo projeto. Em repositório privado são ignorados. Não há mudança de visibilidade ou faturamento; consulte a [política do GitHub Actions](https://docs.github.com/en/billing/concepts/product-billing/github-actions).

A revisão M01–M06 passou localmente por `make check CGO_ENABLED=0` e `make race` em Go **1.26.8** e **1.27.1**. As versões locais não são apresentadas como as últimas disponíveis. A execução remota do workflow atualizado permanece pendente. O relato de sucesso remoto em 18/10 é evidência histórica, anterior a estas alterações.

MySQL/PostgreSQL aguardam execução do [roteiro manual](.specs/E09_manual_validation.md). Testar DSNs e falhas simuladas não comprova conexão real. E04 e E05 avançaram sobre a base corrigida; a validação de consultas e paginação tem evidência automatizada em SQLite e nos três dialetos, mas as evidências manuais dos dois servidores continuam obrigatórias para concluir E09.

## Documentação do projeto

- [Manual de uso](MANUAL.md)
- [Planejamento e requisitos](.specs/0001_planning.md)
- [Plano de execução e progresso](.specs/0002_execution_plan.md)
- [Plano de melhorias e evidências M01–M06](.specs/0003_improvement_plan.md)
- [Base técnica — E01-P01](.specs/E01-P01_decisions.md)
- [Configuração — E02-P01](.specs/E02-P01_decisions.md)
- [Execução e transações — E03-P01](.specs/E03-P01_decisions.md)
- [Mapeamento por reflexão — E04-P01](.specs/E04-P01_decisions.md)
- [Implementação do mapeamento — E04-P02](.specs/E04-P02_decisions.md)
- [Validação e formatação — E04-P03](.specs/E04-P03_decisions.md)
- [Validação do mapeamento — E04-P04](.specs/E04-P04_decisions.md)
- [Seleção, filtros e junções — E05-P01](.specs/E05-P01_decisions.md)
- [Agregações e paginação — E05-P03](.specs/E05-P03_decisions.md)
- [Validação de consultas — E05-P04](.specs/E05-P04_decisions.md)
- [Escritas — E06-P01](.specs/E06-P01_decisions.md)
- [SQL incorporado — E07-P01](.specs/E07-P01_decisions.md)
- [Migrações — E08-P01](.specs/E08-P01_decisions.md)
- [Exemplos de consumo](.specs/0001_use_examples.md)
- [Roteiro de validação manual — E09](.specs/E09_manual_validation.md)
- [Integração real com servidores — E10](.specs/E10_integration_plan.md)
- [Reorganização dos pacotes internos](.specs/0004_reorganization.md)
- [Plano de consolidação e progresso](.specs/0005_consolidation_plan.md)

Decisões da consolidação (C01–C07): [C01-P01](.specs/C01-P01_decisions.md), [C03-P01](.specs/C03-P01_decisions.md)–[C03-P04](.specs/C03-P04_decisions.md), [C04-P01](.specs/C04-P01_decisions.md)–[C04-P03](.specs/C04-P03_decisions.md), [C05-P01](.specs/C05-P01_decisions.md).

## Licença

[Apache License 2.0](LICENSE).
