# E05-P04 — Decisões: Validação da construção e execução

Data: 2026-10-19. Encerra a entrega E05, cobrindo o que E05-P01–E05-P03 implementaram. Depende de [E05-P01_decisions.md](E05-P01_decisions.md) e [E05-P03_decisions.md](E05-P03_decisions.md).

## 1. Contexto

E05-P04 não adiciona capacidade nova: verifica o SQL gerado, os parâmetros e a execução do construtor de leitura. O padrão segue [E04-P04_decisions.md](E04-P04_decisions.md): evidência automatizada local em SQLite e geração de SQL nos três dialetos; execução real em MySQL/PostgreSQL permanece no [roteiro manual E09](E09_manual_validation.md), sem novas dependências no módulo.

## 2. Escopo

- **Sem mudança de comportamento ou de API pública.** Correções só ocorrem se um teste expuser defeito real.
- Verificação em camadas:
  - unitária: SQL e argumentos nos três dialetos, incluindo a consulta de contagem;
  - integração SQLite: execução de seleção padrão, filtros agrupados, junções, agregações, `DISTINCT`, ordenação, limite/deslocamento, `Count`/`Paginate` e transação;
  - manual: os mesmos cenários em MySQL e PostgreSQL (E09).
- A extração interna de `countQuery()` em `internal/query` é apenas para testabilidade; `Count` mantém o contrato.

## 3. Matriz de evidências

| Cenário | SQL ×3 | SQLite | MySQL | PostgreSQL |
| --- | --- | --- | --- | --- |
| Seleção padrão e ordem de declaração | automatizado | automatizado | E09 | E09 |
| `Only`, `Fields` e mistura dos dois | automatizado | automatizado | E09 | E09 |
| Filtros `AND`/`OR`, `LIKE`, `IN` (inclusive vazio) | automatizado | automatizado | E09 | E09 |
| Junções `INNER`/`LEFT` com filtro | automatizado | automatizado | E09 | E09 |
| Agregados (`COUNT(*)`, `COUNT`, `SUM`, `MAX`, `MIN`) e agregado de entidade juntada | automatizado | automatizado | E09 | E09 |
| `GROUP BY` e `ORDER BY` (coluna e agregado) | automatizado | automatizado | E09 | E09 |
| `DISTINCT` | automatizado | automatizado | E09 | E09 |
| `LIMIT`/`OFFSET`, `Limit(0)` e ordem dos argumentos | automatizado | automatizado | E09 | E09 |
| `Count` ignora ordem/limite e preserva `DISTINCT`/grupo | automatizado | automatizado | E09 | E09 |
| `Paginate` (páginas, bordas, vazio, filtro com argumentos) | parcial (SQL) | automatizado | E09 | E09 |
| Execução dentro de `Tx` e bloqueio por erro | parcial | automatizado | E09 | E09 |
| Erros de construção antes do executor | automatizado | automatizado | E09 | E09 |

“Parcial (SQL)” indica que o SQL da consulta de dados é coberto pelos testes de `Build`; a contagem e as estatísticas são verificadas na integração SQLite.

## 4. Semântica de borda de `Paginate`

Os testes travam os valores já decididos em E05-P03 §7:

- conjunto vazio com `offset == 0`: `Total = 0`, `Pages = 0`, `Page = 1`, `HasNext = false`, `HasPrev = false`;
- `offset` além do total: `Pages` continua sendo o necessário para o total, `Page = offset/limit+1`, `HasNext = false`, `HasPrev = true`, destino vazio;
- `limit == total` ou `offset+limit == total`: `HasNext = false`;
- `offset` exato no início da última página: `HasPrev = true` e `HasNext = false` quando a página é a última;
- `Paginate` não altera `Limit`/`Offset` já definidos no builder; seus parâmetros valem só para a chamada.

## 5. Banco a banco

- **SQLite automatizado**: `LIKE` é insensível para ASCII por padrão; `DISTINCT` com `ORDER BY` fora da seleção é aceito.
- **MySQL**: sem execução nesta entrega. O roteiro E09 cobre `LIKE` insensível pela collation padrão, alias obrigatório na tabela derivada e tipos de retorno dos agregados.
- **PostgreSQL**: sem execução nesta entrega. O roteiro E09 cobre `$n`, `LIKE` sensível, `DISTINCT` + `ORDER BY` restrito à seleção e tipos/NULL de agregados.
- Diferenças de sintaxe continuam sem tradução automática; o consumidor é responsável por seleções coerentes com `DISTINCT`/`GROUP BY`.

## 6. Itens resolvidos

- [x] SQL e parâmetros da seleção, filtros, junções, agregações, ordenação e limite/deslocamento nos três dialetos.
- [x] Consulta de contagem sem `ORDER BY`/`LIMIT`/`OFFSET`, com alias de tabela derivada por dialeto.
- [x] Execução SQLite da cadeia completa, inclusive agrupamento com junção e paginação.
- [x] Estatísticas de `Paginate` em primeira página, última, vazio e além do total.
- [x] Consistência em transação e validação antes da execução.
- [x] Roteiro manual E09 ampliado com harness e cenários E05 (pendente de execução real).
- [x] `gofmt`, `go test ./...`, `make check` e `make race` verdes em 2026-10-19.

## 7. Pendências e itens fora do escopo

- Execução real em MySQL/PostgreSQL e registro de evidências (E09).
- `HAVING`, `COUNT(DISTINCT ...)`, janelas/funções analíticas e ordenação por posição.
- Otimização do total para casos sem `DISTINCT`/grupo.
- Testes automatizados contra servidores MySQL/PostgreSQL: o projeto mantém a política de não adicionar drivers ao módulo; a validação é manual em E09.

## 8. Referências

- RF-06 e §6.5 de [0001_planning.md](0001_planning.md).
- [E05-P01_decisions.md](E05-P01_decisions.md): expressões, dialetos e fronteira construção × execução.
- [E05-P03_decisions.md](E05-P03_decisions.md): agregações, `Count` e `Paginate`.
- [E04-P02_decisions.md](E04-P02_decisions.md) e [E04-P04_decisions.md](E04-P04_decisions.md): mapeamento por nome/posição e padrão de validação.
- [E09_manual_validation.md](E09_manual_validation.md): roteiro manual.
- Passo E05-P04 em [0002_execution_plan.md](0002_execution_plan.md).
