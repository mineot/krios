# E05-P03 — Decisões: agregações, ordenação e paginação

## 1. Contexto

E05-P02 entregou a construção e a execução de `SELECT` com seleção de colunas, filtros, grupos lógicos e junções. Ficaram pendentes, conforme [E05-P01_decisions.md](E05-P01_decisions.md) §11 e [0001_planning.md](0001_planning.md) §6.5, as agregações (`COUNT`/`SUM`/`MAX`/`MIN`), `DISTINCT`, `ORDER BY`, `GROUP BY`, `LIMIT`/`OFFSET` e a paginação por limite/deslocamento com total e estatísticas (`krios.Paginate`). Este documento fixa esses contratos antes da implementação.

## 2. Agregações como expressões

- Agregações são **expressões selecionáveis**, construídas a partir de uma coluna: `users.Col("id").Count()`, `.Sum()`, `.Max()`, `.Min()`. `COUNT(*)` usa o construtor livre `krios.Count()`.
- `Select.Fields(exprs ...krios.Expr)` acrescenta expressões à seleção, na ordem das chamadas. Expressões também podem ser ordenadas (`Asc`/`Desc`).
- Alias explícito por `As(nome)`: `users.Col("id").Count().As("total")`. O alias é obrigatório para mapear o resultado por nome em `Rows.All`/`All`, pois o rótulo nativo da expressão varia por driver.
- Alias vazio significa “sem alias”; alias só com espaços, alias repetido ou alias que colida com uma coluna selecionada é erro em `Build()` (`ErrInvalidQuery`). A coluna da expressão é validada contra a entidade dona, e a entidade precisa participar da consulta.
- O renderizador emite `COUNT(...)`, `SUM(...)`, `MAX(...)` e `MIN(...)`; a sintaxe é comum aos três bancos, sem variação de dialeto.
- `SUM`/`MAX`/`MIN` podem devolver `NULL` (conjunto vazio) e o tipo nativo do driver. O consumidor escolhe o destino (`*T`, `sql.Null*`); a biblioteca não converte nem substitui o valor.

## 3. Seleção mista

- Sem `Only` e sem `Fields`: todas as colunas mapeadas da entidade base, na ordem de declaração (contrato de E04/E05-P01).
- Com `Fields` e sem `Only`: a seleção contém **somente** as expressões de `Fields`.
- Com `Only` e `Fields`: colunas de `Only` (na ordem informada) seguidas das expressões de `Fields`.
- `Only` continua restrito às colunas da entidade base; `Fields` pode selecionar colunas de entidades participantes (inclusive junções).
- Rótulos duplicados na seleção (nome de coluna de `Only`/`Fields` ou alias) são erro em `Build()`.

## 4. `DISTINCT`, `GROUP BY`, `ORDER BY`

- `Distinct()` liga a eliminação de duplicatas sobre a lista de seleção. Sem `Distinct()`, não há `DISTINCT`.
- `GroupBy(cols ...krios.Column)` acumula colunas de agrupamento na ordem das chamadas. Colunas podem pertencer a qualquer entidade participante; duplicatas são permitidas (SQL válido, sem efeito adicional).
- `OrderBy(sorts ...krios.Sort)` acumula critérios na ordem das chamadas; cada critério é `coluna.Asc()`, `coluna.Desc()`, `agregacao.Asc()` ou `agregacao.Desc()`. `ASC`/`DESC` são sempre emitidos explicitamente. A ordenação por agregação usa a expressão completa (não o alias).
- Não há validação semântica de coerência entre `DISTINCT`/`GROUP BY` e a seleção: o consumidor deve selecionar apenas colunas agrupadas ou agregadas. O PostgreSQL rejeita `DISTINCT` com `ORDER BY` fora da seleção; MySQL e SQLite podem aceitar. A diferença é documentada, sem tradução de sintaxe.
- Toda coluna/agregação de `Fields`, `GroupBy` ou `OrderBy` exige que sua entidade participe da consulta; caso contrário, `Build()` retorna `ErrInvalidQuery`.

## 5. `LIMIT` e `OFFSET`

- `Limit(n)` e `Offset(n)` são setters: a última chamada vence. `Limit(0)` é válido e produz zero linhas. Valores negativos são erro.
- `Offset(n)` com `n > 0` exige `Limit` definido, para manter portabilidade com MySQL; sem `Limit`, `Build()` retorna `ErrInvalidQuery`.
- Os valores são enviados como parâmetros após os argumentos do `WHERE`: `LIMIT ? OFFSET ?` (PostgreSQL continua a numeração `$n`). O `LIMIT`/`OFFSET` definidos no builder valem para `Build`/`Rows`/`Row`/`All`/`One`; `Count` e `Paginate` os ignoram (ver §6).

## 6. Contagem total

- `Count(ctx) (int64, error)` conta as linhas que a consulta retornaria, desconsiderando `ORDER BY`, `LIMIT` e `OFFSET` (e o que o builder tenha definido nesses campos).
- Implementação única: `SELECT COUNT(*) FROM (<consulta interna>) AS "krios_total"`, preservando `DISTINCT`, `GROUP BY`, `JOIN` e `WHERE`. A subconsulta é exata com agrupamento e duplicatas; o custo de materialização é aceito nesta versão (otimizações futuras podem trocar por `COUNT(*)` direto quando seguro).
- O alias da tabela derivada é aspeado pelo dialeto (MySQL/PostgreSQL exigem alias).
- A subconsulta interna mantém a mesma lista de seleção; agregados na seleção contam como uma linha de resultado, como em qualquer consulta.

## 7. `Paginate`

```go
type Paginate struct {
    Total   int64
    Limit   int
    Offset  int
    Page    int
    Pages   int
    HasNext bool
    HasPrev bool
}

func (s *Select) Paginate(ctx context.Context, limit, offset int, dest any) (Paginate, error)
```

- `limit` deve ser maior que zero e `offset` não negativo; caso contrário, `ErrInvalidQuery` antes de executar.
- `dest` segue `Rows.All` (ponteiro para slice de structs); `Paginate` não é genérico, preservando o padrão de tipagem no destino.
- `Page` é base 1 (`offset/limit + 1`); `Pages` é `ceil(Total/limit)` (zero quando `Total == 0`); `HasPrev` é `offset > 0`; `HasNext` é `offset+limit < Total`.
- `Paginate` sobrescreve quaisquer `Limit`/`Offset` definidos no builder **apenas na consulta de dados**, sem mutar o builder. Primeiro executa a contagem total, depois a página; erro em qualquer etapa é devolvido sem resultado parcial.
- Em `*Tx`, as duas consultas participam da mesma transação e são serializadas. Em `*DB`, são duas execuções independentes: total e itens podem divergir sob escrita concorrente; consistência exige `Begin`.

## 8. Erros

Continuam valendo `ErrInvalidQuery` (construção/validação) e os erros de execução de E02/E03. Novos casos de `ErrInvalidQuery`:

- alias de agregação vazio (só espaços), repetido ou colidindo com coluna selecionada;
- coluna/agregação de `Fields`, `GroupBy` ou `OrderBy` com entidade ausente da consulta ou coluna desconhecida;
- `Limit`/`Offset` negativos, `Offset > 0` sem `Limit`, e parâmetros inválidos de `Paginate`.

`sql.ErrNoRows` não se aplica à contagem: `Count`/`Total` de conjunto vazio é `0`. Falha de leitura tardia na contagem (`Scan`) é registrada na transação como qualquer outra.

## 9. Diferenças por banco na versão inicial

| Ponto | SQLite | MySQL | PostgreSQL |
| --- | --- | --- | --- |
| Funções de agregação | sim | sim | sim |
| `LIMIT ? OFFSET ?` | sim | sim | sim |
| `COUNT(*)` sobre tabela derivada | sim (alias opcional) | exige alias (emitido) | exige alias (emitido) |
| `DISTINCT` + `ORDER BY` fora da seleção | aceita | aceita | rejeita (documentado) |

## 10. Itens resolvidos

- [x] `COUNT`/`SUM`/`MAX`/`MIN` como expressões com alias e ordenação.
- [x] `DISTINCT`, `GROUP BY`, `ORDER BY` e `LIMIT`/`OFFSET`.
- [x] Total de resultados por subconsulta, com `DISTINCT` e agrupamento.
- [x] `krios.Paginate` com total e estatísticas, validação e vínculo com `DB`/`Tx`.

Permanecem fora desta entrega: `COUNT(DISTINCT ...)`, agregações em `HAVING`, janelas/funções analíticas, ordenação por posição e otimização do total para casos simples.

## 11. Referências

- RF-06 de [0001_planning.md](0001_planning.md); §6.5 (consultas e paginação).
- [E05-P01_decisions.md](E05-P01_decisions.md): dialetos, expressões, `Build` e fronteira construção × execução.
- [E04-P01_decisions.md](E04-P01_decisions.md) e [E04-P02_decisions.md](E04-P02_decisions.md): colunas, ordem de declaração e mapeamento por nome.
- [E03-P01_decisions.md](E03-P01_decisions.md): contrato de execução e política de `Tx`.
- Passos E05-P03–E05-P04 em [0002_execution_plan.md](0002_execution_plan.md).
