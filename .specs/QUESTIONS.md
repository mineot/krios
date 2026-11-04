# Dúvidas por tarefa

## Tarefa: revisão técnica de DEFINITIONS.md — 2026-11-04

Estado: perguntas abertas para o desenvolvedor. Nenhuma decisão de API ou implementação foi tomada. Referências às linhas do documento na data desta revisão. Esta revisão não atribui numeração às futuras tarefas de implementação.

### Configuração e execução

1. **API de configuração (linhas 15–38):** o campo `timeout` deve ser exportado como `Timeout`? Qual será a assinatura de `New` para retornar erros de configuração e de conexão?
   - Resposta:
2. **Conexão (linhas 5–12):** a validação será feita na criação, antes de cada execução ou em outro momento? Quem será responsável por fechar o `*sql.DB` recebido? Quais drivers e versões mínimas de Go e dos bancos serão suportados?
   - Resposta:
3. **Contexto e timeout (linhas 19–20):** haverá contexto por operação? Como tratar timeout zero ou negativo, contexto cancelado e prazos já existentes? O timeout cobre ping, contagem da paginação, leitura de linhas e migrações? Como manter o contexto válido enquanto o consumidor lê `*sql.Rows`?
   - Resposta:
4. **Erros e retornos (linhas 10–12, 188–205 e 283–285):** quais serão as assinaturas completas, incluindo ponteiros, de `Exec`, `ExecTx`, `Migrate` e `MigrateTx`? Qual valor de `Page` será retornado sem paginação? Os erros do banco serão preservados para `errors.Is`/`errors.As`? O consumidor ficará responsável por `Rows.Close`, `Rows.Scan` e `Rows.Err`?
   - Resposta:
5. **Transações (ocorrências de `ExecTx` e `MigrateTx`):** o argumento será `*sql.Tx`? Quem executará commit e rollback? Como tratar transação nula, encerrada ou incompatível com o cliente? Toda operação auxiliar de paginação ou migração utilizará a mesma transação?
   - Resposta:

### Critérios e construção das consultas

6. **Parâmetros (linhas 43–54 e 173):** confirmar que SQL e argumentos serão enviados separadamente ao driver, sem interpolação textual. Qual será a sintaxe por dialeto/driver e como ocorrerá a numeração conjunta em subconsultas, joins e unions?
   - Resposta:
7. **Tipos (linhas 43–61):** o contrato inclui `nil`, floats, inteiros sem sinal dentro dos limites de conversão, ponteiros, tipos que implementam `driver.Valuer` e extensões dos drivers? Quais validações cabem ao Krios e quais serão delegadas ao driver/banco?
   - Resposta:
8. **IN/NOT IN (linha 53):** listas serão expandidas em um argumento por elemento? Qual comportamento para listas vazias, nulas, heterogêneas, com `NULL`, arrays e `[]byte`? Haverá suporte a subconsultas nesses predicados?
   - Resposta:
9. **NULL e composição (linhas 67–89):** `Eq(campo, nil)` e `Ne(campo, nil)` serão traduzidos para `IS NULL`/`IS NOT NULL`? Como tratar `And()`/`Or()` vazios e critérios nulos? Os predicados existirão apenas no cliente ou também no pacote, como mostram os exemplos com `krios.Eq`?
   - Resposta:
10. **Identificadores e expressões (linhas 98–144):** strings representam nomes de colunas/tabelas, expressões SQL ou ambos? Como representar comparação entre duas colunas no `ON` sem transformar a segunda coluna em valor textual? Como tratar nomes qualificados, curingas, funções, schemas, aliases e escape de identificadores por dialeto?
   - Resposta:
11. **Estado do builder (linhas 97–185):** chamadas sucessivas de `Where`, `On`, `Columns`, `GroupBy` e `Set` acumulam ou substituem? A que elemento `As`, `Asc`, `Desc` e `Schema` se aplicam? Os builders e critérios são mutáveis, reutilizáveis e seguros para uso concorrente? Uma subconsulta é copiada ou mantida por referência?
   - Resposta:
12. **SELECT e subconsultas (linhas 110–159):** qual regra determina a seleção implícita de `*` ou `u.*` e sua remoção após adicionar colunas? `Count("*")` deve preservar `COUNT(*)`? `SubSelect` pretende gerar produto cartesiano? Como representar correlação e restringir o formato de subconsultas escalares?
   - Resposta:
13. **UNION (linhas 148–159):** como `query1` participará da união, já que não é passado no exemplo? Qual será a assinatura de `Union`/`UnionAll`, e onde se aplicarão ordenação, paginação e parênteses?
   - Resposta:
14. **JOIN e HAVING (linhas 127–144 e 163–164):** `Having` fará parte do escopo inicial? Como representar agregações nos critérios? Quais tipos de join serão incluídos e como tratar recursos indisponíveis na versão do banco?
   - Resposta:

### Paginação e escrita

15. **Semântica da paginação (linhas 177–205):** os argumentos são página/tamanho ou offset/limite? Se forem página/tamanho, confirmar `offset = (pagina - 1) * tamanho`. Como tratar zero, valores fora do intervalo, resultado vazio e overflow? Qual diferença entre `LastPage` e `TotalPages`?
   - Resposta:
16. **Contagem e navegação (linhas 182–205):** `TotalRows` contará as linhas do resultado completo, inclusive após `DISTINCT`, `GROUP BY`, `HAVING` e `UNION`? Quando recalcular a contagem? `Next`/`Prev` executam SQL ou apenas mudam o estado? Como funcionam antes da primeira execução? O que significa zerar a paginação? Qual consistência será prometida diante de alterações concorrentes dos dados e qual será a regra de ordenação?
   - Resposta:
17. **Validações de escrita (linhas 208–240):** como tratar `Insert` sem valores, quantidade de valores diferente das colunas, `Update` sem `Set` e `Set` repetido para a mesma coluna? `Where()` vazio ou um grupo vazio satisfaz a exigência de filtro? Em que momento esses erros serão retornados?
   - Resposta:

### Arquivos SQL e migrações

18. **Execução de arquivos (linhas 243–250):** qual retorno atende arquivos com SELECT, comandos sem linhas e múltiplos resultados? Arquivos podem conter vários comandos, funções, procedures e comandos de clientes como `DELIMITER`? Serão enviados inteiros ao driver ou interpretados pelo Krios? Como resolver caminhos relativos, passar parâmetros e reportar falhas parciais?
   - Resposta:
19. **Versões e histórico (linhas 6, 17 e 253–277):** `DbVersion` é a versão desejada do schema? Como representar banco sem migrações e retorno à versão zero? Versões precisam ser contíguas? Como tratar versões duplicadas, alvo ausente, lista vazia e histórico divergente? Qual limite numérico será adotado, considerando `uint` e a coluna SQL? A tabela `migrations` guarda versão atual, migrações aplicadas ou histórico de eventos? Quais seus nomes de colunas, tipos, chave, schema e regra de data/fuso?
   - Resposta:
20. **Aplicação e recuperação de migrações (linhas 255–285):** confirmar ordem de `Up` e `Down` e qual versão identifica cada arquivo de descida. A atomicidade será por arquivo ou por conjunto? Como registrar falha parcial, retomar a execução e impedir duas instâncias de migrarem simultaneamente? Como lidar com DDL que causa commit implícito no MySQL e com uma tabela `migrations` já existente? Qual contrato de erro e de controle da transação para `MigrateTx`?
   - Resposta:
