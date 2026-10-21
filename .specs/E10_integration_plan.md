# E10 — Integração completa em SQLite, MySQL e PostgreSQL

Data: 2026-10-20. **Estado: planejado; suíte e jobs ainda não implementados.**

## 1. Objetivo e decisão de escopo

Criar testes especializados que executem o fluxo completo da biblioteca contra os três bancos reais, localmente e no GitHub Actions: preparar ambiente vazio, criar o esquema com migrações Krios, executar as operações públicas, conferir dados e histórico, reverter, reaplicar e limpar os recursos de teste.

Esta entrega foi solicitada pelo consumidor e substitui a decisão anterior de manter a integração MySQL/PostgreSQL exclusivamente manual. Os testes unitários, simulados e SQLite existentes continuam compondo a verificação. O [roteiro E09](E09_manual_validation.md) passa a ser fonte de cenários para automação e registro dos casos residuais que exijam execução manual.

**Ordem de execução:** consolidação C01–C07 → E10 → conclusão de E09/publicação. O identificador E10 é adicional e não renumera as entregas anteriores. A preparação documental de E09 pode avançar, mas sua validação final e publicação dependem das evidências da E10. A E10 depende dos contratos consolidados de E01–E08, sem depender de uma versão já publicada.

## 2. O que significa testar o fluxo completo

O provisionamento cria a instância e o banco vazio (e usuário de teste quando necessário); no SQLite, reserva um arquivo temporário. **A criação das tabelas de aplicação e suas alterações deve ocorrer por `Config.Migrations` e `RunMigrations`.** Criar o banco de servidor com `CREATE DATABASE` é preparação do ambiente, não uma capacidade nova exigida da API Krios.

Fluxo principal obrigatório por banco:

1. Provisionar ambiente isolado e verificar que o banco está pronto para conexões, com prazo máximo.
2. Abrir o acesso pela API pública com o driver real; conferir ausência de tabelas de aplicação.
3. Aplicar migrações incorporadas via `go:embed`, com mais de uma versão e múltiplos arquivos por versão; conferir tabelas, relações e histórico.
4. Derivar entidades, inserir dados e exercitar consultas, alterações, exclusões, validação, formatação, SQL direto, SQL incorporado e transações.
5. Conferir resultados e efeitos persistidos; fechar e reabrir para verificar que o comportamento não depende somente do estado em memória.
6. Reverter parcialmente, reaplicar, retornar à versão `0`, reaplicar desde zero e repetir a chamada no mesmo alvo. Conferir esquema e estado efetivo após cada transição.
7. Fechar recursos e remover somente o banco/arquivo criado pelo teste. A limpeza externa deve funcionar mesmo quando `down` falhar; não pode mascarar uma falha de reversão.

Casos negativos usam ambientes próprios para não contaminar o fluxo principal. Criação direta de objetos é permitida quando ela é a condição que o cenário precisa testar, como esquema preexistente sem histórico; não substitui migrações na preparação normal.

## 3. Organização e reprodutibilidade

- Adotar módulo consumidor de integração separado, proposto em `integration/`, com import público `github.com/mineot/krios` e `replace` local para a revisão em teste. Os drivers ficam nesse módulo; a biblioteca de produção continua sem imports automáticos de drivers.
- Manter `go.mod`/`go.sum` próprios, versões fixadas dos drivers e imagens de servidor versionadas (digest quando adotado). Registrar a versão efetivamente usada dos servidores e do SQLite incorporado. Escolher e conferir versões compatíveis em E10-P01; não usar `latest` variável em cada execução.
- Compartilhar cenários e asserções de contrato; concentrar diferenças de configuração, arquivos SQL e capacidades em adaptadores/fixtures por banco. Evitar três cópias independentes da mesma suíte.
- Usar SQLite real via `modernc.org/sqlite`, MySQL via `go-sql-driver/mysql` e PostgreSQL via `pgx/stdlib`, nas versões fixadas. MySQL deve usar tabelas transacionais nos cenários que dependam de rollback.
- Isolar banco/esquema/arquivo por teste ou grupo explicitamente sequencial. Fixar timezone, collation e outras condições necessárias às expectativas, registrando as diferenças relevantes.
- Registrar limpeza imediatamente após cada criação; usar contexto próprio com prazo para limpeza após cancelamentos. Não usar `dev.db` nem conexões apontadas a bancos de aplicação existentes.
- Configuração por ambiente pertence ao harness de testes. Os exemplos continuam preenchendo `krios.Config` em Go; a biblioteca não passa a exigir variáveis de ambiente.
- A execução normal da raiz pode continuar sem servidores. O comando explícito de integração deve falhar se um banco solicitado estiver indisponível; no CI obrigatório, ausência de configuração/serviço nunca deve virar `Skip` com sucesso.
- Como `go test ./...` na raiz não atravessa módulos aninhados, Makefile e workflow devem chamar explicitamente o módulo de integração. Verificar isso em E10-P07.

## 4. Matriz de cobertura exigida

A [matriz C05-P03](C05-P03_traceability.md) fornece contratos, implementação, testes locais e pendências por RF/critério/API. E10-P02 deve vinculá-los aos IDs dos cenários reais; a evidência local não substitui os resultados de E10.

“Nos três bancos” significa executar o contrato aplicável em cada um. Recursos sem suporte devem ter testes de rejeição antes de efeitos, com o erro esperado. Não se exige SQL idêntico nem capacidades inexistentes em todos os motores.

| Área                     | Cenários mínimos contra banco real                                                                                                                                                  | Diferenças e verificações                                                                                                                                                           |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Configuração e conexão   | Abertura/fechamento, configuração em código, driver real, contexto cancelado, timeout, recurso fechado, reabertura e uso sem migrações                                              | Credenciais/identificadores especiais e portas nos servidores; SQLite temporário; não imprimir segredos nos erros de ambiente.                                                      |
| Migrações normais        | Banco vazio, múltiplas versões/arquivos, alvo já alcançado, `0 → 1 → 0 → 1`, `0 → 2 → 1 → 2`, retorno a zero e versões não contíguas                                                | Conferir estrutura, dados, estado efetivo e histórico após reabertura; consumir o contrato decidido em C01/C02.                                                                     |
| Migrações inválidas      | Alvo ausente/negativo/não declarado, arquivos ausentes/vazios, direção necessária ausente, configurações inconsistentes e objeto gerenciado sem histórico                           | Erro antes de efeitos; tabelas homônimas em outros bancos/esquemas não devem alterar a decisão local.                                                                               |
| Migrações e recuperação  | Falha no segundo arquivo e na segunda versão; estado parcial, bloqueio e recuperação explícita; repetição após recuperação                                                          | SQLite/PostgreSQL: rollback no escopo contratado. MySQL: observar efeitos de DDL sem prometer rollback; testar sequência de arquivos mesmo sem suporte do driver a multi-statement. |
| Entidades e leitura      | Tags, ordem, campos ignorados, `TableName`, cache/isolamento de registros, `One` posicional, `All` por nome, seleção parcial, aliases, `NULL`, `sql.Scanner` e conversões inválidas | Usar tipos reais retornados por cada driver, inclusive valores temporais e numéricos representativos; não pressupor igualdade de representação interna.                             |
| Validação e formatação   | Interfaces por campo, receptores por valor/ponteiro, nil, primeiro erro, `ValidateAll`, `Format`, validação automática na escrita                                                   | Valor formatado não altera o valor persistido; falha de validação não pode produzir escrita.                                                                                        |
| Consultas                | Seleção padrão/`Only`/`Fields`, comparadores, coluna × coluna, `And`/`Or`, `IN` vazio, nulidade, `LIKE` literal, `INNER`/`LEFT JOIN`                                                | Escape de `%`, `_`, `!`; collation registrada; entidade/coluna inválida rejeitada.                                                                                                  |
| Agregação e paginação    | `COUNT`/`SUM`/`MAX`/`MIN`, aliases, grupos, `DISTINCT`, ordenação, limites, offset, total, páginas vazias e fora do intervalo                                                       | `SUM` vazio com NULL, parâmetros na ordem correta, contagem preservando grupos/distintos, ordenação determinística e consistência conforme isolamento.                              |
| Escritas                 | Insert, Update e Delete, zero/false, nil, tipos anuláveis, `driver.Valuer`, `Omit`, default do banco, PK gerada, múltiplas linhas afetadas                                          | Conferir valores realmente persistidos; `RowsAffected`/`LastInsertId` conforme suporte; rejeitar Update/Delete sem filtros.                                                         |
| Retorno de escritas      | `Returning().One/All`, valores gerados/alterados/removidos, destino inválido e modos incompatíveis                                                                                  | SQLite/PostgreSQL: conferir linhas. MySQL: conferir rejeição e ausência de alteração; não pular silenciosamente o cenário.                                                          |
| Transações               | Commit, rollback, grupo de operações, independência do DB, primeiro erro imediato/tardio, `ErrNoRows`, finalização repetida, contexto cancelado                                     | Confirmar dados por outra conexão; coordenar concorrência por sinais/etapas, sem sleeps usados como garantia; respeitar o isolamento do motor.                                      |
| SQL direto e incorporado | Exec/Query/QueryRow/One/All, `go:embed`, argumentos, leitura/escrita dentro e fora da Tx, arquivo inválido, SQL inválido                                                            | Fixtures por dialeto; não traduzir placeholders; um statement por arquivo no fluxo portátil; multi-statement em cenário específico documentado.                                     |
| Recursos e erros         | Consumo parcial, fechamento, destino inválido, scan/iteração com falha, cancelamento e reutilização posterior do acesso                                                             | Observar liberação de recursos quando viável no driver real e manter contadores simulados para caminhos não injetáveis de forma determinística.                                     |
| Consumidor completo      | Criar usuários/tarefas por migrações, CRUD, consultas relacionadas, agregados, paginação, transação confirmada e revertida, SQL incorporado, down/up                                | Compilar e executar um consumidor a partir da API pública em cada banco; conferir saída/estado esperado.                                                                            |

Inventariar toda a API pública em E10-P02: cada operação/comportamento deve apontar para um cenário existente ou novo. Métodos puramente locais podem manter evidência unitária compartilhada, desde que os fluxos de integração exercitem seu uso. Percentual de cobertura não substitui a matriz nem prova que “todo código possível” foi executado.

## 5. Passos de implementação

- [ ] **E10-P01 — Fixar ambientes e comandos de execução.** Escolher versões de drivers/servidores compatíveis com as linhas Go suportadas; documentar imagens, provisionamento, readiness, autenticação de teste, isolamento, limites de tempo e limpeza. Definir comandos para um banco e para a matriz completa, reproduzíveis localmente e no CI.
- [ ] **E10-P02 — Construir o harness e a matriz rastreável.** Criar o módulo separado, adaptadores por banco e registro de cenários com IDs estáveis. Mapear RF-01–RF-09, API pública e cenários V01–V23/M01–M07, classificando cada caso como automatizado real, automatizado simulado, manual residual ou inaplicável com justificativa. Evitar skips silenciosos nos casos obrigatórios.
- [ ] **E10-P03 — Criar esquema e validar ciclos por migrações.** Implementar fixtures incorporadas por dialeto e executar o fluxo de criação, avanço, down, reaplicação, reabertura e alvo já alcançado. Acrescentar erros de configuração/histórico, estados parciais, falhas em diferentes arquivos/versões e recuperação. Conferir o estado real além do plano retornado.
- [ ] **E10-P04 — Cobrir operações e resultados da API.** Executar a matriz de entidades, validação/formatação, consultas, agregações, paginação, escritas, SQL direto e SQL incorporado nos três bancos. Testar capacidades específicas e rejeições suportadas; afirmar ausência de efeitos quando o contrato exige erro anterior à execução.
- [ ] **E10-P05 — Cobrir transações, recursos e falhas.** Exercitar commit/rollback, grupos, operações externas, erros tardios, contextos e concorrência determinística. Reproduzir as regressões de C02–C04 nos bancos aplicáveis. Manter injeções simuladas para falhas raras; documentar as que exigirem infraestrutura adicional, como perda de conexão durante commit, sem declarar sucesso real por simulação.
- [ ] **E10-P06 — Executar o consumidor completo nos três bancos.** Adaptar o fluxo KriosTasks ou criar consumidor equivalente que use as migrações para formar o esquema e percorra as operações da biblioteca. Demonstrar recursos SQL incorporados no binário e ciclo de vida completo, incluindo reversão e nova criação. Asserções devem verificar dados, histórico e efeitos, não apenas ausência de erro.
- [ ] **E10-P07 — Integrar GitHub Actions e execução local.** Criar jobs de integração separados por banco, usando serviços MySQL/PostgreSQL e SQLite temporário, com matriz nas linhas Go 1.26/1.27 previstas pelo projeto. Fixar versões, checar readiness, executar testes sem cache (`-count=1`), impor timeout e registrar diagnóstico de falhas. Preservar check/race existentes e a condição de execução em repositório público; adicionar comandos explícitos do módulo aninhado ao Makefile. Registrar versões/revisão e resumo de cenários nos logs, preservando a política atual de não publicar artefatos persistentes.
- [ ] **E10-P08 — Corrigir divergências e registrar evidências.** Corrigir incompatibilidades encontradas com regressões, executar novamente os cenários afetados e a matriz consolidada. Atualizar decisões, manual e rastreabilidade. Relacionar os resultados remotos à revisão testada e transferir para E09 somente a conferência final, distribuição e casos manuais residuais justificados.

Ordem: P01 → P02 → P03 → P04 → P05 → P06 → P07 → P08. A infraestrutura dos jobs pode ser preparada junto ao harness, mas nenhuma entrega é concluída somente por existir configuração YAML.

## 6. Critérios de conclusão

- Os três bancos reais executam o fluxo de esquema criado por migrações até reversão/reaplicação, com dados e histórico verificados.
- Cada área da matriz tem evidência identificável; recursos não suportados têm rejeições testadas. Cenários obrigatórios ausentes impedem conclusão.
- A suíte pode ser executada localmente por comando documentado e roda automaticamente em pushes/PRs no CI habilitado, nas seis combinações de banco e linha Go previstas.
- Falha de inicialização de serviço, ausência de testes ou execução de zero cenários obrigatórios falha o job. Não basta o workflow ficar verde por skips.
- As regressões existentes, check e race continuam passando; a integração não adiciona imports de drivers à compilação de produção.
- Cada combinação registra revisão, Go, driver, motor, cenários e resultado. Execução simulada/local não substitui evidência real remota.
- Casos excepcionais ainda manuais têm motivo, procedimento e requisito associado. Se um caso residual bloquear um requisito obrigatório, ele continua impedindo a conclusão da validação correspondente em E09.
- A E09 usa os resultados para sua conferência final e não publica antes da conclusão da E10 e de suas próprias verificações.

## 7. Registro de progresso

| Passo   | Estado   | Evidência |
| ------- | -------- | --------- |
| E10-P01 | Pendente | —         |
| E10-P02 | Pendente | —         |
| E10-P03 | Pendente | —         |
| E10-P04 | Pendente | —         |
| E10-P05 | Pendente | —         |
| E10-P06 | Pendente | —         |
| E10-P07 | Pendente | —         |
| E10-P08 | Pendente | —         |

Este documento planeja testes e automação. Não instala drivers, inicia servidores, altera o workflow nem comprova compatibilidade com bancos reais.
