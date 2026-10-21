# C06-P02 — Revisão dos cenários de migração (D42–D44)

Data: 2026-10-21. Base `efb3869`, mais alterações locais desta tarefa; nenhum commit criado. Entrega documental: [M01–M14](C06-P02_migration_scenarios.md), integrada ao [roteiro E09](E09_manual_validation.md) §4.1. Código novo restrito a consumidor observacional no módulo separado de C06-P01; biblioteca intacta.

## D42 — Preservar IDs e distinguir escopos

M01 passa a usar dois arquivos de um statement para reproduzir parcialidade MySQL sem depender de multi-statement. M02/M03 conferem bloqueio e efeitos reais. M04 explicita recuperação por reversão do prefixo com reconciliação prévia e alteração seletiva do histórico. M05 exige rollback da versão em execução e preservação das anteriores; M06 distingue Open sem Migrations de RunMigrations sem configuração, alvo nil de zero explícito e validação de preparação. M07 separa nomes simples/qualificados e erros de inspeção.

Não se exige ErrPartialMigration na primeira falha de arquivo transacional MySQL: ela retorna erro comum, mas o prefixo persistido faz a próxima preparação bloquear. M09 registra essa diferença. M10 separa diagnóstico estruturado do estado realmente persistido quando a própria escrita de histórico falha.

## D43 — Cenários adicionais e evidência

M08 cobre R01–R04, segunda reversão/reaplicação e múltiplos arquivos por versão; M09 trata escopo transacional por arquivo; M10 falhas de registro/limpeza; M11 down não contíguo; M12 histórico incompatível; M13 multi-statement com suporte comprovado; M14 cancelamento coordenado. Os procedimentos especificam estado inicial, injeção, observações, resultado esperado e limpeza.

A API pública não oferece hooks de falha de histórico/commit. Para M10/M14, o catálogo aponta comandos/testes simulados exatos e o mecanismo necessário para E10 preparar evidência real; não se apresenta simulação como servidor nem se sugere temporização arbitrária. Não foram provisionados bancos, usuários ou serviços externos.

## D44 — Achado A16: segunda reversão produz sucesso aparente

**Severidade alta; aberto, bloqueador da passagem C07-P03.** Ao ampliar M08 além de uma reaplicação, o consumidor público SQLite reproduziu:

```sh
cd .specs/consumers/e06e07
go run ./cmd/migration-cycles -targets 1,0,1,0
```

Resultado observado, sem depender de endereços de memória impressos no plano:

| Alvo | CurrentVersion do plano | Passos | Erro RunMigrations | parent presente |
| --- | --- | --- | --- | --- |
| 1 | 0 | up | nil | sim |
| 0 | 1 | down | nil | não |
| 1 | 0 | up | nil | sim |
| 0 | 0 | nenhum | nil | **sim** |

O programa detecta a divergência, imprime `schema does not match target 0` e termina com status 1. Após a última fase, o histórico ainda contém up e down applied. Esperado: a reaplicação deve restabelecer versão efetiva 1; o novo alvo 0 deve executar down e remover parent.

Inspeção associada: `effectiveVersion` em `internal/migrate/migrate.go` exige completeUp e ausência de completeDown; `insertInProgress` em `internal/migrate/history.go` substitui a linha up do próprio arquivo, mas a linha down antiga permanece. Isso explica o estado observado. A correção deve revisar a validade das direções históricas e ciclos repetidos, preservando compatibilidade/recuperação e evitando conflitos de chave em down. Este diagnóstico não define antecipadamente a solução.

A16 deve ser corrigido com regressões antes de C07-P03; E10-P03 também deve executar os ciclos estendidos nos servidores. Não houve mudança de escopo nem redução do resultado exigido. C06-P02 conclui a **preparação** dos cenários, não a correção desse defeito; C06-P03 pode preparar registros enquanto a correção está pendente. A marcação histórica de C02 e a evidência C05-P02 continuam limitadas aos percursos efetivamente testados.

## Verificações locais observadas

Go 1.26.8, Linux amd64, base `efb3869`:

- `go run ./cmd/migration-cycles -targets 1,0,1,0`: falha de contrato A16 reproduzida em SQLite real; saída acima. As três fases iniciais passaram, a quarta falhou.
- `go run ./cmd/migration-cycles -targets 2,1,2`: aprovado; versão 2 tem dois arquivos up, esquema/seed conferidos em reaberturas.
- `go run ./cmd/migration-cycles -targets 1,1`: aprovado, segunda fase sem passos.
- `go vet ./...` no módulo consumidor: aprovado. Houve repetição autorizada após bloqueio de gravação do cache no sandbox.
- `go test -count=1 -run 'TestMigrations(ReproC02|FailureRollsBackSQLite|SchemaWithoutHistory|NotConfigured|MissingDownDirection)' ./tests`: aprovado, SQLite real; não cobre a quarta fase de A16.
- `go test -count=1 ./internal/migrate`: aprovado, testes unitários/simulados; não elimina A16.
- Go 1.27, MySQL/PostgreSQL reais e CI remoto: não executados nesta tarefa. M08 estendido com duas versões: preparado, não executado nesta tarefa.

O novo comando só usa banco temporário próprio e o remove ao terminar, inclusive em falha; o histórico é impresso antes da remoção. O módulo principal e suas dependências não mudaram. Não se adicionou teste permanentemente falhando à suíte principal: o consumidor reproduz o defeito explicitamente e preserva o oráculo para a regressão da futura correção.
