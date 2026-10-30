# Dúvidas — passo 004

Revisar [004.md](004.md), responder às questões e autorizar explicitamente a implementação. Preservar as respostas até registrar todas as decisões no plano e limpar este arquivo, conforme [AGENTS.md](../AGENTS.md).

## 004-Q01 — Organização interna das transações

- **Passo:** 4 — Implementar transações.
- **Contexto:** a conexão e seu contador estão em `internal/connection`; transações terão estado e finalização próprios.
- **Pergunta:** aprova criar `internal/transaction`, mantendo abertura/contagem na conexão e usando uma função interna de liberação para evitar dependência circular, com wrappers na raiz e arquivos separados conforme `004.md`?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

## 004-Q02 — Cancelamento e liberação automática

- **Passo:** 4 — Implementar transações.
- **Contexto:** `database/sql` pode desfazer a transação quando o contexto de `BeginTx` é cancelado. Sem acompanhamento no Krios, seu contador poderia continuar impedindo `DB.Close`.
- **Pergunta:** aprova que Krios acompanhe o cancelamento, coordene o encerramento com a transação nativa e libere automaticamente o registro, mesmo sem chamada posterior da aplicação a `Commit/Rollback`? Durante a finalização, `Close` ainda poderá retornar `ErrActiveTransactions`.
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

## 004-Q03 — Erros de finalização

- **Passo:** 4 — Implementar transações.
- **Contexto:** commit/rollback podem retornar erro, inclusive de contexto; o retorno nativo após cancelamento pode ser erro de contexto ou `sql.ErrTxDone`, dependendo da etapa do encerramento.
- **Pergunta:** aprova preservar o erro nativo recebido, considerar o wrapper encerrado ao concluir o tratamento da finalização e liberar seu registro uma única vez? Para falha de `Commit`, a proposta é tentar rollback de limpeza, retornar o erro original do commit independentemente do resultado dessa tentativa e não repetir a confirmação. Chamadas posteriores retornariam `sql.ErrTxDone`.
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

## 004-Q04 — Close durante abertura de transação

- **Passo:** 4 — Implementar transações.
- **Contexto:** `BeginTx` pode aguardar uma conexão disponível. O fechamento precisa ser coordenado desde o início dessa espera, sem manter o mutex de `DB` preso durante acesso ao driver.
- **Pergunta:** aprova reservar o contador antes de chamar `BeginTx`, fazendo `Close` retornar `ErrActiveTransactions` também enquanto a abertura estiver em andamento, e desfazer essa reserva caso a abertura falhe?
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.

## 004-Q05 — Entradas inválidas e opções

- **Passo:** 4 — Implementar transações.
- **Contexto:** precisamos especificar a validação das novas operações sem acrescentar interpretação das opções do banco.
- **Pergunta:** aprova rejeitar contexto nulo em `Begin` com `ErrInvalidConfig`, banco fechado/não inicializado com `ErrClosed` e `Tx` nula/não inicializada com `sql.ErrTxDone`, encaminhando `*sql.TxOptions` sem alterações para `database/sql`? Se contexto e banco forem inválidos simultaneamente, a proposta é validar o contexto primeiro.
- **Resposta do utilizador:**
- **Situação:** aguardando resposta.
