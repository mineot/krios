# Dúvidas — passo 007

Revisar [007.md](007.md), responder abaixo e autorizar explicitamente a implementação. Preservar respostas até resolver todas as questões e registrá-las no plano, conforme [AGENTS.md](../AGENTS.md).

## 007-Q01 — Escopo de Build

- **Contexto:** o passo 7 menciona `Build`, mas SELECT começa no passo 8 e os builders de escrita no passo 10; não há builders para receber esse método agora.
- **Pergunta:** aprova entregar neste passo somente o mecanismo interno de compilação de fragmentos, deixando `Build` público junto a cada builder nos passos 8 a 10, sem criar builders provisórios ou métodos públicos de compilação nas expressões?
- **Resposta do utilizador:**
- **Situação:** pendente.

## 007-Q02 — Organização e acesso à AST

- **Contexto:** os campos da AST são privados; novos métodos exportados nos tipos internos também apareceriam nos aliases públicos da raiz.
- **Pergunta:** aprova a organização de `007.md`: escape/placeholders em `internal/dialect`, acumulador em `internal/compiler` e funções de renderização em `internal/expression`, com dependência `expression → compiler → dialect`, sem expor nós/getters ou criar ciclo de imports? Cada compilação terá seu próprio writer, compartilhado entre os fragmentos dessa compilação.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 007-Q03 — Formato do SQL gerado

- **Contexto:** precisamos fixar a saída para que seja previsível e validável posteriormente.
- **Pergunta:** aprova o formato de `007.md`: nomes sempre delimitados por componente (aspas duplas em PostgreSQL/SQLite, crases em MySQL, duplicando delimitadores internos), `AS` explícito, palavras-chave maiúsculas, grupos entre parênteses, espaços simples e sem `;` final? Listas vazias gerarão `1 = 0`/`1 = 1`, sem eliminar outros ramos; `Where/On` gerarão somente o grupo, deixando a palavra da cláusula para o builder.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 007-Q04 — Argumentos e composição

- **Contexto:** parâmetros precisam acompanhar a ordem do SQL, inclusive quando vários fragmentos usam o mesmo acumulador.
- **Pergunta:** aprova um argumento por ocorrência de valor, sem deduplicação/conversão, numeração contínua no mesmo writer e reinício em cada nova compilação? Sem parâmetros, retornar `args == nil`; com parâmetros, devolver cópia superficial do slice, mantendo referências dos valores e sem limite próprio de quantidade de parâmetros.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 007-Q05 — Snapshot e capacidades

- **Contexto:** `dialect.Info` já guarda versão e capacidades detectadas na inicialização.
- **Pergunta:** aprova validar dialeto/versão mínima ao criar o writer (`ErrInvalidConfig`/`ErrUnsupported`), consumir as capacidades desse snapshot sem nova detecção e exigir `ExplicitNullOrder` para `NullsFirst/NullsLast`, sem emulação? `FullJoin` e `Returning` serão verificados somente nos passos que implementarem essas cláusulas.
- **Resposta do utilizador:**
- **Situação:** pendente.

## 007-Q06 — Prioridade dos erros e resultado parcial

- **Contexto:** uma falha pode ocorrer depois de outros fragmentos já terem acrescentado SQL/argumentos.
- **Pergunta:** aprova validar configuração na criação, estrutura completa de cada fragmento antes de sua capacidade, preservar o primeiro erro e impedir novas emissões após falha? O resultado final será sempre `"", nil, err` em erro, sem prefixos de SQL/argumentos, exigindo novo writer para tentar novamente.
- **Resposta do utilizador:**
- **Situação:** pendente.
