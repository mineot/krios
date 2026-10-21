# E04-P02 — Decisões: Implementação do mapeamento e do acesso tipado

> **Rastreabilidade atualizada em C05-P03 (2026-10-21):** os encaminhamentos históricos de `ColumnTypes` para E04/E07 não representam entrega. A avaliação permanece pendente em E09-P01, conforme D01 da [matriz](C05-P03_traceability.md). Aliases de resultados já são suportados; alias de tabela/esquema no builder são extensões registradas (D02), distintas de `ManagedObjects` qualificado.


Data: 2026-10-19. Continua [E04-P01_decisions.md](E04-P01_decisions.md) e resolve as pendências de §6.7 apontadas em [0002_execution_plan.md](0002_execution_plan.md).

## 1. Contexto

E04-P01 fixou o mapeamento por reflexão sobre structs. Restavam pendentes a sintaxe definitiva das tags, a correspondência entre colunas e campos, o tratamento de coluna não selecionada, as conversões incompatíveis e a forma de acesso tipado. Este documento registra as decisões adotadas antes da implementação.

## 2. Tags de campo

- Sintaxe: `krios:"nome_da_coluna"`, com opções separadas por vírgula.
- `krios:"-"` exclui o campo do mapeamento (qualquer opção após `-` é ignorada).
- `krios:"coluna,pk"` marca a coluna como chave primária; `krios:",pk"` usa o nome derivado e marca a PK.
- Opção desconhecida é erro de derivação (`ErrInvalidEntity`).
- Campo sem tag, ou com tag vazia, usa o nome derivado do campo em `snake_case` (`UserName` → `user_name`, `UserID` → `user_id`, `HTTPServer` → `http_server`). Decisão de consumidor em 2026-10-19; `lowercase` sem separador e nome literal do campo foram considerados e descartados.
- Campo não exportado sem tag é ignorado; campo não exportado **com** tag é erro (evita tag silenciosamente ineficaz).
- Campos anônimos (embutidos) são rejeitados nesta entrega; a decisão pode ser revisitada com um caso de uso concreto.
- Nomes de coluna duplicados são erro, inclusive quando difiram apenas por maiúsculas/minúsculas.

## 3. Nome da tabela

Go não permite tag no tipo struct, apenas em campos; portanto a tabela não vem de tag:

- Padrão: `snake_case` do nome do tipo (`User` → `user`).
- Sobrescrita opcional pelo método `func (T) TableName() string` (receptor por valor ou ponteiro), avaliado na derivação.
- `TableName` com assinatura diferente, retorno vazio, ou tipo anônimo sem o método, é erro de derivação.
- Esquema e alias de tabela permanecem para E05 (junções e construção de SQL).

## 4. Derivação e `krios.Entity`

- `krios.EntityOf[T]()` devolve o descritor `*krios.Entity` derivado de `T` (struct ou ponteiro para struct), com erro tipado por `ErrInvalidEntity`.
- O descritor expõe `Table()`, `Columns()`, `PrimaryKeys()` (lista, suporta chave composta) e `Type()`.
- Os metadados são derivados uma única vez por `reflect.Type` e memorizados em cache (`sync.Map`), incluindo erros de derivação.
- A ordem das colunas é a ordem de declaração dos campos exportados mapeados.
- Tipos sem campos mapeáveis são erro.

## 5. Acesso tipado (`*T` e `*[]T`)

- **Leitura de um registro:** `Row.Scan(&registro)` passa a aceitar um único destino `*struct`, além do escaneamento posicional já existente. Como `*sql.Row` não expõe os nomes das colunas, o mapeamento de um único registro é **posicional**: o resultset deve corresponder, em quantidade e ordem, à ordem de declaração dos campos da entidade.
- **Leitura de múltiplos registros:** novo método `Rows.All(&slice)` com destino `*[]T` (`T` struct). Itera até o fim, fecha o iterador em todos os caminhos, substitui o conteúdo do slice somente em caso de sucesso e preserva o registro de erros tardios na `Tx`.
- `Rows.All` corresponde colunas aos campos **por nome**: igualdade exata primeiro, depois comparação sem distinção de maiúsculas/minúsculas. Isso atende aliases e seleções parciais.
- Wrappers posicionais de E03 permanecem inalterados para SQL ad-hoc.
- Um tipo struct que implementa `sql.Scanner` (por exemplo `time.Time`, `sql.Null*`) mantém o comportamento posicional; a precedência é documentada na API.

## 6. Coluna não selecionada, `NULL` e conversões

- **Coluna do resultset sem campo correspondente** (em `Rows.All`): valor descartado; o mapeamento não falha.
- **Campo sem coluna correspondente:** permanece inalterado no registro; em `Rows.All`, cada elemento é novo e permanece com o valor zero.
- **`NULL` e conversões:** delegadas ao `database/sql`, passando os ponteiros dos campos ao `Scan` nativo. `*T` e tipos `sql.Null*` aceitam `NULL`; `NULL` em campo não anulável e conversões incompatíveis retornam erro. Erros de mapeamento são contextualizados com o nome da tabela e embrulham a causa com `%w` (visível a `errors.Is`/`errors.As`); `sql.ErrNoRows` e `sql.ErrTxDone` são preservados sem reescrita, seguindo os contratos de E02/E03.
- Não há novo sentinela para falhas de conversão; `ErrInvalidEntity` cobre somente derivação inválida da entidade e destinos malformados.

## 7. Junções e associações

Sem mudança: tags de associação/junção e o uso do descritor pelos construtores ficam para E05 (conforme E04-P01 §6 e decisão de consumidor em 2026-10-19).

## 8. Itens desta entrega resolvidos

- [x] Sintaxe das tags (tabela, colunas, `pk`, `-`, política de nome sem tag).
- [x] Correspondência entre colunas e campos (por nome, com `Rows.All`; posicional em `Row.Scan`).
- [x] Tratamento de coluna não selecionada e campo não correspondido.
- [x] Conversões incompatíveis e `NULL` por coluna, com causa preservada.
- [x] `krios.Entity` como descritor derivado, com cache e `ErrInvalidEntity`.

## 9. Itens que permanecem em outras entregas

- Junções e associações (E05).
- Metadados de colunas de SQL arbitrário além de `Rows.Columns()` (`ColumnTypes`, E04/E07).
- Momento de execução de validação e formatação (E04-P03).
- Política de escrita de `NULL`/omitido (E06-P01).
- Structs embutidos (revisitável).

## 10. Referências

- RF-09 e §6.7 de [0001_planning.md](0001_planning.md); contrato de leitura em [E03-P01_decisions.md](E03-P01_decisions.md).
- [E04-P01_decisions.md](E04-P01_decisions.md), em especial as pendências da §9.
- Passo E04-P02 em [0002_execution_plan.md](0002_execution_plan.md).
