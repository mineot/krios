# C05-P01 — Descrição vigente consolidada (D25–D29)

Data: 2026-10-21. Resolve C05-P01 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §7 (critério §137). Reconcilia planejamento, decisões, README, MANUAL e comentários de API com o contrato vigente — código e testes como fonte de verdade. Nenhuma assinatura pública ou comportamento muda: são textos e doc comments. Complementa [E04-P02_decisions.md](E04-P02_decisions.md) (§3, tabela), [E06-P01_decisions.md](E06-P01_decisions.md) (§6 `Result`, §8 validação), [E08-P01_decisions.md](E08-P01_decisions.md) (§8 preparação) e [C01-P01_decisions.md](C01-P01_decisions.md) (D03 atomicidade, D05 `DB.Migrations`).

## 1. Contexto e método

**Método.** A descrição vigente é o contrato implementado. A ordem de autoridade é: código + testes > decisão formal (`.specs/*_decisions.md`) > README > MANUAL > exemplos históricos (`0001_use_examples.md`, `examples/`, `tests/`). Quando um texto antigo contradiz o código, o texto cede; quando um documento de decisão é anterior a uma decisão posterior, prevalece a mais recente.

**Colisão de identificadores (a reconciliar).** O texto original de C05-P01 (`0005_consolidation_plan.md:133`) dizia "Corrigir A08–A11". No entanto, na tabela de achados do próprio plano §2 (`:35–38`), **A08–A11** são outros achados:

| ID (§2) | Achado real | Encarregado |
| ------- | ----------- | ----------- |
| A08 | Atomicidade: promessas de rollback de todo o percurso | C01/C05 — decidido em [C01-P01_decisions.md](C01-P01_decisions.md) D03 |
| A09 | `ErrFileNotFound` vs `ErrInvalidQuery`; `One` posicional vs por nome | C05 — texto/exemplares em C05-P02 |
| A10 | Exemplos históricos: validação só explícita, rollback no-op, `All` fecha incondicional | C05 — exemplos em C05-P02 |
| A11 | `DB.Migrations()` devolve ponteiro retido/mutável | C01/C05 — decidido em [C01-P01_decisions.md](C01-P01_decisions.md) D05 |

Os quatro itens exemplificados em `:133` (tabela por `TableName` vs tag, validação automática em `SetValues`, `Rollback` após finalização, `Result` por driver) **não** correspondem a A08–A11; são correções de descrição adicionais. A reconciliação adotada: **o enunciado passa a citar os A08–A11 reais do §2 e, separadamente, os quatro itens de descrição** (D25–D29 abaixo), removendo a ambiguidade. A09/A10 referem-se sobretudo a exemplos executáveis e ficam em [C05-P02] (ver §7); A08/A11 já foram decididos em C01 e aqui apenas se verifica que a descrição os reflete.

## 2. D25 — Tabela por `TableName()`/`snake_case`, sem tag de struct

**Comportamento escolhido.** O nome da tabela deriva do tipo Go via `snake_case` do nome do struct, sobreponível pelo método `TableName() string`. Não há tag de struct para a tabela. Esquema e alias **não** são expostos (extensão futura). Fonte: [`internal/entity/entity.go`](../internal/entity/entity.go) `deriveTableName` (`:202`), `TableName()` (`:97`), accessor `Table()` (`:256`); testes em `internal/entity/entity_test.go`.

**Motivo.** Go não permite tag em tipo struct, apenas em campos; a proposta antiga de "tag de struct" para a tabela é inimplementável e foi abandonada em [E04-P02_decisions.md](E04-P02_decisions.md) §3.

**Alternativas rejeitadas.**

- _Tag de struct nomeando a tabela._ Rejeitada: impossível para a própria struct; exigiria um tipo/registro externo que o contrato não oferece.
- _Registro explícito tipo→tabela._ Rejeitada: contraria o estilo de derivação por reflexão já adotado.

**Impacto de compatibilidade.** Nenhum: só texto. Corrige `0001_planning.md:234` e `E04-P01_decisions.md:21,36,78,84` (+ nota de revisão no topo de E04-P01).

## 3. D26 — Validação automática em `SetValues`

**Comportamento escolhido.** `SetValues` executa `Entity.Validate` sobre o registro e falha rápido com `ErrValidation`, sem chamada explícita do consumidor e sem opção de desligar. Fonte: [`internal/write/insert.go`](../internal/write/insert.go) `:49`, [`internal/write/update.go`](../internal/write/update.go) `:52`; decidido em [E06-P01_decisions.md](E06-P01_decisions.md) §8.

**Alternativas rejeitadas.**

- _Validação apenas explícita pelo consumidor._ Rejeitada em E06 §8: contraria o fail-fast pretendido para escritas.

**Impacto de compatibilidade.** Só texto. Corrige `README.md:91`, `E04-P03_decisions.md:33` (superseded por E06 §8) e `0001_planning.md:320` (menção de validação).

## 4. D27 — `Rollback` após finalização retorna `ErrTxDone`

**Comportamento escolhido.** Após `Commit` ou `Rollback`, a `Tx` está finalizada; chamadas subsequentes (incluindo novo `Rollback`) retornam `ErrTxDone`. Não é no-op silencioso. Fonte: [`internal/transaction/tx.go`](../internal/transaction/tx.go) `:170–175` (tentativa nativa e então `errs.ErrTxDone`); testes `tests/tx_test.go:171–172` ("Rollback depois de Commit = `ErrTxDone`"), `internal/transaction/tx_test.go`.

**Alternativas rejeitadas.**

- _Rollback pós-commit como no-op silencioso._ Rejeitada: esconderia uso incorreto; o defer idiomático já ignora o erro explicitamente (`MANUAL.md:46`).

**Impacto de compatibilidade.** Só texto. README (`:197`) e MANUAL (`:37,46,100`) já estão corretos; trechos históricos de exemplo (`0001_use_examples.md:288,299`, `examples/store.go:117`) ficam para C05-P02.

## 5. D28 — Disponibilidade de `Result` é flag por driver

**Comportamento escolhido.** `Result.RowsAffected()` e `LastInsertId()` retornam `(int64, bool)`; o booleano `false` sinaliza indisponibilidade, **incluindo** erro do driver cujo detalhe não é exposto. A disponibilidade varia por banco/driver (ex.: `LastInsertId` indisponível no PostgreSQL). Fonte: [`internal/result/result.go`](../internal/result/result.go) `:20,33,48`; [E03-P01_decisions.md](E03-P01_decisions.md) §3.1; [E07-P01_decisions.md](E07-P01_decisions.md).

**Alternativas rejeitadas.**

- _Prometer `RowsAffected` "garantido nos três bancos"._ Rejeitada: a matriz de [E06-P01_decisions.md](E06-P01_decisions.md) §6 registrava `ok == true` universal, contradizendo `result.go`, que devolve `false` quando o driver erra. A descrição passa a falar em **disponibilidade comunicada por flag**, não em garantia.

**Impacto de compatibilidade.** Só texto. Corrige `0001_planning.md:320`, `0002_execution_plan.md:218` e `E06-P01_decisions.md:129` (coluna `RowsAffected`).

## 6. D29 — Preparação de migrações consulta o banco; plano preparado

**Comportamento escolhido.** A preparação (`migrate.Prepare` via `DB.RunMigrations`) **lê o banco** — inspeciona a tabela de histórico e a existência de objetos gerenciados — mas **não executa DDL de migração**. Não é operação puramente local. O plano preparado (`Plan.Steps`) enumera todos os passos; `RunMigrations` devolve o plano mesmo quando a aplicação falha (o consumidor vê o que foi **tentado**) e devolve plano sem passos quando já no alvo. Fonte: [`internal/migrate/migrate.go`](../internal/migrate/migrate.go) `:100–119`, [`internal/access/db.go`](../internal/access/db.go) `:251–278`; [E08-P01_decisions.md](E08-P01_decisions.md) §8; `MANUAL.md:242`.

**Operações puramente locais (contraste).** `MigrationsConfig.Validate` (não carrega arquivos), `NewSQLFile` (falha antes de tocar o banco), construção/`Build()` dos builders (renderiza sem executar) e derivação de `Entity` — estas sim locais, sem consulta.

**Alternativas rejeitadas.**

- _Manter "Preparation is pure"._ Rejeitada: confunde "não executa DDL" com "não acessa o banco"; `Prepare` consulta histórico e existência de objetos.
- _Dizer que o plano reflete "o que foi executado" em falha de aplicação._ Rejeitada: em falha só se sabe o que foi **tentado** — alinhar [E08-P01_decisions.md](E08-P01_decisions.md) §8 a `MANUAL.md:242`.

**Impacto de compatibilidade.** Só comentários de API e texto. Corrige `internal/migrate/migrate.go:5`, `internal/access/db.go:254` e `E08-P01_decisions.md:116–117`.

## 7. Escopo entregue e limites

**Entregue nesta entrega (C05-P01).**

- Novo `.specs/C05-P01_decisions.md` (este documento, D25–D29).
- Texto da tripla C05-P01 em `0005_consolidation_plan.md:133` corrigido (A08–A11 reais + quatro itens).
- Correções de descrição: `0001_planning.md` (`:234` tabela, `:320` `Result`/validação), `E04-P01_decisions.md:21,36,78,84` (+ nota), `E04-P03_decisions.md:33`, `E06-P01_decisions.md:129`, `E08-P01_decisions.md:116–117`.
- README: `:5` (migrações entregues) e índice de Documentação (`:248–264`).
- Comentários de API: `internal/migrate/migrate.go:5`, `internal/access/db.go:254`.
- Plano §7 C05-P01 marcado e linha C05 (§10) atualizada.

**Fora de escopo (transferido).**

- A09/A10 em exemplos executáveis (`0001_use_examples.md`, `examples/`, `tests/`) → **C05-P02** (conferir exemplos com código executável).
- Matriz de rastreabilidade RF-01–RF-09 e os treze critérios; classificação de `ColumnTypes` e aliases/esquemas → **C05-P03**.
- Cenário de disponibilidade de `Result` por dialeto em `E09_manual_validation.md` → classificar em C05-P03.

## 8. Verificação

- `rg` das frases stale (deve retornar vazio após as correções): "tag de struct" para tabela, "execução é explícita" no README, "garantido nos três bancos" para `RowsAffected`, "Preparation is pure".
- `gofmt -l internal/migrate internal/access`, `go vet ./...`.
- `make check`, `make race`, `GOTOOLCHAIN=go1.27.2 go test ./...`.
- **Critério de conclusão (0005 §137):** os documentos descrevem os mesmos contratos e limitações; o histórico permanece rastreável e nenhum requisito desaparece sob "melhoria futura" — atendido para o escopo de descrição; os itens residuais estão nomeados e transferidos a C05-P02/P03.
