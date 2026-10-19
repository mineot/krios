# E04-P01 — Decisões: Mapeamento por reflexão

Data: 2026-10-19. Substitui a proposta de entidade explícita descrita originalmente no RF-09.

## 1. Contexto e direcionamento

O RF-09 propunha uma API explícita de entidade (`krios.Entity` com `Column`, `GetString`, `SetString`, `GetTime`, `SetTime` e `GetFormatted`). A revisão de E04-P01 **troca o direcionamento**: o mapeamento passa a ser feito por **reflexão sobre structs Go**, no mesmo espírito de bibliotecas como `database/sql`/`sqlx`/`ent`.

O consumidor descreve tabelas e colunas com structs e tags; a biblioteca deriva os metadados em tempo de execução com `reflect`. A DSL explícita de colunas e os acessores tipados por nome deixam de ser a representação canônica.

Motivos da escolha:

- Menos código de configuração para o caso comum (tabela ↔ struct).
- Distinção natural entre valor zero, valor omitido e `NULL` por meio de ponteiros e tipos anuláveis.
- Ordem das colunas determinística: `reflect` preserva a ordem de declaração dos campos.
- Atende ao requisito anterior de preencher structs do consumidor, inclusive slices recebidos por ponteiro.

## 2. Modelo de entidade

- **Entidade** é um tipo `struct` Go (ou ponteiro para `struct`). A biblioteca inspeciona seus campos exportados.
- **Tabela:** nome declarado em tag de struct; esquema e alias opcionais.
- **Colunas:** campos exportados do struct; o nome da coluna vem de tag de campo. Um marcador identifica a chave primária. Um marcador ignora o campo.
- **Ordem:** a ordem das colunas é a ordem de declaração dos campos; é a ordem usada como seleção padrão quando não há seleção explícita (RF-06).
- **`krios.Entity`:** mantém o nome, mas passa a ser o **descritor derivado por reflexão** de um tipo struct (por exemplo, obtido de um valor/tipo), não uma DSL declarativa de colunas.

Convenção inicial de tags (detalhes de sintaxe a fechar em E04-P02):

```go
type User struct {
    ID   int64   `krios:"id,pk"`
    Name string  `krios:"name"`
    Bio  *string `krios:"bio"`
}
```

- Tag de struct define a tabela (e, opcionalmente, esquema/alias).
- `pk` marca a chave primária.
- `-` exclui o campo do mapeamento.
- Campo sem tag usa o nome do campo; a política de conversão de nome (snake_case) será decidida em E04-P02.

## 3. `NULL`, campo omitido e conversões

- `NULL` é representado por ponteiro (`*T`) ou por tipos `sql.Null*`. Um `NULL` lido em campo não anulável é erro de conversão.
- **Coluna não selecionada:** o campo correspondente permanece inalterado no destino; não é zerado.
- **Campo omitido × valor zero × `NULL`:** a distinção é habilitada pelo modelo. Ponteiro `nil` representa ausência; ponteiro não-`nil` para valor zero representa o zero explícito. A regra exata de escrita (omitir e usar default do banco, gravar `NULL` ou gravar zero) é fixada em E06-P01; a convenção recomendada é: em `INSERT`, `nil` → `NULL` (ou omissão com marcador a definir); em `UPDATE`, campo `nil` não é atualizado e um tipo anulável não-`nil` grava o valor, inclusive `NULL`.
- Conversões incompatíveis retornam erro associado à coluna, preservando a causa para `errors.Is`/`errors.As`.

## 4. Representação de resultados

- Leitura para um único registro: preenchimento de `*T`.
- Leitura de múltiplos: preenchimento de slice recebido por ponteiro (`*[]T`).
- `sql.ErrNoRows` é preservado para consulta sem linha (já definido em E03).
- O mapeamento por reflexão **complementa** os wrappers nativos de E03 (`Row`/`Rows`), que continuam disponíveis para SQL ad-hoc e para acesso posicional.

## 5. Validação e formatação por coluna

- As funções fornecidas pelo consumidor continuam sendo capacidade do RF-09, agora associadas a campos do struct: por interface implementada pelo tipo do campo (por exemplo `Validate`/`Format`) ou por registro na entidade derivada.
- O valor **formatado** destina-se à apresentação; o valor **original** do campo é o usado nas operações de banco.
- O momento de execução (atribuição, leitura ou escrita) é fixado em E04-P03.

## 6. Junções

- Junções continuam no escopo (RF-06), expressas com referência a outro struct/tabela e seus critérios; a reflexão fornece nomes de tabela e colunas.
- A forma de declarar associações (tags de relação ou referência explícita do construtor) será definida em E04-P02/E05.

## 7. Construtores e associação à conexão/transação

- Os construtores de leitura e escrita partem do tipo struct (entidade por reflexão).
- A execução permanece vinculada ao contrato de E03: `*DB` e `*Tx` são os destinos; a forma exata de passagem (receptor `db.Select(...)`/`tx.Select(...)`, ou parâmetro executor) é fixada em E05-P01.
- A ordem de seleção padrão é a ordem dos campos do struct.

## 8. O que é substituído e o que permanece

| Item do RF-09 original | Estado após E04-P01 |
| --- | --- |
| `krios.Entity` como DSL explícita de colunas | Substituído: passa a ser o descritor derivado por reflexão |
| `Column`, `GetString`/`SetString`, `GetTime`/`SetTime`, `GetFormatted` | Substituídos: acesso tipado pelos campos do struct |
| Tabela, esquema, alias, colunas, junções | Mantidos, derivados de struct e tags |
| Funções de validação e formatação por coluna | Mantidas, associadas a campos |
| Mapeamento automático para structs/slices | Adotado como modelo principal |

## 9. Itens ainda pendentes

- Sintaxe definitiva das tags (tabela, esquema, alias, chave primária, associações, política de nome sem tag).
- Regras de correspondência e conversão de tipos e o erro detalhado por coluna.
- Representação e política de escrita de `NULL`/omitido (E06-P01).
- Junções e associações (E04-P02/E05).
- Metadados de colunas (`Columns`/`ColumnTypes`) para SQL arbitrário e aliases (E04/E07).
- Momento de execução da validação e da formatação (E04-P03).

## 10. Referências

- RF-06, RF-07 e RF-09 de [0001_planning.md](0001_planning.md); §6.6 e §6.7.
- Decisão anterior de "reflexão não decidida" em [E01-P01_decisions.md](E01-P01_decisions.md).
- Contratos de execução e leitura tardia em [E03-P01_decisions.md](E03-P01_decisions.md).
- Passos E04-P01–E04-P04 em [0002_execution_plan.md](0002_execution_plan.md).
