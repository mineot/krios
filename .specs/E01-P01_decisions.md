# E01-P01 — Decisões

Data: 2026-10-18

## Decisões

| Item | Decisão | Justificativa |
|-----|---------|---------------|
| Caminho do módulo | `github.com/mineot/krios` | Baseado no remote `origin https://github.com/mineot/krios.git`. |
| Versão mínima de Go | `go 1.26.0` | Desenvolvimento na atualização mais recente da linha 1.27; testes automatizados nas atualizações mais recentes das linhas 1.26 e 1.27. `go.mod` declara `go 1.26.0`. |
| Pacote público de entrada | Pacote `krios` na raiz | Mais idiomático e consistente com importação `github.com/mineot/krios`. |
| Divisão de responsabilidades (inicial) | Configuração, conexão, transações, migrações, entidades, consultas e dialetos. A divisão física em arquivos e pacotes é provisória. | As responsabilidades orientam a implementação; subpacotes devem ser mantidos ou criados conforme uma separação concreta os justifique. |
| Fronteira API vs interna | Público expõe: `Config`, tipos/abstrações necessárias p/ `Open`, `Close`, `Begin`, `Commit`, `Rollback`, entidades, construtores/resultados, configuração/execução de migrações. Interno mantém montagem SQL, dialetos, execução, validação. | Implementação interna não deve ser consumida fora do módulo. |

## Observações

- Não criamos assinaturas completas agora; apenas estrutura compilável.
- A pasta `internal/` e os pacotes que continham apenas documentação foram removidos. Subpacotes internos serão criados somente quando a implementação justificar essa separação; sua existência não é requisito para concluir E01-P01.
- O uso de reflexão para mapeamento não foi decidido. Sua necessidade será avaliada em E04, junto ao modelo de entidades e resultados; não há pacote reservado para esse mecanismo.
- Não há imports automáticos de drivers (regra já definida).
- Próximos passos seguem E01-P02 (rotinas locais) e validação.

## Revisão (2026-10-18)

Mudança **intencional** da organização de arquivos, preservando a decisão original acima para rastreabilidade.

| Item | Decisão original | Decisão revisada | Justificativa |
|-----|------------------|------------------|---------------|
| Organização física do pacote | `krios` na raiz com responsabilidades em **arquivos separados** (`config.go`, `errs.go`, `doc.go`) | Pacote público `krios` na raiz, com o código desta fase **consolidado em um único arquivo `krios.go`** (documentação do pacote, configuração e erros) | Reduzir a dispersão e facilitar a leitura humana nesta fase inicial |

Observações da revisão:

- O **caminho de import permanece `github.com/mineot/krios`**; a raiz do módulo não muda e nenhum exemplo de consumidor é afetado.
- Foi considerada e **descartada** a criação de uma subpasta (por exemplo `krios/` ou `core/`), que produziria import repetido como `github.com/mineot/krios/krios` ou `.../core`.
- Os testes permanecem em arquivos `_test.go` (`config_test.go`, `example_config_test.go`), pois o Go não permite código de teste fora de `_test.go`.
- O arquivo único vale para o código desta fase. Em **E02-P03**, `Open` e `Close` serão adicionados a `krios.go`; nas entregas seguintes (E03 a E08) essa organização pode ser reavaliada.

### Revisão (2026-10-18) — testes

Mudança **intencional** da organização dos testes do pacote, preservando as decisões anteriores para rastreabilidade.

| Item | Decisão anterior | Decisão revisada | Justificativa |
|-----|------------------|------------------|---------------|
| Arquivos de teste do pacote | `config_test.go` e `example_config_test.go` separados, o segundo no pacote externo `krios_test` | Concentrar os testes do pacote em um único `krios_test.go` no pacote `krios` | Organização e leitura mais simples nesta fase |

Observações da revisão:

- Um arquivo `_test.go` tem uma única cláusula `package`; como `krios_test.go` testa funções não exportadas (`validate`, `timeout`, `port`), ele usa **`package krios`**. O exemplo `ExampleConfig` passa a referenciar `Config` diretamente, **sem importar** `github.com/mineot/krios`; o exemplo continua compilando e executando, mas deixa de demonstrar o import externo.
- `tests/sqlite_test.go` **permanece** na pasta `tests`, no pacote `tests`, importando o driver. Isso preserva o isolamento do driver decidido em E01-P03 (o driver não entra na compilação normal da biblioteca nem do executável). A centralização recai apenas sobre os testes do pacote.
- Os testes de detalhes internos do pacote continuam junto ao código, conforme a convenção do Go; não são movidos para `tests/`.
