# C04-P03 — Exercitar os atalhos públicos (D24)

Data: 2026-10-20. Resolve C04-P03 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §6 e conclui o escopo de C04. Complementa [C04-P01_decisions.md](C04-P01_decisions.md) D21 (reprodução) e [C04-P02_decisions.md](C04-P02_decisions.md) D22–D23 (correção), verificando que os atalhos públicos herdam corretamente o ciclo de vida e a política transacional de [E03-P01_decisions.md](E03-P01_decisions.md) §3.3/§3.4.6 e [C01-P01_decisions.md](C01-P01_decisions.md) D04 §5. Nenhum código de produção muda nesta entrega; é evidência.

## 1. Contexto

C04-P01/P02 provaram e corrigiram o ciclo de vida em `Rows.All`/`Row.Scan` e, por consequência, nos atalhos. C04-P03 fecha as lacunas de verificação direta nos atalhos que o mapeamento anterior não cobria com contadores:

| Lacuna                                                                 | Antes                                                | Cobertura nova                                                  |
| ---------------------------------------------------------------------- | ---------------------------------------------------- | --------------------------------------------------------------- |
| "Erro de construção não inicia consulta/escrita" por contador           | Só asserção indireta (erro devolvido)                | `TestWriteShortcutsDoNotStartOnBuildError`, `TestFileBuildErrorDoesNotStartQuery` |
| `sql.ErrNoRows` em `Insert.One`, `File.One`; slice vazio em `Update.All`/`Delete.All` | Ausente                                            | `TestWriteShortcutsErrNoRows`, `TestFileOneNoRowsPreserved`      |
| Bloqueio de `Commit` em `Select.Paginate` e escritas `Returning` com falha de mapeamento | Ausente                              | `TestWriteShortcutsLateFailureBlocksCommit`                      |
| Fechamento de `Insert.One` (sucesso e `ErrNoRows`)                      | Ausente                                              | `TestWriteShortcutsErrNoRows`                                    |

## 2. D24 — Contadores `Execs`/`Queries` no driver simulado

**Comportamento escolhido.** O `testutil.Driver` passa a contar chamadas nativas no `conn`: `Execs` incrementado em `conn.ExecContext` e `Queries` em `conn.QueryContext` (que também serve `QueryRowContext`, pois o `conn` implementa `driver.QueryerContext`). A verificação "nenhuma consulta/escrita iniciada" usa `Execs == 0 && Queries == 0 && LastQuery() == ""`.

Isso é **aditivo**: os campos existentes (`opens`, `commits`, `rollbacks`, hooks) permanecem; nenhum teste existente dependia da ausência desses contadores. A contagem no `conn` (e não por closure nos testes) evita repetir closures frágeis em cada cenário e cobre todos os atalhos uniformemente.

**Alternativas rejeitadas.**

- _Contar por closure em cada teste._ Rejeitada: repetitiva e sujeita a esquecimento; o plano pede a mesma verificação em vários atalhos (Select/File/write), e um contador no driver é mais robusto.
- _Confiar apenas em `LastQuery() == ""`._ Rejeitada isoladamente: `LastQuery` é sobrescrito a cada chamada e não distingue "nunca chamado" de "chamado e limpo"; combiná-lo com os contadores monotônicos é a prova forte.

**Impacto de compatibilidade.** Só afeta a infraestrutura de testes compartilhada (`internal/testutil`); nenhuma API pública muda.

## 3. Política verificada nos atalhos

- **Construção antes do executor.** `Select`, `File` e os builders de escrita reportam falhas de construção (coluna desconhecida, `File(nil)`, sem `SetValues`/`Where`, sem `Returning`, `Exec` após `Returning`) **antes** de tocar o executor: `Execs == 0`, `Queries == 0`, `LastQuery() == ""`. Por serem anteriores ao executor, **não** iniciam statement e **não** bloqueiam `Commit` na `Tx`.
- **`sql.ErrNoRows`.** Preservado por `Insert.One` e `File.One`; não bloqueia `Commit`. `Update.All`/`Delete.All` com resultado vazio devolvem slice vazio (não `ErrNoRows`) e fecham o iterador.
- **Bloqueio de `Commit`.** Falha de mapeamento observada numa consulta já iniciada na `Tx` (`Select.Paginate` na página; `Update.Returning().All`) é registrada e bloqueia `Commit` com auto-rollback (`Commits == 0`, `Rollbacks == 1`).
- **Fechamento.** `Insert.One` fecha o result set nativo no sucesso e no `ErrNoRows`.

## 4. Escopo entregue

- [internal/testutil/driver.go](../internal/testutil/driver.go): contadores `Execs`/`Queries`.
- [internal/access/write_shortcut_test.go](../internal/access/write_shortcut_test.go): `TestWriteShortcutsDoNotStartOnBuildError`, `TestWriteShortcutsErrNoRows`, `TestWriteShortcutsLateFailureBlocksCommit`, fixture `multiRows`, helper `assertNoStatement`.
- [internal/sqlfile/sqlfile_test.go](../internal/sqlfile/sqlfile_test.go): `TestFileOneNoRowsPreserved`, `TestFileBuildErrorDoesNotStartQuery`.

## 5. Verificação

- `go test ./...` limpo. Atalhos cobertos: `Select.All/One/Paginate/Count` (Count via `Paginate`), `File.One/All/Query/QueryRow`, `Insert.Exec/One`, `Update.Exec/All`, `Delete.Exec/All`, em `DB` e `Tx`.
- `make check`/`make race` em Go 1.26.8 e 1.27.2 (registrado no commit/entrega).
- **Critério de conclusão de C04:** cada recurso obtido pelos atalhos tem encerramento verificável em sucesso e falha, e o consumidor recebe a causa correta com a política transacional documentada — atendido.
