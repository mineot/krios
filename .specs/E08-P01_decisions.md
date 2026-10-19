# E08-P01 — Decisões: Migrações com DDL não-transacional (MySQL)

## 1. Contexto

Migrações podem conter DDL. No MySQL, muitos DDLs causam commit implícito, impossibilitando rollback integral. PostgreSQL e SQLite possuem DDL transacional. RF-05 deve ser condicional quanto à atomicidade.

## 2. Promessa transacional

- PostgreSQL e SQLite: quando aplicável, executar com escopo transacional que permita rollback integral em caso de falha.
- MySQL: **não** prometer rollback integral quando o arquivo/conjunto contiver comandos com commit implícito. Usar escopo máximo viável, registrar estado parcial e reportar claramente ao consumidor.

## 3. Capacidades transacionais por dialeto

O dialeto utilizado em migrações deve expor:

```go
type MigrationDialect interface {
  // SupportsTransactionalDDL indica se TODOS os DDLs são transacionais neste dialeto.
  SupportsTransactionalDDL() bool
  // IsTransactionalStatement retorna false se o statement é conhecido por causar commit implícito.
  // Deve ser conservador: em dúvida, retornar false.
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

## 4. Escopo transacional

| Cenário                                     | Escopo                                              | Comportamento                                                                                                                                                           |
| ------------------------------------------- | --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PostgreSQL/SQLite                           | Por versão (quando toda a sequência é transacional) | Envolver sequência de arquivos até versão desejada em única transação. Falha → rollback integral.                                                                       |
| MySQL – arquivo 100% transacional           | Por arquivo                                         | Envolver arquivo em transação própria. Arquivos anteriores permanecem aplicados.                                                                                        |
| MySQL – arquivo contém DDL não-transacional | Sem transação                                       | Executar statements sequencialmente. Falha em statement pode deixar banco em estado parcial deste arquivo. **Não** tentar reverter statements já executados/commitados. |

Nunca dividir um único arquivo. Se qualquer arquivo da sequência (MySQL) for não-transacional, não usar transação por versão.

## 5. Estrutura da tabela `migrations`

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

Chave única sugerida: `(version, filename, direction)`. Histórico deve ser imutável em caso de falha.

## 6. Estados e transições

- Iniciar arquivo → registrar `in_progress`.
- Sucesso → atualizar para `applied`, setar `applied_at`.
- Falha → atualizar para `failed`, registrar `error`. Nunca marcar `applied`.
- Estado "versão atual" considera apenas `status='applied'`.
- Ao carregar, se existir qualquer registro `in_progress` → bloquear execução.

## 7. Erro para estado parcial

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

## 8. Verificações

- Ausência de versão desejada → erro (não interpretado como reversão).
- Versão `0` solicita reversão de todas as migrações `applied`.
- Banco sem histórico + objetos de aplicação → erro.
- Banco sem histórico + sem objetos → versão atual = 0.

## 9. Referências

- MySQL 8.4 – Implicit Commit: https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html
- MySQL 8.4 – Atomic DDL: https://dev.mysql.com/doc/refman/8.4/en/atomic-ddl.html
- PostgreSQL Transactions: https://www.postgresql.org/docs/current/tutorial-transactions.html
- SQLite Transaction: https://sqlite.org/lang_transaction.html
