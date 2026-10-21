# C05-P02 — Exemplos de consumo conferidos (D30–D34)

Data: 2026-10-21. Escopo: [C05-P02](0005_consolidation_plan.md#7-documentação-exemplos-e-rastreabilidade--c05), com reconciliação de A09/A10. Base: `7b7135f` mais alterações locais; nenhum commit foi criado nesta tarefa. Complementa [C05-P01](C05-P01_decisions.md), [E06-P01](E06-P01_decisions.md), [E07-P01](E07-P01_decisions.md) e [E08-P01](E08-P01_decisions.md).

## D30 — Origem dos trechos, assinaturas e leitura

[0001_use_examples.md](0001_use_examples.md) distingue blocos Go extraídos literalmente de `examples/` de fragmentos ilustrativos. A compilação e execução do pacote sustentam os primeiros; os fragmentos ilustrativos não são apresentados como programas compilados. Os 25 blocos extraídos foram comparados com os arquivos citados, como substrings contíguas.

`Exec` devolve `(Result, error)`; `QueryContext` devolve `(*Rows, error)` e `QueryRowContext` devolve `Row`, com erro observado em `Scan`. `One`/`File.One` leem posicionalmente: a seleção deve respeitar a ordem dos campos mapeados do destino. `All`/`File.All` mapeiam por nome e encerram o iterador conforme C04, preservando o destino em falha. `sql.Scanner` e `time.Time` mantêm tratamento escalar.

O documento de E07 foi reconciliado: arquivo vazio/ausente em `NewSQLFile` produz `ErrFileNotFound`; runner sem arquivo ou executor produz `ErrInvalidQuery`. Não houve mudança de contrato.

## D31 — Limpeza de transações

O comentário de `CreateUserWithFirstTasks` e o documento explicam que `defer tx.Rollback(ctx)` descarta o erro retornado. Depois de `Commit`, esse erro é `ErrTxDone`; não se promete retorno nil. A variante ilustrativa mostra como distinguir esse sentinela de falha de limpeza.

## D32 — Validação automática

`SetValues` executa os validadores. Foram removidas chamadas redundantes de validação em `CreateUser`, `CreateTask` e nas tarefas do grupo. A validação do usuário antes de `Begin` permanece para evitar abrir uma transação para registro inválido. `ValidateUsers` continua demonstrando validação explícita de lotes.

Estas mudanças afetam o exemplo consumidor; nenhuma assinatura ou implementação de `internal/` foi modificada.

## D33 — Valores, NULL e omissão

A tabela do documento agora distingue INSERT e UPDATE, conforme `internal/entity/write.go` e os builders:

| Campo | INSERT | UPDATE |
| --- | --- | --- |
| Primitivo, inclusive zero | Grava valor | Grava valor |
| Ponteiro nil | Grava NULL | Preserva coluna |
| Ponteiro não nil | Grava valor apontado | Grava valor apontado |
| `sql.Null*` inválido | Grava NULL | Grava NULL |
| Coluna em `Omit` | Default do banco | Preserva coluna |

O rascunho local dizia incorretamente que zeros eram omitidos e recomendava `Set`, método inexistente. Essas afirmações foram corrigidas antes da conclusão. `Omit("id")` permite gerar o identificador no INSERT; `Omit("id", "title", "done")` restringe o UPDATE de transferência a `owner_id`. A disponibilidade de `Result` continua comunicada pelo booleano de cada método.

## D34 — Migrações executáveis

[examples/migrations.go](../examples/migrations.go) embute duas versões com arquivos up/down, um statement por arquivo. `MigrationsConfig(target)` passa `&target`, versões ordenadas e `ManagedObjects` obrigatório. O ciclo abre o mesmo arquivo SQLite separadamente para os alvos `2 → 0 → 2`, observando plano, tabelas e histórico.

As tabelas `projects`/`project_members` são distintas de `users`/`tasks`, criadas por SQL embutido no exemplo E07. Assim os exemplos podem compartilhar o arquivo sem gerenciar as mesmas tabelas. A introspecção e o SQL destes exemplos são específicos de SQLite; configurações MySQL/PostgreSQL não comprovam execução nesses servidores.

[examples/migrations_test.go](../examples/migrations_test.go) confere o percurso `0 → 2 → 0 → 2`, existência/remoção/recriação dos objetos e conteúdo completo das linhas do histórico (versão, arquivo, direção e status), além de executar a narrativa `Migrate`. A reaplicação substitui as linhas up e mantém as linhas down, totalizando quatro registros neste ciclo.

A documentação distingue plano preparado de execução confirmada e registra que a preparação consulta o banco. Também esclarece o escopo por versão em SQLite/PostgreSQL, por arquivo no MySQL transacional e a possibilidade de versão parcial entre arquivos. `DB.Migrations()` expõe a configuração retida e mutável; o exemplo prefere reabrir com alvo explícito.

## Verificação e limites

Verificações locais em 2026-10-21, Linux amd64:

- `make check` e `make race`: aprovados com Go 1.26.8.
- `GOTOOLCHAIN=go1.27.2 make check race`: aprovado com Go 1.27.2.
- `go test -count=1 -run '^(TestRun|TestMigrate|TestMigrationsAdvanceRevertReapply)$' -v -parallel=1 ./examples`: aprovado em Go 1.26.8; saídas de `Run` e `Migrate` conferidas com as duas saídas documentadas.
- `go test -count=1 -run '^Example' ./tests`: aprovado em Go 1.26.8, cobrindo os exemplos executáveis existentes dos contratos centrais.
- Comparação literal dos 25 blocos Go classificados como extraídos: todos encontrados nos arquivos citados. `git diff --check`: aprovado.

O check normal reutilizou cache em parte dos pacotes; `make race` executou a suíte sem cache (`-count=1`) nas duas linhas. A tentativa inicial de usar o cache Go 1.27 no sandbox foi bloqueada pelo sistema de arquivos; a repetição autorizada concluiu. Não houve teste de regressão falhando antes de correção da biblioteca: as divergências desta tarefa eram documentais; o novo teste valida o exemplo consumidor e seu histórico real em SQLite.

C05-P03 permanece responsável pela matriz RF-01–RF-09 e classificação de adiamentos. C06 prepara cenários de servidores; E10 entrega integração real automatizada. CI remoto e publicação não fazem parte desta tarefa.
