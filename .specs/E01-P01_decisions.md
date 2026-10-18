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
