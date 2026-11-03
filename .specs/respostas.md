Li o [DEFINITIONS.AI.md](/home/mineot/Projetos/krios/.specs/DEFINITIONS.AI.md). Abaixo estão as decisões pendentes, agrupadas para evitar repetição. Incluí também o que ainda está marcado como **proposta não aprovada**. Não alterei o documento.

1. **Validação da configuração:** quando verificar conexão, dialeto e demais campos, e como retornar os erros, considerando que `krios.New(config)` atualmente retorna apenas o cliente. Validar antes da execução ou oferecer um método explícito de validação?

2. **Contexto ausente:** quando `Config.Context == nil`, retornar erro ou utilizar um contexto padrão?

3. **Alterações na configuração:** copiar a configuração ao criar o cliente? Como impedir que mudanças posteriores afetem consultas já construídas?

4. **Compatibilidade:** quais versões mínimas de SQLite, PostgreSQL e MySQL serão suportadas e quais drivers serão homologados? Confirmar também se Go 1.26.0 será a versão mínima.

5. **Extensão de dialetos:** novos bancos poderão ser implementados somente dentro do Krios ou também por terceiros?

6. **Recursos indisponíveis:** retornar erro para funcionalidades não suportadas pelo banco ou oferecer emulações específicas? A proposta atual é erro explícito.

7. **Contrato do encadeamento:** definir as assinaturas e os tipos retornados por métodos como `From`, `Column`, `As`, `Table` e `OrderBy`. Formalizar se modificam o objeto existente ou retornam outro.

8. **Chamadas repetidas:** decidir quando uma nova chamada substitui, acumula ou provoca erro. Isso inclui métodos como `From`, `Schema`, `As` e `Criteria`.

9. **Reutilização e concorrência:** builders e critérios poderão ser compartilhados entre consultas ou goroutines? Os slices e argumentos recebidos serão copiados ou mantidos por referência?

10. **Aliases e referências:** definir tratamento de aliases repetidos, self-joins, alteração de alias após criar critérios e referências a tabelas pertencentes a outra consulta.

11. **Colunas e ordenação:** definir a combinação de colunas explícitas com `Column("*")`, nomes já qualificados e aliases de saída. Esclarecer como `OrderBy` referencia um alias da projeção.

12. **Geração sem execução:** disponibilizar uma operação para obter SQL, argumentos e erro sem acessar o banco?

13. **Erros de construção:** decidir quando uma consulta inválida é detectada e como o consumidor diferencia erros de configuração, construção, incompatibilidade e execução, preservando a causa do driver.

14. **Assinaturas de retorno:** o documento ainda marca como proposta formal `(*sql.Rows, *krios.Page, error)` para SELECT e `(sql.Result, error)` para escritas sem linhas retornadas. Confirmar essas assinaturas.

15. **Paginação ausente e falhas:** confirmar `page == nil` quando não houver paginação. Aprovar a proposta de não retornar sucesso parcial quando a seleção ou sua contagem falhar.

16. **Limites do query builder:** delimitar o suporte inicial a SQL livre, CTEs, unions, funções de janela, upserts e demais recursos não exemplificados.

17. **Agregadores e agrupamentos:** definir entradas válidas para cada agregador, suporte a `COUNT(DISTINCT ...)` e `HAVING`, e até onde o Krios validará a compatibilidade entre projeções e `GROUP BY`.

18. **Tipos dos valores:** definir suporte a booleanos, bytes, ponteiros, nulos e tipos com `driver.Valuer`, além de limites numéricos, precisão e fuso de datas.

19. **Listas de `IN` e `NOT IN`:** definir coleções aceitas, expansão em parâmetros, listas vazias ou nulas e tratamento de elementos nulos. A proposta de rejeitar listas vazias ainda não foi aprovada.

20. **Comparações com nulos:** `Eq("campo", nil)` deve gerar erro ou ser convertido para `IS NULL`? Definir as regras equivalentes para outros operadores.

21. **Subconsultas:** definir o tipo aceito por `Exists` e `NotExists`, a composição de argumentos e como representar referências à consulta externa em subconsultas correlacionadas.

22. **Grupos lógicos:** definir o tratamento de grupos vazios, critérios nulos e grupos com apenas um elemento, sem descartar silenciosamente filtros inválidos.

23. **Critérios de update e delete:** aprovar a proposta de `Table` retornar uma referência aceita por `NewCriteria` e definir seus tipos públicos.

24. **API dos joins:** definir o tipo padrão de `Join`, o contrato de `Ons`/`Equal`, o significado de `nil` como marcador de literal e a representação compartilhada de expressões entre `ON` e `WHERE`. Definir condições obrigatórias por tipo de join e quais joins adicionais entrarão no escopo.

25. **Base da paginação:** a primeira página será zero ou um? O exemplo atual usa um apenas como proposta.

26. **Metadados da paginação:** definir o significado exato de `TotalRows` e `LastPage`. A proposta para `TotalRows` é contar as linhas do resultado antes da paginação, considerando joins, agrupamentos e `DISTINCT`.

27. **Limites da paginação:** definir resultado vazio, tamanho zero, entradas negativas, página além do fim, overflow e conversões de `uint64`.

28. **Consistência da paginação:** estabelecer as garantias entre contagem e leitura quando executadas pelo pool e documentar o efeito do isolamento da transação em `ExecTx`.

29. **Navegação entre páginas:** `NextPage` e `PreviousPage` apenas alteram estado ou consultam o banco? Como funcionam antes da primeira execução ou depois de mudanças nos filtros e nos dados? Se consultarem o banco, falta definir como recebem a escolha da transação.

30. **Validação de inserts:** definir regras para tabela e colunas inválidas, colunas duplicadas, quantidade incorreta de valores, lote vazio e limites de parâmetros e tamanho.

31. **Lotes grandes:** retornar erro ou dividir em vários comandos? Se houver divisão, definir atomicidade e como comunicar resultados parciais.

32. **Reutilização de inserts:** os valores permanecem após sucesso ou falha? Qual é o alcance exato de `ClearValues`? Como funcionam a retenção e a cópia dos argumentos?

33. **Valores especiais e resultados gerados:** definir suporte a `DEFAULT`, nulos, expressões e referências a colunas em escritas, além de eventual recuperação de dados gerados pelo banco.

34. **Updates e proteção de escritas:** definir update sem atribuições, chamadas repetidas de `Set` para a mesma coluna e o tratamento de filtros sempre verdadeiros. As regras para critérios vazios ou que se tornem universais precisam ser detalhadas para update e delete.

35. **Tipos e limites das migrations:** escolher os tipos numéricos de versão e ordem, limites permitidos, tratamento de nomes duplicados, lacunas entre versões, versão zero e alvo inexistente. Definir também como os cadastros retornam erros.

36. **Múltiplos conjuntos de migrations:** decidir se representam partes do mesmo histórico, módulos independentes ou alternativas por dialeto. Definir assinatura variádica, seleção de listas, dependências e namespaces.

37. **Scripts novos em versões já aplicadas:** permitir ou rejeitar a inclusão? Se permitir, como identificá-los e executá-los, principalmente quando versões posteriores já tiverem sido aplicadas?

38. **Downgrade:** definir quando a descida pode ocorrer, se exige autorização/configuração explícita, como tratar migrations irreversíveis e o que fazer quando o banco estiver em versão superior à da aplicação.

39. **Concorrência das migrations:** escolher coordenação pelo banco ou garantia operacional de executor único. Definir locks, timeout, perda de conexão e manutenção da mesma sessão quando necessária.

40. **Atomicidade das migrations:** definir políticas por banco e operação, incluindo scripts não transacionais. Aprovar ou ajustar a proposta de rejeitar previamente planos incompatíveis com uma transação recebida. Considerar que o retorno de `RunMigrationsTx` ocorre antes do commit feito pela aplicação.

41. **Falhas parciais e recuperação:** definir estados de execução, registro de falhas e procedimento de reparo. Incluir o caso em que o SQL termina, mas a gravação do histórico falha.

42. **Integridade do histórico:** usar checksum? Qual será a política para conteúdo alterado, migrations desconhecidas no banco e arquivos removidos?

43. **Tabela `migrations`:** definir colunas, chaves, identificação dos registros e conjuntos, schema, evolução da estrutura e detecção de colisão com uma tabela existente da aplicação.

44. **Conteúdo dos scripts:** cada migration aceitará um comando, uma lista estruturada ou um script completo com vários comandos? Definir restrições de execução e requisitos dos drivers.

45. **Arquivos de migrations:** aprovar `fs.FS` como fonte; definir caminhos, momento da leitura, validação antecipada, arquivos ausentes e prioridade entre conteúdo inline e arquivo.

46. **Operação e diagnóstico das migrations:** definir comportamento em cancelamento e interrupção do processo, possibilidade de inspecionar o plano antes de executar, permissões exigidas e informações apresentadas nos erros.

47. **Verificações durante o desenvolvimento:** permitir compilação e exemplos mínimos durante essa fase, mantendo a fase formal de testes depois? A ordem geral das fases já está definida.

48. **Estratégia de testes e aceitação:** definir a matriz de ambientes, isolamento dos testes e critérios de aceitação de cada tarefa, incluindo entradas inválidas.

49. **Organização e publicação:** escolher licença, política de versões e tags, critérios de release e localização de manual, licença e CI conforme as restrições de `AGENTS.md`.

As escolhas já aprovadas — `Config.Context`, `client.Context()`, transação por execução, várias migrations por versão e ordenação de UP/DOWN — não precisam ser decididas novamente.