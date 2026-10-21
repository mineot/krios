# E08-P01 — Decisões: Migrações versionadas (versões, arquivos e histórico)

Data: 2026-10-20. Resolve as pendências de E08-P01 em [0002_execution_plan.md](0002_execution_plan.md) e a seção §6.4 de [0001_planning.md](0001_planning.md), sob os RF-04 e RF-05. Contratos de consumidor decididos pelo consumidor em 2026-10-20.

## 1. Contexto

E07 fixou o objeto de SQL incorporado (`krios.NewSQLFile(fs.FS, path)`) e o runner `db.File(f)`/`tx.File(f)` sobre o `executor` interno de E03. E05/E06 fixaram os construtores ancorados no acesso e o padrão de interface de dialeto segregada no mesmo tipo concreto (`ReturningDialect`, [E06-P01_decisions.md](E06-P01_decisions.md) §7). Restava especificar o RF-04/RF-05: como `krios.Config` recebe a versão desejada e a associação de arquivos às versões, a ordem de execução, os pares `up`/`down`, a estrutura da tabela `migrations`, o estado `0`, a delimitação do esquema gerenciado, o sentinela `ErrPartialMigration` e o contrato de dialeto para capacidades transacionais. Este documento fixa esses contratos antes da implementação em E08-P02.

Os arquivos de migração são recursos `.sql` incorporados pelo consumidor via `go:embed` (RF-04); a biblioteca apenas lê recursos já incorporados, reutilizando `krios.NewSQLFile` de [E07-P01_decisions.md](E07-P01_decisions.md). Nenhuma tradução de sintaxe ou de marcadores entre bancos acontece ([0001_planning.md](0001_planning.md) §4.2).

## 2. Escopo e adiamentos

Decidido aqui: formato de `MigrationsConfig` e associação arquivo→versão (§4); ordem de execução e pares `up`/`down` (§5); delimitação do esquema gerenciado (§6); estrutura da tabela `migrations` e estados (§7); assinatura de `RunMigrations()` e comparação de versões (§8); sentinela `ErrPartialMigration` (§9); contrato de dialeto transacional (§10); promessa e escopo transacional (§11).

| Item                                                                                                  | Destino                                            |
| ----------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| Implementação (`internal/migrate`, fachada em `krios.go`, testes unitários e integração SQLite)       | E08-P02 (preparação) e E08-P03 (execução/recuperação) |
| Validação dos percursos e falhas (banco vazio, avanço, reversão, estado parcial, recuperação)         | E08-P04                                            |
| Evidência real MySQL/PostgreSQL (cenários M01–M06 do [roteiro E09](E09_manual_validation.md) §4.1)     | E09                                                |
| Divisão de um arquivo em múltiplos statements (splitting)                                             | Extensão herdada de E07 §9; não adotada            |
| Reescrita de marcadores (`?` ↔ `$n`)                                                                  | Nunca — proibido pelo RF-04                        |
| Associação implícita por prefixo numérico do nome do arquivo                                          | Não adotada; o prefixo é só convenção (§5)         |
| Introspecção genérica do esquema para descobrir objetos de aplicação                                  | Não adotada; objetos são declarados em `Config` (§6) |

## 3. Promessa transacional

- PostgreSQL e SQLite: executar por versão com escopo transacional quando a sequência for transacional; **rollback da versão em execução** em caso de falha, com versões já confirmadas permanecendo (sem rollback de todo o percurso).
- MySQL: **não** prometer rollback integral quando o arquivo/conjunto contiver comandos com commit implícito. Usar escopo máximo viável (por arquivo quando 100% transacional, senão sem transação), registrar estado parcial e reportar claramente ao consumidor. Ver [C01-P01_decisions.md](C01-P01_decisions.md) (D03).

## 4. Configuração: versão desejada e associação de arquivos

`MigrationsConfig` deixa de ser uma reserva vazia ([E02-P01_decisions.md](E02-P01_decisions.md) §2) e passa a ter a forma abaixo. É **opcional**: quando `Config.Migrations` é `nil`, o uso da biblioteca não carrega nem executa migrações (RF-05), e `RunMigrations` é rejeitado por erro de configuração.

```go
// krios.MigrationVersion associa uma versão do esquema aos arquivos que a
// implementam em cada direção.
type MigrationVersion struct {
    // Version é o número da versão. Obrigatório, único e > 0.
    Version int
    // Up são os arquivos up, na ordem de execução da versão.
    Up []string
    // Down são os arquivos down, na ordem de execução da reversão.
    Down []string
}

// krios.MigrationsConfig é a configuração de migrações, preenchida em código.
type MigrationsConfig struct {
    // FS recebe os recursos incorporados pelo consumidor (ex.: embed.FS).
    FS fs.FS
    // TargetVersion é a versão desejada. Obrigatória e sempre explícita.
    // Sempre um ponteiro: nil é erro (ErrInvalidConfig); &0 solicita reversão total.
    TargetVersion *int
    // Versions associa versões a arquivos. A ordem declarada define a
    // sequência entre versões.
    Versions []MigrationVersion
    // ManagedObjects lista os objetos (tabelas) sob gestão do Krios,
    // usados na verificação de um banco sem histórico (§6).
    ManagedObjects []string
}
```

- **Versão desejada explícita:** `TargetVersion` é sempre obrigatória quando `Migrations != nil` e deve ser um ponteiro não-nulo (`*int`). `nil` é erro de configuração (`ErrInvalidConfig`). Se o consumidor deseja reverter tudo, informa `TargetVersion: ptr(0)` explicitamente (§8). Nunca interpretado como reversão (RF-05). Ver [C01-P01_decisions.md](C01-P01_decisions.md) (D01).
- **Associação explícita:** a versão vem de `MigrationVersion.Version`, e os arquivos vêm de `Up`/`Down`. O prefixo numérico do nome do arquivo **não** é fonte da versão nem da ordem (RF-04); é apenas convenção de organização.
- **Recursos incorporados:** `FS` segue a semântica de `io/fs` já usada por `krios.NewSQLFile` ([E07-P01_decisions.md](E07-P01_decisions.md) §3). Os nomes em `Up`/`Down` são caminhos relativos ao `FS`, carregados por `NewSQLFile` na preparação (E08-P02).
- **Estrutura explícita por versão (decisão do consumidor, 2026-10-20):** a associação não é descoberta por varredura do `FS`; o consumidor declara cada versão e seus arquivos. Isso mantém RF-04 (prefixo não é definitivo) e permite múltiplos arquivos por direção sem depender de nomes.

## 5. Ordem e pares `up`/`down`

- **Ordem entre versões:** a sequência é a ordem em que `Versions` é declarada, filtrada por `Version <= TargetVersion` (avanço) ou `Version > TargetVersion` (reversão). Não há ordenação implícita por número; declarar versões fora de ordem é responsabilidade do consumidor, e a divergência entre a ordem declarada e a numérica é erro de configuração (`ErrInvalidConfig`) — evita percursos ambíguos.
- **Ordem dentro da versão:** no avanço, `Up` na ordem declarada; na reversão, `Down` na ordem declarada. A biblioteca nunca reordena os arquivos de uma direção; inverter a ordem de reversão é responsabilidade do consumidor ao declarar `Down`.
- **Pares `up`/`down`:** não é obrigatório que toda versão tenha as duas direções — uma versão inicial pode ter apenas `Up`. Se `Up` e `Down` existirem, ambos devem estar completos e com arquivos válidos. Ausência da direção necessária para o percurso até `TargetVersion` é erro antes de executar (`ErrInvalidQuery`): reverter exige `Down` em todas as versões acima da desejada; avançar exige `Up` em todas as versões alcançadas.
- **Validação de arquivos:** arquivo ausente no `FS`, diretório ou conteúdo em branco resultam em `ErrFileNotFound` (via `NewSQLFile`, [E07-P01_decisions.md](E07-P01_decisions.md) §8). Nome repetido dentro da mesma direção e versão é `ErrInvalidQuery`.
- **Versões duplicadas:** `Version` repetida em `Versions` é `ErrInvalidConfig`.

## 6. Esquema gerenciado e verificação inicial

- **Delimitação por declaração explícita (decisão do consumidor, 2026-10-20):** `ManagedObjects` lista as tabelas/objetos sob gestão do Krios. A verificação de "banco sem histórico" inspeciona exatamente esses objetos, e não o esquema inteiro.
- **Sem introspecção genérica:** a biblioteca não enumera todas as tabelas de usuário nem interpreta `information_schema`/`sqlite_master` além da consulta pontual de existência dos objetos declarados em `ManagedObjects` e da tabela `migrations`. Isso torna a verificação previsível entre dialetos e evita colidir com objetos não geridos.
- **Regra de inicialização:** sem tabela `migrations` e sem nenhum objeto de `ManagedObjects` presente → versão atual = 0 e percursos normais. Sem tabela `migrations` mas com algum objeto de `ManagedObjects` presente → erro antes de executar (`ErrInvalidQuery`), pois adotar esquema preexistente sem histórico está fora do escopo ([0001_planning.md](0001_planning.md) §3).
- **`ManagedObjects` obrigatório ao executar:** quando `Migrations != nil`, uma lista vazia é `ErrInvalidConfig` — sem ela a biblioteca não consegue distinguir "banco vazio" de "esquema preexistente". O consumidor que gerencia tabelas deve declará-las.
- **Revisão C03-P01 (2026-10-20) em [C03-P01_decisions.md](C03-P01_decisions.md) (D6–D9):** cada nome é tabela simples ou `schema.tabela` (máx. duas partes não vazias; SQLite rejeita qualificação). A consulta de existência é parametrizada e delimitada ao esquema da conexão (`DATABASE()` no MySQL, `current_schema()` no PostgreSQL, `sqlite_master` no SQLite; nome qualificado usa `table_schema` explícito), sem interpolar o nome no SQL. Falhas da inspeção preservam a causa por `%w` com o objeto no contexto, sem sentinela nova; `isMissingTable` permanece textual apenas para a sonda da tabela de histórico, com os padrões dos motores documentados.

## 7. Estrutura da tabela `migrations`

Campos mínimos:

| Coluna       | Tipo        | Descrição                                                              |
| ------------ | ----------- | ---------------------------------------------------------------------- |
| `version`    | INT/BIGINT  | Versão numérica                                                        |
| `filename`   | VARCHAR     | Nome exato do arquivo (`0001_xxx.up.sql`)                              |
| `direction`  | VARCHAR(4)  | `up` ou `down`                                                         |
| `status`     | VARCHAR(12) | `applied` \| `failed` \| `in_progress`                                 |
| `checksum`   | VARCHAR(64) | SHA-256 do conteúdo (recomendado v1; pode ser obrigatório futuramente) |
| `applied_at` | TIMESTAMP   | Preenchido ao tornar `applied`                                         |
| `error`      | TEXT        | Mensagem resumida quando `failed`                                      |

Chave única: `(version, filename, direction)`. Histórico deve ser imutável em caso de falha: um registro `failed` nunca é sobrescrito para `applied` pela biblioteca.

### 7.1. Estados e transições

- Iniciar arquivo → registrar `in_progress`.
- Sucesso → atualizar para `applied`, setar `applied_at`.
- Falha → atualizar para `failed`, registrar `error`. Nunca marcar `applied`.
- Estado "versão atual" considera apenas `status='applied'`.
- Ao carregar, se existir qualquer registro `in_progress` → bloquear execução.
- **Versão 0:** a tabela registra o histórico; a versão `0` (estado anterior à primeira migração) é o **estado vazio** da tabela — não há linha sentinela para `0` (`Version` é sempre `> 0` em `MigrationVersion`; §4). "Versão atual = 0" significa ausência de registros `applied`.

## 8. `RunMigrations()` e comparação de versões

Assinatura pública (RF-05):

```go
// RunMigrations moves the schema to Config.Migrations.TargetVersion, applying
// up or down migrations as needed, and returns the plan it prepared (so the
// caller can inspect what was executed). The configuration is always
// explicit: a missing Migrations config, a non-positive/absent target or
// verified partial state block execution.
func (db *DB) RunMigrations(ctx context.Context) (*Plan, error)
```

> **Ajuste de implementação (E08-P03, 2026-10-20):** a assinatura passou a retornar `(*Plan, error)` em vez de apenas `error` — o `Plan` é útil para o consumidor inspecionar o que foi preparado/executado, e a fachada já o expunha desde E08-P02. O comportamento de erro é o mesmo descrito abaixo.


- **Execução explícita:** o consumidor decide quando chamar; abrir o banco e executar operações comuns nunca dispara migrações (RF-05).
- **Configuração obrigatória:** `Config.Migrations == nil` → `ErrInvalidConfig`. `TargetVersion` em `MigrationsConfig` é sempre um ponteiro explícito não-nulo (§4); `nil` → `ErrInvalidConfig`. A ausência de toda a config é o único "não configurado".
- **Estado `0`:** `*TargetVersion == 0` solicita reversão de todas as migrações `applied`. Versões já confirmadas podem permanecer se a reversão falhar em uma versão posterior (escopo por versão), com bloqueio/estado parcial conforme D03.
- **Comparação:** versão atual = versão efetiva derivada do histórico, considerando apenas versões **integralmente concluídas** (todos os arquivos da versão com `status='applied'`). A mera existência de uma linha `applied` não basta; versões parcialmente executadas são tratadas como inconsistentes. Igual a `TargetVersion` → nenhuma migração. Inferior → aplicar `up` até alcançar; superior → aplicar `down` até alcançar (§5). Ciclos repetidos (`0→1→0→1`) são suportados após redução do estado efetivo (D02).
- **Verificações antes de executar** (§6 e §7): ausência de histórico com objetos geridos presentes → erro; registro `in_progress`/`failed` → `ErrPartialMigration` (§9); direção necessária incompleta (§5) → `ErrInvalidQuery`.
- **Concorrência:** a biblioteca não promete coordenação entre chamadas simultâneas de `RunMigrations` em processos distintos nem a mesma `DB`; a resolução de conflito de histórico é por erro (chave única da tabela, §7). Nada desta entrega cria caminho de execução alternativo ao `executor` de E03 — os statement e o DDL usam `DB.ExecContext`/`Begin`.

## 9. Erro para estado parcial

`krios.ErrPartialMigration` (sentinela) com contexto:

- `Direction string`
- `TargetVersion int`
- `LastVersion int` (última versão aplicada com sucesso)
- `LastFile string`
- `Status string` (`in_progress`|`failed`)
- `Message string`

Comportamento:

- Nunca continuar automaticamente.
- Nunca tentar reverter DDL já committed no MySQL.
- Execução bloqueada até resolução explícita pelo consumidor.
- Preservar mensagem original com `%w` quando envolver erro.

A mensagem resume o estado; o contexto é exposto por um tipo de erro estruturado que satisfaz `errors.As` e carrega os campos acima. A implementação do tipo e da sentinela fica em E08-P02 (`internal/errs`), no padrão de `NormalizeTxError` ([E03-P01_decisions.md](E03-P01_decisions.md)).

## 10. Capacidades transacionais por dialeto

O dialeto já implementa esta capacidade em `internal/dialect.migration.go`; o contrato está fixado aqui e é consumido pela camada de migrações pela interface `MigrationDialect` (segregação no mesmo tipo concreto, padrão de [E06-P01_decisions.md](E06-P01_decisions.md) §7):

```go
type MigrationDialect interface {
  // SupportsTransactionalDDL indica se TODOS os DDLs são transacionais neste dialeto.
  SupportsTransactionalDDL() bool
  // IsTransactionalStatement retorna false se o statement não é conhecido
  // como transacional (em dúvida, false) e erro para comandos de controle
  // de transação, que são rejeitados antes de aplicar.
  IsTransactionalStatement(sql string) (bool, error)
}
```

Valores (conservadores):

| Dialeto    | SupportsTransactionalDDL | Observação                                                |
| ---------- | ------------------------ | --------------------------------------------------------- |
| PostgreSQL | true                     | DDL transacional                                          |
| SQLite     | true                     | DDL transacional dentro da transação                      |
| MySQL      | false                    | Assumir não-universal por segurança. Atomic DDL limitado. |

Detecção por statement deve ignorar case, comentários e espaços. Palavras-chave típicas que justificam tratar como não-transacional: `CREATE`, `ALTER`, `DROP`, `RENAME`, `TRUNCATE`, `LOCK`, `UNLOCK`, `REPAIR`, `OPTIMIZE`, `ANALYZE`, `FLUSH`.

> **Revisado por [C03-P02_decisions.md](C03-P02_decisions.md) (D10–D13, 2026-10-20):** o comportamento passou a ser uma allowlist de DML (`INSERT`, `UPDATE`, `DELETE`, `SELECT`, `REPLACE`, `WITH`) somada à denylist acima, ampliada para `GRANT`, `REVOKE`, `LOAD`, `CHECK`, `CACHE` e `RESET` conforme o §15.3.3 do manual MySQL; keyword fora das duas listas resulta em `false`. `SET` ficou fora da allowlist (`SET PASSWORD`/`SET autocommit` causam commit implícito). Comandos de controle (`BEGIN`, `START`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `RELEASE`, `END`, `ABORT`) retornam erro em todos os dialetos; e `fileTransactional` varre os statements também em SQLite/PostgreSQL, propagando esses erros e comentários de bloco não terminados.

## 11. Escopo transacional

| Cenário                                     | Escopo                                              | Comportamento                                                                                                                                                           |
| ------------------------------------------- | --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PostgreSQL/SQLite                           | Por versão (quando toda a sequência é transacional) | Envolver arquivos da versão em transação. Falha → rollback da versão em execução; versões já confirmadas permanecem.                                                       |
| MySQL – arquivo 100% transacional           | Por arquivo                                         | Envolver arquivo em transação própria. Arquivos anteriores permanecem aplicados.                                                                                        |
| MySQL – arquivo contém DDL não-transacional | Sem transação                                       | Executar statements sequencialmente. Falha em statement pode deixar banco em estado parcial deste arquivo. **Não** tentar reverter statements já executados/commitados. |

Nunca dividir um único arquivo. Se qualquer arquivo da sequência (MySQL) for não-transacional, não usar transação por versão.

## 12. Itens resolvidos e permanentes

Resolvidos nesta entrega:

- [x] Versão desejada e associação de arquivos em `krios.Config` (§6.4, RF-04/RF-05): `MigrationsConfig{FS, TargetVersion, Versions, ManagedObjects}` com `MigrationVersion{Version, Up, Down}` (§4).
- [x] Papel do prefixo numérico e tratamento de divergências (§6.4): prefixo é convenção; versão e ordem vêm da config; divergência/duplicidade é erro (§5).
- [x] Ordem de execução dentro da versão e entre versões (§6.4): ordem declarada; reversão usa `Down` na ordem declarada (§5).
- [x] Pares `up`/`down` incompletos (§6.4): validação por direção necessária ao percurso (§5).
- [x] Delimitação do esquema gerenciado e objetos da verificação inicial (§6.4): `ManagedObjects` declarado; sem introspecção genérica (§6).
- [x] Estrutura da tabela `migrations` e estado `0` (§6.4): §7, com `0` como estado vazio da tabela.
- [x] Comparação de versões e execução explícita (§6.4/RF-05): `RunMigrations(ctx)` em `*DB` (§8).
- [x] Sentinela de estado parcial (`ErrPartialMigration`) e recuperação guiada (§6.4/RF-05): §9, sem reverter DDL já committed e sem continuar automaticamente.
- [x] Contrato de dialeto transacional (`SupportsTransactionalDDL`, `IsTransactionalStatement`) (§6.2/§6.4): §10, interface segregada no mesmo tipo concreto.
- [x] Escopo transacional máximo viável (§6.4/RF-05): §11, por versão (PG/SQLite), por arquivo (MySQL transacional) ou sem transação.

Permanecem em outras entregas: implementação da preparação (E08-P02, concluída); execução e recuperação de falhas (E08-P03, concluída); validação dos percursos e falhas (E08-P04); evidência real MySQL/PostgreSQL dos cenários M01–M06 (E09). Splitting de arquivos e reescrita de marcadores continuam proibidos/não adotados, herdados de [E07-P01_decisions.md](E07-P01_decisions.md) §9.

## 13. Referências

- RF-04 e RF-05 de [0001_planning.md](0001_planning.md); §6.2 e §6.4 (drivers/dialetos e migrações).
- [E02-P01_decisions.md](E02-P01_decisions.md): `Config` recebida por valor, validação, `ErrInvalidConfig`.
- [E03-P01_decisions.md](E03-P01_decisions.md): contrato `executor`, transações e normalização de erros.
- [E06-P01_decisions.md](E06-P01_decisions.md): padrão de interface de dialeto segregada no mesmo tipo concreto.
- [E07-P01_decisions.md](E07-P01_decisions.md): `NewSQLFile(fs.FS, path)`, recebimento de recursos e `ErrFileNotFound`.
- [E09_manual_validation.md](E09_manual_validation.md): cenários M01–M06 (estado parcial MySQL, rollback integral PG/SQLite, `0` e versão ausente).
- Passos E08-P01–E08-P04 em [0002_execution_plan.md](0002_execution_plan.md).
- MySQL 8.4 – Implicit Commit: https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html
- MySQL 8.4 – Atomic DDL: https://dev.mysql.com/doc/refman/8.4/en/atomic-ddl.html
- PostgreSQL Transactions: https://www.postgresql.org/docs/current/tutorial-transactions.html
- SQLite Transaction: https://sqlite.org/lang_transaction.html
