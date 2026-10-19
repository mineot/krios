# E03-P01 – Decisões sobre Contratos de Execução

Data original: 2026-10-19. Revisão vigente: M01–M06, na mesma data.

## 1. Objetivo

Definir a execução compartilhada por `DB`, `Tx` e futuros construtores/arquivos SQL. A revisão corrige propagação de erros e estado terminal, mantendo as assinaturas públicas. Implementação em `internal/core`, nos arquivos `execution.go`, `rows.go`, `result.go`, `db.go` e `tx.go`. A fachada `krios.go` na raiz expõe aliases e delega `Open`; o consumidor mantém o import público original.

## 2. Decisões resolvidas

Os resultados nativos permanecem encapsulados. A interface de execução é interna; o consumidor usa os métodos públicos de `DB`/`Tx`. O consumidor fecha e verifica resultados de leitura. A política de transações está definida em §3.4; associação dos construtores e SQL incorporado permanece para E05/E07.

## 3. Contrato vigente

### 3.1. Resultados

```go
type Result interface {
    RowsAffected() (int64, bool)
    LastInsertId() (int64, bool)
}
```

`true` indica valor disponível, inclusive zero. `false` indica informação indisponível, inclusive erro do driver ao obtê-la; o erro desse acesso não é exposto pela interface. A escrita já executada não é repetida para obter o resultado.

`Row` encapsula uma linha e um eventual erro pendente. `Rows` encapsula um iterador. Seus valores úteis são obtidos pelos métodos de consulta; não instanciar wrappers vazios para executar consultas.

### 3.2. Assinaturas de execução

O contrato abaixo é satisfeito por `*DB` e `*Tx` e permanece não exportado:

```go
type executor interface {
    ExecContext(context.Context, string, ...any) (Result, error)
    QueryContext(context.Context, string, ...any) (*Rows, error)
    QueryRowContext(context.Context, string, ...any) Row
}
```

Parâmetros são separados do SQL. O SQL recebido não sofre tradução entre bancos. Construtores futuros geram marcadores pelo dialeto; SQL fornecido pelo consumidor já deve usar o formato aceito pelo driver.

Erros preservam `errors.Is`/`errors.As`. Contexto cancelado/expirado, `ErrClosed` e `ErrTxDone` não são convertidos em ausência de resultado. `QueryRowContext` adia o erro até `Scan`; `sql.ErrNoRows` indica consulta executada sem linha. `sql.ErrTxDone` nativo é normalizado para também preservar o sentinela de Krios.

### 3.3. Propriedade e encerramento de recursos de leitura

O consumidor registra `defer rows.Close()` imediatamente após uma consulta bem-sucedida, trata erros de `Scan` e consulta `Err` ao terminar a iteração. Exaustão pode fechar o iterador nativo automaticamente, mas o fechamento explícito protege retornos antecipados.

`Close` é idempotente e `Err` continua acessível após fechar. Uma falha de iteração não pode desaparecer por causa da limpeza. Erros de conversão em `Scan` são tratados no ponto da chamada, não necessariamente por `Rows.Err`.

`Row` deve ser consumida por `Scan`, que conclui seu ciclo de vida. Fechar resultados não confirma nem reverte transações. Helpers de mapeamento ficam para E04.

### 3.4. Associação à conexão ou à transação

#### 3.4.1. Executor interno

`executor` usa tipos Krios e é o contrato para os consumidores internos, preservando guards e bloqueio de commit. `sqlExecutor` é o contrato de delegação nativa, satisfeito por `*sql.DB`/`*sql.Tx`. As duas interfaces têm verificações em compilação. Construtores não devem contornar `DB`/`Tx` para executar diretamente no contrato nativo.

#### 3.4.2. Ciclo de vida da transação

`Begin(ctx) (*Tx, error)` usa `BeginTx`. O contexto permanece válido por toda a transação; seu cancelamento provoca reversão nativa. Operações feitas no DB continuam fora da transação. `Tx` não retém o DB nem sua configuração completa.

Após finalização, operações retornam `ErrTxDone`, inclusive em `Row.Scan`. Falha na chamada nativa de confirmação/reversão também torna o wrapper terminal. Cancelamento rejeitado antes da tentativa nativa de commit, sem erro prévio, não encerra por si só a transação. Cancelamento do contexto de `Begin` é um caso diferente e pode já ter provocado rollback nativo.

#### 3.4.3. Concorrência

`DB` permite uso por goroutines distintas. Chamadas de execução e finalização na mesma `Tx` são serializadas, sem ordem garantida entre goroutines. Essa serialização não abrange a vida útil dos resultados retornados.

Um iterador tem um consumidor responsável por `Next`/`Scan`. O fluxo portátil consome ou fecha resultados antes de outra operação na mesma transação e aguarda consumidores antes de finalizar. Fechamento concorrente e finalização não são substitutos para coordenação da aplicação.

#### 3.4.4. Commit/Rollback com contexto

As assinaturas `Commit(ctx) error` e `Rollback(ctx) error` foram preservadas. O contexto fornecido não interrompe chamadas nativas de finalização, que não possuem versões contextuais em `database/sql`.

`Rollback(ctx)` tenta a limpeza mesmo com contexto cancelado/expirado. Retorna `nil` se a limpeza tiver sucesso e preserva erro se falhar; chamadas após término retornam `ErrTxDone`. Essa regra substitui a validação original de `ctx.Err()` que podia impedir a limpeza.

Sem falha imediata registrada, `Commit` verifica o contexto antes de chamar o método nativo. Após a tentativa nativa, o wrapper deixa de ser reutilizável. Uma confirmação que falha não comprova o estado remoto e não autoriza repetição automática de escrita. Ver as [orientações de transações do Go](https://go.dev/doc/database/execute-transactions).

#### 3.4.5. Falha de statement e Commit (decidido em E03-P03)

A transação memoriza o primeiro erro imediato de `ExecContext`/`QueryContext`. Se o consumidor tentar `Commit`, a biblioteca executa rollback mesmo com o contexto dessa chamada cancelado, encerra o wrapper e retorna o erro original. Havendo falha de rollback, ambos os erros permanecem inspecionáveis por `errors.Is`/`errors.As`. Estado terminal nativo é tratado como término, não como novo statement que precise ser revertido.

A garantia é limitada a erros imediatos desses dois métodos. Erros de `Row.Scan` e iteração não são registrados automaticamente e exigem rollback explícito do consumidor. Ausência de linhas e falhas de conversão não ganham políticas implícitas de abortar transações. A biblioteca não garante rollback de comandos que o banco confirme implicitamente.

## 4. Histórico e compatibilidade

| Decisão original | Estado na revisão |
| --- | --- |
| Interface de execução interna | Mantida; agora há distinção explícita entre contrato Krios e contrato nativo |
| Result como interface `(int64, bool)` | Mantida, com disponibilidade documentada |
| Fechamento de Rows pelo consumidor | Mantido; preservação do erro corrigida |
| Serialização de chamadas em Tx | Mantida, com limite explícito para resultados abertos |
| Commit/Rollback verificavam contexto antes de tudo | Superada: limpeza tem precedência; rejeição prévia de commit sem erro registrado continua |
| Preservação de `sql.ErrNoRows` | Mantida somente para consulta vazia; teste que mascarava encerramento foi corrigido |
| E03-P02: finalização marcava done apenas no sucesso | Superada: tentativas nativas são terminais mesmo quando falham |
| E03-P03: commit bloqueado por primeiro erro imediato | Mantida; erro duplo preservado e contexto cancelado não impede a limpeza |

Código que usava `ErrNoRows` para detectar encerramento deve usar `ErrClosed`/`ErrTxDone`. Código que esperava cancelamento de `Rollback` deve tratar o resultado da limpeza. A assinatura de nenhum método público mudou.

## 5. Evidências

Os nomes de testes internos abaixo são relativos a `internal/core/`; integração e exemplos continuam em `tests/`.

- `rows_test.go`: ausência de linhas, erros pendentes e de driver, conversão, propriedade e erro após fechamento, disponibilidade de Result.
- `tx_test.go` e `driver_test.go`: finalização que falha, limpeza cancelada, erro duplo, término por contexto, resultados abertos e finalizações concorrentes.
- `execution_test.go`: contrato interno preserva guards/política transacional e cancelamento em operações em andamento.
- `tests/tx_test.go` e `tests/tx_group_test.go`: persistência/reversão em SQLite, grupo, independência e concorrência.
- `tests/examples_test.go`: consumo externo, consulta, rollback registrado, commit bloqueado e erro tardio tratado pelo consumidor.

Resultados das verificações estão no [plano de melhorias](0003_improvement_plan.md). Integração real MySQL/PostgreSQL segue o [roteiro E09](E09_manual_validation.md) e permanece pendente.
