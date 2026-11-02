# Dúvidas — passo 008

Revisar [008.md](008.md), responder abaixo e autorizar explicitamente a implementação. Preservar respostas até resolver todas as questões e registrá-las no plano, conforme [AGENTS.md](../AGENTS.md).

## 008-Q01 — Organização do builder

- **Contexto:** precisamos associar o builder à transação sem criar dependência circular nem expor a AST.
- **Pergunta:** aprova a organização de `008.md`, com wrapper público de valor `SelectBuilder`, lógica em `internal/selectquery`, acesso interno ao snapshot/validação da transação e inspeção contextual no pacote `expression`? Slices serão copiados ao receber/acrescentar itens, mantendo versões anteriores independentes.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 008-Q02 — Build após finalizar a transação

- **Contexto:** `Build` apenas gera SQL; o snapshot de dialeto permanece disponível após commit/rollback. Essa política ficou pendente no passo 7.
- **Pergunta:** aprova permitir `Select` e `Build` usando uma transação originalmente inicializada, mesmo após commit, rollback, cancelamento de `Begin` ou posterior fechamento de `DB`, mantendo `Exec` bloqueado por `sql.ErrTxDone`? `Tx` nula/não inicializada produzirá builder inválido, e seu `Build`, assim como o builder zero, retornará `"", nil, sql.ErrTxDone`.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 008-Q03 — Modificadores e chamadas vazias

- **Contexto:** omitir projeção/WHERE não equivale necessariamente a chamar esses métodos sem itens.
- **Pergunta:** aprova `Columns()` e `Where()` vazios como inválidos, `OrderBy()` vazio como operação sem efeito, `Distinct()` idempotente e `All()` como troca da projeção por `*`, preservando DISTINCT e demais cláusulas? Projeção inválida poderá ser substituída por All/Columns; condições e termos inválidos acumulados permanecerão inválidos, sem API de limpeza.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 008-Q04 — Composição do SQL

- **Contexto:** chamadas sucessivas de WHERE devem combinar por AND sem perder os grupos explícitos.
- **Pergunta:** aprova o formato de `008.md`: FROM obrigatório, um grupo por chamada Where e um AND externo quando houver várias chamadas, preservando todos os grupos? Parâmetros seguirão a ordem do SQL, primeiro projeção e depois WHERE; nenhuma condição/ordenação será deduplicada ou simplificada.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 008-Q05 — Prioridade de erros e execução

- **Contexto:** o builder precisa combinar validação da consulta com o estado/contexto da transação.
- **Pergunta:** aprova `Build` validar vínculo/snapshot, estrutura completa (tabela → projeção → WHERE → ordenação) e só então capacidades? Em `Exec`, a ordem será contexto nulo → transação inválida/encerrada → erro do contexto → Build → Query com nova checagem de estado/contexto, retornando linhas nativas sem consumi-las ou fechar a transação automaticamente.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 008-Q06 — Agregações nesta etapa

- **Contexto:** `Count/Sum/...` já existem como expressões, embora GroupBy/Having estejam no passo 9.
- **Pergunta:** aprova aceitar agregações válidas na projeção desde agora (por exemplo, `Columns(Count("*"))`) e rejeitá-las em WHERE com `ErrInvalidQuery`, inclusive em grupos aninhados? Não será inferido GROUP BY nem validada a existência de nomes ou toda combinação entre colunas/agregações; demais regras do banco serão verificadas na execução.
- **Resposta do utilizador:**
- **Situação:** pendente.
