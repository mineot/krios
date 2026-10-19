# E04-P04 — Decisões: Validação do mapeamento

Data: 2026-10-19. Encerra a entrega E04, cobrindo o que E04-P01–E04-P03 implementaram. Depende de [E04-P02_decisions.md](E04-P02_decisions.md) e [E04-P03_decisions.md](E04-P03_decisions.md).

## 1. Contexto

E04-P04 não adiciona capacidade nova: verifica o mapeamento por reflexão e a validação/formatação por campo com consultas de E03 (`QueryContext`/`QueryRowContext`), sem depender dos construtores de E05. O critério de conclusão do plano (§E04) pede um exemplo que leia e modifique valores de registros distintos via reflexão.

## 2. Escopo

- **Sem mudança de comportamento ou de API.** A entrega é de testes, exemplo e documentação; correções só ocorreriam se um teste expusesse defeito real.
- Cobre, em integração SQLite e em reforço unitário: múltiplos registros, ordem das colunas, aliases, nulos, tipos incompatíveis e erros de validação e formatação.

## 3. Ordem posicional em `Row.Scan`

- Reafirma E04-P02 §5: `Row.Scan(&T)` é **posicional**. Colunas em ordem diferente da declaração **não** são erro — preenchem os campos na ordem recebida. O teste documenta esse comportamento como esperado, não como falha.

## 4. `sql.Null*` e formatação

- Campos `sql.Null*` são aceitos na leitura (`NULL` ↔ `Valid=false`) e mantêm o comportamento documentado em E04-P03 §5: sem `Formatter`, o fallback imprime o valor interno do struct (ex.: `{false }`).
- Esta entrega **mantém** esse comportamento e o cobre com teste; um texto amigável para `sql.Null*` fica como melhoria futura, fora do escopo, por não alterar contrato aqui.

## 5. Independência entre registros

- `Rows.All` cria um elemento novo por linha; o teste comprova que valores de registros distintos não se misturam nem vazam entre elementos.

## 6. Itens desta entrega resolvidos

- [x] Múltiplos registros sem confusão de valores.
- [x] Ordem posicional em `Row.Scan` e ordem por nome em `Rows.All`.
- [x] Aliases de coluna e de tabela.
- [x] `NULL` em `*T`/`sql.Null*` e em campos não anuláveis.
- [x] Tipos incompatíveis com causa preservada.
- [x] Erros de validação (`ErrValidation`) e formatação (`ErrFormat`), com fail-fast por registro.

## 7. Itens que permanecem em outras entregas

- Texto amigável para `sql.Null*` no fallback de `Format`.
- Junções, esquema/alias e `ColumnTypes` (E05/E07).
- Evidência manual MySQL/PostgreSQL (E09).

## 8. Referências

- RF-09 e §6.7 de [0001_planning.md](0001_planning.md).
- [E04-P02_decisions.md](E04-P02_decisions.md) e [E04-P03_decisions.md](E04-P03_decisions.md).
- Passo E04-P04 em [0002_execution_plan.md](0002_execution_plan.md).
