# Plano de implementação — esboço

- **Base do plano:** seguir [DEFINITIONS.md](DEFINITIONS.md) e [MANUAL.md](MANUAL.md); detalhar as etapas após as considerações sobre este esboço.
- **Execução:** plano ainda não finalizado. Planejar cada passo antes de implementar, registrar dúvidas em [QUESTIONS.md](QUESTIONS.md) e aguardar respostas e autorização explícita do utilizador, conforme [AGENTS.md](../AGENTS.md). Nenhum passo está autorizado por este esboço.
- **1. Consolidar os contratos:** revisar assinaturas, retornos, erros e limites de suporte por banco/versão. Planejamento e decisões em [001.md](001.md); aguardando complementos às respostas e autorização para retomar.
- **2. Estruturar o projeto:** definir módulo Go, organização dos pacotes e separação dos drivers usados nos testes de integração.
- **3. Implementar conexão:** configuração, `Open`, `Wrap`, `Close`, propriedade do pool e timeout de abertura.
- **4. Implementar transações:** `Begin`, `Commit`, `Rollback`, contextos e ciclo de vida de `DB`/`Tx`.
- **5. Implementar SQL direto e arquivos:** `Query`, `Exec`, `QueryFile` e `ExecFile`, com parâmetros e `fs.FS`/`embed.FS`.
- **6. Implementar expressões:** tabelas, colunas, `As`, valores, operadores, agrupamentos e validações da AST.
- **7. Implementar dialetos e compilação:** placeholders, escape de identificadores, capacidades e `Build` nos três bancos.
- **8. Implementar SELECT básico:** projeção, aliases, schemas, `Distinct`, WHERE, ordenação e execução com `sql.Rows`.
- **9. Implementar SELECT avançado:** JOINs, subconsultas, `Exists`, agregações, `GroupBy`, `Having`, paginação, contagem, `First` e `Last`.
- **10. Implementar escritas:** INSERT, UPDATE, DELETE, proteção de WHERE, `sql.Result` e `Returning` conforme suporte.
- **11. Implementar migrações:** manifesto, arquivos, versões, checksums, histórico, Up/Down, exclusividade, modo atômico e controle de estado `dirty` no modo não atômico.
- **12. Validar a integração:** testar cada etapa e consolidar cenários nos três bancos, incluindo cancelamento, rollback, concorrência, falhas e `go test -race`.
- **13. Consolidar a documentação:** conferir os exemplos contra a implementação, atualizar o manual e registrar compatibilidade e limitações; README reservado ao passo específico.
- **14. Criar testes unitários:** passo exclusivo para planejar e criar a suíte unitária, incluindo configuração, expressões, compilação SQL, builders, validações e regras de migração.
- **15. Documentar o README:** aguardar a definição do conteúdo pelo utilizador nesse passo. Ignorar o arquivo e seu conteúdo por enquanto.
- **16. Preparar a distribuição pelo GitHub:** planejar com o utilizador o módulo Go, repositório, licença, versionamento e preparação da biblioteca para consumo externo. Publicação, tags e releases dependem de autorização explícita.

## Planejamento do passo 1

- **Escopo:** consolidar documentalmente API pública, regras, resultados, erros e compatibilidade; detalhes em [001.md](001.md).
- **Alterações previstas:** consolidar `DEFINITIONS.md`, ajustar exemplos afetados em `MANUAL.md` e manter os registros em `001.md`, `QUESTIONS.md` e neste plano, após autorização.
- **Validações:** consistência de assinaturas e exemplos, regras de ciclo de vida, mapeamento de erros e matriz de suporte aprovada; sem implementação ou testes de código nesta etapa.
- **Dúvidas pendentes:** `001-Q08` a `001-Q10`, complementando as respostas de `001-Q02`, `001-Q04` e `001-Q07`, em [QUESTIONS.md](QUESTIONS.md).
- **Situação:** respostas revisadas em 30/10/2026 e decisões registradas em `001.md`. Autorização para continuar condicionada à resolução das dúvidas; consolidação aguarda os complementos e autorização para retomar. Demais passos não iniciados.
