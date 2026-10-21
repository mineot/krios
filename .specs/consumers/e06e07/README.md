# Consumidor C06-P01 — E06/E07

Módulo manual separado da biblioteca. Importa somente API pública de Krios e os drivers de teste. Os arquivos em `sql/{sqlite,mysql,postgres}` são incorporados por `go:embed`, um statement por arquivo (exceto `blank.sql`, recurso inválido intencional). O `replace` relativo aponta para a raiz deste checkout; não comprova instalação de versão publicada.

## Execução

Da raiz do repositório:

```sh
cd .specs/consumers/e06e07
go mod download
go vet ./...
go build -o /tmp/krios-c06-e06e07 .
go run . -db sqlite
```

SQLite usa um diretório temporário novo e o remove ao terminar. O binário em `/tmp` também pode ser executado de outro diretório: os arquivos SQL já estão incorporados. `go.mod`/`go.sum` fixam MySQL v1.10.1, pgx v5.9.2 e modernc SQLite v1.60.1; são versões escolhidas para este consumidor, não uma afirmação de versão mais recente. E10-P01 define os ambientes oficiais de integração.

Para servidor, preparar banco **exclusivo de teste**, usuário com permissão de CREATE/DROP/DML e portas padrão (3306/5432). Preencher `KRIOS_TEST_HOST`, `KRIOS_TEST_USER`, `KRIOS_TEST_PASSWORD`, `KRIOS_TEST_DATABASE` no ambiente local e executar:

```sh
go run . -db mysql
go run . -db postgres
```

As variáveis são opção deste consumidor, convertidas em `krios.Config` por `main.go`; não são exigência da biblioteca. Para porta diferente, preencher `Port` na configuração do consumidor antes de compilar. Nenhum serviço, conta ou banco é provisionado por este programa. Não registrar senha no relatório.

## Pré-condições e limpeza

O nome reservado é `krios_c06_entries`. Ele deve estar ausente antes de cada execução. `CREATE TABLE` não usa `IF NOT EXISTS`; se falhar, nenhuma limpeza é registrada e nenhum objeto preexistente é removido. Após CREATE bem-sucedido, o consumidor remove apenas essa tabela em defer, com contexto independente de 10 segundos; falha de limpeza é impressa e devolvida como erro. MySQL usa InnoDB. O contexto principal dura dois minutos.

As verificações interrompem a execução com erro/panic e executam os defers; encerramento forçado do processo pode exigir remoção manual da tabela, somente após confirmar que ela pertence a esta execução. O sucesso final exige dados esperados e limpeza bem-sucedida.

## Cenários e resultados

Os IDs W01–W07/F01–F05 são estáveis e complementam V01–V23/M01–M07 do [roteiro](../../E09_manual_validation.md). A ordem é a do programa; W06 conclui sua exclusão múltipla após F05.

| ID | Procedimento e resultado exigido | Rastreabilidade |
| --- | --- | --- |
| W01 | INSERT de zeros (`0`, `false`), ponteiro nil e `sql.NullString` inválido; ler e conferir zero/NULL. Segundo INSERT omite colunas: defaults 7/true/`default` persistem. | RF-07.a, CA-08 |
| W02 | UPDATE com zero/false, ponteiro nil e NullString inválido; zero é gravado, ponteiro nil preserva `note`, nullable vira NULL; ID omitido permanece. | RF-07.a, CA-08 |
| W03 | Na Tx, rejeitar Validator inválido e UPDATE/DELETE sem Where; Commit funciona e snapshot ordenado permanece idêntico. | RF-07.a, RF-03, CA-08 |
| W04 | SQLite/PostgreSQL: Insert.Returning.One e Update/Delete.Returning.All conferem registros e efeitos. MySQL: os três retornam ErrInvalidQuery e snapshot permanece idêntico. Sem skips para MySQL. | RF-07.b, CA-13 |
| W05 | INSERT confirmado e DELETE revertido dentro de Tx; leituras no DB após finalização confirmam os efeitos. | RF-03, RF-07, CA-04 |
| W06 | UPDATE atinge duas linhas; DELETE final atinge todas as quatro remanescentes. Conferir dados e tabela vazia, registrando Result sem presumir disponibilidade. | RF-07.a/b, CA-08/13 |
| W07 | INSERT com ID omitido, chave gerada pelo banco; recuperar pela coluna name. Se LastInsertId estiver disponível, comparar com ID persistido. Remover registro. | RF-07.b, CA-13 |
| F01 | File.Exec com dois parâmetros; File.One seleciona id/name na ordem do destino; File.All seleciona name/id e mapeia por nome. Conferir valores reais. | RF-08/09, CA-09 |
| F02 | Recurso ausente/branco → ErrFileNotFound; File(nil).Exec → ErrInvalidQuery dentro de Tx; Commit permitido e snapshot intacto. | RF-08, CA-09 |
| F03 | File.Exec/One/All na mesma Tx veem escrita não confirmada. Rollback preserva snapshot; segunda Tx confirma e leitura externa encontra o registro. | RF-03/08, CA-04/09 |
| F04 | Escrever e converter texto não numérico para int64 via File.QueryRow.Scan; erro tardio preservado em Commit, escrita revertida e snapshot intacto. Em outra Tx, File.One sem linhas retorna sql.ErrNoRows e Commit funciona. | RF-03/08/09, CA-04/09 |
| F05 | Arquivo válido como recurso, mas SQL inválido: Exec falha, Commit preserva causa e desfaz escrita anterior; snapshot intacto. | RF-03/08, CA-04/09 |

Não se compara `RowsAffected` de UPDATE sem mudança entre drivers: W06 altera os valores, e a verificação principal lê o estado persistido. `LastInsertId` é interpretado como chave gerada somente em W07; números impressos após UPDATE/DELETE não são promessa de significado. A disponibilidade (`available`) é registrada, inclusive false.

`Wxx/Fxx PASS` confirma apenas as verificações daquele caminho/banco. O resultado final é `C06-P01 PASS <banco>`. Se qualquer etapa falhar, guardar ID, erro, estado observado e limpeza; não contar os cenários subsequentes como executados.

## Evidência

Ver [C06-P01_decisions.md](../../C06-P01_decisions.md). Compilar com os drivers não prova conectividade nem comportamento do servidor. A construção deste consumidor não inicia a automação de infraestrutura/CI de E10. C06-P02 continua responsável pelos ciclos e falhas de migração; C06-P03 consolidará o formato de registro de ambiente/resultados.

## Migrações — C06-P02

O comando adicional `go run ./cmd/migration-cycles -targets 1,0,1` observa
M08 em SQLite isolado, com SQL incorporado e reabertura entre alvos.
`-targets 2,1,2` inclui versão com dois arquivos up. O percurso
`-targets 1,0,1,0` reproduz o defeito A16 e termina com código não zero;
não integra os marcadores PASS de W/F. Ver [catálogo M01–M14](../../C06-P02_migration_scenarios.md)
e [evidência/limites](../../C06-P02_decisions.md). Os demais cenários continuam
com procedimentos e reproduções indicados no catálogo; o comando não é
harness completo nem validação de servidores.
