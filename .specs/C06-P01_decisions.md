# C06-P01 — Consumidor de escritas e SQL incorporado (D39–D41)

Data: 2026-10-21. Base `dcf7e52`, mais alterações locais desta tarefa; nenhum commit criado. Escopo: C06-P01 do [plano de consolidação](0005_consolidation_plan.md). Contratos: E06-P01, E07-P01, C04 e [matriz C05-P03](C05-P03_traceability.md).

## D39 — Módulo consumidor separado e reproduzível

O [consumidor](consumers/e06e07/README.md) fica em `.specs/consumers/e06e07`, com `go.mod`/`go.sum` próprios e replace relativo para o checkout. Importa a API pública, MySQL v1.10.1, pgx v5.9.2 e modernc SQLite v1.60.1. São versões explicitamente fixadas para reprodução; não se afirma que sejam as mais recentes. A escolha dos ambientes oficiais permanece em E10-P01. `go.mod`/`go.sum` da biblioteca não foram alterados.

Configuração do programa: `-db sqlite|mysql|postgres`; servidores recebem parâmetros por variáveis locais convertidas em Config, com portas padrão. SQLite cria banco temporário. SQL incorporado tem fixtures separadas por dialeto e um statement por arquivo. Placeholders são escritos explicitamente; não há tradução pelo consumidor nem pela biblioteca.

Esta entrega prepara um consumidor manual com asserções, não provisionamento, serviços ou jobs de CI. E10 reutilizará os comportamentos no harness com esquema criado por migrações. C06-P02 permanece responsável pelos cenários M01–M07/ciclos e C06-P03 pelo registro consolidado de ambientes/resultados.

## D40 — Cenários W01–W07 e F01–F05

A tabela completa de procedimento/resultado/rastreabilidade está no [README do consumidor](consumers/e06e07/README.md), referenciada no §4.2 do [roteiro E09](E09_manual_validation.md).

- W01/W02: zeros, nil, tipos anuláveis, defaults e Omit, com leitura dos valores persistidos.
- W03: validação automática e filtros obrigatórios, rejeições sem efeitos e sem bloquear Commit.
- W04: Returning de INSERT/UPDATE/DELETE; MySQL deve rejeitar com ErrInvalidQuery e manter snapshot idêntico; SQLite/PostgreSQL devem retornar os registros esperados.
- W05/W06: commit/rollback e UPDATE/DELETE de múltiplas linhas; observar dados, não apenas Result.
- W07: ID gerado com Omit, disponibilidade de LastInsertId e comparação com o registro persistido quando disponível.
- F01/F03: Exec/One/All em DB/Tx, parâmetros separados, One na ordem de campos e All com colunas invertidas por nome; commit/rollback observáveis.
- F02: recursos ausente/branco e runner nil rejeitados sem efeitos.
- F04: conversão tardia via QueryRow.Scan bloqueia Commit e descarta escrita anterior, preservando causa; ErrNoRows não bloqueia.
- F05: SQL inválido distingue falha de execução de carregamento; Commit preserva erro e reverte a escrita anterior.

Result é registrado por flags. Não se promete RowsAffected universal nem significado de LastInsertId após UPDATE/DELETE. Execuções de servidor não são substituídas por compilação. Um marcador PASS parcial não significa conclusão; a linha final só aparece depois da limpeza sem erro.

## D41 — Isolamento e limpeza

A única tabela criada/removida é `krios_c06_entries`, em banco de teste exclusivo. CREATE não usa IF NOT EXISTS. A limpeza só é registrada após criação bem-sucedida; usa contexto independente e propaga/imprime erro. MySQL usa InnoDB. Há teste SQLite `TestPreexistingTableIsPreserved`, que cria tabela com estrutura/dado próprios, provoca colisão e comprova a preservação do valor 42.

Asserções interrompem a execução com panic, permitindo a execução dos defers. Encerramento forçado pode exigir limpeza manual, conforme o README. Nenhum banco de servidor foi acessado nesta tarefa. As fixtures não adicionam drivers à biblioteca nem usam dev.db.

## Verificação observada

Ambiente local: Linux amd64. Comandos executados dentro do módulo consumidor:

| Comando / cenário | Resultado |
| --- | --- |
| `go mod tidy` | Dependências fixadas, go.sum gerado; acesso autorizado ao cache/rede após bloqueio de gravação do sandbox |
| `go run . -db sqlite`, Go 1.26.8 | W01–W07/F01–F05 aprovados; linha final `C06-P01 PASS sqlite` |
| `go vet ./...`, Go 1.26.8 | Aprovado, inclusive após inclusão do teste de colisão; repetição autorizada após bloqueio do cache |
| `go build -o /tmp/krios-c06-e06e07 .`, Go 1.26.8 | Compilado com imports dos três drivers |
| `GOTOOLCHAIN=go1.27.2 go run . -db sqlite` | W01–W07/F01–F05 aprovados |
| `/tmp/krios-c06-e06e07 -db sqlite`, diretório corrente `/tmp` | Aprovado: execução independente dos arquivos SQL no checkout |
| `go test -count=1 ./...`, Go 1.26.8 | `TestPreexistingTableIsPreserved` aprovado sem cache |
| MySQL W01–W07/F01–F05 | **Não executado**; compilação apenas |
| PostgreSQL W01–W07/F01–F05 | **Não executado**; compilação apenas |

SQLite: W06 observou RowsAffected 2 no UPDATE e 4 no DELETE; W07 observou LastInsertId disponível e igual ao ID persistido. Estes resultados são específicos desta execução/driver. As saídas de ambos os Go incluíram todos os IDs.

A raiz não foi retestada: não há alteração no código nem nas dependências da biblioteca. Módulos aninhados não são incluídos por `go test ./...` da raiz; os comandos explícitos acima são necessários. Não foram criados jobs de integração nem executado CI remoto.
