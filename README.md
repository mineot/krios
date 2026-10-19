# Krios

Biblioteca Go em desenvolvimento para acesso a SQLite3, MySQL e PostgreSQL, com configuração em código, transações, consultas e migrações.

O projeto já oferece configuração em código (`krios.Config`), abertura e encerramento do acesso (`Open`/`Close`) e transações explícitas (`Begin`/`Commit`/`Rollback` com execução em `*DB` e `*Tx`). Consultas encadeadas, SQL incorporado e migrações ainda serão implementados.

## Desenvolvimento local

- Versão mínima: Go 1.26.0.
- Ambiente de desenvolvimento previsto: atualização mais recente da linha Go 1.27.
- Compatibilidade a validar: atualizações mais recentes das linhas Go 1.26 e 1.27.
- `make` é opcional; os comandos Go também podem ser executados diretamente.

Execute os comandos a partir da raiz do repositório. Não são necessários servidores de banco nem arquivos externos de configuração. O Go baixa o driver e suas dependências de teste conforme `go.mod` e `go.sum` na primeira execução.

| Comando | Finalidade | Comando Go equivalente |
| --- | --- | --- |
| `make fmt` | Formatar o código, alterando os arquivos necessários | `go fmt ./...` |
| `make fmt-check` | Rejeitar código fora do padrão, sem alterar arquivos | `gofmt -l .` lista os arquivos; o alvo falha se a lista não estiver vazia |
| `make build` | Compilar todos os pacotes | `go build ./...` |
| `make vet` | Executar análise estática | `go vet ./...` |
| `make test` | Executar os testes existentes | `go test ./...` |
| `make check` | Verificar formatação, compilação, análise estática e testes | Executar as verificações acima, exceto a formatação que altera arquivos |
| `make dev-db` | Criar ou abrir `dev.db` localmente, preservando os dados existentes | `KRIOS_DEV_DB_PATH="$PWD/dev.db" go test -run '^TestPrepareDevelopmentDatabase$' -count=1 -v ./tests` |

Antes de concluir uma alteração, execute:

```sh
make fmt
make check
```

Executar apenas `make` também chama `check`. Qualquer falha nas verificações faz o comando retornar erro.

`go build ./...` verifica os pacotes do projeto. O exemplo executável está em `cmd/krios` e pode ser executado com `go run ./cmd/krios`. Os testes atuais cobrem o ambiente SQLite (escrita parametrizada e persistência após reabrir o arquivo), a configuração (`krios.Config`), o ciclo de vida (`Open`/`Close`) e as transações: abertura e encerramento, configuração inválida, driver ausente, cancelamento, prazo esgotado, falha de conexão, `Begin`/`Commit`/`Rollback`, rejeição de operações após o fim da transação (`ErrTxDone`), confirmação e reversão de grupo de inserts relacionados, isolamento entre transação e instância principal, `Commit` bloqueado com auto-rollback após falha de statement e concorrência serializada na mesma transação. Consultas encadeadas, SQL incorporado e migrações ainda não estão implementados.

## Organização

- `go.mod`: módulo `github.com/mineot/krios` e versão mínima de Go.
- `krios.go`: pacote público `krios` — documentação, `krios.Config`, tipos por banco, padrões, validações, erros exportados, ciclo de vida (`Open`/`Close`) e transações (`Begin`/`Commit`/`Rollback`, `Result`/`Rows`/`Row`).
- `krios_test.go`: testes do pacote e exemplo de configuração em Go.
- `cmd/krios/main.go`: exemplo executável separado da biblioteca.
- `tests/sqlite_test.go`: validação isolada do driver e preparação opcional do banco local.
- `tests/lifecycle_test.go`, `tests/lifecycle_three_test.go`: testes de caixa-preta do ciclo de vida.
- `tests/tx_test.go`, `tests/tx_group_test.go`: testes de transações, grupo de inserts e concorrência.
- `Makefile`: comandos de desenvolvimento local.
- `.github/workflows/check.yml`: verificações em pushes e pull requests.
- `.specs/`: requisitos, plano de execução e decisões técnicas.

Novos arquivos e pacotes serão criados conforme a implementação exigir. Os drivers serão escolhidos e importados pelo consumidor; a biblioteca não os importará automaticamente. A configuração é fornecida em código pela struct `krios.Config` e entregue a `Open`.

## SQLite local e testes

Os testes usam `modernc.org/sqlite v1.60.1`, resolvido por `@latest` em 18/10/2026 e fixado no módulo. O driver incorpora SQLite **3.53.4**, confirmado por `SELECT sqlite_version()`. O uso dessa versão foi aceito pelo consumidor em 18/10/2026. A política é acompanhar a versão estável do driver, usando o SQLite incorporado por ela, sem exigir coincidência com a versão upstream mais recente. As atualizações da dependência devem ser deliberadas e validadas.

Para preparar o banco local:

```sh
make dev-db
```

O comando cria ou abre `dev.db` na raiz, sem apagar nem reinicializar dados existentes. O banco e seus arquivos auxiliares estão no `.gitignore`. A preparação é feita por um auxiliar em `_test.go`, habilitado apenas por esse comando, para manter o driver fora da compilação normal.

`make test` e `make check` usam `t.TempDir()` para criar bancos temporários próprios. Não abrem nem removem `dev.db`. O auxiliar de preparação local é ignorado normalmente e rejeita execução quando a variável `CI` está definida.

O driver aparece no `go.mod`, mas seu único import está em `_test.go`. Ele não faz parte das dependências compiladas da biblioteca ou do exemplo executável. O consumidor continua escolhendo o driver da sua aplicação.

## Integração contínua

O GitHub Actions executa `make check` em pushes e pull requests, nas atualizações mais recentes das linhas Go 1.26 e 1.27, usando `ubuntu-latest` e SQLite sem CGO. O workflow tem permissões somente de leitura, tempo máximo de 15 minutos por job e cancela execuções anteriores da mesma referência. Não publica artefatos nem utiliza cache persistente do Actions.

Para respeitar a condição de gratuidade, os jobs executam **somente quando o repositório é público**. Em repositórios privados, são ignorados: a franquia gratuita e a eventual cobrança dependem da conta. Não há alteração automática de visibilidade ou faturamento. Consulte a [política de cobrança do GitHub Actions](https://docs.github.com/en/billing/concepts/product-billing/github-actions).

Em 18/10/2026, o consumidor confirmou o sucesso dos checks remotos nas linhas Go 1.26 e 1.27 após enviar os arquivos ao GitHub. O ambiente remoto não acessa o `dev.db` do seu computador.

MySQL e PostgreSQL serão testados manualmente pelo consumidor posteriormente; testes unitários também cobrirão as regras e a geração de SQL dos dialetos.

## Documentação do projeto

- [Planejamento e requisitos](.specs/0001_planning.md)
- [Plano de execução e progresso](.specs/0002_execution_plan.md)
- [Decisões da base técnica — E01-P01](.specs/E01-P01_decisions.md)
- [Decisões da configuração — E02-P01](.specs/E02-P01_decisions.md)
- [Decisões dos contratos de execução — E03-P01](.specs/E03-P01_decisions.md)

## Licença

[Apache License 2.0](LICENSE).
