# Krios

Biblioteca Go em desenvolvimento para acesso a SQLite3, MySQL e PostgreSQL por `database/sql`.

Implementados: configuração em código, abertura/fechamento, SQL parametrizado, transações explícitas e mapeamento de entidades por reflexão sobre structs (`krios.EntityOf`, `Row.Scan` para um registro e `Rows.All` para múltiplos). As consultas encadeadas, o SQL incorporado e as migrações pertencem às próximas entregas. A integração automatizada real usa SQLite; MySQL/PostgreSQL têm testes locais de regras/DSN e aguardam integração manual.

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
- A execução é explícita: chame os métodos quando precisar. O mesmo motor será reutilizado antes das escritas de E06.
- Falhas retornam `ErrValidation` (campo reprovado) ou `ErrFormat` (falha de formatação), contextualizadas com tabela e coluna e preservando a causa (`errors.Is`/`errors.As`). Destino inválido retorna `ErrInvalidEntity`.

Campos primitivos (`string`, `int`, …) não têm validação/formatação sem um tipo nomeado. O exemplo `Example_validationFormatting` demonstra o fluxo; as decisões estão em [E04-P03](.specs/E04-P03_decisions.md).

## Erros e propriedade dos resultados

- Tratar erros com `errors.Is`/`errors.As`. Erros do driver são preservados.
- `QueryRowContext(...).Scan(...)` preserva cancelamento, prazo, `ErrClosed` e `ErrTxDone`. `sql.ErrNoRows` significa consulta sem linha.
- Após `QueryContext`, registrar `defer rows.Close()`, tratar `Scan` e verificar `rows.Err()` depois da iteração. O erro continua disponível após fechar.
- `Result.RowsAffected()` e `LastInsertId()` retornam valor e disponibilidade; zero com `true` é válido. `false` inclui erro do driver ao obter essa informação, cujo detalhe não é exposto pela interface.
- Wrappers de acesso e resultados são obtidos por `Open`, `Begin` e consultas; seus valores zero não representam recursos prontos para uso.
- `ErrInvalidEntity` indica struct/tag/`TableName` inválidos na derivação ou destino de mapeamento malformado. Erros de mapeamento preservam a causa do driver.
- `ErrValidation` e `ErrFormat` indicam campo reprovado por `Validate()` ou falha em `Format()`, com tabela e coluna no contexto.

## Transações e concorrência

Após `Begin`, registrar `defer tx.Rollback(ctx)` para limpar também retornos antecipados. `Rollback(ctx)` tenta a limpeza mesmo com contexto cancelado; não é interrompido pelo contexto. Depois da finalização, novas chamadas retornam `ErrTxDone`.

`Commit(ctx)` rejeita contexto cancelado antes da tentativa nativa quando não há erro registrado anterior. A transação memoriza o primeiro erro observado — falha imediata de `ExecContext`/`QueryContext` ou erro tardio de leitura (`Row.Scan`, `Rows.Scan`, iteração e fechamento) — e, nesse caso, o commit é bloqueado e tenta rollback mesmo com contexto cancelado, preservando o primeiro erro e eventual erro de limpeza. `sql.ErrNoRows` não bloqueia o commit, para preservar o padrão de verificação de existência. O rollback explícito no primeiro erro continua sendo o comportamento recomendado; o bloqueio do commit é uma proteção adicional.

O contexto passado a `Begin` controla toda a vida da transação e seu cancelamento provoca reversão nativa. Falha de finalização torna o wrapper inutilizável e pode deixar o estado remoto incerto; não existe repetição automática de escrita.

`DB` permite uso concorrente. Chamadas na mesma `Tx` são serializadas, mas isso não coordena a leitura dos resultados. Use um consumidor por iterador, consuma/feche resultados antes de outra operação na mesma transação e aguarde os consumidores antes de finalizar. Não há ordem garantida entre goroutines.

## Organização

| Arquivo | Responsabilidade |
| --- | --- |
| `krios.go` | Fachada pública: aliases de tipos, constantes, erros e delegação de `Open` |
| `internal/core/config.go`, `dsn.go`, `errors.go` | Configuração, DSNs e erros |
| `internal/core/db.go`, `tx.go` | Conexão e transação |
| `internal/core/execution.go` | Contrato interno Krios e delegação nativa |
| `internal/core/rows.go`, `result.go` | Resultados e recursos de leitura |
| `internal/core/entity.go`, `mapping.go`, `validation.go` | Entidades por reflexão (tags, cache), mapeamento para `*T`/`*[]T` e validação/formatação por campo |
| `internal/core/*_test.go` | Testes internos; driver simulado com contadores/hooks |
| `tests/` | Integração SQLite e exemplos de consumo externo |
| `cmd/krios/main.go` | Placeholder que imprime uma saudação; ainda não é exemplo funcional de acesso |
| `.specs/` | Requisitos, decisões, melhorias e roteiro manual |

Drivers são importados somente nos testes do repositório; não entram na compilação normal da biblioteca nem do placeholder. `DB`/`Tx` não retêm a configuração completa sem uso. O pacote público permanece na raiz e delega à implementação em `internal/core`, que não é importada pelo consumidor. Os aliases preservam os tipos e métodos disponíveis pelo import `github.com/mineot/krios`; a fachada não duplica a lógica.

## SQLite local

Os testes usam `modernc.org/sqlite v1.60.1`, fixado no módulo. A seleção original e aceitação do SQLite incorporado foram registradas em E01-P03 em 18/10/2026. Atualizações são deliberadas; não há exigência de coincidir com a versão upstream mais recente.

`make dev-db` habilita um auxiliar de teste para criar/abrir `dev.db`; esse auxiliar é ignorado em testes normais e rejeita execução com `CI` definido. O banco e seus arquivos auxiliares estão no `.gitignore`. Não executar esse comando para validar uma alteração: use os bancos temporários da suíte.

## Integração contínua e evidências

O workflow possui jobs `check` (sem CGO) e `race` (CGO habilitado e compilador C), ambos nas linhas Go 1.26/1.27. Usa `ubuntu-latest`, permissões de leitura, timeout de 15 minutos por job e cancelamento de execuções antigas da mesma referência. Não publica artefatos nem utiliza cache persistente do Actions.

Os jobs executam somente quando o repositório é público, preservando a condição de gratuidade adotada pelo projeto. Em repositório privado são ignorados. Não há mudança de visibilidade ou faturamento; consulte a [política do GitHub Actions](https://docs.github.com/en/billing/concepts/product-billing/github-actions).

A revisão M01–M06 passou localmente por `make check CGO_ENABLED=0` e `make race` em Go **1.26.8** e **1.27.1**. As versões locais não são apresentadas como as últimas disponíveis. A execução remota do workflow atualizado permanece pendente. O relato de sucesso remoto em 18/10 é evidência histórica, anterior a estas alterações.

MySQL/PostgreSQL aguardam execução do [roteiro manual](.specs/E09_manual_validation.md). Testar DSNs e falhas simuladas não comprova conexão real. E04 pode avançar sobre a base corrigida; as evidências manuais continuam obrigatórias para concluir E09.

## Documentação do projeto

- [Planejamento e requisitos](.specs/0001_planning.md)
- [Plano de execução e progresso](.specs/0002_execution_plan.md)
- [Plano de melhorias e evidências M01–M06](.specs/0003_improvement_plan.md)
- [Base técnica — E01-P01](.specs/E01-P01_decisions.md)
- [Configuração — E02-P01](.specs/E02-P01_decisions.md)
- [Execução e transações — E03-P01](.specs/E03-P01_decisions.md)
- [Mapeamento por reflexão — E04-P01](.specs/E04-P01_decisions.md)
- [Implementação do mapeamento — E04-P02](.specs/E04-P02_decisions.md)
- [Validação e formatação — E04-P03](.specs/E04-P03_decisions.md)

## Licença

[Apache License 2.0](LICENSE).
