# E09 — Roteiro manual da base E01–E05

Data: 2026-10-19. **Estado: preparado; MySQL e PostgreSQL ainda não executados.**

Este roteiro coleta evidência real de conexão, transações e consultas encadeadas. DSNs e drivers simulados já têm testes locais, mas não substituem este roteiro. SQL incorporado e migrações serão acrescentados quando E07–E08 estiverem implementados.

## 1. Preparar e registrar o ambiente

Usar um banco exclusivo de validação, com usuário autorizado a criar/remover as tabelas do roteiro. Para MySQL, usar tabelas InnoDB. O roteiro só remove as tabelas que criou; se um nome já existir, a criação deve falhar antes de registrar a limpeza.

Registrar sem senhas: versão do Go, SO, revisão/commit do Krios, driver e versão exatos, versão do servidor, mecanismo das tabelas MySQL, configurações de isolamento e endereço IPv4/IPv6 usado. Consultar `SELECT VERSION()` e, no MySQL, `SELECT @@default_storage_engine`.

Criar um módulo consumidor em um diretório temporário separado. Os drivers deste módulo não alteram as dependências da biblioteca:

```sh
mkdir krios-manual
cd krios-manual
go mod init example.local/krios-manual
go mod edit -replace github.com/mineot/krios=/home/mineot/Projetos/krios
go get github.com/mineot/krios
```

Escolher versões estáveis dos drivers compatíveis com o Go utilizado e instalá-las explicitamente com `go get github.com/go-sql-driver/mysql@VERSAO` e `go get github.com/jackc/pgx/v5@VERSAO`, substituindo `VERSAO` pela versão escolhida. Preservar `go.mod`/`go.sum` e registrar a saída de `go list -m all`. Não usar atualizações automáticas entre execuções comparadas.

## 2. Consumidor reproduzível

Criar `main.go` com o conteúdo abaixo. Preencher `cfg` para o banco de teste; para PostgreSQL, trocar por `Type: krios.DBTypePostgres`, `Driver: "pgx"` e `Postgres: &krios.PostgresConfig{...}`, removendo o campo `MySQL`. Importar o driver escolhido; abaixo os dois estão disponíveis apenas neste consumidor manual.

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/mineot/krios"
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
    cfg := krios.Config{
        Type: krios.DBTypeMySQL,
        Driver: "mysql",
        MySQL: &krios.MySQLConfig{
            Host: "127.0.0.1", User: "USUARIO_TESTE",
            Password: "SENHA_TESTE", DBName: "BANCO_TESTE",
        },
    }
    if err := verify(cfg); err != nil { panic(err) }
    fmt.Println("base validada")
}

func verify(cfg krios.Config) error {
    ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
    defer cancel()
    db, err := krios.Open(ctx, cfg)
    if err != nil { return err }
    defer db.Close()
    ddl := "CREATE TABLE krios_manual_entries (id INTEGER PRIMARY KEY)"
    insert := "INSERT INTO krios_manual_entries (id) VALUES (?)"
    if cfg.Type == krios.DBTypeMySQL { ddl += " ENGINE=InnoDB" }
    if cfg.Type == krios.DBTypePostgres {
        insert = "INSERT INTO krios_manual_entries (id) VALUES ($1)"
    }
    if _, err := db.ExecContext(ctx, ddl); err != nil { return err }
    defer func() {
        cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cleanupCancel()
        if _, err := db.ExecContext(cleanupCtx, "DROP TABLE krios_manual_entries"); err != nil {
            fmt.Println("limpeza pendente: remover krios_manual_entries do banco de teste")
        }
    }()
    count := func(want int) error {
        var got int
        if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM krios_manual_entries").Scan(&got); err != nil {
            return err
        }
        if got != want { return fmt.Errorf("count=%d, esperado=%d", got, want) }
        return nil
    }
    // Confirma uma operação externa ao grupo seguinte.
    if _, err := db.ExecContext(ctx, insert, 100); err != nil { return err }
    tx, err := db.Begin(ctx)
    if err != nil { return err }
    defer tx.Rollback(context.Background())
    if _, err := tx.ExecContext(ctx, insert, 1); err != nil { return err }
    if err := tx.Commit(ctx); err != nil { return err }
    if err := count(2); err != nil { return err }

    group, err := db.Begin(ctx)
    if err != nil { return err }
    defer group.Rollback(context.Background())
    if _, err := group.ExecContext(ctx, insert, 2); err != nil { return err }
    _, statementErr := group.ExecContext(ctx, insert, 2)
    if statementErr == nil { return errors.New("duplicata deveria falhar") }
    canceled, stop := context.WithCancel(ctx)
    stop()
    if err := group.Commit(canceled); !errors.Is(err, statementErr) {
        return fmt.Errorf("commit bloqueado não preservou a causa: %w", err)
    }
    if err := count(2); err != nil { return err }
    var value int
    if err := group.QueryRowContext(ctx, "SELECT 1").Scan(&value); !errors.Is(err, krios.ErrTxDone) {
        return errors.New("transação encerrada não retornou ErrTxDone")
    }

    cleanup, err := db.Begin(ctx)
    if err != nil { return err }
    defer cleanup.Rollback(context.Background())
    if _, err := cleanup.ExecContext(ctx, insert, 3); err != nil { return err }
    if err := cleanup.Rollback(canceled); err != nil { return err }
    return count(2)
}
```

Executar `go mod tidy`, `go run .` e guardar o resultado e versões. Usar somente credenciais de teste; revisar erros antes de compartilhar o relatório. O código de acesso é configurado em Go; as variáveis de ambiente dos clientes de banco não são requisito do Krios.

## 3. Consultas e paginação (E05)

Reutilizar o mesmo módulo consumidor. Depois de executar V01–V12, substituir `main.go` pelo programa abaixo (ou colocá-lo em outro diretório) para validar o construtor de leitura. Ajustar `cfg` para o banco de teste como em §2.

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/mineot/krios"
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

type manualUser struct {
    ID   int64   `krios:"id,pk"`
    Name string  `krios:"name"`
    Bio  *string `krios:"bio"`
}

func (manualUser) TableName() string { return "krios_manual_users" }

type manualTask struct {
    ID      int64  `krios:"id,pk"`
    OwnerID int64  `krios:"owner_id"`
    Title   string `krios:"title"`
}

func (manualTask) TableName() string { return "krios_manual_tasks" }

func main() {
    cfg := krios.Config{
        Type: krios.DBTypeMySQL,
        Driver: "mysql",
        MySQL: &krios.MySQLConfig{
            Host: "127.0.0.1", User: "USUARIO_TESTE",
            Password: "SENHA_TESTE", DBName: "BANCO_TESTE",
        },
    }
    // PostgreSQL: Type: krios.DBTypePostgres, Driver: "pgx",
    // Postgres: &krios.PostgresConfig{...}, sem o campo MySQL.

    ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
    defer cancel()
    db, err := krios.Open(ctx, cfg)
    if err != nil { panic(err) }
    defer db.Close()
    if err := verifySelects(ctx, db, cfg.Type); err != nil { panic(err) }
    fmt.Println("consultas E05 validadas")
}

func verifySelects(ctx context.Context, db *krios.DB, typ krios.DBType) error {
    users, err := krios.EntityOf[manualUser]()
    if err != nil { return err }
    tasks, err := krios.EntityOf[manualTask]()
    if err != nil { return err }

    userDDL := "CREATE TABLE krios_manual_users (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL, bio VARCHAR(100))"
    taskDDL := "CREATE TABLE krios_manual_tasks (id INTEGER PRIMARY KEY, owner_id INTEGER NOT NULL, title VARCHAR(100) NOT NULL)"
    if typ == krios.DBTypeMySQL {
        userDDL += " ENGINE=InnoDB"
        taskDDL += " ENGINE=InnoDB"
    }
    if _, err := db.ExecContext(ctx, userDDL); err != nil { return err }
    defer func() {
        cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cleanupCancel()
        if _, err := db.ExecContext(cleanupCtx, "DROP TABLE krios_manual_tasks"); err != nil {
            fmt.Println("limpeza pendente: remover krios_manual_tasks do banco de teste")
        }
        if _, err := db.ExecContext(cleanupCtx, "DROP TABLE krios_manual_users"); err != nil {
            fmt.Println("limpeza pendente: remover krios_manual_users do banco de teste")
        }
    }()
    if _, err := db.ExecContext(ctx, taskDDL); err != nil { return err }

    placeholder := func(n int) string {
        if typ == krios.DBTypePostgres { return fmt.Sprintf("$%d", n) }
        return "?"
    }
    insertUser := func(id int64, name string, bio any) error {
        _, err := db.ExecContext(ctx,
            "INSERT INTO krios_manual_users (id, name, bio) VALUES ("+placeholder(1)+", "+placeholder(2)+", "+placeholder(3)+")",
            id, name, bio)
        return err
    }
    if err := insertUser(1, "alice", "reader"); err != nil { return err }
    if err := insertUser(2, "bob", nil); err != nil { return err }
    if err := insertUser(3, "carol", nil); err != nil { return err }
    if _, err := db.ExecContext(ctx,
        "INSERT INTO krios_manual_tasks (id, owner_id, title) VALUES ("+placeholder(1)+", "+placeholder(2)+", "+placeholder(3)+"), ("+
            placeholder(4)+", "+placeholder(5)+", "+placeholder(6)+"), ("+
            placeholder(7)+", "+placeholder(8)+", "+placeholder(9)+")",
        int64(1), int64(1), "write", int64(2), int64(1), "review", int64(3), int64(2), "deploy"); err != nil {
        return err
    }

    // Only restringe às colunas pedidas, na ordem pedida.
    type userName struct {
        Name string `krios:"name"`
    }
    var onlyNames []userName
    if err := db.Select(users).Only("name").OrderBy(users.Col("name").Asc()).All(ctx, &onlyNames); err != nil {
        return fmt.Errorf("Only: %w", err)
    }
    if len(onlyNames) != 3 || onlyNames[0].Name != "alice" || onlyNames[2].Name != "carol" {
        return fmt.Errorf("Only(name) = %+v", onlyNames)
    }

    // Seleção padrão e mapeamento posicional em One.
    var alice manualUser
    if err := db.Select(users).Where(users.Col("name").Eq("alice")).One(ctx, &alice); err != nil {
        return fmt.Errorf("One: %w", err)
    }
    if alice.ID != 1 || alice.Name != "alice" || alice.Bio == nil || *alice.Bio != "reader" {
        return fmt.Errorf("seleção padrão/One = %+v", alice)
    }

    // Filtros agrupados, LIKE e IN vazio.
    var filtered []manualUser
    if err := db.Select(users).Where(krios.Or(
        users.Col("name").Eq("bob"),
        krios.And(users.Col("bio").IsNotNull(), users.Col("name").Ne("bob")),
    )).All(ctx, &filtered); err != nil { return fmt.Errorf("filtros agrupados: %w", err) }
    if len(filtered) != 2 { return fmt.Errorf("filtros agrupados = %d linhas, esperado 2", len(filtered)) }

    var starts []manualUser
    if err := db.Select(users).Where(users.Col("name").Like(krios.StartsWith, "a")).All(ctx, &starts); err != nil {
        return fmt.Errorf("LIKE: %w", err)
    }
    if len(starts) != 1 { return fmt.Errorf("LIKE 'a%%' = %d linhas, esperado 1", len(starts)) }

    var none []manualUser
    if err := db.Select(users).Where(users.Col("id").In()).All(ctx, &none); err != nil {
        return fmt.Errorf("IN vazio: %w", err)
    }
    if len(none) != 0 { return fmt.Errorf("IN vazio = %d linhas, esperado 0", len(none)) }

    // Junções INNER e LEFT.
    var aliceTasks []manualTask
    if err := db.Select(tasks).
        Join(users, krios.InnerJoin, users.Col("id").Eq(tasks.Col("owner_id"))).
        Where(users.Col("name").Eq("alice")).All(ctx, &aliceTasks); err != nil {
        return fmt.Errorf("INNER JOIN: %w", err)
    }
    if len(aliceTasks) != 2 { return fmt.Errorf("INNER JOIN alice = %d tarefas, esperado 2", len(aliceTasks)) }

    var withoutTasks []manualUser
    if err := db.Select(users).
        Join(tasks, krios.LeftJoin, tasks.Col("owner_id").Eq(users.Col("id"))).
        Where(tasks.Col("id").IsNull()).All(ctx, &withoutTasks); err != nil {
        return fmt.Errorf("LEFT JOIN: %w", err)
    }
    if len(withoutTasks) != 1 || withoutTasks[0].Name != "carol" {
        return fmt.Errorf("LEFT JOIN sem tarefas = %+v, esperado carol", withoutTasks)
    }

    // Agregações com agrupamento: COUNT, MIN, MAX e COUNT(*).
    type ownerTotal struct {
        OwnerID int64 `krios:"owner_id"`
        Total   int64 `krios:"total"`
        MinID   int64 `krios:"min_id"`
        MaxID   int64 `krios:"max_id"`
    }
    var totals []ownerTotal
    if err := db.Select(tasks).
        Fields(tasks.Col("owner_id"), krios.Count().As("total"),
            tasks.Col("id").Min().As("min_id"), tasks.Col("id").Max().As("max_id")).
        GroupBy(tasks.Col("owner_id")).
        OrderBy(tasks.Col("owner_id").Asc()).All(ctx, &totals); err != nil {
        return fmt.Errorf("agregação: %w", err)
    }
    wantTotals := []ownerTotal{
        {OwnerID: 1, Total: 2, MinID: 1, MaxID: 2},
        {OwnerID: 2, Total: 1, MinID: 3, MaxID: 3},
    }
    if len(totals) != len(wantTotals) || totals[0] != wantTotals[0] || totals[1] != wantTotals[1] {
        return fmt.Errorf("agregação por dono = %+v, esperado %+v", totals, wantTotals)
    }

    // SUM sem linhas devolve NULL (ponteiro nil), não zero.
    type sumTotal struct {
        Total *int64 `krios:"total"`
    }
    var sums []sumTotal
    if err := db.Select(tasks).
        Fields(tasks.Col("id").Sum().As("total")).
        Where(tasks.Col("id").Lt(int64(0))).All(ctx, &sums); err != nil {
        return fmt.Errorf("SUM vazio: %w", err)
    }
    if len(sums) != 1 || sums[0].Total != nil {
        return fmt.Errorf("SUM vazio = %+v, esperado NULL", sums)
    }

    // DISTINCT sobre coluna ordenada pela própria coluna (portável).
    type owner struct {
        OwnerID int64 `krios:"owner_id"`
    }
    var owners []owner
    if err := db.Select(tasks).
        Fields(tasks.Col("owner_id")).
        Distinct().
        OrderBy(tasks.Col("owner_id").Desc()).All(ctx, &owners); err != nil {
        return fmt.Errorf("DISTINCT: %w", err)
    }
    if len(owners) != 2 || owners[0].OwnerID != 2 || owners[1].OwnerID != 1 {
        return fmt.Errorf("DISTINCT owners = %+v, esperado [2 1]", owners)
    }

    // Ordenação com LIMIT/OFFSET.
    var ordered []manualUser
    if err := db.Select(users).
        OrderBy(users.Col("name").Asc()).
        Limit(2).Offset(1).All(ctx, &ordered); err != nil {
        return fmt.Errorf("LIMIT/OFFSET: %w", err)
    }
    if len(ordered) != 2 || ordered[0].Name != "bob" || ordered[1].Name != "carol" {
        return fmt.Errorf("LIMIT/OFFSET = %+v, esperado bob e carol", ordered)
    }
    var zeroRows []manualUser
    if err := db.Select(users).OrderBy(users.Col("id").Asc()).Limit(0).All(ctx, &zeroRows); err != nil {
        return fmt.Errorf("Limit(0): %w", err)
    }
    if len(zeroRows) != 0 { return fmt.Errorf("Limit(0) = %d linhas, esperado 0", len(zeroRows)) }

    // Count ignora ordenação e limite, e preserva DISTINCT/grupo.
    count, err := db.Select(tasks).OrderBy(tasks.Col("id").Desc()).Limit(1).Count(ctx)
    if err != nil { return fmt.Errorf("Count: %w", err) }
    if count != 3 { return fmt.Errorf("Count com limite = %d, esperado 3", count) }
    count, err = db.Select(tasks).Fields(tasks.Col("owner_id")).Distinct().Count(ctx)
    if err != nil { return fmt.Errorf("Count DISTINCT: %w", err) }
    if count != 2 { return fmt.Errorf("Count DISTINCT = %d, esperado 2", count) }
    count, err = db.Select(tasks).
        Fields(tasks.Col("owner_id"), krios.Count().As("total")).
        GroupBy(tasks.Col("owner_id")).Count(ctx)
    if err != nil { return fmt.Errorf("Count agrupado: %w", err) }
    if count != 2 { return fmt.Errorf("Count agrupado = %d, esperado 2", count) }

    // Paginate: primeira/segunda página, filtro com argumentos e vazio.
    var first []manualUser
    page, err := db.Select(users).OrderBy(users.Col("id").Asc()).Paginate(ctx, 2, 0, &first)
    if err != nil { return fmt.Errorf("Paginate primeira: %w", err) }
    if page.Total != 3 || page.Pages != 2 || page.Page != 1 || !page.HasNext || page.HasPrev || len(first) != 2 {
        return fmt.Errorf("primeira página = %+v, linhas = %d", page, len(first))
    }
    var second []manualUser
    page, err = db.Select(users).OrderBy(users.Col("id").Asc()).Paginate(ctx, 2, 2, &second)
    if err != nil { return fmt.Errorf("Paginate segunda: %w", err) }
    if page.Page != 2 || page.Pages != 2 || page.HasNext || !page.HasPrev || len(second) != 1 || second[0].Name != "carol" {
        return fmt.Errorf("segunda página = %+v, linhas = %+v", page, second)
    }
    var filteredPage []manualTask
    page, err = db.Select(tasks).
        Where(tasks.Col("owner_id").Eq(int64(1))).
        OrderBy(tasks.Col("id").Asc()).
        Paginate(ctx, 1, 1, &filteredPage)
    if err != nil { return fmt.Errorf("Paginate filtrado: %w", err) }
    if page.Total != 2 || page.Page != 2 || page.Pages != 2 || page.HasNext || !page.HasPrev ||
        len(filteredPage) != 1 || filteredPage[0].Title != "review" {
        return fmt.Errorf("página filtrada = %+v, linhas = %+v", page, filteredPage)
    }
    var emptyPage []manualTask
    page, err = db.Select(tasks).
        Where(tasks.Col("title").Eq("missing")).
        Paginate(ctx, 3, 0, &emptyPage)
    if err != nil { return fmt.Errorf("Paginate vazio: %w", err) }
    if page.Total != 0 || page.Pages != 0 || page.Page != 1 || page.HasNext || page.HasPrev || len(emptyPage) != 0 {
        return fmt.Errorf("página vazia = %+v, linhas = %d", page, len(emptyPage))
    }

    // Tx: Count/Paginate enxergam escrita não confirmada; rollback desfaz.
    tx, err := db.Begin(ctx)
    if err != nil { return err }
    defer tx.Rollback(context.Background())
    if _, err := tx.ExecContext(ctx,
        "INSERT INTO krios_manual_tasks (id, owner_id, title) VALUES ("+placeholder(1)+", "+placeholder(2)+", "+placeholder(3)+")",
        int64(4), int64(1), "ship"); err != nil {
        return err
    }
    count, err = tx.Select(tasks).Count(ctx)
    if err != nil { return fmt.Errorf("Count na Tx: %w", err) }
    if count != 4 { return fmt.Errorf("Count na Tx = %d, esperado 4", count) }
    var txPage []manualTask
    page, err = tx.Select(tasks).OrderBy(tasks.Col("id").Asc()).Paginate(ctx, 2, 2, &txPage)
    if err != nil { return fmt.Errorf("Paginate na Tx: %w", err) }
    if page.Total != 4 || len(txPage) != 2 || txPage[1].Title != "ship" {
        return fmt.Errorf("página na Tx = %+v, linhas = %+v", page, txPage)
    }
    if err := tx.Rollback(ctx); err != nil { return err }
    count, err = db.Select(tasks).Count(ctx)
    if err != nil { return fmt.Errorf("Count após rollback: %w", err) }
    if count != 3 { return fmt.Errorf("Count após rollback = %d, esperado 3", count) }

    // Erro de construção antes da execução não bloqueia Commit.
    tx, err = db.Begin(ctx)
    if err != nil { return err }
    if _, err := tx.Select(tasks).Paginate(ctx, 0, 0, &[]manualTask{}); !errors.Is(err, krios.ErrInvalidQuery) {
        return fmt.Errorf("Paginate(limit=0) = %v, esperado ErrInvalidQuery", err)
    }
    if err := tx.Commit(ctx); err != nil { return fmt.Errorf("Commit após consulta inválida: %w", err) }

    if err := db.Select(users).Where(users.Col("missing").Eq(1)).All(ctx, &[]manualUser{}); !errors.Is(err, krios.ErrInvalidQuery) {
        return fmt.Errorf("coluna desconhecida = %v, esperado ErrInvalidQuery", err)
    }

    // Não falha o roteiro: registra a diferença de collation entre os bancos.
    var upper []manualUser
    if err := db.Select(users).Where(users.Col("name").Like(krios.StartsWith, "A")).All(ctx, &upper); err != nil {
        return fmt.Errorf("LIKE 'A%%': %w", err)
    }
    fmt.Printf("LIKE 'A%%' encontrou %d usuário(s); PostgreSQL esperado 0, MySQL esperado 1\n", len(upper))

    return nil
}
```

Resultado esperado: `consultas E05 validadas`, com a linha de collation registrada. A limpeza remove as duas tabelas; se uma criação falhar, a tabela já criada é removida pelo `defer` correspondente.

## 4. Cenários complementares

| ID  | Procedimento                                                                                                                                            | Resultado esperado                                                                                                                                                                                         |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| V01 | Executar o consumidor acima em cada banco                                                                                                               | Mensagem de sucesso; grupos confirmados/revertidos; operação externa preservada                                                                                                                            |
| V02 | Criar usuário de teste com senha contendo `@`, `:`, `%` e espaço; repetir V01                                                                           | Autenticação com senha literal, sem percent-encoding manual                                                                                                                                                |
| V03 | Criar banco com nome contendo `%` e espaço, se aceito pelo servidor; preencher nome literal                                                             | Acesso ao banco exato; não a um nome decodificado incorretamente                                                                                                                                           |
| V04 | Repetir V01 com host `::1`, se o servidor escutar em IPv6                                                                                               | Endereço composto corretamente; registrar indisponibilidade do ambiente separadamente                                                                                                                      |
| V05 | Antes de `Open`, cancelar o contexto; repetir com deadline passado                                                                                      | Erro de contexto verificável, sem `ErrConnectionFailed`                                                                                                                                                    |
| V06 | Executar `SELECT SLEEP(5)` (MySQL) ou `SELECT pg_sleep(5)` (PostgreSQL) com contexto de 100 ms, via Query/Scan                                          | Cancelamento/prazo conforme driver; registrar duração e causa; fechar resultados e reverter Tx quando utilizada                                                                                            |
| V07 | Abrir consulta com várias linhas, ler somente a primeira e fechar; depois fazer nova consulta                                                           | Recurso liberado; nova consulta utilizável; consultar `Err` após fechar                                                                                                                                    |
| V08 | Dentro de Tx, provocar erro de conversão em `Scan` e solicitar rollback explicitamente                                                                  | Erro de leitura preservado; escritas anteriores do grupo desfeitas                                                                                                                                         |
| V09 | Enquanto Tx contém insert ainda não confirmado, consultar via DB; depois rollback                                                                       | Operação no DB não participa do grupo nem enxerga escrita não confirmada no isolamento configurado; registrar bloqueios/isolamento                                                                         |
| V10 | Consumir/fechar resultados e finalizar; repetir todos os métodos na Tx                                                                                  | `ErrTxDone`, inclusive em `Row.Scan`                                                                                                                                                                       |
| V11 | Em ambiente descartável, interromper a conexão durante confirmação, se houver infraestrutura para controle                                              | Erro preservado; objeto terminal; estado remoto registrado como incerto até consulta independente                                                                                                          |
| V12 | Dentro de Tx, executar escrita e depois leitura com erro de conversão (ex.: texto em `int`) ou erro de iteração; tentar `Commit` sem rollback explícito | `Commit` bloqueado com auto-rollback (E03-P04): erro original recuperável por `errors.Is`, escrita não persistida, Tx terminal. Repetir com consulta sem linhas (`ErrNoRows`) → `Commit` **não** bloqueado |
| V13 | Executar o harness de §3 em cada banco | `consultas E05 validadas`; seleção padrão/One, filtros, junções, agregações, `DISTINCT`, ordem, limite/deslocamento, `Count` e `Paginate` conferem com os valores esperados |
| V14 | Repetir os filtros com `LIKE` contendo `!`, `%` e `_` literais e com `IN()` vazio | Valor tratado literalmente, sem curinga acidental; `IN` vazio retorna zero linhas |
| V15 | Executar `LEFT JOIN` e filtrar `tasks.id IS NULL` | Apenas usuários sem tarefa (carol); junção não descarta a linha da base |
| V16 | Agregar `COUNT(*)`, `COUNT(coluna)`, `SUM`, `MAX` e `MIN` por dono; agregar entidade juntada | Totais por grupo conferem; linha da esquerda sem correspondência conta 0 em `COUNT(coluna)` |
| V17 | Selecionar `SUM(id)` sem linhas | Uma linha de resultado com `NULL`; o destino `*int64` fica nil, não zero |
| V18 | Executar `DISTINCT` com `ORDER BY` pela coluna selecionada | Uma linha por valor distinto; ordenação preservada |
| V19 | Executar `OrderBy` com `Limit(0)`, `Limit(n)` e `Offset`; comparar `Count` com/sem limite e ordem | `Limit(0)` retorna zero linhas; `Count` ignora ordem/limite e mantém `DISTINCT`/grupo |
| V20 | Executar `Paginate` na primeira página, última, vazia e com filtro que usa parâmetros | `Total`, `Page`, `Pages`, `HasNext` e `HasPrev` conferem; parâmetros do filtro precedem `LIMIT`/`OFFSET` |
| V21 | Dentro de Tx, inserir tarefa e chamar `Count`/`Paginate`; reverter. Em outra Tx, `Paginate` inválido e depois `Commit` | Escrita não confirmada é visível na mesma Tx; rollback desfaz; erro de construção não bloqueia o commit |
| V22 | PostgreSQL: `Fields(owner_id).Distinct().OrderBy(tasks.id)`; repetir em MySQL/SQLite | PostgreSQL rejeita ordem fora da seleção; MySQL/SQLite podem aceitar. Registrar a diferença, sem tradução automática |
| V23 | Buscar com `Like(StartsWith, "A")` em cada banco | MySQL/SQLite encontram `alice`; PostgreSQL não (case-sensitive). Registrar a collation e não tratar como falha |

V11 não autoriza repetir automaticamente a escrita: confirmar a situação por outra conexão. Se o ambiente não permitir injeção controlada, registrar o cenário como não executado; os testes simulados continuam sendo evidência somente do contrato local.

### 4.1 Cenários de migrações — estado parcial (E08)

Aplicam-se quando E08 estiver implementado; ver [E08-P01_decisions.md](E08-P01_decisions.md), que fixa `MigrationsConfig{FS, TargetVersion, Versions, ManagedObjects}` e `RunMigrations(ctx)`. As migrações são configuradas em código: `ManagedObjects` deve listar as tabelas geridas pelo cenário para a verificação de banco sem histórico (§6 de E08-P01).

| ID  | Procedimento                                                                                                                                            | Resultado esperado                                                                                                                  |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| M01 | MySQL: migração com DDL não-transacional seguido de comando que falha no meio do arquivo                                                                | Arquivo registrado com `in_progress`/`failed`; erro com contexto; `applied_at` não preenchido para o arquivo falho                  |
| M02 | Após M01, chamar `RunMigrations` novamente                                                                                                              | Bloqueio com `ErrPartialMigration` (`Direction`, `TargetVersion`, `LastVersion`, `LastFile`, `Status`); nenhuma execução automática |
| M03 | Após M01, inspecionar o banco                                                                                                                           | DDL já executado **não** revertido (sem rollback de commit implícito); estado real registrado como evidência                        |
| M04 | Recuperar conforme política: concluir manualmente e marcar registro como `applied` (ou reverter manualmente e limpar registro); repetir `RunMigrations` | Execução segue da versão correta até a desejada                                                                                     |
| M05 | PostgreSQL e SQLite: sequência com falha deliberada em arquivo                                                                                          | Rollback integral do percurso; nenhum registro `in_progress` remanescente; banco como antes da execução                             |
| M06 | Estado `0` e versão desejada                                                                                                                            | `TargetVersion: 0` reverte tudo; configurar migrações sem `Migrations` (ou com `ManagedObjects` vazio) retorna erro antes de executar |
| M07 | Banco sem histórico: declarar `ManagedObjects` inclui uma tabela existente; repetir com `ManagedObjects` apontando só para tabelas ausentes               | Com objeto gerido presente → erro antes de executar; sem nenhum objeto gerido presente → versão atual `0` e percursos normais       |

## 5. Registro de evidências

| Banco      | Driver/versão | Servidor/versão | Go/revisão Krios | Cenários | Resultado     | Pendências      |
| ---------- | ------------- | --------------- | ---------------- | -------- | ------------- | --------------- |
| MySQL      | A preencher   | A preencher     | A preencher      | Nenhum (V01–V23) | Não executado | Integração real |
| PostgreSQL | A preencher   | A preencher     | A preencher      | Nenhum (V01–V23) | Não executado | Integração real |

Para cada cenário, guardar: data, configuração sem segredos, passos executados, resultado esperado, resultado observado e limitação do ambiente. Falha conhecida deve gerar correção e nova evidência. Ao completar os cenários aplicáveis, atualizar E02-P04 e a validação final E09; ausência de evidência não deve ser marcada como sucesso.
