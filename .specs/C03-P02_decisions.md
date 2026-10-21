# C03-P02 — Decisões de classificação transacional (D10–D13)

Data: 2026-10-20. Resolve C03-P02 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §5 e o achado A06. Registra o comportamento da classificação de atomicidade dos arquivos de migração antes da correção: keywords desconhecidas, comandos de controle de transação, inspeção em SQLite/PostgreSQL e tokens entre aspas. As escolhas foram apresentadas e aprovadas pelo consumidor em 2026-10-20. Complementa [E08-P01_decisions.md](E08-P01_decisions.md) §10/§11, [C01-P01_decisions.md](C01-P01_decisions.md) (D03) e [C03-P01_decisions.md](C03-P01_decisions.md) (D6–D9).

## 1. Contexto e conferência nas fontes oficiais

O achado A06 mostrou duas falhas: `fileTransactional` aceita todo arquivo em SQLite/PostgreSQL pelo sinalizador `SupportsTransactionalDDL` sem inspecionar nenhum statement, e `isTransactionalStatement` retorna `true` para keywords fora da lista de exclusão — invertendo o contrato "em dúvida, false" de [E08-P01_decisions.md](E08-P01_decisions.md) §10. Comandos de controle (`BEGIN`, `COMMIT`…) fornecidos pelo consumidor não são tratados em lugar algum.

A lista foi conferida na documentação oficial do MySQL (§15.3.3 "Statements That Cause an Implicit Commit", Refman 8.4/8.0/5.7, consultada em 20/10/2026). Categorias relevantes e o tratamento aqui:

| Categoria (docs)                                              | Keywords iniciais                     | Tratamento em D10/D11                          |
| ------------------------------------------------------------- | ------------------------------------- | ---------------------------------------------- |
| DDL que define/modifica objetos                               | `CREATE`, `ALTER`, `DROP`, `RENAME`, `TRUNCATE` | denylist → não transacional                    |
| Statements da tabela `mysql` (usuários/privileges)            | `GRANT`, `REVOKE` (+ `* USER` cobertos por CREATE/ALTER/DROP/RENAME) | denylist → não transacional                    |
| Controle de transação e locks                                 | `BEGIN`, `START TRANSACTION`, `LOCK TABLES`, `UNLOCK TABLES` | controle → rejeitado (D11); `LOCK`/`UNLOCK` → denylist |
| Carga de dados                                                | `LOAD DATA`                           | denylist (`LOAD`) → não transacional           |
| Administrativas                                               | `ANALYZE`, `CACHE INDEX`, `CHECK TABLE`, `FLUSH`, `LOAD INDEX INTO CACHE`, `OPTIMIZE`, `REPAIR`, `RESET` | denylist (`ANALYZE`, `CACHE`, `CHECK`, `FLUSH`, `LOAD`, `OPTIMIZE`, `REPAIR`, `RESET`) → não transacional |
| Controle de replicação                                        | `START GROUP_REPLICA`/`START REPLICA`… | keyword inicial `START` → rejeitada junto do controle (D11) |
| DML (transacional em InnoDB, sem commit implícito)            | `INSERT`, `UPDATE`, `DELETE`, `SELECT`, `REPLACE`, `WITH` | allowlist → transacional                       |

Exceções documentadas e não tratadas por parser (fora do escopo, [E08-P01_decisions.md](E08-P01_decisions.md) §12): `CREATE TABLE … SELECT` e `CREATE/DROP TABLE` sem `TEMPORARY` já são capturados pela keyword inicial `CREATE`; `SET PASSWORD`, `SET autocommit = 1` e `SET TRANSACTION` causam commit implícito mas compartilham a inicial `SET` com statements inofensivos (`SET NAMES`, `SET FOREIGN_KEY_CHECKS`), por isso **`SET` não entra na allowlist** — na dúvida, não transacional; corpos `BEGIN … END` de procedimentos ficam dentro de `CREATE PROCEDURE` (keyword inicial `CREATE`) e o splitter de statements não entende esse delimitador (limitação conhecida, um statement por arquivo permanece a recomendação).

## 2. D10 — Keywords desconhecidas: allowlist + denylist

**Comportamento escolhido.** `isTransactionalStatement` usa duas listas fechadas e é conservador:

- **Denylist** (commit implícito no MySQL, conforme as fontes acima): `CREATE`, `ALTER`, `DROP`, `RENAME`, `TRUNCATE`, `LOCK`, `UNLOCK`, `GRANT`, `REVOKE`, `LOAD`, `CHECK`, `CACHE`, `RESET`, `REPAIR`, `OPTIMIZE`, `ANALYZE`, `FLUSH` → `false`.
- **Allowlist** (DML sem commit implícito): `INSERT`, `UPDATE`, `DELETE`, `SELECT`, `REPLACE`, `WITH` → `true`.
- Keyword fora das duas listas (`GRANT` já está na denylist; exemplos: `CALL`, `MERGE`, `PRAGMA`, `EXPLAIN`, `DO`, `SET`, `INSTALL`) → `false` ("em dúvida, false").

**Alternativas rejeitadas.**

- _Manter o comportamento antigo (fora da denylist → true) e reescrever o contrato._ Rejeitada: `GRANT`/`CALL`/`BEGIN` de consumidor entrariam em transação no MySQL, contradizindo as fontes oficiais e a conservadoria exigida por E08 §10.
- _Só denylist com desconhecida → false, sem allowlist._ Rejeitada: `INSERT`/`UPDATE`/`DELETE` também seriam classificados como não transacionais, rebaixando arquivos só-DML ao caminho sem transação no MySQL e perdendo atomicidade que o InnoDB oferece.

**Impacto de compatibilidade.** Corrige comportamento incorretamente permissivo (A06). Arquivos MySQL só-DML continuam transacionais por arquivo; arquivos com statements desconhecidos passam ao caminho sem transação com registro de histórico (comportamento documentado em E08 §11). Nenhuma assinatura muda.

## 3. D11 — Comandos de controle de transação

**Comportamento escolhido.** Keyword inicial `BEGIN`, `START`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `RELEASE`, `END` ou `ABORT` → `IsTransactionalStatement` retorna erro (mensagem nomeando o comando), nos três dialetos. `fileTransactional` propaga e `loadStep` converte em `ErrInvalidQuery` com o nome do arquivo; nada é aplicado. A biblioteca é dona do escopo transacional das migrações (E08 §11); controle no arquivo conflitaria com a transação por versão/arquivo e com o registro de histórico.

**Alternativas rejeitadas.**

- _Encaminhar ao caminho sem transação._ Rejeitada: `BEGIN`/`COMMIT` explícitos ainda conflitariam com a sessão da biblioteca (histórico escrito fora do controle do arquivo) e produziriam falhas obscuras na execução, não um contrato verificável.
- _Ignorar e deixar o driver decidir._ Rejeitada: mesma obscuridade, agora sem diagnóstico antes de aplicar.

**Impacto de compatibilidade.** Arquivos legítimos que hoje contêm controle explícito (raro e já arriscado) passam a falhar cedo e com mensagem clara, em vez de potencialmente corromper o escopo. Nenhuma assinatura muda.

## 4. D12 — Varredura em SQLite/PostgreSQL

**Comportamento escolhido.** `fileTransactional` deixa de ter o atalho cego: nos três dialetos cada statement de `splitStatements` passa pelo classificador. Em SQLite/PostgreSQL o resultado booleano é descartado (DDL continua transacional por `SupportsTransactionalDDL`), mas **erros são propagados** — em particular o comando de controle de D11 e comentário de bloco não terminado (malformado, que antes passaria despercebido em SQLite/PG).

**Alternativas rejeitadas.**

- _Classificação completa (bool) nos três dialetos._ Rejeitada: em SQLite/PG todo DDL é transacional; aplicar o veredito da denylist rebaixaria arquivos legítimos ao caminho sem transação, contradizendo `SupportsTransactionalDDL`.
- _Manter o atalho cego._ Rejeitada: `BEGIN`/`COMMIT` do consumidor falhariam de forma obscura na execução, sem diagnóstico em `Prepare`.

**Impacto de compatibilidade.** SQLite/PostgreSQL ganham rejeição precoce de arquivos malformados ou com controle explícito; arquivos bemformados continuam intocados.

## 5. D13 — Tokens entre aspas

**Comportamento escolhido.** O classificador não ganha tratamento de aspas: o primeiro token é delimitado por espaço/tab/newline/`(`/`;` como antes. Um token iniciado por `"`, `` ` `` ou `'` (ex.: `"CREATE" TABLE`, `'CREATE TABLE'`, `` `INSERT` ``) não casa com nenhuma lista → `false` (conservador). `splitStatements` já protege o splitting contra aspas; a análise continua sem parser genérico e o texto enviado ao driver nunca é reescrito (E07-P01 §9, invariante preservada).

**Alternativas rejeitadas.**

- _Detectar e pular tokens entre aspas para achar a keyword real._ Rejeitada: aproxima o classificador de um parser SQL, com superfície de erro crescente e benefício marginal para arquivos de migração.
- _Rejeitar token entre aspas como erro._ Rejeitada: seria mais estrito que o necessário; tratar como desconhecida já alcança o efeito seguro (`false`) sem novo modo de falha.

**Impacto de compatibilidade.** Nenhum caso novo rejeitado além do já conservador; comportamento de arquivos bemformados inalterado.

## 6. Verificação nas fontes oficiais e evidência

- Fontes: MySQL Refman 8.4 §15.3.3 (e espelhos 8.0/5.7), consultadas em 2026-10-20; categorias mapeadas na §1 deste documento.
- Testes: `TestIsTransactionalStatement` ampliado em [dialect_test.go](../internal/dialect/dialect_test.go) (desconhecidas, allowlist, controle, aspas, comentários); novos `TestFileTransactional*` e `TestSplitStatements*` em [scope_test.go](../internal/migrate/scope_test.go) com os três dialetos e escopo MySQL em Apply simulado. Evidência real de MySQL/PostgreSQL segue em E09/E10.

**Código afetado.** `internal/dialect/migration.go` (listas e `isTransactionalStatement`), `internal/migrate/migrate.go` (`fileTransactional`), testes em `internal/dialect/dialect_test.go` e `internal/migrate/scope_test.go`, [E08-P01_decisions.md](E08-P01_decisions.md) §10, MANUAL e cabeçalho de [tests/migrations_test.go](../tests/migrations_test.go).
