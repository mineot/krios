# C04-P02 — Correção do ciclo de vida de leitura (D22–D23)

Data: 2026-10-20. Resolve C04-P02 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §6 e a correção do achado A05 reproduzido por [C04-P01_decisions.md](C04-P01_decisions.md) D21. Implementa [C01-P01_decisions.md](C01-P01_decisions.md) D04 §5 e preserva a política de [E03-P01_decisions.md](E03-P01_decisions.md) §3.3/§3.4.6. Nenhuma assinatura pública muda.

## 1. Contexto

A reprodução de C04-P01 comprovou que [Rows.All](../internal/scan/mapping.go) retornava sem fechar nos early returns (destino inválido, entidade inválida, erro de metadados) e que [Row.Scan](../internal/scan/rows.go) rejeitava a entidade sem concluir o ciclo da linha nativa. Os atalhos públicos herdavam o vazamento, e a falha de mapeamento não registrada não bloqueava `Commit` quando o resultado pertencia a uma `Tx`.

A correção torna verde todo o conjunto de C04-P01 sem editar as regressões (exceto o subteste de `Tx`, que afirma o comportamento **corrigido**, não o defeituoso).

## 2. D22 — Fechamento uniforme e registro em `Rows.All`

**Comportamento escolhido.** `Rows.All` passa a fechar o iterador nativo em **todos** os caminhos:

- O corpo foi dividido em `All` (orquestração) e `all` (mapeamento). `All` executa `all`, registra qualquer erro no recorder da `Tx` (`rw.record`) e conclui com `rw.withClose(err)`.
- `all` retorna o erro primário de cada etapa: `allDest` (destino inválido), `entity.OfType` (entidade inválida), `rw.Columns()` (metadados), `Scan` (`krios: map <tabela>: ...`) e iteração (`rw.Err()`). Nenhum early return escapa do fechamento.
- `withClose` chama `rw.Close()` (que registra o erro de fechamento/iteração pendente) e combina o primário com o de limpeza.

Efeito: um destino inválido numa `Tx` agora é registrado como erro tardio e bloqueia `Commit`, conforme D04 §5 e E03 §3.4.6. O destino de `All` continua **intacto em falha** (o `slice.Set(mapped)` só ocorre no caminho de sucesso) e substituído por slice vazio em sucesso sem linhas.

**Alternativas rejeitadas.**

- _Manter `defer rw.Close()` e retornar sem combinar erros._ Rejeitada: um `defer` com `Close` silencioso descartaria o erro de limpeza, contrariando D04 §5 (preservar o secundário).
- _Registrar apenas em `All` e não em `all`._ Rejeitada: o registro precisa abranger todos os caminhos, inclusive os que não passam por `rw.Scan`; centralizar em `All` garante cobertura uniforme.
- _Deixar o erro de metadados fora do registro._ Rejeitada: compartilha a causa-raiz e o contrato de fechamento uniforme.

## 3. D23 — `errors.Join` para o erro duplo; `releaseRow` conclui o `Row` rejeitado

**Comportamento escolhido.**

- **Erro duplo.** Quando o primário e o de limpeza concorrem, `Rows.All` retorna `errors.Join(primary, closeErr)`. Ambos permanecem inspecionáveis por `errors.Is`/`errors.As` e o primário lidera a mensagem. Um primário nil com falha de `Close` retorna a falha de limpeza isolada.
- **`Row.Scan` rejeitado.** No caminho em que `entityDest` rejeita o destino (incluindo ponteiro de entidade nil), `Scan` registra o erro (como já fazia) e chama `releaseRow`, que executa `rw.r.Scan()` sem destinos. `database/sql` registra `defer r.rows.Close()` **antes** de validar os destinos (`src/database/sql/sql.go:3504`), então a chamada libera o `driver.Rows` subjacente mesmo retornando o erro de aridade, que é descartado — a causa real é o destino rejeitado.

**Alternativas rejeitadas.**

- _`fmt.Errorf("...: %w (close failed: %w)", primary, closeErr)`._ Rejeitada: casa com o texto do `Commit` (`tx.go:153`), mas perde `Unwrap() []error` limpo e dificulta distinguir as duas causas por `errors.Is` quando ambas têm sentinelas. `errors.Join` é a forma idiomática para múltiplos erros independentes.
- _Priorizar `closeErr` sobre o primário._ Rejeitada: contradiz D04 §5; o consumidor precisa da causa original.
- _Não liberar o `Row` rejeitado e confiar no GC._ Rejeitada: em drivers reais o iterador retém a conexão até o fechamento; o wrapper não pode transferir essa responsabilidade.
- _Preencher destinos descartáveis por coluna em `releaseRow`._ Rejeitada: `*sql.Row` não expõe a contagem/nomes de colunas; consumir sem destinos é suficiente para acionar o `Close` interno.

**Impacto de compatibilidade.** Não muda assinaturas. Corrige vazamento e propagação; um consumidor que ignore a falha de mapeamento passa a ter `Commit` bloqueado (comportamento correto e documentado). O erro duplo é aditivo: passa a preservar a limpeza, sem alterar o tipo do primário. `sql.ErrNoRows` continua excluído do bloqueio e `Row.Scan` mantém a leitura posicional.

## 4. Escopo entregue

- [internal/scan/mapping.go](../internal/scan/mapping.go): `Rows.All` reescrito (`All`/`all`/`withClose`); early returns fecham e registram; erro duplo por `errors.Join`.
- [internal/scan/rows.go](../internal/scan/rows.go): `Row.Scan` chama `releaseRow` ao rejeitar a entidade.
- [internal/access/read_shortcut_test.go](../internal/access/read_shortcut_test.go): subteste `Tx` passa a exigir `Commit` bloqueado com rollback nativo.
- [internal/scan/mapping_test.go](../internal/scan/mapping_test.go): `TestRowsAllPreservesPrimaryAndCleanupFailure`.
- Atalhos em `internal/query`, `internal/sqlfile`, `internal/write`: **sem mudança** — delegam a `Rows.All`/`Row.Scan`; os atalhos de C04-P01 agora fecham.

## 5. Verificação

- As quatro suítes de C04-P01 ficam verdes: `TestRowsAllClosesOnInvalidDestination`, `TestRowsAllClosesOnInvalidEntity`, `TestRowScanReleasesNativeRowOnRejectedEntity`, `TestFileOneAndAllRejectInvalidDest`, `TestReadShortcutsCloseOnInvalidDestination`.
- Suítes preservadas verdes: `TestRowsAllRejectsInvalidDestinations`, `TestRowsAllFailureKeepsDestinationAndCloses`, `TestRowsAllEmptyResultReplacesDestination`, `TestRowsAllPropagatesIterationError`, `TestMappedReadFailuresBlockCommit`, `TestQueryRowErrors`, `TestFileLateFailureBlocksCommit`.
- `go test ./...` limpo. `make check`/`make race` em Go 1.26.8 e 1.27.2 (registrado no commit/entrega).
