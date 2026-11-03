> Suas respostas foram preservadas. Os textos marcados como **Sugestão IA** aguardam sua validação; ainda não alteram o contrato em `DEFINITIONS.AI.md`. Os esclarecimentos explicam os pontos que geraram dúvidas.

1. **Validação da configuração:** quando verificar conexão, dialeto e demais campos, e como retornar os erros, considerando que `krios.New(config)` atualmente retorna apenas o cliente. Validar antes da execução ou oferecer um método explícito de validação?

`krios.New(config)` deve retornar alem do cliente o erro, que deve ser validado antes da execução.

2. **Contexto ausente:** quando `Config.Context == nil`, retornar erro ou utilizar um contexto padrão?

Retornar um erro, até onde sei esse contexto pode ser criado de formas diferentes, correto? Então vamos deixar isso a cargo do utilizador

3. **Alterações na configuração:** copiar a configuração ao criar o cliente? Como impedir que mudanças posteriores afetem consultas já construídas?

**Sugestão IA:** copiar `Config` ao criar o cliente, sem setters posteriores. Manter as mesmas referências de `*sql.DB` e `context.Context`: alterar a struct original não muda o cliente, mas fechar o pool ou cancelar o contexto continua afetando suas operações.

**Minha Resposta:** Confordo, vamos manter assim.

4. **Compatibilidade:** quais versões mínimas de SQLite, PostgreSQL e MySQL serão suportadas e quais drivers serão homologados? Confirmar também se Go 1.26.0 será a versão mínima.

**Recuperado do branch `0002`:** PostgreSQL 16+, MySQL 8.0+, SQLite 3.39+ e Go 1.26+, com `go 1.26.0` no módulo. Fontes: `.spec/001.md`, resposta `001-Q01`, e `.spec/002.md`, resposta `002-Q02`.

O `.spec/MANUAL.md` desse branch exemplifica `github.com/jackc/pgx/v5/stdlib`, `github.com/go-sql-driver/mysql` e `modernc.org/sqlite`. Não encontrei homologação concluída nem versões fixadas desses drivers. **Sugestão IA:** usá-los na matriz inicial de testes e fixar suas versões nessa etapa; o utilizador continua fornecendo o pool.

**Minha Resposta:** Pode Ignorar tudo do branch 0002, essa aqui é uma nova proposta, use apenas as versões e nos testes.

5. **Extensão de dialetos:** novos bancos poderão ser implementados somente dentro do Krios ou também por terceiros?

Somete dentro do krios, não quero nada de terceiros, por enquanto.

6. **Recursos indisponíveis:** retornar erro para funcionalidades não suportadas pelo banco ou oferecer emulações específicas? A proposta atual é erro explícito.

Eu não quero emular nada, vamos retornar um erro

7. **Contrato do encadeamento:** definir as assinaturas e os tipos retornados por métodos como `From`, `Column`, `As`, `Table` e `OrderBy`. Formalizar se modificam o objeto existente ou retornam outro.

**Sugestão IA:** manter métodos que modificam o builder, preservando os exemplos atuais. `From` retorna uma referência pública de tabela; `Column`, uma referência de coluna; `OrderBy`, um item de ordenação; `As`, `Schema` e `Desc` retornam o próprio objeto correspondente. SELECT, UPDATE e DELETE usam `From`, e INSERT usa `Into`, conforme sua resposta 23. Os nomes e assinaturas completos ficam para as definições.

**Sugestão IA sobre repetição:** `From`, `Into`, `Schema` e `As` substituem; `Criteria` acumula com AND por padrão, aceitando grupos explícitos AND/OR. Operações de lista, como adicionar colunas, joins, ordenações e linhas de valores, acumulam; `Columns(...)` do INSERT substitui a lista completa.

**Minha Resposta:** Concordo

8. **Chamadas repetidas:** decidir quando uma nova chamada substitui, acumula ou provoca erro. Isso inclui métodos como `From`, `Schema`, `As` e `Criteria`.

O Critéria acumula, mas os demais substituem. A propósito, em criteria, se ele definir dois critérias, será composto, agregado, e pode aceitar uma composição usando krios.Or ou krios.And

9. **Reutilização e concorrência:** builders e critérios poderão ser compartilhados entre consultas ou goroutines? Os slices e argumentos recebidos serão copiados ou mantidos por referência?

Sim, inclusive penso no Krios pensado para isso.

**Sugestão IA para viabilizar esse uso:** proteger builders e critérios contra acesso concorrente e executar uma cópia consistente do estado de cada consulta. Copiar slices, bytes e critérios recebidos; disponibilizar `Clone()` para montar variações independentes. Uma sequência de chamadas de construção não é uma operação indivisível: goroutines que precisam de filtros ou páginas diferentes devem usar clones. Objetos personalizados recebidos por referência não podem ser modificados externamente durante o uso.

**Minha Resposta:** Concordo

10. **Aliases e referências:** definir tratamento de aliases repetidos, self-joins, alteração de alias após criar critérios e referências a tabelas pertencentes a outra consulta.

**Sugestão IA:** rejeitar aliases de tabela repetidos no mesmo escopo e exigir aliases diferentes nos self-joins. Vincular critérios à referência da tabela, para que acompanhem mudanças de alias. Referências de outra consulta exigem associação explícita à tabela de destino; referências externas de subconsultas seguem a regra do item 21. Substituir um `From` invalida referências antigas, em vez de redirecionar filtros silenciosamente.

**Minha Resposta:** Concordo

11. **Colunas e ordenação:** definir a combinação de colunas explícitas com `Column("*")`, nomes já qualificados e aliases de saída. Esclarecer como `OrderBy` referencia um alias da projeção.

**Sugestão IA:** a primeira projeção explícita remove o `*` automático. Permitir `Column("*")` junto de outras projeções, preservando a ordem; na referência de tabela, ele significa `alias.*`. Aceitar nomes qualificados somente quando correspondem a uma origem registrada. Para ordenar pelo alias de saída, oferecer `OrderByAlias("nome")`; rejeitar aliases explícitos de saída duplicados.

**Minha Resposta:** Concordo

12. **Geração sem execução:** disponibilizar uma operação para obter SQL, argumentos e erro sem acessar o banco?

**Esclarecimento:** seria possível chamar `sqlText, args, err := query.Build()` para ver o SQL e seus parâmetros sem executar nada no banco. Isso ajuda a depurar e testar. **Sugestão IA:** incluir `Build()`; `Exec` e `ExecTx` usam a mesma validação e geração internamente.

**Minha Resposta:** Concordo, pode criar.

13. **Erros de construção:** decidir quando uma consulta inválida é detectada e como o consumidor diferencia erros de configuração, construção, incompatibilidade e execução, preservando a causa do driver.

**Sugestão IA:** validar a configuração em `New`, a consulta em `Build`/`Exec`/`ExecTx` e só acessar o banco depois das validações locais. Oferecer categorias de erro reconhecíveis com `errors.Is`/`errors.As`, preservando o erro original do driver. Não usar panic para entradas inválidas nem repetir escritas automaticamente por iniciativa do Krios.

**Minha Resposta:** Concordo

14. **Assinaturas de retorno:** o documento ainda marca como proposta formal `(*sql.Rows, *krios.Page, error)` para SELECT e `(sql.Result, error)` para escritas sem linhas retornadas. Confirmar essas assinaturas.

Confirmado

15. **Paginação ausente e falhas:** confirmar `page == nil` quando não houver paginação. Aprovar a proposta de não retornar sucesso parcial quando a seleção ou sua contagem falhar.

Se não houver paginação definida, deve retornal nil, porem se falhar, e estourar erro deve ser retornado nil, e o erro que ocorreu

**Sugestão IA:** sem paginação e com sucesso, retornar `(rows, nil, nil)`; com falha na preparação, contagem ou abertura da seleção, retornar `(nil, nil, err)` e fechar recursos intermediários. Falhas posteriores, durante a leitura de `rows`, continuam sendo verificadas pela aplicação em `Scan` e `rows.Err()`. [Referência: database/sql](https://pkg.go.dev/database/sql#Rows).

**Minha Resposta:** Concordo

16. **Limites do query builder:** delimitar o suporte inicial a SQL livre, CTEs, unions, funções de janela, upserts e demais recursos não exemplificados.

**Esclarecimento:** a dúvida é se a primeira versão também deve montar consultas com `WITH` (CTEs), juntar resultados com `UNION`, calcular funções com `OVER` ou inserir/atualizar em um único comando (upsert). **Sugestão IA:** começar com os recursos já exemplificados e as subconsultas do item 21; deixar esses recursos e SQL livre no builder para depois. Scripts SQL das migrations continuam permitidos.

**Minha Resposta:** Concordo

17. **Agregadores e agrupamentos:** definir entradas válidas para cada agregador, suporte a `COUNT(DISTINCT ...)` e `HAVING`, e até onde o Krios validará a compatibilidade entre projeções e `GROUP BY`.

**Sugestão IA:** aceitar `Count("*")` e agregadores sobre uma coluna válida; rejeitar `*` nos demais agregadores. Deixar `COUNT(DISTINCT ...)` e `HAVING` para uma próxima etapa. Quando houver agrupamento ou agregação, exigir que cada projeção não agregada esteja no `GroupBy`, sem tentar inferir dependências entre colunas pelo schema.

**Minha Resposta:** Concordo

18. **Tipos dos valores:** definir suporte a booleanos, bytes, ponteiros, nulos e tipos com `driver.Valuer`, além de limites numéricos, precisão e fuso de datas.

O krios deve aceitar todos.

**Sugestão IA:** aceitar todos os tipos citados, respeitando a conversão de `database/sql` e as capacidades do driver/banco. Não converter números silenciosamente com perda de precisão, nem alterar o fuso de datas por iniciativa do Krios; valores não representáveis retornam erro. Nulos são permitidos em escritas, mas comparações continuam seguindo sua regra do item 20. Isso não significa aceitar qualquer struct ou mapa Go sem conversão. [Referência: conversão de parâmetros](https://pkg.go.dev/database/sql/driver#DefaultParameterConverter).

**Minha Resposta:** Concordo

19. **Listas de `IN` e `NOT IN`:** definir coleções aceitas, expansão em parâmetros, listas vazias ou nulas e tratamento de elementos nulos. A proposta de rejeitar listas vazias ainda não foi aprovada.

Deve rejeitar com erro

20. **Comparações com nulos:** `Eq("campo", nil)` deve gerar erro ou ser convertido para `IS NULL`? Definir as regras equivalentes para outros operadores.

Deve retornar erro, encorajando o uso do IsNull ou IsNotNull, até mesmo porque a comparação pode ter dado nil por um erro.

21. **Subconsultas:** definir o tipo aceito por `Exists` e `NotExists`, a composição de argumentos e como representar referências à consulta externa em subconsultas correlacionadas.

**Sugestão IA:** `Exists` e `NotExists` recebem um builder SELECT, copiado ao ser incorporado. A consulta externa reúne os parâmetros e ajusta os placeholders na ordem do SQL completo. A correlação usa referências explícitas a colunas da consulta externa, nunca strings interpretadas como SQL; exigir o mesmo dialeto e rejeitar referências fora desse vínculo.

**Minha Resposta:** Concordo

22. **Grupos lógicos:** definir o tratamento de grupos vazios, critérios nulos e grupos com apenas um elemento, sem descartar silenciosamente filtros inválidos.

Do meu ponto de vista devem dar erro

23. **Critérios de update e delete:** aprovar a proposta de `Table` retornar uma referência aceita por `NewCriteria` e definir seus tipos públicos.

Eu acho que houve um erro na minha definição, pois select usa from, e eu coloquei que update e delete usam table, acho que podemos normalizar tudo para ficar coerente, transformando tudo em from, similar ao do select, já o insert deve ser mantido como into para deixar a assinatura coerente.

24. **API dos joins:** definir o tipo padrão de `Join`, o contrato de `Ons`/`Equal`, o significado de `nil` como marcador de literal e a representação compartilhada de expressões entre `ON` e `WHERE`. Definir condições obrigatórias por tipo de join e quais joins adicionais entrarão no escopo.

**Sugestão IA:** `Join` significa INNER JOIN. Usar os mesmos operadores de critérios em `On`, distinguindo explicitamente referência de coluna e valor; substituir o desenho `Ons`/`Equal` com `nil` como marcador de literal. INNER, LEFT, RIGHT e FULL exigem condição válida; CROSS não aceita `On`. Incluir somente esses tipos inicialmente, retornando erro quando o banco não suportar; deixar `CrossApply` e outros para depois.

**Minha Resposta:** Concordo

25. **Base da paginação:** a primeira página será zero ou um? O exemplo atual usa um apenas como proposta.

Deve ser 1, se tiver 25 paginas, a primeira pagina é 1 e a ultima é 25

26. **Metadados da paginação:** definir o significado exato de `TotalRows` e `LastPage`. A proposta para `TotalRows` é contar as linhas do resultado antes da paginação, considerando joins, agrupamentos e `DISTINCT`.

**Sugestão IA:** `TotalRows` conta as linhas do resultado completo, antes da paginação, considerando joins, agrupamentos e `DISTINCT`. `TotalPages` é a divisão arredondada para cima por `RowsPerPage`; `LastPage` tem o mesmo valor de `TotalPages`, indicando a última página disponível. `CurrentPage` indica a página solicitada.

**Minha Resposta:** Concordo

27. **Limites da paginação:** definir resultado vazio, tamanho zero, entradas negativas, página além do fim, overflow e conversões de `uint64`.

**Sugestão IA:** página e tamanho devem ser positivos; rejeitar overflow e valores não representáveis pelo banco. Resultado vazio: `TotalRows`, `TotalPages` e `LastPage` iguais a zero, sem próxima/anterior; a solicitação da página 1 continua válida. Página além do fim retorna linhas vazias, preservando a página solicitada; não corrigir silenciosamente. Em resultado não vazio, `HasNextPage` compara com `LastPage` e `HasPreviousPage` indica `CurrentPage > 1`.

**Minha Resposta:** Concordo

28. **Consistência da paginação:** estabelecer as garantias entre contagem e leitura quando executadas pelo pool e documentar o efeito do isolamento da transação em `ExecTx`.

**Sugestão IA — item sem resposta:** contar antes de abrir as linhas da página. Pelo pool, contagem e leitura podem observar alterações concorrentes; em `ExecTx`, ambas usam a transação recebida, com as garantias do isolamento escolhido pela aplicação. Não prometer uma visão estável apenas por existir uma transação.

**Minha Resposta:** Concordo

29. **Navegação entre páginas:** `NextPage` e `PreviousPage` apenas alteram estado ou consultam o banco? Como funcionam antes da primeira execução ou depois de mudanças nos filtros e nos dados? Se consultarem o banco, falta definir como recebem a escolha da transação.

Precisamos dar um jeito de marcar que a query ja foi executada, e se for invocado o next ou previus, sem alterar nada, deve executar a query modificando apenas o offset.

**Sugestão IA:** registrar a revisão da consulta executada com sucesso. `NextPage()` e `PreviousPage()` executam pelo pool, retornam os mesmos valores de `Exec` e preservam filtros, ordenação e tamanho, mudando somente a página/offset. Exigir uma execução paginada prévia e nenhuma alteração posterior na construção; caso contrário, retornar erro e exigir novo `Exec`.

**Minha Resposta:** Concordo

Para transações, oferecer `NextPageTx(tx)` e `PreviousPageTx(tx)`, sem guardar `tx`. Atualizar a posição somente após sucesso; nos limites conhecidos, permanecer na página atual. Refazer a contagem em cada chamada, pois os dados podem mudar. A aplicação deve fechar as linhas anteriores; navegações independentes em goroutines usam clones, conforme o item 9.

**Minha Resposta:** Concordo

30. **Validação de inserts:** definir regras para tabela e colunas inválidas, colunas duplicadas, quantidade incorreta de valores, lote vazio e limites de parâmetros e tamanho.

**Sugestão IA:** exigir tabela e colunas válidas, sem colunas duplicadas, pelo menos uma linha e a mesma quantidade de valores e colunas em cada linha. Validar limites conhecidos de parâmetros e tamanho antes da execução; limites dependentes da configuração do servidor podem retornar erro pelo driver. Não descartar linhas ou valores inválidos.

**Minha Resposta:** Concordo

31. **Lotes grandes:** retornar erro ou dividir em vários comandos? Se houver divisão, definir atomicidade e como comunicar resultados parciais.

**Sugestão IA:** retornar erro se o lote exceder os limites, sem divisão automática. O utilizador pode dividir o lote e usar `ExecTx` em cada parte quando precisar que todas sejam confirmadas juntas.

**Minha Resposta:** Concordo

32. **Reutilização de inserts:** os valores permanecem após sucesso ou falha? Qual é o alcance exato de `ClearValues`? Como funcionam a retenção e a cópia dos argumentos?

Se chamado, deve remover todos os valores, se a query for executada depois desse clear, deve dar erro.

**Sugestão IA complementar:** manter os valores após sucesso ou falha até `ClearValues` ser chamado. Esse método limpa somente as linhas, preservando destino e colunas; os argumentos seguem as regras de cópia do item 9.

**Minha Resposta:** Concordo

33. **Valores especiais e resultados gerados:** definir suporte a `DEFAULT`, nulos, expressões e referências a colunas em escritas, além de eventual recuperação de dados gerados pelo banco.

**Sugestão IA:** aceitar `nil` como SQL NULL e valores parametrizados. Inicialmente, usar defaults omitindo a coluna no INSERT; deixar marcador `DEFAULT`, expressões, atribuições entre colunas e `RETURNING` para depois. Manter `sql.Result` para escritas, sem prometer IDs gerados quando o driver não os disponibilizar.

**Minha Resposta:** Concordo

34. **Updates e proteção de escritas:** definir update sem atribuições, chamadas repetidas de `Set` para a mesma coluna e o tratamento de filtros sempre verdadeiros. As regras para critérios vazios ou que se tornem universais precisam ser detalhadas para update e delete.

Sobre o `Set` se alterar o valor de um item que ja foi definido, deve ser substituido.

**Sugestão IA:** rejeitar UPDATE sem atribuições e UPDATE/DELETE sem filtro efetivo. Rejeitar critérios inválidos e condições constantes reconhecidas como sempre verdadeiras; não oferecer escrita irrestrita nesta primeira versão. Um filtro válido ainda pode atingir todas as linhas existentes, portanto essa proteção não garante seletividade.

**Minha Resposta:** Concordo

35. **Tipos e limites das migrations:** escolher os tipos numéricos de versão e ordem, limites permitidos, tratamento de nomes duplicados, lacunas entre versões, versão zero e alvo inexistente. Definir também como os cadastros retornam erros.

**Sugestão IA:** usar `int64` para versão, ordem e alvo. Versão e ordem começam em 1, permitem lacunas e não podem ultrapassar o limite de `int64`; alvo 0 significa desfazer todas as migrations aplicadas. Exigir nomes únicos no histórico e rejeitar alvo positivo não cadastrado. `Append` e `AppendFiles` retornam `error`; erros de cadastro também impedem a execução do conjunto, evitando executar uma lista incompleta.

**Minha Resposta:** Concordo

36. **Múltiplos conjuntos de migrations:** decidir se representam partes do mesmo histórico, módulos independentes ou alternativas por dialeto. Definir assinatura variádica, seleção de listas, dependências e namespaces.

**Sugestão IA:** `RunMigrations(target, conjuntos...)` e `RunMigrationsTx(tx, target, conjuntos...)` recebem partes de um único histórico, todas do dialeto do cliente. O utilizador escolhe as listas; o Krios une os cadastros, valida duplicatas e ordena por versão/ordem. Não criar módulos independentes, namespaces ou resolução automática de dependências nesta etapa; a ordem informada deve respeitar as dependências.

**Minha Resposta:** Concordo

37. **Scripts novos em versões já aplicadas:** permitir ou rejeitar a inclusão? Se permitir, como identificá-los e executá-los, principalmente quando versões posteriores já tiverem sido aplicadas?

**Esclarecimento:** imagine que a versão 1 já criou `users` no banco. Depois, alguém adiciona `create_roles` também na versão 1: o banco está marcado como versão 1, mas ainda não tem essa tabela. **Sugestão IA:** rejeitar novos scripts em versões já aplicadas; registrar a mudança em uma versão superior à maior já distribuída/aplicada. Vários scripts na mesma versão continuam permitidos antes da primeira aplicação.

**Minha Resposta:** Concordo

38. **Downgrade:** definir quando a descida pode ocorrer, se exige autorização/configuração explícita, como tratar migrations irreversíveis e o que fazer quando o banco estiver em versão superior à da aplicação.

Se a versão informada no momento da migração for inferior a do banco, deve-se fazer o downgrade, caso contrario, deve-se fazer o upgrade

**Sugestão IA:** seguir o alvo informado sem uma segunda autorização; alvo igual apenas valida o histórico e não executa scripts. Antes de qualquer DOWN, verificar se todos os passos necessários existem e são reversíveis; se faltar algum, retornar erro antes de iniciar. Um banco mais novo só pode descer quando a aplicação possui os scripts correspondentes; histórico desconhecido bloqueia a execução.

**Minha Resposta:** Concordo

39. **Concorrência das migrations:** escolher coordenação pelo banco ou garantia operacional de executor único. Definir locks, timeout, perda de conexão e manutenção da mesma sessão quando necessária.

A execução deve ser unica, não deve ter concorrencias

40. **Atomicidade das migrations:** definir políticas por banco e operação, incluindo scripts não transacionais. Aprovar ou ajustar a proposta de rejeitar previamente planos incompatíveis com uma transação recebida. Considerar que o retorno de `RunMigrationsTx` ocorre antes do commit feito pela aplicação.

**Sugestão IA:** em `RunMigrations`, usar transação por migration quando o banco e os comandos permitirem, incluindo o registro no histórico; migrations anteriores já concluídas permanecem aplicadas se uma posterior falhar. Fora dessas condições, executar com controle de falha parcial, sem prometer rollback. Scripts não devem controlar transações por conta própria.

Em `RunMigrationsTx`, usar somente a transação recebida e aceitar apenas planos cuja compatibilidade transacional esteja estabelecida; rejeitar incompatibilidade ou incerteza antes dos scripts. O sucesso depende do commit posterior da aplicação. Isso exclui, por exemplo, DDL do MySQL com commit implícito desse modo de execução. [Referência: MySQL](https://dev.mysql.com/doc/refman/8.0/en/implicit-commit.html).

**Minha Resposta:** Concordo

41. **Falhas parciais e recuperação:** definir estados de execução, registro de falhas e procedimento de reparo. Incluir o caso em que o SQL termina, mas a gravação do histórico falha.

**Sugestão IA:** registrar estado, direção e início antes de cada execução não transacional; marcar conclusão somente após o SQL e o histórico terminarem com sucesso. Estado pendente/falho bloqueia novas migrations até reparo manual, sem repetição automática. Se o SQL concluir e o histórico falhar, tratar como estado incerto. Em transações, o registro acompanha commit/rollback; não prometer preservar um registro de falha que também será desfeito, nem registrar fora da transação recebida.

**Minha Resposta:** Concordo

42. **Integridade do histórico:** usar checksum? Qual será a política para conteúdo alterado, migrations desconhecidas no banco e arquivos removidos?

**Sugestão IA:** guardar checksums SHA-256 dos conteúdos UP e DOWN e tornar imutáveis os cadastros já aplicados. Conteúdo alterado, migration desconhecida ou arquivo necessário removido bloqueia a execução com erro. Corrigir o banco por uma nova versão; não atualizar checksums automaticamente para esconder divergências. O checksum verifica scripts, não alterações manuais no banco.

**Minha Resposta:** Concordo

43. **Tabela `migrations`:** definir colunas, chaves, identificação dos registros e conjuntos, schema, evolução da estrutura e detecção de colisão com uma tabela existente da aplicação.

Eu acredito que deve retorar um erro, pois o banco nao ira permitir, correto?

**Esclarecimento:** criar uma tabela com nome já ocupado pode falhar, mas isso não identifica se a tabela existente pertence ao Krios. O próprio Krios precisa reconhecê-la antes de reutilizá-la.

**Sugestão IA:** usar `migrations` no schema/database padrão da conexão, com identificação do formato Krios e validação da estrutura. Registrar versão, ordem, nome, checksums UP/DOWN, estado, direção e horários; chave por `(versão, ordem)` e nome único. Tabela incompatível ou formato desconhecido gera erro; não alterar automaticamente uma tabela da aplicação. A evolução desse formato deve ter versionamento próprio.

**Minha Resposta:** Concordo

44. **Conteúdo dos scripts:** cada migration aceitará um comando, uma lista estruturada ou um script completo com vários comandos? Definir restrições de execução e requisitos dos drivers.

Pode aceitar varios commando de uma vez, eu posso criar mais de uma tabela em uma migração.

45. **Arquivos de migrations:** aprovar `fs.FS` como fonte; definir caminhos, momento da leitura, validação antecipada, arquivos ausentes e prioridade entre conteúdo inline e arquivo.

Aprovado

46. **Operação e diagnóstico das migrations:** definir comportamento em cancelamento e interrupção do processo, possibilidade de inspecionar o plano antes de executar, permissões exigidas e informações apresentadas nos erros.

**Sugestão IA:** respeitar `Config.Context`, parar no primeiro erro/cancelamento e não iniciar o próximo script. Ao reiniciar, verificar o histórico e bloquear estados incompletos conforme o item 41. Oferecer inspeção do plano que consulta o histórico sem executar scripts nem criar a tabela de controle. Erros identificam versão, ordem, nome, direção e etapa, preservando a causa; documentar as permissões para o histórico e os scripts, sem expor credenciais ou SQL sensível automaticamente.

**Minha Resposta:** Concordo

47. **Verificações durante o desenvolvimento:** permitir compilação e exemplos mínimos durante essa fase, mantendo a fase formal de testes depois? A ordem geral das fases já está definida.

**Sugestão IA:** permitir `gofmt`, compilação, `go vet` e exemplos mínimos durante o desenvolvimento. Manter a criação da suíte completa na fase de testes, preservando a ordem geral já escolhida.

**Minha Resposta:** Concordo

48. **Estratégia de testes e aceitação:** definir a matriz de ambientes, isolamento dos testes e critérios de aceitação de cada tarefa, incluindo entradas inválidas.

**Sugestão IA:** testar geração de SQL por dialeto, entradas inválidas e integração com os três bancos nos mínimos do item 4 e em versões adicionais explicitamente fixadas. Usar bancos/arquivos isolados por teste; cobrir transações, cancelamento, paginação, UP/DOWN, falhas parciais e concorrência dos builders com detector de races. Cada tarefa deve indicar exemplos aceitos, erros esperados e verificações necessárias para ser concluída.

**Minha Resposta:** Concordo

49. **Organização e publicação:** escolher licença, política de versões e tags, critérios de release e localização de manual, licença e CI conforme as restrições de `AGENTS.md`.

As escolhas já aprovadas — `Config.Context`, `client.Context()`, transação por execução, várias migrations por versão e ordenação de UP/DOWN — não precisam ser decididas novamente.

A licença será apache 2
o manual deve ficar no mesmo nivel do Readme, já que será chamado por ele

**Sugestão IA sobre CI:** GitHub Actions em `.github/workflows`, executando formatação, compilação, `go vet` e, quando disponíveis, testes unitários, detector de races e integrações. Usar PostgreSQL/MySQL em serviços isolados e SQLite em arquivo temporário, com versões fixadas. Releases exigem essas verificações aprovadas e documentação atualizada.

**Minha Resposta:** Concordo

**Sugestão IA sobre publicação:** começar com tags `v0.x.y` enquanto a API estiver em validação e publicar `v1.0.0` quando o contrato estiver estável, seguindo versionamento semântico.

**Minha Resposta:** Concordo

**Esclarecimento sobre AGENTS.md:** a regra atual proíbe novos arquivos ou pacotes na raiz e direciona novas criações para `internal`. Sua resposta já autoriza o manual ao lado do README. **Sugestão IA:** registrar também exceções explícitas para `LICENSE` na raiz, com Apache-2.0, e `.github/workflows` para CI; manter a implementação em `internal` e a API pública em `krios.go`. A revisão dessa regra será feita depois de você validar esta proposta.

**Minha Resposta:** Concordo
