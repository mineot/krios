# C05-P03 — Rastreabilidade e revisão dirigida (D35–D38)

Data: 2026-10-21. Base `9dd207e`, inicialmente sem alterações locais. Escopo: C05-P03 do [plano de consolidação](0005_consolidation_plan.md). Entrega: [matriz RF/CA/API/adiamentos](C05-P03_traceability.md). Não altera assinaturas nem comportamento da biblioteca.

## D35 — Implementação e evidência são dimensões separadas

A matriz desdobra os nove RF e relaciona os treze critérios originais CA-01–CA-13. Cada comportamento aponta para contrato vigente, implementação, teste/exemplo, evidência e etapa responsável pela pendência. A seção de critérios herda essas referências, sem duplicar contratos.

L1 identifica execução atual local; L2, reprodução pontual; H1, resultados históricos de C05-P02. Testes de SQL e drivers simulados não comprovam MySQL/PostgreSQL reais. C07 verifica a passagem e o CI existente; C06 prepara cenários; E10 automatiza/mede nos servidores e E09 confere/distribui. Não se declara a versão publicada apenas porque o import local compila.

## D36 — Adiamentos não apagam obrigações

Os IDs D01–D11 na seção 5 da matriz são classificações locais de adiamentos, distintos da numeração global de decisões D35–D38 deste documento.

`ColumnTypes` continua ausente. E04/E05 o encaminharam a E07, enquanto M06 dizia avaliar sua necessidade. Não há resolução posterior localizada: a classificação conservadora é pendência contratual/definição ainda não atendida, com avaliação em E09-P01. RF-09 não exige nominalmente esse método, por isso a matriz não infere que todo o mapeamento esteja incompleto. E09 deve implementar se necessário ou registrar explicitamente uma mudança de escopo; esta entrega não toma essa decisão nem acrescenta API.

Alias de tabela/esquema e self-join são extensões já registradas em E05; não se confundem com aliases de resultado nem com nomes qualificados de objetos gerenciados nas migrações. Reflexão no lugar da DSL e escopo transacional por versão/arquivo são mudanças registradas. Integração real e distribuição continuam obrigações pendentes, não extensões.

## D37 — Revisão pública dirigida e achados

Foram inspecionados fachada, métodos dos aliases públicos e contratos das áreas, com testes indicados na matriz. O inventário agrupa operações; E10-P02 detalhará os cenários por banco. Não é auditoria exaustiva de todos os caminhos internos.

| Achado | Impacto e evidência | Tratamento |
| --- | --- | --- |
| A12 — comentário de `ErrPartialMigration` inclui banco sem histórico com objetos | Médio: consumidor pode testar o sentinela errado. `verifyNoManagedObjects` retorna `ErrInvalidQuery`; reprodução pública abaixo confirma. `TestMigrationsSchemaWithoutHistory` já verifica rejeição sem criação do histórico, mas não o sentinela. | Corrigidos apenas comentários em `krios.go` e `internal/errs/errors.go`; nenhum comportamento muda. |
| A13 — `ColumnTypes` encaminhado, ausente e sem encerramento da decisão | Médio: obrigação documental poderia desaparecer sob “extensão”. Reprodução por reflexão abaixo retorna false; busca em E04/E05/0003 estabelece os encaminhamentos. | Mantido aberto como D01 na matriz, responsabilidade E09-P01; notas nos documentos históricos preservam rastreabilidade. |
| A14 — C05-P02 descrevia `QueryRowContext` como `*Row` | Baixo: assinatura textual incorreta. `internal/access/db.go` e `internal/transaction/tx.go` retornam `scan.Row` por valor. | Corrigido o registro de C05-P02; código intacto. |
| A15 — planejamento ainda dizia pendentes contratos já resolvidos | Baixo: RF-08 e §6 misturavam histórico com estado atual de builders, dialetos e validação. Comparação com E05/E06/E07/E08 reproduz a divergência. | Referências atualizadas no planejamento; pendências reais continuam na matriz. |

Não foram introduzidos testes permanentes para alterações de comentários/documentação. A reprodução observacional de A12/A13 é independente de qualquer mudança de código funcional.

### Reprodução L2

Salvar o programa abaixo como `/tmp/krios-c05p03-probe.go` e executar `go run /tmp/krios-c05p03-probe.go` a partir da raiz do módulo. Usa SQLite em memória e apenas API pública; não acessa `dev.db`.

```go
package main
import (
 "context"
 "errors"
 "fmt"
 "reflect"
 "testing/fstest"
 "github.com/mineot/krios"
 _ "modernc.org/sqlite"
)
func main() {
 ctx := context.Background()
 target := 1
 db, err := krios.Open(ctx, krios.Config{Type:krios.DBTypeSQLite, Driver:"sqlite", SQLite:&krios.SQLiteConfig{DBPath:":memory:"}, Migrations:&krios.MigrationsConfig{FS:fstest.MapFS{"up.sql":&fstest.MapFile{Data:[]byte("CREATE TABLE users(id INTEGER)")}},TargetVersion:&target,Versions:[]krios.MigrationVersion{{Version:1,Up:[]string{"up.sql"}}},ManagedObjects:[]string{"users"}}})
 if err != nil { panic(err) }; defer db.Close()
 if _,err = db.ExecContext(ctx,"CREATE TABLE users(id INTEGER)");err != nil { panic(err) }
 _,err = db.RunMigrations(ctx)
 fmt.Printf("preexisting: invalid_query=%t partial=%t\n", errors.Is(err,krios.ErrInvalidQuery),errors.Is(err,krios.ErrPartialMigration))
 _,ok:=reflect.TypeOf((*krios.Rows)(nil)).MethodByName("ColumnTypes")
 fmt.Printf("Rows.ColumnTypes=%t\n",ok)
}
```

Saída observada com Go 1.26.8:

```text
preexisting: invalid_query=true partial=false
Rows.ColumnTypes=false
```

## D38 — Conclusão da tarefa e limites da passagem

C05-P03 encerra quando todos os RF/CA têm referências ou lacunas explícitas, os adiamentos estão classificados e os achados têm impacto/reprodução/destino. Isso permite concluir a documentação de C05 com D01 ainda aberto; não permite concluir E09 nem publicar com uma obrigação sem resolução. Nenhum método novo, alteração de contrato ou correção funcional foi incorporado automaticamente.

### Verificação atual

- `go version`: `go1.26.8 linux/amd64`.
- `go test -count=1 ./...`: aprovado, sem cache, base `9dd207e`; executado antes das alterações de comentários e documentos, sem alterações funcionais posteriores.
- Reprodução L2: saída acima, executada nesta tarefa.
- H1: check/race Go 1.26.8/1.27.2 de C05-P02 permanece histórico; não foi reapresentado como execução nova.
- Links locais dos dois documentos novos, nomes de testes da matriz e presença de RF-01–RF-09/CA-01–CA-13: verificação por script aprovada. `gofmt -l krios.go internal/errs/errors.go` sem saída e `git diff --check` aprovado.
- MySQL/PostgreSQL reais, CI remoto e consumidor de versão publicada: não executados; pendências explícitas em E10/E09.

Nenhum commit foi criado nesta tarefa. Os planos registram a conclusão documental e conservam C06/C07/E10/E09 pendentes.
