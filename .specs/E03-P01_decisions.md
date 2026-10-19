# E03-P01 – Decisões sobre Contratos de Execução

## 1. Objetivo

Definir os contratos de execução que servirão como base compartilhada entre construtores (SELECT/INSERT/UPDATE/DELETE) e arquivos SQL incorporados via `go:embed`.

Estas decisões visam padronizar como `DB` e `Tx` executam queries, como recursos são gerenciados e como manter o isolamento entre a camada pública da biblioteca e `database/sql`.

## 2. Pendências

- **Tipos expostos de database/sql** – Definir se a API pública expõe tipos nativos (`*sql.DB`, `*sql.Tx`, `*sql.Rows`, `sql.Result`) ou se os encapsula. _(Respondida na §3.1; decisão na §4, pergunta 2.)_
- **Assinaturas de execução** – Definir métodos para distinguir escrita (`Exec`) e consulta com linhas (`Query`/`QueryRow`), uso de `context.Context`, recebimento de parâmetros e tratamento de erros. _(Respondida na §3.2; decisão na §4, pergunta 1.)_
- **Propriedade e encerramento de recursos de leitura** – Definir quem é responsável por fechar `*sql.Rows`, como evitar vazamentos e como garantir segurança para mapeamento de entidades (RF-09). _(Decidido na §3.3: consumidor responsável; `Close()` idempotente; pergunta 3 resolvida.)_
- **Associação à conexão ou transação** – Criar um `Executor` comum, implementado tanto por `*DB` quanto por `*Tx`, permitindo reutilizar o mesmo contrato em ambos os contextos. _(Decidido na §3.4: `executor` interno comum; serialização de `*Tx` via `sync.Mutex` (pergunta 4); `Commit`/`Rollback` com `context.Context` validando `ctx.Err()` (pergunta 5); pergunta 6 resolvida em §3.2.)_

## 3. Decisões Propostas

### 3.1. Tipos expostos de database/sql

A recomendação é encapsular ao invés de expor diretamente os tipos da `database/sql`. Isso reduz acoplamento, facilita tradução/normalização de erros e prepara o terreno para o mapeamento de entidades em E04.

| Tipo database/sql | Exposição Proposta             | Observações                                                                                                      |
| ----------------- | ------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| `*sql.DB`         | `*krios.DB` (struct)           | Mantém estado interno (dialeto, `closed`, etc.) e controla acesso.                                               |
| `*sql.Tx`         | `*krios.Tx` (struct)           | Controla ciclo de vida (`done`), rejeita operações após Commit/Rollback e aplica regras de concorrência.         |
| `sql.Result`      | `krios.Result` (interface)     | Expor via métodos explícitos. Evita depender de campos/semântica específica por driver.                          |
| `*sql.Rows`       | `*krios.Rows` (struct wrapper) | Encapsula `*sql.Rows` com `Next`, `Scan`, `Err`, `Close`. Garante `Close()` idempotente.                         |
| `*sql.Row`        | `krios.Row` (struct wrapper)   | Equivalente a `*sql.Row`, porém sem expor ponteiro bruto. `Scan` deve manter o comportamento de `sql.ErrNoRows`. |

Definição decidida para `Result` (interface, conforme §4 pergunta 2):

```go
type Result interface {
    RowsAffected() (int64, bool)
    LastInsertId() (int64, bool)
}
```

- Retornar `(0, false)` quando o driver não suporta `LastInsertId()` ou `RowsAffected()`.
- Manter o valor numérico quando suportado, com `bool` indicando disponibilidade.

### 3.2. Assinaturas de Execução

Todos os métodos devem obrigatoriamente aceitar `context.Context`. Parâmetros devem permanecer separados da string SQL, cabendo ao dialeto apenas fornecer os marcadores corretos (`?`, `$1`, `@p1`, etc.), conforme RF-02 e RF-08.

Contrato de execução — métodos públicos de `*DB` e `*Tx`. A interface `Executor` **não será exportada** nesta etapa (decisão na §4, pergunta 1); é exposta apenas se surgir um caso real claro de uso pelo consumidor. A forma do contrato:

```go
type Executor interface {
    ExecContext(ctx context.Context, query string, args ...any) (Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) Row
}
```

Regras para tratamento de erros:

- Preservar `context.Canceled` e `context.DeadlineExceeded` sem re-embrulhar de forma que quebre `errors.Is`.
- Retornar `ErrClosed` ao tentar executar em `*DB` já fechado.
- Retornar `ErrTxDone` (ou equivalente) ao tentar executar em `*Tx` após Commit/Rollback.
- Manter erros nativos do driver quando relevantes, utilizando `%w` apenas quando for introduzir um wrapper próprio.
- Não realizar tradução de sintaxe SQL. Essa responsabilidade pertence exclusivamente ao dialeto.
- `Row.Scan` deve retornar `sql.ErrNoRows` quando não houver resultado, mantendo total compatibilidade com `database/sql`.

### 3.3. Propriedade e Encerramento de Recursos de Leitura

O padrão adotado deve ser o mesmo de `database/sql`: o consumidor é o único responsável por fechar `*krios.Rows`.

Regras obrigatórias:

- `QueryContext` só retorna `(Rows, error)`. Em caso de erro, nenhum handle de linhas é retornado ou, se parcialmente obtido, já deve estar fechado.
- `*krios.Rows.Close()` deve ser idempotente. Chamadas múltiplas não devem gerar erro.
- Iterar até `Next() == false` não substitui `Close()`. O fechamento deve ocorrer mesmo em: abandono antecipado do loop, erro durante Scan, panic ou retorno parcial.
- `QueryRowContext` não retorna um recurso passível de `Close()` pelo consumidor. Seu ciclo de vida é limitado à chamada de `Scan`.
- Fechar `Rows` não confirma, nem reverte uma transação. Commit e Rollback permanecem operações explícitas e independentes.
- Para facilitar mapeamento em E04, recomenda-se utilizar `defer rows.Close()` imediatamente após a verificação de erro.
- Métodos utilitários do tipo `QueryAll[T](...) ([]T, error)` que leem e fecham automaticamente não fazem parte de E03-P01. Pertencem a E04–E05.

### 3.4. Associação à Conexão ou à Transação (Executor Comum)

Para permitir que construtores e SQL incorporado sejam reutilizáveis tanto em escopo de conexão quanto em escopo transacional, ambos `*DB` e `*Tx` devem implementar o mesmo contrato de execução.

#### 3.4.1. Executor Interno

Recomenda-se manter uma interface `executor` não exportada nesta etapa. Seu único propósito é evitar duplicação entre `DB` e `Tx` e facilitar futuros testes internos.

```go
// interno
type executor interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// público
type DB struct {
    db     *sql.DB
    dial   Dialect // definido em E05-P01
    closed atomic.Bool
    mu     sync.RWMutex
}

type Tx struct {
    tx   *sql.Tx
    done atomic.Bool
    mu   sync.Mutex
}
```

As versões públicas (`ExecContext`/`QueryContext`/`QueryRowContext`) delegam ao executor subjacente e aplicam os wrappers (`wrapResult`, `wrapRows`, `wrapRow`) antes de retornar à API pública.

#### 3.4.2. Ciclo de Vida da Transação

- `Begin(ctx context.Context) (*Tx, error)` – Deve respeitar `ctx`. Retornar erro caso `*DB` já esteja fechado.
- Após `Commit(ctx context.Context)` ou `Rollback(ctx context.Context)`, qualquer nova chamada de execução em `*Tx` deve retornar `ErrTxDone`.
- `*Tx` não deve reter referência à `*DB` com intuito de "reativar" conexão. Caso precise acessar dialeto, pode armazená-lo por cópia no momento do Begin.
- Operações em `*DB` nunca utilizam uma "transação ativa global". Apenas `*Tx` executa dentro do escopo daquela transação (alinhado a RF-03).

#### 3.4.3. Concorrência

- **Entre DB e Tx distintos:** Não há compartilhamento direto de estado mutável que exija sincronização cruzada nesta camada. Podem ser usados concorrentemente.
- **Dentro do mesmo `*Tx`:** Recomenda-se serializar chamadas com `sync.Mutex`. Isso evita estados ambíguos decorrentes de encadeamentos incorretos, sem impor complexidade excessiva nesta etapa.
- **Dentro do mesmo `*DB`:** `*sql.DB` já é thread-safe. O wrapper apenas adiciona checagens de `closed`, podendo utilizar `sync.RWMutex` caso surja necessidade futura.

> **Recomendação:** Serializar acessos ao mesmo `*Tx`. Não assumir "safe for concurrent use" para `*Tx`, deixando essa expectativa explícita na documentação da API.

#### 3.4.4. Commit/Rollback com context.Context

`database/sql` não possui `*sql.Tx.CommitContext`/`RollbackContext`. Ainda assim, é desejável aceitar `context.Context` para:

- Respeitar `context.Canceled` antes de efetuar a chamada.
- Permitir aplicação de timeout via `context.WithTimeout` quando houver justificativa prática em casos futuros.
- Manter consistência com o restante da API (`ExecContext`, `QueryContext`, `Begin`, etc.).

**Decisão proposta:** `Commit(ctx context.Context) error` e `Rollback(ctx context.Context) error` devem aceitar `context.Context`. Nesta etapa, pelo menos validar `ctx.Err()` no início. A adoção de `context.Context` com deadline para as chamadas nativas pode ser refinada em E03-P02, porém a assinatura já deve ser definida aqui.

## 4. Perguntas para Validação

| #   | Pergunta                                                                                         | Opções                                           | Recomendado                                                                                                                                                 |
| --- | ------------------------------------------------------------------------------------------------ | ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Superfície pública:** Expor a interface `Executor` publicamente?                               | interna / pública                                | **Decidido (2026-10-19): interna.** Manter não exportada nesta etapa; expor apenas se houver um caso real claro de uso pelo consumidor.                     |
| 2   | **Result:** Retornar struct wrapper próprio ou manter `sql.Result` exposto?                      | `krios.Result` (struct/interface) / `sql.Result` | **Decidido (2026-10-19): `krios.Result` como interface** com `RowsAffected()` e `LastInsertId()`. Mais claro, isolado do driver e facilita mocks em testes. |
| 3   | **Rows:** Exigir `defer rows.Close()` explícito ou oferecer helpers que fecham?                  | iterador + `Close()` / helpers                   | **Decidido (2026-10-19): iterador bruto + `Close()` obrigatório.** Helpers (`ScanAll`, etc.) ficam para E04.                                               |
| 4   | **Concorrência em `*Tx`:** Como tratar chamadas concorrentes?                                    | serializar / permitir / retornar erro            | **Decidido (2026-10-19): serializar com `sync.Mutex`.** Simples, seguro e evita erros sutis nesta fase. Não assumir "safe for concurrent use" para `*Tx`.   |
| 5   | **Commit/Rollback com contexto:** Deve respeitar `context.Context` mesmo sem equivalente nativo? | sim / não                                        | **Decidido (2026-10-19): sim.** Validar `ctx.Err()` antes da chamada e manter assinatura consistente com o restante da API.                                 |
| 6   | **QueryRow:** Manter `sql.ErrNoRows` em `Row.Scan`?                                              | sim / não                                        | **Decidido (2026-10-19): sim.** Essencial para manter semântica familiar e interoperabilidade com código existente.                                        |

> **Status (2026-10-19):** perguntas 1 e 2 registradas nas §3.1 e §3.2; perguntas 3–6 decididas nas respectivas propostas (§3.3, §3.4.3 e §3.4.4 e regra em §3.2). Todas as seis perguntas de validação foram resolvidas.

## 5. Plano de Ação Imediato

1. **Criar `Result`, `Rows`, `Row` + `wrap*`** – Implementar wrappers mínimos com idempotência em `Close()` e semântica correta para Scan.
2. **Definir executor interno** – Interface compartilhada, implementada por `DB` e `Tx`.
3. **Adicionar métodos em `*DB`** – `ExecContext`, `QueryContext`, `QueryRowContext` delegando ao `*sql.DB` com wrappers.
4. **Adicionar métodos em `*Tx`** – Idênticos aos de `*DB`, acrescidos de `Commit(ctx context.Context)`, `Rollback(ctx context.Context)`, checagem de `done` e serialização via `mu`.
5. **Implementar `Begin(ctx context.Context)`** – Respeitar contexto, validar DB fechado e retornar `*Tx` inicializado.
6. **Aplicar checagens de estado** – Retornar `ErrClosed` em `*DB` e `ErrTxDone` em `*Tx` quando aplicável.
7. **Documentar decisões** – Registrar propriedade de `Rows`, regra de concorrência para `*Tx` e comportamento de Commit/Rollback com `context.Context`.

## 6. Critério de Aceite

- `*DB` e `*Tx` compartilham exatamente o mesmo contrato de execução.
- `Rows` possui `Close()` idempotente e sua propriedade é claramente definida como do consumidor.
- `Begin` retorna `*Tx` utilizável pelos mesmos fluxos que utilizam `*DB` (contrato unificado).
- Todas as operações de execução aceitam `context.Context`.
- Estado inválido (fechado / já commitado/rollbackado) é rejeitado com erros bem definidos.
- Decisões 1–6 da seção 4 já estão registradas e decididas; a fase de consenso está concluída. E03-P01 pode prosseguir para marcação como concluída após a implementação/refino conforme Plano de Ação (§5).

## 7. Impacto

- Desbloqueia E03-P02 (Begin/Commit/Rollback) com contrato já estável.
- Viabiliza E05–E06 (construtores) e E07 (SQL incorporado via `go:embed`), pois ambos podem operar sobre `Executor` sem conhecer `DB` ou `Tx` diretamente.
- Prepara E04 (mapeamento entidade→linha) ao fornecer `Rows` encapsulado e regras claras de fechamento.
- Reduz acoplamento com `database/sql`, mantendo superfície pública mínima, previsível e testável.
