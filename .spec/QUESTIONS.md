# Dúvidas para decisão do utilizador

- Registrar cada dúvida com identificador, passo relacionado, contexto e pergunta.
- Deixar a resposta para o utilizador; preservar perguntas e respostas enquanto houver questões pendentes.
- Quando todas as questões estiverem resolvidas, registrar as decisões no planejamento do passo correspondente e limpar todo o conteúdo deste arquivo, mantendo o arquivo.
- Aguardar respostas e autorização explícita antes de iniciar ou retomar a implementação.

## Dúvidas pendentes

### 001-Q01 — Versões dos bancos

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** os três bancos estão definidos, mas não suas versões mínimas; a matriz de JOINs, NULLs e retorno de colunas depende desse limite.
- **Pergunta:** quais versões mínimas de PostgreSQL, MySQL e SQLite a primeira versão da biblioteca deverá suportar?
- **Resposta do utilizador:** PostgreSQL 16+, MySQL 8.0+ e SQLite 3.39+
- **Situação:** respondido.

### 001-Q02 — Identificação das capacidades

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** a API deve rejeitar recursos indisponíveis antes de executar, mas `Config` e `Wrap` não recebem versão/capacidades; `Build` também precisa conhecer essas restrições.
- **Pergunta:** a biblioteca deve identificar a versão consultando o banco, receber essa informação explicitamente do utilizador ou trabalhar com um conjunto fixo de capacidades por dialeto? Como deve proceder se a versão não puder ser identificada?
- **Resposta do utilizador:** Detecção automática, usar os recursos disponiveis de cada banco de dados
- **Situação:** respondida.

### 001-Q03 — Chamadas repetidas dos builders

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** WHERE acumula por AND e a projeção SELECT é substituída; os demais casos ainda não têm regra completa.
- **Pergunta:** qual deve ser o comportamento de cada chamada repetida abaixo?
  - `OrderBy`, `GroupBy` e `Returning`: acumular ou substituir?
  - `Having`: acumular por AND ou substituir?
  - `Insert.Columns`: acumular ou substituir?
  - `Insert.Values`: substituir a linha, acrescentar outra linha ou rejeitar a repetição?
  - `Update.Set` para a mesma coluna: substituir o valor ou retornar erro?
- **Resposta do utilizador (parcial):**
  - Item 1 — acumular.
  - Item 2 — erro no `Exec`.
  - Item 3 — a ordem é preservada e duplicatas não são permitidas.
  - Item 4 — acumular linhas.
  - Item 5 — deve seguir o mesmo do item 4.
  - Item 6 — sim, chamadas vazias são inválidas.
  - Item 7 — `Where` acumula; `Columns` tem prioridade sobre `All`.
- **Situação:** parcialmente respondida; pendentes `001-Q03a`, `001-Q03b`, `001-Q03c` (linhas idênticas), `001-Q03e` e `001-Q03f`.

### 001-Q03a — Escopo de "acumular" e regra de `Having`

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** a resposta "acumular" foi dada para o item 1, que abrange métodos com semânticas diferentes; `Having` é uma condição e `Update.Set` na mesma coluna geraria duplicata.
- **Pergunta:** "acumular" vale para todos os métodos `OrderBy`, `GroupBy`, `Returning`, `Having`, `Insert.Columns`, `Insert.Values` e `Update.Set`? Em `Having`, acumular significa combinar por `AND`, como em `Where`? Em `Update.Set` para a mesma coluna, a segunda chamada deve ser rejeitada como duplicata no `Exec`?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q03b — Definição de duplicata

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** "duplicatas não são permitidas" exige saber o que conta como duplicata em cada método.
- **Pergunta:** duplicata é o mesmo identificador textual (por exemplo, `"id"` repetido)? `Asc("id")` e `Desc("id")` contam como duplicata em `OrderBy`? `Column("u.id")` e a string `"u.id"` são equivalentes? A comparação diferencia maiúsculas/minúsculas?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q03c — `Insert.Values` e multi-linha (item 4)

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** `Insert.Values` pode representar uma única linha ou várias; a regra "acumular sem duplicatas" não distingue linhas repetidas intencionais de chamadas repetidas inválidas.
- **Pergunta:** chamadas repetidas de `Insert.Values` devem substituir a linha, acrescentar outra linha (`VALUES (...), (...)`) ou retornar erro? Se acrescentarem, linhas com valores idênticos são permitidas? Como validar a quantidade de valores em relação a `Insert.Columns`?
- **Resposta do utilizador:** acumular linhas.
- **Pendência:** confirmar se linhas com valores idênticos são permitidas; a quantidade de valores por linha deve corresponder a `Insert.Columns`.
- **Situação:** parcialmente respondida.

### 001-Q03d — Imutabilidade dos builders (item 5)

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** os builders já são imutáveis; ao acumular, é necessário copiar os slices internos para não alterar versões anteriores.
- **Pergunta:** confirmar que cada método continua retornando uma nova versão e que a versão anterior permanece inalterada, mesmo quando a nova chamada acumula itens?
- **Resposta do utilizador:** deve seguir o mesmo do item 4.
- **Situação:** respondida; interpretação registrada: a imutabilidade se aplica ao acúmulo, como no item 4.

### 001-Q03e — Prioridade de `Columns` sobre `All`

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** a resposta do item 7 define que `Columns` tem prioridade sobre `All`, mas o texto atual diz que ambos substituem a projeção anterior.
- **Pergunta:** a prioridade de `Columns` sobre `All` vale em qualquer ordem de chamadas, ou seja, `.All().Columns(...)` e `.Columns(...).All()` produzem a mesma projeção apenas com as colunas?
- **Resposta do utilizador:**
- **Situação:** aguardando confirmação.

### 001-Q03f — Momento da validação (item 2)

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** a resposta do item 2 indica erro no `Exec`, mas `Build()` também é documentado como validador e pode ser chamado sem executar.
- **Pergunta:** duplicatas e demais invalidações devem ser detectadas também em `Build()`, ou somente em `Exec()`? Se somente em `Exec()`, `Build()` pode retornar SQL que seria rejeitado na execução?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q04 — NULL em listas e valores especiais

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** `Eq/Ne` com `nil` literal e IN vazio estão definidos. A documentação também permite `sql.Null*` e `driver.Valuer`, sem estabelecer se o builder deve interpretar valores que representem NULL.
- **Pergunta:** `In/NotIn` devem aceitar `nil` entre os itens ou rejeitá-lo? Para `Eq/Ne`, a conversão para `IS NULL`/`IS NOT NULL` deve valer apenas para `nil` literal ou também para ponteiros nulos e valores especiais que representem NULL?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q05 — Fechamento com transação ativa

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** `DB.Close` invalida a instância e respeita a propriedade do pool, mas o contrato não especifica o destino de transações já iniciadas.
- **Pergunta:** ao chamar `DB.Close` com uma transação ativa, a biblioteca deve permitir que ela termine, retornar erro por existir transação ativa ou cancelá-la? A regra deve ser a mesma para pools próprios e fornecidos por `Wrap`?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q06 — Garantia do modo atômico das migrações

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** o modo atômico deve rejeitar comandos incompatíveis, mas os arquivos contêm SQL arbitrário e a biblioteca não traduz nem divide scripts. Ainda falta definir como essa compatibilidade será estabelecida.
- **Pergunta:** qual contrato deve ser adotado: aceitar somente um conjunto documentado e validado de instruções atômicas, exigir declaração de compatibilidade pelo utilizador ou outra abordagem? Como tratar instruções cuja compatibilidade não possa ser determinada?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q07 — Classificação de condições inválidas

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** grupos vazios são inválidos; UPDATE/DELETE com WHERE vazio exigem `ErrMissingWhere`. Falta distinguir ausência de restrição de erros internos de um predicado fornecido.
- **Pergunta:** em UPDATE/DELETE, `Where(And())` e predicados com coluna vazia devem retornar `ErrMissingWhere`, `ErrInvalidQuery` ou permitir identificação de ambos? Quando houver mais de uma condição inválida, qual erro deve prevalecer?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

## Autorização do passo 1

- **Planejamento:** redigido em [001.md](001.md) para revisão.
- **Execução da consolidação:** aguardando autorização explícita do utilizador, além das respostas às dúvidas.
