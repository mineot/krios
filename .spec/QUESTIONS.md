# Dúvidas — agregações adiadas para o passo 009

O utilizador autorizou o passo 8 com Q01–Q05 e adiou agregações para o passo 9. Q06–Q09 serão retomadas nessa etapa; não bloqueiam o SELECT básico. Preservar as respostas e não limpar enquanto houver questões futuras pendentes, conforme [AGENTS.md](../AGENTS.md).

## 008-Q01 — Organização do builder

- **Contexto:** precisamos associar o builder à transação sem criar dependência circular nem expor a AST.
- **Pergunta:** aprova a organização de `008.md`, com wrapper público de valor `SelectBuilder`, lógica em `internal/selectquery`, acesso interno ao snapshot/validação da transação e inspeção contextual no pacote `expression`? Slices serão copiados ao receber/acrescentar itens, mantendo versões anteriores independentes.
- **Resposta do utilizador:** aprovado
- **Situação:** respondido.

## 008-Q02 — Build após finalizar a transação

- **Contexto:** `Build` apenas gera SQL; o snapshot de dialeto permanece disponível após commit/rollback. Essa política ficou pendente no passo 7.
- **Pergunta:** aprova permitir `Select` e `Build` usando uma transação originalmente inicializada, mesmo após commit, rollback, cancelamento de `Begin` ou posterior fechamento de `DB`, mantendo `Exec` bloqueado por `sql.ErrTxDone`? `Tx` nula/não inicializada produzirá builder inválido, e seu `Build`, assim como o builder zero, retornará `"", nil, sql.ErrTxDone`.
- **Resposta do utilizador:** aprovado
- **Situação:** respondido.

## 008-Q03 — Modificadores e chamadas vazias

- **Contexto:** omitir projeção/WHERE não equivale necessariamente a chamar esses métodos sem itens.
- **Pergunta:** aprova `Columns()` e `Where()` vazios como inválidos, `OrderBy()` vazio como operação sem efeito, `Distinct()` idempotente e `All()` como troca da projeção por `*`, preservando DISTINCT e demais cláusulas? Projeção inválida poderá ser substituída por All/Columns; condições e termos inválidos acumulados permanecerão inválidos, sem API de limpeza.
- **Resposta do utilizador:** aprovado
- **Situação:** respondido.

## 008-Q04 — Composição do SQL

- **Contexto:** chamadas sucessivas de WHERE devem combinar por AND sem perder os grupos explícitos.
- **Pergunta:** aprova o formato de `008.md`: FROM obrigatório, um grupo por chamada Where e um AND externo quando houver várias chamadas, preservando todos os grupos? Parâmetros seguirão a ordem do SQL, primeiro projeção e depois WHERE; nenhuma condição/ordenação será deduplicada ou simplificada.
- **Resposta do utilizador:** aprovado
- **Situação:** respondido.

## 008-Q05 — Prioridade de erros e execução

- **Contexto:** o builder precisa combinar validação da consulta com o estado/contexto da transação.
- **Pergunta:** aprova `Build` validar vínculo/snapshot, estrutura completa (tabela → projeção → WHERE → ordenação) e só então capacidades? Em `Exec`, a ordem será contexto nulo → transação inválida/encerrada → erro do contexto → Build → Query com nova checagem de estado/contexto, retornando linhas nativas sem consumi-las ou fechar a transação automaticamente.
- **Resposta do utilizador:** aprovado
- **Situação:** respondido.

## 008-Q06 — Agregações nesta etapa

- **Contexto:** `Count/Sum/...` já existem como expressões, embora GroupBy/Having estejam no passo 9.
- **Pergunta:** aprova aceitar agregações válidas na projeção desde agora (por exemplo, `Columns(Count("*"))`) e rejeitá-las em WHERE com `ErrInvalidQuery`, inclusive em grupos aninhados? Não será inferido GROUP BY nem validada a existência de nomes ou toda combinação entre colunas/agregações; demais regras do banco serão verificadas na execução.
- **Resposta do utilizador:** aprovado parcialmente, não gostei disso `Columns(Count("*"))`, eu acho que devemos simplificar, tipo `krios.Count("*")`, seria tipo um extensão de `Columns`, o mesmo vale para os outros `Max`, `Min` e etc...
- **Situação:** discussão adiada para o passo 9 por orientação do utilizador; resposta preservada.

## 008-Q07 — Forma de acrescentar agregações à consulta

- **Contexto:** `krios.Count("*")` já existe e retorna `Expr`, assim como `Max/Min/Sum/Avg`. A expressão isolada não identifica a consulta que deve recebê-la. Precisamos definir a composição desejada sem `Columns(...)`.
- **Pergunta:** qual forma deseja adotar? Escolha uma alternativa ou escreva um exemplo completo do encadeamento desejado.

```go
// A — método de projeção para receber as expressões existentes.
q := tx.Select(krios.Table("users")).
    Aggregate(krios.Count("*").As("total"), krios.Max("age").As("max_age"))

// B — métodos de agregação diretamente no builder.
q := tx.Select(krios.Table("users")).Count("*").Max("age")
```

- **Impacto de B:** já está previsto `query.Count(ctx) (int64, error)` no passo 9 para executar a contagem dos resultados. Go não permite os dois métodos `Count` com assinaturas diferentes no mesmo tipo. Se preferir B, indique o novo nome da contagem executável (por exemplo, `CountRows(ctx)`) e como deseja informar aliases das agregações.
- **Resposta do utilizador:**
- **Situação:** adiada para o planejamento do passo 9; não bloqueia o passo 8.

## 008-Q08 — Acumular ou substituir a projeção

- **Contexto:** os novos métodos precisam interagir com a projeção padrão `*` e com `Columns/All`, já definidos como substituição da projeção.
- **Pergunta:** aprova que os novos métodos acrescentem agregações à projeção explícita, removendo apenas o `*` implícito quando forem a primeira configuração? Chamadas posteriores a `Columns/All` substituiriam toda a projeção, inclusive agregações anteriores. Caso prefira substituição também nos novos métodos, informe.

```go
// Exemplos usando A, caso escolhida em Q07:
tx.Select(krios.Table("users")).Aggregate(krios.Count("*"))
// Projeção: COUNT(*)

tx.Select(krios.Table("users")).Columns(krios.Column("active")).
    Aggregate(krios.Count("*"))
// Projeção: active, COUNT(*) — validade da combinação depende das regras do banco.

tx.Select(krios.Table("users")).Aggregate(krios.Count("*")).All()
// Projeção: *
```

- **Resposta do utilizador:**
- **Situação:** adiada para o planejamento do passo 9; não bloqueia o passo 8.

## 008-Q09 — Compatibilidade e validação de WHERE

- **Contexto:** `Columns(...Expr)` foi definido para aceitar expressões, incluindo agregações, e isso aparece nos exemplos anteriores. Sua resposta pode indicar preferência por um atalho ou intenção de restringir essa API.
- **Pergunta:** a nova forma será um atalho, mantendo `Columns(krios.Count("*"))` válido, ou deseja proibir agregações em `Columns`? Confirme também se permanece aprovada a rejeição de agregações em WHERE com `ErrInvalidQuery`, inclusive em grupos aninhados, conforme a outra parte de Q06.
- **Resposta do utilizador:**
- **Situação:** adiada para o planejamento do passo 9; não bloqueia o passo 8.
