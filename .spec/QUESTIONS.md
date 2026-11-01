# Dúvidas — passo 006

Revisar [006.md](006.md), responder abaixo e autorizar explicitamente a implementação. Preservar respostas até resolver todas as questões e registrá-las no plano, conforme [AGENTS.md](../AGENTS.md).

## 006-Q01 — Organização e tipos públicos

- **Contexto:** os contratos nomeiam `Expr`, `Predicate` e `TableRef`, mas ainda não fixam sua representação nem o nome do tipo de ordenação.
- **Pergunta:** aprova `internal/expression`, os arquivos por família descritos em `006.md` e aliases públicos de structs opacas `Expr`, `Predicate`, `TableRef` e `OrderTerm`? A AST terá estado privado, tipos distintos e valores zero inválidos; somente os modificadores contratados serão métodos públicos, sem `Validate/Build` nesta etapa. `Value` retornará `Expr`, também com `As`.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 006-Q02 — Agregações, ordenação e subconsultas

- **Contexto:** agregações/ordenação são expressões independentes; `Exists/NotExists` dependem do `SelectBuilder`, ainda inexistente.
- **Pergunta:** aprova construir agora `Count/Avg/Sum/Min/Max`, `Asc/Desc` e `NullsFirst/NullsLast`, deixando sua integração com builders/dialetos para os passos seguintes, e implementar `Exists/NotExists` somente no passo 9, sem tipos provisórios?
- **Resposta do utilizador:**
- **Situação:** pendente.

## 006-Q03 — Validação adiada e chamadas repetidas

- **Contexto:** construtores não retornam `error`; a validação será consumida internamente pela compilação futura.
- **Pergunta:** aprova validar o estado final com `ErrInvalidQuery`, na ordem dos operandos, sem panic por entrada inválida nem avaliação de valores? `As/InSchema/NullsFirst/NullsLast` usarão a última configuração; alias/schema explicitamente vazio será inválido, mas poderá ser corrigido por uma chamada posterior válida. Não haverá API pública para consultar erros antes dos futuros builders.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 006-Q04 — Identificadores

- **Contexto:** nomes qualificados precisam ser separados antes do escape por dialeto.
- **Pergunta:** aprova a política de `006.md`: tabela/schema/alias com um componente sem ponto; colunas qualificadas por `.` sem componentes vazios; rejeição de vazio, somente espaços, NUL e UTF-8 inválido; preservação dos demais caracteres sem trim ou interpretação de SQL? Qualificar tabelas exigirá `InSchema`; ponto literal dentro de um componente não será representável nesta API.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 006-Q05 — Curingas e aliases por contexto

- **Contexto:** `*` e `As` não são válidos em todas as posições de uma consulta.
- **Pergunta:** aprova a matriz de `006.md`: `Column("*")/Column("u.*")` somente em projeção e sem alias; `Count("*")` permitido; curingas rejeitados em demais agregações, predicados, ordenação, tabela/schema/alias? Expressões com alias serão válidas na projeção e rejeitadas como operandos de `*Expr`, sem remover alias silenciosamente.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 006-Q06 — Imutabilidade dos valores

- **Contexto:** copiar a árvore e seus slices não duplica automaticamente objetos guardados em `any`.
- **Pergunta:** aprova copiar slices de filhos/argumentos, mas manter referências dos valores internos, incluindo `[]byte`, ponteiros e `driver.Valuer`, sem deep copy nem avaliação antecipada? A aplicação deverá evitar modificar esses valores entre a construção e o fim da execução.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 006-Q07 — Análise de constantes

- **Contexto:** a proteção futura de UPDATE/DELETE precisa reconhecer predicados constantes verdadeiros sem ocultar nós inválidos ou alterar a semântica de NULL.
- **Pergunta:** aprova preservar a AST e fazer análise separada `AlwaysTrue/AlwaysFalse/Unknown`, propagando apenas as constantes de `In/NotIn` vazios por AND/OR, conforme `006.md`? Validação e classificação serão independentes, mantendo erros em ramos dominados; o futuro builder aplicará a prioridade de `ErrMissingWhere` quando também houver `ErrInvalidQuery`.
- **Resposta do utilizador:**
- **Situação:** pendente.
