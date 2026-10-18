# Krios

Biblioteca Go em desenvolvimento para acesso a SQLite3, MySQL e PostgreSQL, com configuração em código, transações, consultas e migrações.

O projeto possui apenas a base do módulo. A API funcional ainda será implementada; os tipos e métodos descritos no planejamento não estão disponíveis para uso.

## Desenvolvimento local

- Versão mínima: Go 1.26.0.
- Ambiente de desenvolvimento previsto: atualização mais recente da linha Go 1.27.
- Compatibilidade a validar: atualizações mais recentes das linhas Go 1.26 e 1.27.
- `make` é opcional; os comandos Go também podem ser executados diretamente.

Execute os comandos a partir da raiz do repositório. Nesta etapa, não são necessários bancos de dados, drivers ou arquivos externos de configuração.

| Comando | Finalidade | Comando Go equivalente |
| --- | --- | --- |
| `make fmt` | Formatar o código, alterando os arquivos necessários | `go fmt ./...` |
| `make fmt-check` | Rejeitar código fora do padrão, sem alterar arquivos | `gofmt -l .` lista os arquivos; o alvo falha se a lista não estiver vazia |
| `make build` | Compilar todos os pacotes | `go build ./...` |
| `make vet` | Executar análise estática | `go vet ./...` |
| `make test` | Executar os testes existentes | `go test ./...` |
| `make check` | Verificar formatação, compilação, análise estática e testes | Executar as verificações acima, exceto a formatação que altera arquivos |

Antes de concluir uma alteração, execute:

```sh
make fmt
make check
```

Executar apenas `make` também chama `check`. Qualquer falha nas verificações faz o comando retornar erro.

Como o projeto é uma biblioteca, `go build ./...` valida a compilação dos pacotes e não gera um executável da aplicação. Ainda não há testes implementados; a mensagem `[no test files]` não comprova funcionalidades futuras.

## Organização

- `go.mod`: módulo `github.com/mineot/krios` e versão mínima de Go.
- `doc.go`: documentação do pacote público `krios`.
- `Makefile`: comandos de desenvolvimento local.
- `.specs/`: requisitos, plano de execução e decisões técnicas.

Novos arquivos e pacotes serão criados conforme a implementação exigir. Os drivers serão escolhidos e importados pelo consumidor; a biblioteca não os importará automaticamente. A configuração será fornecida pela struct `krios.Config`, ainda a implementar.

Os ambientes de bancos, drivers de validação e integração contínua serão preparados na E01-P03. A execução da matriz Go 1.26/1.27 também deverá ser configurada nessa etapa.

## Documentação do projeto

- [Planejamento e requisitos](.specs/0001_planning.md)
- [Plano de execução e progresso](.specs/0002_execution_plan.md)
- [Decisões da base técnica — E01-P01](.specs/E01-P01_decisions.md)

## Licença

[Apache License 2.0](LICENSE).
