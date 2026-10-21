# C06-P02 — Cenários de migração para E10/E09

Data: 2026-10-21. Contratos: [C01 D01–D03](C01-P01_decisions.md), [C03-P01](C03-P01_decisions.md), [C03-P03](C03-P03_decisions.md), [C03-P04](C03-P04_decisions.md) e [E08](E08-P01_decisions.md). Rastreabilidade: RF-04/05 e CA-05/11/12 da [matriz C05](C05-P03_traceability.md).

Este catálogo prepara procedimentos e oráculos (resultados exigidos); não declara os servidores executados. M01–M07 mantêm seus identificadores, com expectativas corrigidas. M08–M14 ampliam ciclos, falhas e histórico. O harness e a injeção real de falhas ficam em E10-P03/P05; o registro uniforme de ambiente é C06-P03.

## 1. Ambiente, fixtures e observação comuns

Cada cenário começa em banco/esquema de teste exclusivo, sem `migrations`, `c06_parent`, `c06_child` ou `c06_tail`. Verificar ausência antes de criar; colisão encerra o cenário sem remover objetos preexistentes. Em MySQL, todas as tabelas de aplicação são InnoDB. No PostgreSQL, registrar `current_schema()`/search_path e no MySQL `DATABASE()`. Na variante com homônimos, usar um segundo esquema/banco exclusivo.

As fixtures portáveis abaixo têm **um statement por arquivo**; acrescentar `ENGINE=InnoDB` aos CREATE do conjunto MySQL. São SQL fornecido pelo consumidor, sem tradução pelo Krios. Nomes de arquivos não definem versão: a lista em `MigrationVersion` define a associação e a ordem.

| Arquivo | Conteúdo |
| --- | --- |
| `parent.up.sql` | `CREATE TABLE c06_parent (id INTEGER PRIMARY KEY);` |
| `parent.down.sql` | `DROP TABLE c06_parent;` |
| `child.up.sql` | `CREATE TABLE c06_child (id INTEGER PRIMARY KEY);` |
| `seed.up.sql` | `INSERT INTO c06_child (id) VALUES (1);` |
| `child.down.sql` | `DROP TABLE c06_child;` |
| `tail.up.sql` | `CREATE TABLE c06_tail (id INTEGER PRIMARY KEY);` |
| `tail.down.sql` | `DROP TABLE c06_tail;` |
| `fail-ddl.sql` | `CREATE TABLE c06_parent (id INTEGER PRIMARY KEY);` — falha intencional por tabela já existente |
| `dml-a.sql` | `INSERT INTO c06_parent (id) VALUES (10);` |
| `dml-b.sql` | `INSERT INTO c06_parent (id) VALUES (10);` — duplicata intencional |

Configuração-base: versão 1 = up `[parent.up.sql]`, down `[parent.down.sql]`; versão 2 = up `[child.up.sql, seed.up.sql]`, down `[child.down.sql]`. `ManagedObjects` lista todas as tabelas da variante. Cada fase abre uma nova instância sobre o mesmo banco com `TargetVersion: &target`; fecha resultados e DB antes da próxima fase. Não fazer mutação concorrente de `DB.Migrations()`.

Em cada fase, registrar:

1. Alvo, `plan == nil`, CurrentVersion/TargetVersion, arquivos/direções/Transactional de Steps e erro. Um plano preparado não é prova de execução.
2. `errors.Is` para ErrInvalidConfig/ErrInvalidQuery/ErrPartialMigration e `errors.As` para PartialMigrationError, incluindo Direction/TargetVersion/LastVersion/LastFile/Status.
3. Existência dos objetos e dados por conexão independente após fechar a instância da fase. Para tabelas esperadas, SELECT e contagem; para ausência, consultar catálogo do motor, sem converter erro de permissão/conexão em “ausente”.
4. Histórico ordenado: `SELECT version,filename,direction,status,checksum,applied_at,error FROM migrations ORDER BY version,filename,direction`. Comparar chaves, estados e completude, não apenas contagem. Não comparar timestamps entre motores nem depender de sleeps para correção.

Limpeza: depois de guardar evidências, remover somente objetos comprovadamente criados pela execução (tail, child, parent e migrations, nessa ordem) e esquemas exclusivos criados pelo operador. Em falhas parciais, preservar dados/histórico até inspeção e recuperação; o comando SQLite de M08 usa diretório temporário próprio, imprime estado e o remove ao terminar. Nunca executar DROP em esquema compartilhado.

## 2. M01–M07 revisados

### M01 — Falha MySQL sem transação, sem depender de multi-statement

Em banco novo, aplicar versão 1 (parent). Versão 2 tem `[child.up.sql, fail-ddl.sql]`. Pedir alvo 2. O primeiro arquivo da versão 2 cria child; o segundo tenta recriar parent e falha.

Esperado: erro ErrPartialMigration, arquivo falho `fail-ddl.sql`, LastVersion 1; parent/child presentes. Histórico: versão 1 aplicada, child aplicado, arquivo falho failed, sem applied_at de sucesso para ele. A criação do histórico não é prometida como reversível. Os dois arquivos são execuções separadas. MySQL real ainda não executado; base simulada: `TestMySQLPartialNoTxRestartBlocks`.

### M02 — Reabertura bloqueada

Após M01, guardar snapshot do esquema/dados/histórico, fechar e reabrir com a mesma configuração/alvo. Esperado: preparação bloqueada por ErrPartialMigration, plano nil, snapshot idêntico e nenhum arquivo reexecutado. Conferir errors.As e contexto do erro; não exigir identidade da mensagem textual. Para observar comandos enviados em servidor, E10 pode instrumentar seu adaptador; snapshot sozinho não prova ausência de tentativa sem efeito.

### M03 — Conferir efeitos persistidos

Após M01/M02, consultar parent e child independentemente: os dois existem; não houve rollback do DDL anterior. O arquivo que falhou não está applied. Registrar tanto esquema quanto linhas do histórico; não concluir versão 2 completa pela existência de child.

### M04 — Recuperação explícita por reversão do prefixo

Usar somente o banco descartável de M01, interrompendo outros executores. Exportar histórico e consultar objetos antes de qualquer alteração. Como `fail-ddl.sql` falhou por colisão, **não** remover parent, que pertence à versão 1. Remover apenas child criado pela versão 2. Conferir estado físico igual ao fim da versão 1.

Em uma transação de manutenção, apagar somente as linhas da versão 2/direção up dos dois filenames exatos dessa tentativa; não apagar histórico global nem linhas da versão 1. Confirmar e registrar as linhas afetadas. Essa é intervenção explícita do operador, não recuperação automática da biblioteca. Se não for possível provar os efeitos, não editar o histórico: parar e reconciliar primeiro.

Substituir a fixture falha na configuração pela sequência correta `[child.up.sql, seed.up.sql]`, mantendo a associação auditada; reabrir no alvo 2. Esperado: versão 1 reconhecida, child recriada, id=1 presente, todos os arquivos da versão 2 applied. Reabrir no alvo 2: nenhum passo. A alternativa de concluir manualmente e marcar applied exige provar cada efeito e preencher o histórico coerentemente; este cenário usa apenas reversão do prefixo, evitando marcar “applied” um SQL que nunca foi concluído.

### M05 — Atomicidade por versão SQLite/PostgreSQL

Banco novo; versão 1 cria parent; versão 2 usa `[child.up.sql, fail-ddl.sql]`. Pedir alvo 2. Esperado: erro comum da falha, versão 1 permanece aplicada, parent existe; child e todas as linhas da versão 2 são revertidas. A tabela migrations permanece. Não esperar rollback do percurso todo, nem ErrPartialMigration apenas por rollback bem-sucedido da versão.

Reabrir com versão 2 corrigida `[child.up.sql, seed.up.sql]`, alvo 2. Esperado: aplicação começa no estado da versão 1 e conclui versão 2. Reexecutar o alvo confirma completude. Evidência local existente: `TestMigrationsFailureRollsBackSQLite` (SQLite real); `TestSQLiteVersionScopeRollsBackWholeVersion` (driver simulado, apesar do nome).

### M06 — Presença do alvo e ausência de configuração

Executar cada variante isoladamente, observando momento da falha e ausência de efeitos:

| Variante | Resultado exigido |
| --- | --- |
| `Config.Migrations == nil` | Open válido; uso normal permitido. Chamar RunMigrations explicitamente falha com ErrInvalidConfig. |
| Migrations presente, TargetVersion nil | Open/Validate falha com ErrInvalidConfig. Não é reversão implícita. |
| Migrations presente, ponteiro para 0 | Config válida; em banco virgem, plano 0→0 sem passos; após aplicação, reverte versões. |
| Alvo negativo | ErrInvalidConfig na validação. |
| Alvo positivo não declarado | Config pode validar; preparação rejeita com ErrInvalidQuery, sem aplicar passos. |
| ManagedObjects vazio | ErrInvalidConfig quando Migrations está presente. |
| Alvo já alcançado | Plano sem passos e dados/histórico preservados. |
| Falta direção down necessária | Preparação rejeita antes de aplicar qualquer down. |

Não afirmar que Open exige migrações. Conferir os guards com `TestMigrationsNotConfigured`, R01–R04 e testes de configuração; valores devem ser variáveis locais/ponteiros, sem helper público inexistente.

### M07 — Escopo de objetos e falhas de inspeção

Sem tabela migrations, testar:

- Objeto declarado presente no esquema da conexão: ErrInvalidQuery antes de aplicar, sem criar histórico.
- Objeto declarado ausente: plano começa em 0. Objetos não declarados não são varredura global do banco; ManagedObjects é responsabilidade do consumidor.
- Homônimo apenas no esquema/banco secundário: nome simples não deve bloquear; nome qualificado aponta o objeto secundário e bloqueia. Em PostgreSQL registrar current_schema/search_path; no MySQL registrar DATABASE().
- SQLite com `main.c06_parent`: ErrInvalidConfig. Não extrapolar qualificação das migrações para builders públicos.
- Nomes inválidos/vazios: rejeição de configuração. Nome válido contendo apóstrofo: valor parametrizado, sem alteração da consulta; usar fixture/catalogo coerente com o motor.
- Retirar permissão de inspeção ou interromper conexão em ambiente controlado: erro preservado, nunca interpretado como banco vazio. E10 deve distinguir indisponibilidade de ambiente de cenário aprovado.

DDL de preparação de esquemas/usuários deve ser executado pelo operador no banco exclusivo; registrar posse antes da limpeza. SQL/argumentos dos três dialetos já possuem testes unitários em `internal/migrate/inspect_test.go`; isso não demonstra isolamento de esquemas em servidor real.

## 3. Cenários adicionais

### M08 — Ciclos, reabertura e múltiplos arquivos

Executar em bancos novos independentes:

| Sequência desde 0 | Resultado exigido |
| --- | --- |
| 0→1→0→1 | parent existe no fim e é recriada após down; regressões C02 R01/R04 |
| 0→2→1→2 | child é removida/recriada, seed id=1 existe; parent permanece; versão 2 só completa com seus dois arquivos up; C02 R02 |
| 0→1→1 | segunda aplicação sem passos; C02 R03 |
| 0→1→0→1→0→1 | duas reversões e duas reaplicações reais; nenhuma decisão depende da resolução de applied_at |
| 0→2→0→2→0→2 | mesmo requisito com múltiplos arquivos e duas versões |

Consumidor SQLite reproduzível no módulo separado de C06-P01:

```sh
cd .specs/consumers/e06e07
go run ./cmd/migration-cycles -targets 1,0,1
go run ./cmd/migration-cycles -targets 2,1,2
go run ./cmd/migration-cycles -targets 1,1
go run ./cmd/migration-cycles -targets 1,0,1,0,1
go run ./cmd/migration-cycles -targets 2,0,2,0,2
```

O [programa](consumers/e06e07/cmd/migration-cycles/main.go) incorpora SQL e usa arquivo temporário; imprime plano, existência das tabelas e histórico, verifica schema/seed e termina com erro se divergir. Não aceita servidores: E10-P03 deve portar os mesmos percursos. Campos File no plano imprimem endereços de memória, não são valores estáveis para comparação.

**Observado em C06-P02:** `1,0,1,0` falhou: o último RunMigrations devolveu plano 0→0 sem passos e erro nil, mas parent continuou presente. Achado A16 aberto; os percursos estendidos não estão validados. Não modificar o resultado esperado para acomodar a implementação.

### M09 — Parcial MySQL por arquivo transacional

Aplicar versão 1; versão 2 usa dois arquivos DML `[dml-a.sql,dml-b.sql]`. InnoDB obrigatório. Alvo 2: primeiro insert e sua linha histórica confirmam; duplicata no segundo arquivo faz rollback apenas desse arquivo, sem linha failed persistida daquele escopo. A chamada original retorna erro comum; não exigir ErrPartialMigration nesse primeiro retorno.

Reabrir: versão 2 incompleta bloqueia com ErrPartialMigration, LastVersion 1 e LastFile do prefixo aplicado `dml-a.sql`. O id=10 continua presente; o histórico não pode declarar versão 2 completa. Recuperação: reconciliar/remover o id=10 e somente a linha correspondente, corrigir segundo arquivo e repetir, seguindo os cuidados M04. Referência simulada: `TestMySQLPartialFileScopeRestartBlocks`.

### M10 — Falha ao persistir histórico

Injetar falhas em pontos determinísticos, com causas distintas e sentinelas recuperáveis via errors.Is. A API pública não expõe hook para isso: executar as reproduções simuladas abaixo; E10-P05 deve preparar adapter/proxy/permissões controladas se houver execução real. Não substituir por um sleep ou corte de conexão aleatório que não identifique o ponto atingido.

| Ponto da falha | Oráculo | Reprodução simulada |
| --- | --- | --- |
| insertInProgress antes do corpo | Corpo não executado, erro de registro preservado; não inventar efeito parcial | `TestNoTxInsertInProgressFailure` |
| Corpo sem transação falha e markFailed também | ErrPartialMigration, status do erro failed, estado incerto; errors.Is alcança ambas as causas; linha persistida pode continuar in_progress | `TestNoTxBodyFailureMarkFailedAlsoFails` |
| Corpo sem transação conclui e markApplied falha | Efeito físico permanece; ErrPartialMigration/status in_progress; reabertura bloqueada pelo histórico, sem repetir SQL | `TestNoTxMarkAppliedFailure` |
| Commit de arquivo/versão falha | Limpeza defensiva tentada; erro indica desfecho incerto e preserva falhas de commit/rollback; não prometer reversão nem exigir ErrPartialMigration | `TestFileScopeCommitFailureCleansUp`, `TestVersionScopeCommitFailureCleansUp` |

Capturar snapshot antes/depois e ponto realmente injetado; se o servidor não permitir a injeção controlada, manter “não executado real”. Erro estruturado descreve diagnóstico e pode diferir do status persistido se a atualização falhou. M04 não autoriza limpar registros sem apurar o efeito.

### M11 — Down parcial e versões não contíguas

Configurar versões 1 e 3. Versão 3 contém dois objetos/arquivos e dois downs ordenados. Em MySQL, provocar falha no segundo down (por injeção determinística ou dependência externa criada exclusivamente para o teste); primeiro objeto já removido, segundo permanece. Reabrir e conferir bloqueio, Direction down, LastFile do último prefixo aplicado e LastVersion conforme última versão completa, não “versão parcial concluída”. Após recuperação completa, estado efetivo deve ser 1, não uma versão 2 inexistente. Conferir erro durante aplicação e durante preparação separadamente. SQLite/PostgreSQL devem reverter a versão 3 inteira quando os comandos suportam transação.

Referências: `TestDetectPartialDownReportsLastFile`, `TestEffectiveVersionNonContiguous`, `TestAppliedAfterDownNonContiguous` em `progress_test.go`. São funções puras, não observação real de down em servidores; E10-P03/P05 deve exercitar a sequência física.

### M12 — Histórico incompatível

Em cópia descartável de banco de teste, capturar histórico e depois alterar **uma condição por vez**: status desconhecido, direção desconhecida, versão desconhecida ou arquivo da versão ausente. Reabrir no alvo: preparação deve recusar histórico incompatível/incompleto, sem efeitos nem execução automática. Guardar erro/sentinela real; não exigir o mesmo sentinela para corrupção e parcialidade sem contrato específico. Restaurar snapshot ou descartar banco entre variantes; não reconstruir versão usando apenas MAX(version).

### M13 — Multi-statement opcional, separado do percurso portátil

Arquivo único contém `CREATE TABLE c06_child (...)` seguido de SQL inválido. Antes, registrar driver/versão, protocolo e configuração que habilitam múltiplos statements, e provar com um arquivo de dois statements válidos que ambos executam. Só depois executar a variante de falha.

MySQL: se o driver suportar esse modo, o primeiro DDL pode persistir e o arquivo falho deve produzir estado parcial; inspecionar efeito/histórico. Se o driver rejeitar o arquivo antes do primeiro statement, registrar essa rejeição e não declarar validado o cenário de efeito parcial. O Config atual não oferece opção pública multiStatements; não inventar campo/DSN. E10 pode preparar driver/adaptador de teste em seu módulo, com suporte explicitamente registrado, ou classificar a variante como não executada. M01 já cobre o caminho obrigatório com arquivos separados.

### M14 — Cancelamento entre passos

Injetar cancelamento após uma versão/arquivo confirmado e antes do seguinte. Registrar o ponto, erro de contexto e snapshot. Versões confirmadas permanecem; escopo ativo deve ser limpo conforme contrato. Reabertura precisa reconhecer estado completo ou parcial real, sem marcar passos não executados. Teste simulado: `TestCancelBetweenSteps`; caso real em E10-P05 exige coordenação determinística.

## 4. Comandos locais de referência e limites

Da raiz do módulo principal:

```sh
go test -count=1 -run 'TestMigrations(ReproC02|FailureRollsBackSQLite|SchemaWithoutHistory|NotConfigured|MissingDownDirection)' ./tests
go test -count=1 ./internal/migrate
```

Os testes existentes não substituem M08 estendido: C02 cobre uma reaplicação, e a segunda reversão revelou A16. Uma suíte verde não resolve esse achado. Os resultados da tarefa estão em [C06-P02_decisions.md](C06-P02_decisions.md); resultados reais MySQL/PostgreSQL continuam não executados. A16 precisa de correção com regressão antes da passagem C07-P03; C06-P03 pode preparar o registro sem esperar a correção.
