# Instruções do projeto

## Planejamento e execução

- Seguir o plano em [.spec/PLANNING.md](.spec/PLANNING.md), que ainda será finalizado pelo utilizador.
- Executar o plano por passos, um de cada vez, somente após autorização explícita do utilizador.
- Antes de implementar cada passo, registrar seu planejamento em `PLANNING.md`: escopo, alterações previstas, validações e dúvidas pendentes.
- Submeter esse planejamento ao utilizador e aguardar autorização antes de implementar.
- A IA não deve tomar decisões por conta própria, preencher lacunas com suposições ou escolher alternativas sem a decisão do utilizador. Executar somente o que já estiver definido e autorizado.
- Registrar todas as dúvidas em [.spec/QUESTIONS.md](.spec/QUESTIONS.md), informar o utilizador e interromper a implementação até receber suas respostas e autorização explícita para continuar.
- Respostas às dúvidas não equivalem a autorização para implementar. Silêncio ou tempo decorrido também não constituem autorização.
- Se surgirem dúvidas durante um passo, registrar as novas questões e pausar novamente. Atualizar o planejamento conforme as respostas antes de retomar a implementação autorizada.
- Enquanto houver questões pendentes, preservar as perguntas, respostas e decisões do utilizador; não responder em seu nome nem sobrescrever suas respostas.
- Quando todas as questões estiverem resolvidas, registrar as decisões no planejamento do passo correspondente e limpar todo o conteúdo de `QUESTIONS.md`, mantendo o arquivo. Não limpar enquanto houver qualquer questão pendente. A limpeza não equivale a autorização para continuar a implementação.
- Ao concluir um passo, registrar o resultado e as validações em `PLANNING.md`. Não iniciar automaticamente o passo seguinte.

## Documentação, testes e distribuição

- Manter um passo exclusivo para criação dos testes unitários.
- O conteúdo de [README.md](README.md) será definido pelo utilizador no passo específico de documentação. Ignorar esse arquivo até lá; não planejar seu conteúdo nem alterá-lo agora.
- A preparação para distribuição como biblioteca Go pelo GitHub terá um passo próprio. Decisões sobre módulo, repositório, licença e versionamento devem ser submetidas ao utilizador.
- Preparar a distribuição não autoriza publicar, criar tags ou releases; essas ações dependem de autorização explícita.
