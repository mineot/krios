# C01-P01 — Decisões de contrato D01–D05

Data: 2026-10-20. Resolve C01-P01 de [0005_consolidation_plan.md](0005_consolidation_plan.md) §3. Registra as decisões necessárias antes das correções C02–C05, com comportamento escolhido, alternativas rejeitadas, impacto de compatibilidade e exemplos de migração de código. **Nenhuma implementação é feita aqui**; a reconciliação documental dos requisitos afetados é C01-P02 e as correções pertencem a C02–C05.

## 1. Contexto e método

A inspeção dirigida da consolidação (achados A01–A11) identificou comportamentos que exigem uma escolha de contrato antes de qualquer correção. Este documento fixa essas escolhas para que C02–C05 tenham alvo verificável. As decisões são coerentes com os contratos vigentes de [E03-P01_decisions.md](E03-P01_decisions.md) (transações e erros), [E07-P01_decisions.md](E07-P01_decisions.md) (SQL incorporado) e [E08-P01_decisions.md](E08-P01_decisions.md) (migrações), e revisam apenas o que ficou comprovadamente ambíguo ou incorreto.

Cada decisão segue o formato: **comportamento escolhido**, **alternativas rejeitadas**, **impacto de compatibilidade**, **exemplo de migração de código** e **código afetado**.

As escolhas que alteram a API pública foram apresentadas ao consumidor e aprovadas em 2026-10-20; as que preservam a API estão marcadas como decisão interna.

## 2. D01 — Alvo de migração: presente versus zero explícito

**Comportamento escolhido.** `MigrationsConfig.TargetVersion` passa de `int` para `*int`.

- `MigrationsConfig.Migrations != nil` e `TargetVersion == nil` → `ErrInvalidConfig` (versão desejada ausente). Nunca interpretado como reversão.
- `TargetVersion` aponta para `0` → solicita reversão total (estado `0`, conforme [E08-P01_decisions.md](E08-P01_decisions.md) §8).
- `TargetVersion` aponta para valor negativo → `ErrInvalidConfig` (domínio: `>= 0`).
- `TargetVersion` positivo não declarado em `Versions` → `ErrInvalidConfig` na preparação (alcançabilidade; corrigido em C02-P02).

Isso torna explícita a distinção exigida por RF-05: presença da versão desejada é obrigatória, e a reversão total só ocorre quando o consumidor pede `0` de propósito.

**Alternativas rejeitadas.**

- _Manter `int` e tratar `0` como reversão, com ausência indistinguível._ Rejeitada: viola RF-05 ("a ausência da versão desejada deve produzir erro antes de executar e não deve ser interpretada como reversão") e mantém a ambiguidade que originou A02.
- _Sentinela numérica de omissão (ex.: `-1`)._ Rejeitada: sobrecarrega o domínio e ainda permitiria alvo inválido passar como omissão; ponteiro expressa presença sem valor reservado.
- _Campo separado `TargetVersionSet bool`._ Rejeitada: duas fontes de verdade para o mesmo alvo, mais fácil de divergir que o ponteiro.

**Impacto de compatibilidade.** Muda a API pública: `TargetVersion` deixa de ser um `int` e passa a `*int`. Afeta todos os exemplos, o roteiro E09, os testes de migração e o consumidor KriosTasks (quando usar migrações). Não é correção documental — é mudança de contrato; deve ser registrada como tal. Ajuda de migração: uma expressão `intPtr(n)` ou variável local.

**Exemplo de migração de código.**

```go
// antes
cfg.Migrations.TargetVersion = 2

// depois
target := 2
cfg.Migrations.TargetVersion = &target
// ou
cfg.Migrations.TargetVersion = krios.Int(2) // helper opcional, se adotado em C02
```

**Código afetado.** [`internal/config/config.go:84`](../internal/config/config.go) (`TargetVersion int`), `MigrationsConfig.Validate` (`:195`), `internal/migrate/migrate.go` (uso do alvo na preparação/comparação), `internal/access/db.go` (retenção da config), testes em [`tests/migrations_test.go`](../tests/migrations_test.go) e `internal/migrate/*_test.go`, exemplos e `E09_manual_validation.md`.

## 3. D02 — Histórico e versão efetiva

**Comportamento escolhido.** O histórico distingue **execução** (linhas registradas) de **estado aplicado** (versão efetiva do esquema), de modo que ciclos repetidos reflitam o esquema real.

- A **versão efetiva** é derivada da sequência declarada e do conjunto de versões integralmente concluídas, não da maior versão com uma linha `applied` isolada.
- Uma versão só conta como concluída quando **todos** os arquivos necessários da direção terminaram com `status='applied'` em execução bem-sucedida.
- Uma reversão completa até `0` **reduz** o estado efetivo e remove o efeito das versões revertidas, permitindo reaplicação subsequente (`0 → 1 → 0 → 1` volta a recriar a tabela).
- A chave histórica continua `(version, filename, direction)` ([E08-P01_decisions.md](E08-P01_decisions.md) §7); reaplicações resolvem o conflito de chave sem apagar histórico de falhas silenciosamente. A política de sobrescrita por reaplicação legítima (nova execução `applied` para a mesma chave) fica restrita à mesma direção/versão e é registrada em C02-P03.
- Histórico incompatível com a configuração (versão corrente sem correspondência declarada, direções/status inválidos, versão parcialmente executada) é **erro antes de executar**, não sucesso aparente.
- **Compatibilidade com bancos já criados pelo Krios:** a biblioteca **não** migra automaticamente registros antigos ambíguos. Ela detecta o histórico inconsistente e bloqueia, cabendo ao consumidor reconciliar ou recriar o esquema. O procedimento documentado fica em C02-P04.

**Alternativas rejeitadas.**

- _Manter `lastApplied` = maior `Version` com uma linha `applied` (A01)._ Rejeitada: reprodução SQLite mostrou `0 → 1 → 0 → 1` terminando com sucesso e nenhum passo, com a tabela ausente — sucesso aparente sobre esquema errado.
- _Contar apenas linhas `applied` por direção sem validar a completude da versão (A03)._ Rejeitada: uma linha aplicada não prova que todos os arquivos da versão terminaram; no MySQL por arquivo, isso marcaria versão completa indevidamente.
- _Reescrever automaticamente históricos antigos._ Rejeitada: apagaria evidência de falhas e assumiria formato que a biblioteca não controla; detecção explícita é mais segura.

**Impacto de compatibilidade.** Não muda assinaturas públicas; muda o significado observável de `Plan.CurrentVersion`/`CurrentVersion` retornados por `RunMigrations`, corrigindo resultados incorretos. Bancos criados pela implementação anterior com histórico ambíguo passam a ser detectados como inconsistentes — mudança de comportamento esperada e documentada.

**Exemplo de comportamento.**

```text
target=1 current=0 steps=1 table_exists=1 error=<nil>
target=0 current=1 steps=1 table_exists=0 error=<nil>
target=1 current=0 steps=1 table_exists=1 error=<nil>   // antes: steps=0, table_exists=0
```

**Código afetado.** [`internal/migrate/migrate.go`](../internal/migrate/migrate.go) (`lastApplied` `:309`, `currentVersion` `:281`), `internal/migrate/apply.go` (registro `down` como `applied`), [`tests/migrations_test.go`](../tests/migrations_test.go), `internal/migrate/*_test.go`, `E09_manual_validation.md` (M01–M07), [E08-P01_decisions.md](E08-P01_decisions.md) §7/§8.

## 4. D03 — Atomicidade

**Comportamento escolhido.** Manter a unidade de atomicidade já definida em [E08-P01_decisions.md](E08-P01_decisions.md) §11, corrigindo as promessas de rollback global que a contrariam (A08).

- **SQLite/PostgreSQL:** uma transação **por versão**, envolvendo todos os arquivos da versão que está sendo aplicada/revertida, quando a sequência da versão é integralmente transacional. Falha → rollback da **versão em execução**; versões já confirmadas permanecem.
- **MySQL — arquivo 100% transacional:** transação por arquivo. Arquivos anteriores permanecem aplicados.
- **MySQL — arquivo com DDL de commit implícito:** sem transação. Falha pode deixar estado parcial **deste arquivo**; nunca tentar reverter DDL já confirmado; registrar `failed`/`ErrPartialMigration`.
- **Limite explícito:** não há rollback de **todo o percurso** até o alvo. Versões anteriormente confirmadas não são revertidas por uma falha em versão posterior.
- A criação da tabela de infraestrutura `migrations` é fora do escopo transacional dos arquivos de aplicação; seu comportamento em falha de commit é documentado em C03-P03, não prometido como revertível. Resolvido em [C03-P03_decisions.md](C03-P03_decisions.md) (D16): falha de `Commit` tenta limpeza defensiva e reporta desfecho incerto, sem prometer reversão nem emitir `ErrPartialMigration`.
- `TestMigrationsFailureRollsBackSQLite` deve ser reinterpretado como **rollback da versão em execução** (a versão anterior `applied` permanece), e não como rollback do percurso completo.

> **Revisado por [C03-P04_decisions.md](C03-P04_decisions.md) (D18–D20, 2026-10-20):** a unidade "versão" pressupõe que o histórico registre **um arquivo por linha** (`(version, filename, direction)`); `insertInProgress` passou a limpar apenas a linha do próprio arquivo, e não a versão inteira, para que versões com múltiplos arquivos possam concluir. A atomicidade por versão em SQLite/PostgreSQL foi confirmada (falha no meio reverte a versão, versão anterior permanece) e `LastVersion` do estado parcial passou a ser sempre a última versão completa.

**Alternativas rejeitadas.**

- _Uma transação por todo o percurso até o alvo (rollback global).'_ Rejeitada: inviável no MySQL e desnecessária; viola "escopo máximo viável" de RF-05 e as versões confirmadas devem persistir.
- _Sem transação em SQLite/PostgreSQL._ Rejeitada: desperdiça suporte transacional integral e contraria RF-05.
- _Transação por versão também no MySQL quando "parece" transacional por heurística de arquivo único._ Rejeitada: o contrato de `MigrationDialect` é conservador ("na dúvida, false", §10); não prometer rollback sem suporte demonstrado.

**Impacto de compatibilidade.** Não muda assinaturas públicas. Corrige texto de E08 §3 ("rollback integral") e do cenário M05 para deixar claro que a unidade é a versão/percurso contratado, não o percurso inteiro.

**Exemplo de contrato.**

```text
SQLite/PG, versão 2 falha:
  versão 1 permanece applied
  versão 2 é revertida integralmente (sem registro applied remanescente)
  sche ma = estado após versão 1

MySQL, arquivo não-transacional falha no statement 2 de 3:
  statement 1 permanece aplicado (não reversível)
  histórico marca failed + ErrPartialMigration
  retomada automática bloqueada
```

**Código afetado.** [`internal/migrate/apply.go`](../internal/migrate/apply.go), [`internal/migrate/migrate.go`](../internal/migrate/migrate.go) (`fileTransactional` `:258`), [`internal/dialect/migration.go`](../internal/dialect/migration.go), `tests/migrations_test.go`, [E08-P01_decisions.md](E08-P01_decisions.md) §3/§11, `E09_manual_validation.md` M05.

## 5. D04 — Falhas de leitura e mapeamento

**Comportamento escolhido.** Preservar a política de erros de [E03-P01_decisions.md](E03-P01_decisions.md) §3.4.6 e §3.3, corrigindo os ciclos de vida que a violam (A05).

- **Fechamento uniforme:** `Rows.All` fecha o iterador nativo em **todos** os caminhos — destino inválido, entidade inválida, erro de metadados (`ColumnTypes`/`Columns`), erro de `Scan`, erro de iteração e sucesso. Early returns deixam de vazar o recurso.
- **`Row.Scan`:** conclui seu ciclo de vida mesmo quando rejeita a entidade/destino antes de consumir a linha nativa, devolvendo o recurso ao `database/sql`.
- **Propagação:** o erro primário não é substituído por erro de limpeza. Falha de `Close` que concorra com a causa original é preservada como erro secundário inspecionável por `errors.Is`/`errors.As`, seguindo o padrão de erro duplo já adotado em transações.
- **Registro na Tx:** se a consulta já foi iniciada no executor pertencente a uma `Tx`, a falha de mapeamento é memorizada como erro tardio (back-reference de `Row`/`Rows` à `Tx`), bloqueando `Commit` com auto-rollback. Falha **anterior** ao executor (construção, destino inválido antes da consulta) não inicia consulta e não bloqueia a transação.
- **Exclusões mantidas:** `sql.ErrNoRows` não memoriza; resultados originados no `DB` (sem back-reference) não afetam `Tx`.
- **Destino de `All`:** em falha, o destino não é preenchido com dados parciais; em sucesso sem linhas, substitui por slice vazio (preservado).
- **Leitura posicional:** `One` continua posicional (`Row.Scan`); `All` continua por nome de coluna. Recursos de leitura mantêm a responsabilidade de fechamento observável.

**Alternativas rejeitadas.**

- _Deixar `database/sql` coletar o recurso em early return (A05)._ Rejeitada: em drivers reais o iterador retém conexão até ser fechado; o wrapper não pode transferir essa responsabilidade ao GC.
- _Não registrar falha de mapeamento na Tx quando a consulta já começou._ Rejeitada: contraria §3.4.6 (erros tardios de leitura bloqueiam `Commit`).
- _Priorizar o erro de limpeza sobre o erro primário._ Rejeitada: o consumidor precisa da causa original; ambiguidade é resolvida preservando ambas.

**Impacto de compatibilidade.** Não muda assinaturas. Corrige vazamento e propagação; consumidores que dependiam de erro de limpeza substituindo a causa passam a ver o erro primário (comportamento correto e documentado).

**Exemplo de contrato.**

```go
var dst []NotAnEntity            // entidade inválida
err := rows.All(&dst)            // erro de entidade
// recurso nativo já fechado; sem vazamento
// se rows pertence a uma Tx iniciada, Commit posterior é bloqueado
```

**Código afetado.** [`internal/scan/mapping.go`](../internal/scan/mapping.go) (`Rows.All` `:133`), [`internal/scan/rows.go`](../internal/scan/rows.go) (`Row.Scan` `:29`, `Rows.Close` `:166`), [`internal/transaction/tx.go`](../internal/transaction/tx.go) (`recordLate`), testes `rows_test.go`/`mapping_test.go`, atalhos públicos em `internal/query`, `internal/sqlfile`, `internal/write`.

## 6. D05 — Configuração retida e mutabilidade

**Comportamento escolhido.** O `DB` passa a reter um **snapshot** imutável da configuração de migrações; `DB.Migrations()` devolve **cópia defensiva**.

- Em `Open`, os campos escalares e os slices (`Versions`, `ManagedObjects`) da `MigrationsConfig` são copiados para uma estrutura interna. Alterações posteriores na `MigrationsConfig` do consumidor não afetam um `DB` já aberto.
- `DB.Migrations()` devolve uma cópia, com slices próprios, para que o consumidor não possa mutar o estado interno pelo ponteiro retornado.
- **Limite da cópia:** o conteúdo de `fs.FS` **não** é copiado — cópia profunda de um `fs.FS` arbitrário não é viável nem prometida. O documento explicita que mudanças no filesystem subjacente após `Open` são responsabilidade do consumidor.
- Se em revisão surgir necessidade de mutação do alvo por consumidor, isso é um novo contrato e deve ser decidido separadamente; nada nesta decisão promete sincronização concorrente.

**Alternativas rejeitadas.**

- _Manter o ponteiro retido de `Open` (A11)._ Rejeitada: slices mutáveis retidos permitem corrupção de estado interno e sugerem concorrência/mutabilidade que a biblioteca não oferece.
- _Copiar profundamente o `fs.FS`._ Rejeitada: sem contrato genérico de clonagem em `io/fs`; a documentação do limite é suficiente.
- _Expor uma API de mutação do alvo._ Rejeitada nesta rodada: fora do escopo do RF-05; nova capacidade pública exigiria decisão própria.

**Impacto de compatibilidade.** Não muda assinaturas (`Migrations() *config.MigrationsConfig` permanece), mas muda o comportamento observável: mutações via ponteiro deixam de propagar. Consumidores que hoje alteram a config pelo ponteiro — se existirem — precisam reconfigurar antes de `Open`. Não há consumidor conhecido que dependa disso (o KriosTasks não usa migrações).

**Exemplo de contrato.**

```go
cfg := krios.Config{ /* ... */ Migrations: &krios.MigrationsConfig{ /* ... */ }}
db, _ := krios.Open(ctx, cfg)

// mutar a config original não afeta o DB aberto
t := 2
cfg.Migrations.TargetVersion = &t
// db continua com o snapshot anterior

// mutar o retorno também não afeta o interno
db.Migrations().ManagedObjects = nil
```

**Código afetado.** [`internal/access/db.go`](../internal/access/db.go) (`migrations` `:49`, `Migrations` `:231`), [`internal/config/config.go`](../internal/config/config.go) (helper de snapshot/cópia), `internal/migrate/migrate.go` (consumo do snapshot), testes de `access`/`config` e documentação de API.

## 7. Resumo das mudanças públicas

| Decisão | Muda API pública?                    | Requer correção de código | Rodada |
| ------- | ------------------------------------ | ------------------------- | ------ |
| D01     | Sim (`TargetVersion` → `*int`)       | Sim                       | C02    |
| D02     | Não (muda `CurrentVersion` observado)| Sim                       | C02    |
| D03     | Não                                  | Sim (classificação/escopo)| C03    |
| D04     | Não                                  | Sim (ciclo de vida)       | C04    |
| D05     | Não (muda mutabilidade observada)    | Sim                       | C01/C05|

## 8. Encaminhamento

- **C01-P02:** reconciliar RF-05, [E03-P01_decisions.md](E03-P01_decisions.md) §3.3/§3.4.6, [E07-P01_decisions.md](E07-P01_decisions.md) §8, [E08-P01_decisions.md](E08-P01_decisions.md) §3/§7/§8/§11, `E09_manual_validation.md` (M05/M06) e `0001_use_examples.md` com as decisões acima; identificar trechos históricos superados.
- **C02:** reproduzir e corrigir D01/D02 (alvo, versão efetiva, ciclos repetidos).
- **C03:** corrigir D03 (escopo transacional, classificação, diagnóstico e limpeza).
- **C04:** corrigir D04 (fechamento e propagação de falhas de leitura).
- **C05:** aplicar D05 e reconciliar exemplos/documentação.

Nenhuma implementação foi feita neste documento; criar C01-P01 não altera código, testes ou workflows.
