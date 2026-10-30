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
- **Situação:** parcialmente resolvida; detecção automática definida, tratamento da falha pendente em `001-Q08`.

### 001-Q03 — Chamadas repetidas dos builders

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** WHERE acumula por AND e a projeção SELECT é substituída; os demais casos ainda não têm regra completa.
- **Pergunta:** qual deve ser o comportamento de cada chamada repetida abaixo?
  - Item 1: `OrderBy`, `GroupBy` e `Returning`: acumular ou substituir?
  - Item 2:`Having`: acumular por AND ou substituir?
  - Item 3: `Insert.Columns`: acumular ou substituir?
  - Item 4: `Insert.Values`: substituir a linha, acrescentar outra linha ou rejeitar a repetição?
  - Item 5: `Update.Set` para a mesma coluna: substituir o valor ou retornar erro?
- **Resposta do utilizador (parcial):**
  - Item 1 — acumular, a order deve ser respeitada.
  - Item 2 — acumular.
  - Item 3 — acumular, a order deve ser respeitada.
  - Item 4 — acrecentar linhas, insert into table (col1, col2) values (1, 2), (3, 4).
  - Item 5 — Substituir
- **Situação:** respondido.

### 001-Q04 — NULL em listas e valores especiais

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** `Eq/Ne` com `nil` literal e IN vazio estão definidos. A documentação também permite `sql.Null*` e `driver.Valuer`, sem estabelecer se o builder deve interpretar valores que representem NULL.
- **Pergunta:** `In/NotIn` devem aceitar `nil` entre os itens ou rejeitá-lo? Para `Eq/Ne`, a conversão para `IS NULL`/`IS NOT NULL` deve valer apenas para `nil` literal ou também para ponteiros nulos e valores especiais que representem NULL?
- **Resposta do utilizador:**
  - `In/NotIn` aceita `nil` entre os itens: pode aceitar, e deve ser tratado como null, porque no sql existe isso né? in (1, 2, null).
  - `Eq/Ne` aceita `nil` literal: não deve aceitar nil mesmo de ponteiros, nos já temos IsNull e IsNotNull e devem ser usados, retornando erro caso se passe nil.
- **Situação:** parcialmente resolvida; falta definir valores especiais que representem NULL, em `001-Q09`.

### 001-Q05 — Fechamento com transação ativa

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** `DB.Close` invalida a instância e respeita a propriedade do pool, mas o contrato não especifica o destino de transações já iniciadas.
- **Pergunta:** ao chamar `DB.Close` com uma transação ativa, a biblioteca deve permitir que ela termine, retornar erro por existir transação ativa ou cancelá-la? A regra deve ser a mesma para pools próprios e fornecidos por `Wrap`?
- **Resposta do utilizador:**
  - `DB.Close`: deve retornar um erro informando que temos transações ativas, elas devem ser encerradas, todas, antes do fechamento do banco.
  - `Wrap.Close`: mesma coisa
- **Situação:** respondido.

### 001-Q06 — Garantia do modo atômico das migrações

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** o modo atômico deve rejeitar comandos incompatíveis, mas os arquivos contêm SQL arbitrário e a biblioteca não traduz nem divide scripts. Ainda falta definir como essa compatibilidade será estabelecida.
- **Pergunta:** qual contrato deve ser adotado: aceitar somente um conjunto documentado e validado de instruções atômicas, exigir declaração de compatibilidade pelo utilizador ou outra abordagem? Como tratar instruções cuja compatibilidade não possa ser determinada?
- **Resposta do utilizador:** é responsabilidade do utilizador garantir a compatibilidade da instrução, se o banco retornar um erro, o krios tenta fazer o rollback, independente do resultado, deve retornar o erro.
- **Situação:** respondido.

### 001-Q07 — Classificação de condições inválidas

- **Passo:** 1 — Consolidar os contratos.
- **Contexto:** grupos vazios são inválidos; UPDATE/DELETE com WHERE vazio exigem `ErrMissingWhere`. Falta distinguir ausência de restrição de erros internos de um predicado fornecido.
- **Pergunta:** em UPDATE/DELETE, `Where(And())` e predicados com coluna vazia devem retornar `ErrMissingWhere`, `ErrInvalidQuery` ou permitir identificação de ambos? Quando houver mais de uma condição inválida, qual erro deve prevalecer?
- **Resposta do utilizador:** no meu ponto de vista, `ErrMissingWhere` é apenas para o where, para o caso `Where(And())` e outros predicados vazios, deve-se retornar o `ErrInvalidQuery`
- **Situação:** parcialmente resolvida; classificação dos exemplos definida, prioridade entre erros simultâneos pendente em `001-Q10`.

## Complementos pendentes

### 001-Q08 — Falha na detecção automática

- **Passo:** 1 — Consolidar os contratos; complemento de `001-Q02`.
- **Contexto:** a detecção automática está definida. Ainda falta o comportamento quando a consulta da versão falha ou seu resultado não pode ser interpretado.
- **Pergunta:** nesse caso, Krios deve retornar erro e impedir o uso da instância, ou continuar com um conjunto limitado de recursos? Se deve continuar, quais recursos poderão ser usados?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q09 — Valores especiais que representam NULL

- **Passo:** 1 — Consolidar os contratos; complemento de `001-Q04`.
- **Contexto:** `Eq/Ne` devem rejeitar `nil` literal e ponteiros nulos. Entretanto, `sql.NullString{Valid: false}` não é um valor Go `nil`, e um `driver.Valuer` não nulo também pode produzir NULL.
- **Pergunta:** nesses dois casos, `Eq/Ne` também devem retornar erro e exigir `IsNull/IsNotNull`, ou devem encaminhar o valor como parâmetro ao driver? A conversão automática para `IS NULL` não será mantida para `nil`, conforme sua resposta.
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

### 001-Q10 — Prioridade entre erros simultâneos

- **Passo:** 1 — Consolidar os contratos; complemento de `001-Q07`.
- **Contexto:** `Where(And())` e predicados inválidos retornam `ErrInvalidQuery`; a ausência de WHERE retorna `ErrMissingWhere`. Uma operação pode conter os dois problemas: UPDATE sem WHERE e com coluna vazia em `Set`, por exemplo.
- **Pergunta:** nesse caso, deve prevalecer `ErrMissingWhere`, prevalecer `ErrInvalidQuery` ou o erro retornado deve permitir identificar ambos com `errors.Is`?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

## Autorização do passo 1

- **Planejamento:** redigido em [001.md](001.md) para revisão.
- **Execução da consolidação:** o utilizador autorizou continuar se as respostas resolvessem as dúvidas. Como restam `001-Q08` a `001-Q10`, aguardar seus complementos e autorização para retomar.
