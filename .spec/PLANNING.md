# Plano de implementação — esboço

- **Base do plano:** seguir [DEFINITIONS.md](DEFINITIONS.md) e [MANUAL.md](MANUAL.md); detalhar as etapas após as considerações sobre este esboço.
- **Execução:** plano ainda não finalizado. Planejar cada passo antes de implementar, registrar dúvidas em [QUESTIONS.md](QUESTIONS.md) e aguardar respostas e autorização explícita do utilizador, conforme [AGENTS.md](../AGENTS.md). Nenhum passo está autorizado por este esboço.
- **1. Consolidar os contratos:** revisar assinaturas, retornos, erros e limites de suporte por banco/versão. Consolidação documental concluída em 30/10/2026; planejamento, decisões e respostas em [001.md](001.md).
- **2. Estruturar o projeto:** definir módulo Go, organização dos pacotes e separação dos drivers usados nos testes de integração. Estrutura inicial concluída em 30/10/2026; decisões e validações em [002.md](002.md).
- **3. Implementar conexão:** configuração, `Open`, `Wrap`, `Close`, propriedade do pool e timeout de abertura. Implementação concluída em 30/10/2026; decisões, resultados e limites da validação em [003.md](003.md).
- **4. Implementar transações:** `Begin`, `Commit`, `Rollback`, contextos e ciclo de vida de `DB`/`Tx`. Implementação concluída em 01/11/2026; decisões, resultados e validações em [004.md](004.md).
- **5. Implementar SQL direto e arquivos:** `Query`, `Exec`, `QueryFile` e `ExecFile`, com parâmetros e `fs.FS`/`embed.FS`. Implementação concluída em 01/11/2026; decisões, resultados e validações em [005.md](005.md).
- **6. Implementar expressões:** tabelas, colunas, `As`, valores, operadores, agrupamentos e validações da AST. Implementação concluída em 01/11/2026; decisões, resultados e validações em [006.md](006.md).
- **7. Implementar dialetos e compilação:** placeholders, escape de identificadores, capacidades e base de `Build` nos três bancos. Compilação interna concluída em 01/11/2026; decisões e validações em [007.md](007.md). `Build` público reservado aos builders.
- **8. Implementar SELECT básico:** projeção, aliases, schemas, `Distinct`, WHERE, ordenação, `Build` e execução com `sql.Rows`. Planejamento em [008.md](008.md), aguardando decisões e autorização para implementar.
- **9. Implementar SELECT avançado:** JOINs, subconsultas, `Exists`, agregações, `GroupBy`, `Having`, paginação, contagem, `First` e `Last`.
- **10. Implementar escritas:** INSERT, UPDATE, DELETE, proteção de WHERE, `sql.Result` e `Returning` conforme suporte.
- **11. Implementar migrações:** manifesto, arquivos, versões, checksums, histórico, Up/Down, exclusividade, modo atômico e controle de estado `dirty` no modo não atômico.
- **12. Reorganizar os arquivos da raiz:** aguardar as orientações do utilizador no momento de executar este passo. A estrutura e as alterações serão definidas e planejadas nessa etapa, antes da implementação.
- **13. Validar a integração:** testar cada etapa e consolidar cenários nos três bancos, incluindo cancelamento, rollback, concorrência, falhas e `go test -race`.
- **14. Consolidar a documentação:** conferir os exemplos contra a implementação, atualizar o manual e registrar compatibilidade e limitações; README reservado ao passo específico.
- **15. Criar testes unitários:** passo exclusivo para planejar e criar a suíte unitária, incluindo configuração, expressões, compilação SQL, builders, validações e regras de migração.
- **16. Documentar o README:** aguardar a definição do conteúdo pelo utilizador nesse passo. Ignorar o arquivo e seu conteúdo por enquanto.
- **17. Preparar a distribuição pelo GitHub:** planejar com o utilizador o módulo Go, repositório, licença, versionamento e preparação da biblioteca para consumo externo. Publicação, tags e releases dependem de autorização explícita.

## Planejamento do passo 1

- **Escopo:** consolidar documentalmente API pública, regras, resultados, erros e compatibilidade; detalhes em [001.md](001.md).
- **Alterações previstas:** consolidar `DEFINITIONS.md`, ajustar exemplos afetados em `MANUAL.md` e manter os registros em `001.md`, `QUESTIONS.md` e neste plano, após autorização.
- **Validações:** consistência de assinaturas e exemplos, regras de ciclo de vida, mapeamento de erros e matriz de suporte aprovada; sem implementação ou testes de código nesta etapa.
- **Dúvidas pendentes:** nenhuma; respostas de `001-Q01` a `001-Q10` registradas em `001.md` e `QUESTIONS.md` limpo.
- **Situação:** passo 1 concluído documentalmente em 30/10/2026, sob a autorização do utilizador para continuar quando as dúvidas fossem resolvidas.
- **Resultado:** definições e manual alinhados às respostas; catálogo de contratos, regras de erro e matriz de compatibilidade registrados.
- **Validações realizadas:** revisão documental, referências oficiais, links locais, blocos Markdown e verificação de diferenças. Nenhuma implementação ou suíte de testes executada.
- **Continuidade:** passo 2 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 2

- **Escopo:** criar a base compilável do módulo e pacote `krios`, definindo a organização e a separação dos drivers de integração; detalhes em [002.md](002.md).
- **Alterações previstas:** criar `go.mod` e `doc.go`, ajustar a referência de importação em `MANUAL.md` e manter o planejamento e as questões, após aprovação.
- **Validações:** formatação Go, identificação do módulo/pacote, compilação, análise estática, dependências e diferenças de texto; sem criação de testes.
- **Dúvidas pendentes:** nenhuma; respostas registradas em `002.md`.
- **Situação:** passo 2 concluído em 30/10/2026, após autorização do utilizador.
- **Resultado:** `go.mod` e `doc.go` criados; módulo `github.com/mineot/krios`, Go 1.26+, pacote público `krios` e importação atualizada no manual. Pacotes `internal/` serão definidos posteriormente; módulo de integração reservado ao passo 13.
- **Validações realizadas:** `gofmt`, resolução do módulo/pacote, ausência de dependências externas, `go build ./...`, `go vet ./...` e `git diff --check`, com sucesso em Go 1.26.8. Nenhum teste criado ou executado.
- **Continuidade:** passo 3 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 3

- **Escopo:** configuração, conexão própria/externa, detecção automática de versão/capacidades, timeout e fechamento; detalhes em [003.md](003.md).
- **Alterações previstas:** wrappers públicos, pacotes internos aprovados para conexão/dialeto/erros, documentação afetada e registros do plano.
- **Validações:** formatação, compilação, análise estática, dependências, revisão de ciclo de vida e consistência documental. Testes reais de conexão e transações permanecem nas etapas correspondentes.
- **Dúvidas pendentes:** nenhuma; decisões registradas em `003.md`.
- **Situação:** passo 3 concluído em 30/10/2026, após aprovação das respostas e autorização explícita.
- **Resultado:** configuração, wrappers de conexão, detecção de versão/capacidades, propriedade do pool, fechamento sincronizado e erros implementados nos pacotes aprovados; manual e definições atualizados.
- **Validações realizadas:** formatação, listagem de pacotes/dependências, `go build ./...`, `go vet ./...`, revisão de código/documentação e `git diff --check`, com sucesso em Go 1.26.8.
- **Limites:** sem testes ou conexão com bancos reais; contagem de transações será integrada no passo 4. Questões resolvidas, respostas registradas em `003.md` e `QUESTIONS.md` limpo.
- **Continuidade:** passo 4 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 4

- **Escopo:** `DB.Begin`, `Tx.Commit`, `Tx.Rollback`, contexto e integração da contagem de transações com `DB.Close`; detalhes em [004.md](004.md).
- **Alterações previstas:** wrappers públicos, pacote interno de transações e integração com conexão/contador, além das definições, manual e registros do plano.
- **Validações:** formatação, compilação, análise estática, dependências e revisão dos caminhos de abertura, finalização e cancelamento. Testes de execução e concorrência permanecem nas etapas próprias.
- **Dúvidas pendentes:** nenhuma; decisões e respostas preservadas em `004.md`; `QUESTIONS.md` limpo.
- **Situação:** passo 4 concluído em 01/11/2026, após aprovação das respostas e autorização explícita.
- **Resultado:** wrappers `DB.Begin`, `Tx.Commit` e `Tx.Rollback`, pacote interno de transações, reserva/liberação única do contador, cancelamento automático e preservação dos erros implementados; definições e manual atualizados.
- **Validações realizadas:** formatação, listagem de pacotes/módulos, `go build ./...`, `go vet ./...`, revisão dos caminhos de finalização/locks e `git diff --check`, com sucesso em Go 1.26.8. Sem dependências externas.
- **Limites:** sem testes ou bancos reais nesta etapa. `sql.ErrTxDone` informa encerramento lógico nativo, sem garantir limpeza física concluída no driver; detalhes em `004.md`. Comportamento real e concorrência serão verificados nas etapas de testes.
- **Continuidade:** passo 5 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 5

- **Escopo:** SQL parametrizado via `Tx.Query/Exec` e arquivos via `Tx.QueryFile/ExecFile`, usando `fs.FS`/`embed.FS`; detalhes em [005.md](005.md).
- **Alterações previstas:** wrappers públicos, execução/leitura/validações em `internal/transaction`, exposição de `ErrInvalidQuery`, definições, manual e registros do plano, após aprovação.
- **Validações:** formatação, pacotes/módulos, compilação, análise estática e revisão de erros, arquivos, cancelamento, finalização e propriedade de `Rows`. Testes de execução/concorrência permanecem nas etapas próprias.
- **Dúvidas pendentes:** nenhuma; decisões e respostas preservadas em `005.md`; `QUESTIONS.md` limpo.
- **Situação:** passo 5 concluído em 01/11/2026, após aprovação das respostas e autorização explícita.
- **Resultado:** quatro métodos públicos e internos, `ErrInvalidQuery`, validações e leitura síncrona de arquivos implementados; SQL/argumentos e causas preservados; definições e manual atualizados.
- **Validações realizadas:** formatação dos arquivos do passo, listagem de pacotes/módulos, `go build ./...`, `go vet ./...`, revisão de código/documentação e `git diff --check`, com sucesso em Go 1.26.8; sem dependências externas.
- **Observação de formatação:** `gofmt -l .` apontou espaços preexistentes em linhas vazias de `begin.go` e `internal/connection/begin.go`; arquivos preservados, fora do escopo do passo.
- **Limites:** sem testes ou bancos reais. Leitura de `fs.FS` é síncrona e não pode ser interrompida pelo contexto; execução e concorrência serão verificadas nas etapas de testes.
- **Continuidade:** passo 6 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 6

- **Escopo:** AST de tabelas, expressões, predicados e termos de ordenação, modificadores imutáveis, validação estrutural/contextual e análise de constantes; detalhes e delimitação em [006.md](006.md).
- **Alterações previstas:** wrappers/aliases públicos e pacote `internal/expression`, definições, manual e registros do plano, após aprovação. Sem geração ou execução de SQL nesta etapa.
- **Validações:** formatação, pacotes/módulos, compilação, análise estática e revisão de contratos, imutabilidade, identificadores, grupos, aliases, curingas e constantes. Testes permanecem nas etapas próprias.
- **Dúvidas pendentes:** nenhuma; decisões e respostas preservadas em `006.md`; `QUESTIONS.md` limpo.
- **Situação:** passo 6 concluído em 01/11/2026, após aprovação das respostas e autorização explícita.
- **Resultado:** tipos opacos, wrappers, construtores/modificadores imutáveis, validação interna e análise de constantes implementados; definições e manual atualizados. Sem compilação SQL ou builders nesta etapa.
- **Validações realizadas:** formatação dos arquivos do passo, pacotes/módulos, `go build ./...`, `go vet ./...`, inspeção da API por `go doc`, revisão de código/documentação e `git diff --check`, com sucesso em Go 1.26.8; sem dependências externas.
- **Observação de formatação:** a checagem global manteve os espaços preexistentes de `begin.go` e `internal/connection/begin.go`; arquivos preservados.
- **Limites:** sem testes nesta etapa; validadores internos ainda não são chamados por uma API pública de compilação. Valores em `any` mantêm referências, conforme Q06. Execução/integração e suíte unitária permanecem nos passos 13 e 15.
- **Continuidade:** passo 7 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 7

- **Escopo aprovado:** compilação interna dos nós existentes da AST, escape/placeholder por dialeto, composição de argumentos e capacidades; delimitação de `Build` em [007.md](007.md).
- **Alterações previstas:** funções de dialeto, pacote `internal/compiler`, renderizadores em `internal/expression`, definições, manual e registros do plano, após aprovação.
- **Validações:** formatação, pacotes/módulos, compilação, análise estática e revisão de escape, parâmetros, grupos, capacidades, erros e preservação da AST; testes nas etapas próprias.
- **Dúvidas pendentes:** nenhuma; decisões e respostas preservadas em `007.md`; `QUESTIONS.md` limpo.
- **Situação:** passo 7 concluído em 01/11/2026, após aprovação das respostas e autorização explícita.
- **Resultado:** escape/placeholders nos três dialetos, writer compartilhável entre fragmentos, renderizadores da AST, validação de capacidades e resultado sem conteúdo parcial em erro; definições e manual atualizados.
- **Validações realizadas:** formatação dos arquivos do passo, pacotes/módulos, `go build ./...`, `go vet ./...`, inspeção da API por `go doc`, revisão de código/documentação e `git diff --check`, com sucesso em Go 1.26.8; sem dependências externas.
- **Observação de formatação:** permanecem os espaços preexistentes de `begin.go` e `internal/connection/begin.go` apontados pela checagem global; arquivos preservados.
- **Limites:** sem testes ou execução em bancos. Compilação disponível somente internamente; `Build` público será entregue com os builders nos passos 8 a 10. Integração e suíte unitária permanecem nos passos 13 e 15.
- **Continuidade:** passo 8 registrado na seção seguinte; demais passos dependem de planejamento e autorização próprios.

## Planejamento do passo 8

- **Escopo:** `Tx.Select`, builder imutável, projeção/DISTINCT, WHERE, ordenação, `Build` público e `Exec` com `sql.Rows`; detalhes em [008.md](008.md).
- **Alterações previstas:** wrappers públicos, pacote `internal/selectquery`, integração interna com transação/AST/compilador, definições, manual e registros do plano, após aprovação.
- **Validações:** formatação, pacotes/módulos, compilação, análise estática e revisão de imutabilidade, SQL/argumentos, erros, capacidades, ciclos de vida e propriedade de linhas; testes nas etapas próprias.
- **Dúvidas pendentes:** `008-Q01` a `008-Q06`, em [QUESTIONS.md](QUESTIONS.md): organização, ciclo de vida de Build, chamadas vazias, composição SQL, erros/execução e agregações.
- **Situação:** planejamento apresentado para revisão; nenhuma implementação do passo 8 iniciada. Passos 9 a 17 não iniciados.
