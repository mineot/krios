# C03-P01 — Decisões de inspeção de objetos (D6–D9)

Data: 2026-10-20. Resolve C03-P01 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §5 e o achado A04. Registra o comportamento da verificação de `ManagedObjects` antes da correção: validação de nomes, escopo da consulta de existência, parametrização e diagnóstico de erros. As escolhas foram apresentadas e aprovadas pelo consumidor em 2026-10-20. Complementa [E08-P01_decisions.md](E08-P01_decisions.md) §6 (esquema gerenciado), [C01-P01_decisions.md](C01-P01_decisions.md) (D01–D05) e os contratos de erro de [E02-P01_decisions.md](E02-P01_decisions.md).

## 1. Contexto

A inspeção dirigida da consolidação (A04) mostrou que `existenceQuery` interpola o nome do objeto em literais SQL (`name = '%s'`), que o caminho MySQL/PostgreSQL consulta `information_schema.tables` sem delimitar banco/esquema (falso positivo com homônimos em outros esquemas) e que `isMissingTable` depende de fragmentos amplos de mensagens de erro. A validação de `MigrationsConfig` só contava a lista de objetos: nome vazio, qualificado ou com caracteres problemáticos passava despercebido e virava "ausente" silencioso ou erro bruto do driver.

## 2. D6 — Formato dos nomes em `ManagedObjects`

**Comportamento escolhido.** Cada entrada de `ManagedObjects` é um nome de tabela simples (`users`) ou qualificado (`schema.users`), com no máximo duas partes separadas por ponto.

- Validação estrutural em `MigrationsConfig.Validate` (`internal/config`): nome não vazio, sem espaços nas bordas, no máximo duas partes e nenhuma parte vazia (`""`, `"."`, `"a."`, `".b"`, `"a..b"` são `ErrInvalidConfig`).
- Validação de dialeto em `migrate.Prepare`: SQLite rejeita nome qualificado com `ErrInvalidConfig` — SQLite não tem esquemas de usuário; o consumidor deve declarar o nome simples da tabela.
- MySQL e PostgreSQL aceitam ambas as formas; a parte antes do ponto é o esquema e a parte depois é a tabela.

**Alternativas rejeitadas.**

- _Aceitar qualificação também em SQLite (ex.: `main.users`)._ Rejeitada: `sqlite_master` qualificado exigiria caminhos específicos por arquivo/temp schema sem benefício mensurável; uniformidade artificial acrescentaria frágilidade sem requisito correspondente.
- _Rejeitar qualificação em todos os dialetos._ Rejeitada: deixa o consumidor de MySQL/PostgreSQL sem forma de apontar um esquema específico e mantém o falso positivo de homônimos sem solução contratual.

**Impacto de compatibilidade.** Alarga o contrato (nomes qualificados passam a ser válidos em MySQL/PostgreSQL) e restringe um caso antes aceito silenciosamente em SQLite (qualificado passa a falhar cedo, em vez de nunca casar). Nenhuma assinatura muda.

## 3. D7 — Escopo da consulta de existência

**Comportamento escolhido.** A consulta de existência é parametrizada e delimitada ao esquema/banco da conexão, coerente com a tabela `migrations`, que é criada sem qualificação no esquema corrente.

- Nome simples no MySQL: `table_schema = DATABASE() AND table_name = ?`.
- Nome simples no PostgreSQL: `table_schema = current_schema() AND table_name = ?`.
- Nome qualificado em MySQL/PostgreSQL: `table_schema = ?` com o esquema declarado, mais `table_name = ?` (placeholder `$n` no PostgreSQL, conforme `Dialect.Placeholder`).
- SQLite: `sqlite_master WHERE type = 'table' AND name = ?` (esquema único).
- O nome do consumidor nunca é interpolado no texto da consulta; o valor segue como argumento, como já fazem os helpers de histórico (`internal/migrate/history.go`).

**Alternativas rejeitadas.**

- _Sem filtro de esquema (comportamento anterior)._ Rejeitada: homônimo em qualquer esquema visível contava como "presente" e bloqueava a inicialização indevidamente (falso positivo de A04).
- _Qualquer esquema do banco para nomes simples._ Rejeitada pelo mesmo motivo; o escopo correto é o esquema em que a tabela de histórico é criada.
- _Introspecção genérica ou enumeração do esquema._ Rejeitada: mantém E08-P01 §6 (sem introspecção além da consulta pontual).

**Impacto de compatibilidade.** Comportamento corrigido: um objeto homônimo em outro esquema deixa de bloquear. Consumidores que dependiam do falso positivo não têm caso legítimo documentado.

## 4. D8 — Diagnóstico de erros da inspeção

**Comportamento escolhido.**

- Falha da consulta de existência (permissão, conexão, sintaxe) é propagada com contexto e causa preservada por `%w`: `krios: migrations: inspect managed object "<nome>": <causa>`. Sem sentinela nova: o consumidor usa `errors.Is` com os erros do driver ou com `context` já suportados.
- `isMissingTable` permanece textual (a única forma portátil via `database/sql`), com os padrões documentados por motor: SQLite `"no such table"`, MySQL `"doesn't exist"`, PostgreSQL `"does not exist"` (mais `"undefined table"` como variação conservadora). A função é usada apenas para a sonda da tabela de histórico; erros que não casam com esses padrões (permissão, conexão) nunca são tratados como "tabela ausente" e sobem como erro.
- O caso "sem histórico mas objeto gerenciado presente" continua sendo `ErrInvalidQuery` (E08-P01 §6); a divergência documental apontada em errs/krios.go sobre `ErrPartialMigration` pertence a C05-P01 e não é alterada aqui.

**Alternativas rejeitadas.**

- _Nova sentinela (ex.: `ErrInspectionFailed`) ou reaproveitar `ErrConnectionFailed`._ Rejeitada: a classificação dependeria de mensagem de driver e prometeria portabilidade que `database/sql` não oferece; envolver com causa preservada dá a mesma informação sem contrato frágil.
- _Remover o reconhecimento textual e tratar toda falha como erro fatal._ Rejeitada: sem tabela `migrations` é o estado legítimo "versão 0"; excluir o caminho quebraria a inicialização em banco vazio.

**Impacto de compatibilidade.** Aumenta a riqueza do erro (contexto preservado); nenhuma sentinela nova para o consumidor aprender.

## 5. D9 — Verificação por dialeto e evidência

**Comportamento escolhido.** A geração (SQL + argumentos) é verificada por testes unitários nos três dialetos com o driver simulado (`internal/testutil`), conferindo placeholder, filtro de esquema e ordem dos argumentos. O cenário de tabelas homônimas em esquemas distintos não é reproduzível em SQLite e permanece como evidência real obrigatória em MySQL/PostgreSQL (E09/E10, cenário M07 ampliado). Integração SQLite cobre parametrização (nome com apóstrofo), rejeição de qualificado e validação estrutural na abertura.

**Código afetado.** `internal/config/config.go` (`MigrationsConfig.Validate`, `ManagedObjects` doc), `internal/migrate/migrate.go` (`existenceQuery`/`objectExists`/`verifyNoManagedObjects`/`Prepare`/`isMissingTable`), testes novos em `internal/migrate` e `internal/config`, `tests/migrations_test.go`, [E08-P01_decisions.md](E08-P01_decisions.md) §6, MANUAL e `E09_manual_validation.md` (M07).
