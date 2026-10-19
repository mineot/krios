# E09 — Roteiro manual da base E01–E03

Data: 2026-10-19. **Estado: preparado; MySQL e PostgreSQL ainda não executados.**

Este roteiro coleta evidência real de conexão e transações. DSNs e drivers simulados já têm testes locais, mas não substituem este roteiro. Construtores, SQL incorporado e migrações serão acrescentados quando E04–E08 estiverem implementados.

## 1. Preparar e registrar o ambiente

Usar um banco exclusivo de validação, com usuário autorizado a criar/remover as tabelas do roteiro. Para MySQL, usar tabelas InnoDB. O roteiro só remove a tabela que criou; se o nome já existir, a criação deve falhar antes de registrar a limpeza.

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

## 3. Cenários complementares

| ID | Procedimento | Resultado esperado |
| --- | --- | --- |
| V01 | Executar o consumidor acima em cada banco | Mensagem de sucesso; grupos confirmados/revertidos; operação externa preservada |
| V02 | Criar usuário de teste com senha contendo `@`, `:`, `%` e espaço; repetir V01 | Autenticação com senha literal, sem percent-encoding manual |
| V03 | Criar banco com nome contendo `%` e espaço, se aceito pelo servidor; preencher nome literal | Acesso ao banco exato; não a um nome decodificado incorretamente |
| V04 | Repetir V01 com host `::1`, se o servidor escutar em IPv6 | Endereço composto corretamente; registrar indisponibilidade do ambiente separadamente |
| V05 | Antes de `Open`, cancelar o contexto; repetir com deadline passado | Erro de contexto verificável, sem `ErrConnectionFailed` |
| V06 | Executar `SELECT SLEEP(5)` (MySQL) ou `SELECT pg_sleep(5)` (PostgreSQL) com contexto de 100 ms, via Query/Scan | Cancelamento/prazo conforme driver; registrar duração e causa; fechar resultados e reverter Tx quando utilizada |
| V07 | Abrir consulta com várias linhas, ler somente a primeira e fechar; depois fazer nova consulta | Recurso liberado; nova consulta utilizável; consultar `Err` após fechar |
| V08 | Dentro de Tx, provocar erro de conversão em `Scan` e solicitar rollback explicitamente | Erro de leitura preservado; escritas anteriores do grupo desfeitas |
| V09 | Enquanto Tx contém insert ainda não confirmado, consultar via DB; depois rollback | Operação no DB não participa do grupo nem enxerga escrita não confirmada no isolamento configurado; registrar bloqueios/isolamento |
| V10 | Consumir/fechar resultados e finalizar; repetir todos os métodos na Tx | `ErrTxDone`, inclusive em `Row.Scan` |
| V11 | Em ambiente descartável, interromper a conexão durante confirmação, se houver infraestrutura para controle | Erro preservado; objeto terminal; estado remoto registrado como incerto até consulta independente |
| V12 | Dentro de Tx, executar escrita e depois leitura com erro de conversão (ex.: texto em `int`) ou erro de iteração; tentar `Commit` sem rollback explícito | `Commit` bloqueado com auto-rollback (E03-P04): erro original recuperável por `errors.Is`, escrita não persistida, Tx terminal. Repetir com consulta sem linhas (`ErrNoRows`) → `Commit` **não** bloqueado |

V11 não autoriza repetir automaticamente a escrita: confirmar a situação por outra conexão. Se o ambiente não permitir injeção controlada, registrar o cenário como não executado; os testes simulados continuam sendo evidência somente do contrato local.

### 3.1 Cenários de migrações — estado parcial (E08)

Aplicam-se quando E08 estiver implementado; ver [E08-P01_decisions.md](E08-P01_decisions.md).

| ID | Procedimento | Resultado esperado |
| --- | --- | --- |
| M01 | MySQL: migração com DDL não-transacional seguido de comando que falha no meio do arquivo | Arquivo registrado com `in_progress`/`failed`; erro com contexto; `applied_at` não preenchido para o arquivo falho |
| M02 | Após M01, chamar `RunMigrations` novamente | Bloqueio com `ErrPartialMigration` (`Direction`, `TargetVersion`, `LastVersion`, `LastFile`, `Status`); nenhuma execução automática |
| M03 | Após M01, inspecionar o banco | DDL já executado **não** revertido (sem rollback de commit implícito); estado real registrado como evidência |
| M04 | Recuperar conforme política: concluir manualmente e marcar registro como `applied` (ou reverter manualmente e limpar registro); repetir `RunMigrations` | Execução segue da versão correta até a desejada |
| M05 | PostgreSQL e SQLite: sequência com falha deliberada em arquivo | Rollback integral do percurso; nenhum registro `in_progress` remanescente; banco como antes da execução |
| M06 | Estado `0` e ausência de versão desejada | Reversão total apenas em versão `0` explícita; versão ausente retorna erro antes de executar |

## 4. Registro de evidências

| Banco | Driver/versão | Servidor/versão | Go/revisão Krios | Cenários | Resultado | Pendências |
| --- | --- | --- | --- | --- | --- | --- |
| MySQL | A preencher | A preencher | A preencher | Nenhum | Não executado | Integração real |
| PostgreSQL | A preencher | A preencher | A preencher | Nenhum | Não executado | Integração real |

Para cada cenário, guardar: data, configuração sem segredos, passos executados, resultado esperado, resultado observado e limitação do ambiente. Falha conhecida deve gerar correção e nova evidência. Ao completar os cenários aplicáveis, atualizar E02-P04 e a validação final E09; ausência de evidência não deve ser marcada como sucesso.
