# E04-P03 — Decisões: Validação e formatação por campo

> **Rastreabilidade atualizada em C05-P03 (2026-10-21):** os encaminhamentos históricos de `ColumnTypes` para E04/E07 não representam entrega. A avaliação permanece pendente em E09-P01, conforme D01 da [matriz](C05-P03_traceability.md). Aliases de resultados já são suportados; alias de tabela/esquema no builder são extensões registradas (D02), distintas de `ManagedObjects` qualificado.


Data: 2026-10-19. Continua [E04-P02_decisions.md](E04-P02_decisions.md) e resolve as pendências de §6.7 apontadas em [0002_execution_plan.md](0002_execution_plan.md).

## 1. Contexto

O RF-09 prevê funções de validação e de formatação por campo, fornecidas pelo consumidor. E04-P01 §5 associou essas funções a campos do struct e deixou em aberto o mecanismo, as assinaturas, o momento de execução e a relação entre o valor formatado e o valor usado no banco. Este documento registra as decisões adotadas antes da implementação.

## 2. Mecanismo: interfaces no tipo do campo

- As funções são fornecidas por **interface implementada pelo tipo do campo**, detectada na derivação e memorizada por tipo (junto ao descritor de E04-P02). Não há registro por coluna nesta entrega.
- Assinaturas públicas:

  ```go
  type Validator interface { Validate() error }
  type Formatter interface { Format() (string, error) }
  ```

- `Validate` retorna `nil` quando o valor é válido; a "indicação de validade" do RF-09 é expressa pelo erro. `Format` devolve o texto de apresentação e um erro opcional.
- A detecção considera `T` e `*T` (receptor por valor ou por ponteiro), como já ocorre com `sql.Scanner` em E04-P02.
- Campos ignorados (`krios:"-"`) e campos não exportados não participam.
- **Campos primitivos** (`string`, `int`, …) não têm validação/formatação sem um tipo nomeado pelo consumidor (`type Email string`). É a consequência esperada deste mecanismo; o registro por coluna fica fora de escopo e pode voltar com um caso concreto.

## 3. Assinaturas de execução

- `Entity.Validate(record any) error` — aceita `T` ou `*T` do tipo da entidade. Quando recebe um valor, trabalha sobre uma cópia endereçável, permitindo validadores com receptor por ponteiro.
- `Entity.ValidateAll(records any) error` — aceita `[]T` ou `*[]T`; valida na ordem e para no primeiro registro inválido.
- `Entity.Format(record any) (map[string]string, error)` — aceita `T` ou `*T`; devolve **todas** as colunas mapeadas.
- Como os descritores são memorizados por tipo, o custo da reflexão de detecção é pago uma vez.

## 4. Momento de execução

- A validação é **explícita na leitura**: o consumidor chama `Validate`/`ValidateAll` quando desejar. Na escrita, porém, `SetValues` passou a executar `Entity.Validate` **automaticamente** (fail-fast) — ver [E06-P01_decisions.md](E06-P01_decisions.md) §8, que supersede a nota histórica "(ainda inexistente)".
- Os escritores de E06 **reutilizarão** o mesmo motor interno antes de gravar, mantendo um único ponto de verdade para as regras.
- A formatação é igualmente explícita e destina-se à apresentação.

## 5. Valor formatado × valor original

- O texto de `Format` serve apenas à apresentação; as operações de banco usam sempre o valor original do campo (E04-P01 §5).
- **Fallback:** campos sem `Formatter` (ou cujo tipo não o implementa) retornam `fmt.Sprint(valor)`.
- **Ponteiro `nil`:** campo ponteiro nulo é **pulado** na validação e produz texto vazio (`""`) em `Format`; não se invoca método em receptor nulo. Tipos `sql.Null*` sem `Formatter` usam `fmt.Sprint` (documentado).
- O mapa resultante inclui todas as colunas, inclusive as de fallback.

## 6. Erros

- Novos sentinelas: `ErrValidation` (campo reprovado) e `ErrFormat` (falha ao formatar), permitindo `errors.Is` específico.
- Mensagens contextualizam tabela e coluna e preservam a causa com `%w`:
  - `krios: field validation failed: <tabela>: column "<coluna>": <causa>`
  - `krios: field formatting failed: <tabela>: column "<coluna>": <causa>`
- Destino inválido (tipo diferente de `T`/`*T`, ponteiro nulo ou slice de outro tipo) retorna `ErrInvalidEntity`.
- **Fail-fast:** retorna o primeiro erro na ordem de declaração dos campos; `ValidateAll` para no primeiro registro inválido. Agregação de múltiplos erros não está nesta entrega.

## 7. Itens desta entrega resolvidos

- [x] Mecanismo de fornecimento (interfaces no tipo do campo).
- [x] Assinaturas de validação (`Validate() error`) e formatação (`Format() (string, error)`).
- [x] Momento de execução (explícito; reutilizado por E06).
- [x] Relação valor formatado × valor original (apresentação × banco) e fallback.
- [x] Erros por coluna com sentinelas e causa preservada.

## 8. Itens que permanecem em outras entregas

- Registro de validadores/formatters por coluna e validação no nível do struct/junções de campos.
- Integração efetiva antes de escrita (E06-P01/P02).
- Agregação de múltiplos erros de validação/formatação.
- Junções, esquema/alias e `ColumnTypes` (E05/E07).

## 9. Referências

- RF-09 e §6.7 de [0001_planning.md](0001_planning.md); valor formatado × original em [E04-P01_decisions.md](E04-P01_decisions.md) §5.
- [E04-P02_decisions.md](E04-P02_decisions.md), em especial §9.
- Passo E04-P03 em [0002_execution_plan.md](0002_execution_plan.md).
