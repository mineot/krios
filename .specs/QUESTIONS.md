# Dúvidas por tarefa

## Tarefa: revisão técnica de DEFINITIONS.md — 2026-11-04

Estado: perguntas abertas para o desenvolvedor. Nenhuma decisão de API ou implementação foi tomada. Referências às linhas do documento na data desta revisão. Esta revisão não atribui numeração às futuras tarefas de implementação.

### Configuração e execução

1. **API de configuração (linhas 15–38):** o campo `timeout` deve ser exportado como `Timeout`? Qual será a assinatura de `New` para retornar erros de configuração e de conexão?

- Resposta: Timeout foi ajustado, e foi corrigido sobre o erro, que deve retornar e ser tratado pelo utilizador.

2. **Conexão (linhas 5–12):** a validação será feita na criação, antes de cada execução ou em outro momento? Quem será responsável por fechar o `*sql.DB` recebido? Quais drivers e versões mínimas de Go e dos bancos serão suportados?

- Resposta: A validação será feito no momento da criação, do `New`, a versão minima do golang será a 1.26+, PostgreSQL 16+, MySQL 8+ e SQLite 3.39+. O utilizador é responsavel por fechar o `*sql.DB`.

3. **Contexto e timeout (linhas 19–20):** haverá contexto por operação? Como tratar timeout zero ou negativo, contexto cancelado e prazos já existentes? O timeout cobre ping, contagem da paginação, leitura de linhas e migrações? Como manter o contexto válido enquanto o consumidor lê `*sql.Rows`?

- Resposta: A sugestão da IA foi aprovado.

**Resposta sugerida pela IA — pendente de aprovação:**

- Usar `Config.Context` nas operações do cliente, sem contexto adicional nos argumentos de `Exec()` e `ExecTx(tx)`.
- `Timeout` negativo gera erro de configuração em `New`; zero usa somente o prazo/cancelamento de `Config.Context`; positivo cria um contexto derivado por operação. Havendo prazo anterior no contexto original, prevalece o que vencer primeiro.
- O timeout começa na execução, não na criação do builder. Cobre o ping em `New`, toda a execução de escritas e a chamada completa de migração, sem reiniciar a contagem por comando. Em SELECT paginado, contagem e consulta compartilham o mesmo prazo, que também abrange a leitura das linhas.
- Contexto cancelado impede iniciar novos comandos e retorna erro. O cancelamento de `Config.Context` afeta também as próximas operações do mesmo cliente. Propagar o contexto às chamadas de `database/sql`.
- Ao retornar `*sql.Rows`, não cancelar o contexto derivado na saída de `Exec`: ele precisa continuar válido enquanto o consumidor lê as linhas. Em falha anterior à entrega das linhas, cancelar imediatamente; nas operações sem linhas, liberar o contexto derivado ao término.
- Como `*sql.Rows` é retornado diretamente, o Krios não intercepta `Rows.Close()` para liberar seu timer. A proposta mantém `Timeout` automático e aceita que, em sucesso, o timer possa permanecer até o prazo vencer, mesmo após o fechamento das linhas. Documentar esse comportamento.
- Alternativa apresentada, não escolhida: a aplicação controlar integralmente o prazo pelo contexto e chamar `cancel()` após consumir as linhas, permitindo dispensar `Timeout` na configuração. A recomendação acima preserva a configuração atual, mas ainda depende de aprovação.

Referência: [Cancelamento de operações no Go](https://go.dev/doc/database/cancel-operations).

4. **Erros e retornos (linhas 10–12, 188–205 e 283–285):** quais serão as assinaturas completas, incluindo ponteiros, de `Exec`, `ExecTx`, `Migrate` e `MigrateTx`? Qual valor de `Page` será retornado sem paginação? Os erros do banco serão preservados para `errors.Is`/`errors.As`? O consumidor ficará responsável por `Rows.Close`, `Rows.Scan` e `Rows.Err`?

- Resposta: Porem o valor de page quando não houver paginação deve ser nil, e sim, os erros do banco devem ser preservados e retornados para o utilizados, e sim, o consumidor ficará responsavel por tratar tudo referente ao `Rows`. A sugestão da IA foi aprovada

**Resposta sugerida pela IA — complemento pendente de aprovação:**

As propostas abaixo preservam as respostas do desenvolvedor sobre `page == nil`, preservação dos erros e responsabilidade do consumidor por `Rows`. Os nomes dos tipos dos builders ainda poderão ser definidos; `Client` é o nome proposto para o tipo retornado por `New`.

| Operação                | Assinatura sugerida                            |
| ----------------------- | ---------------------------------------------- |
| Criação                 | `New(config Config) (*Client, error)`          |
| SELECT                  | `Exec() (*sql.Rows, *Page, error)`             |
| SELECT em transação     | `ExecTx(tx *sql.Tx) (*sql.Rows, *Page, error)` |
| INSERT, UPDATE e DELETE | `Exec() (sql.Result, error)`                   |
| Escritas em transação   | `ExecTx(tx *sql.Tx) (sql.Result, error)`       |
| Migração                | `Migrate() error`                              |
| Migração em transação   | `MigrateTx(tx *sql.Tx) error`                  |

- Falha em `New`: retornar cliente `nil` e erro.
- SELECT bem-sucedido sem paginação: `(rows, nil, nil)`; com paginação: `(rows, page, nil)`.
- Falha de validação, contagem ou abertura do SELECT: `(nil, nil, err)`, liberando recursos intermediários.
- SELECT sem registros: retornar `Rows` normalmente; o primeiro `Next()` retorna `false`. O consumidor verifica `Rows.Err()` para distinguir fim dos resultados de falha na leitura.
- Escrita com erro: `(nil, err)`.
- Acrescentar contexto aos erros, quando útil, usando `%w`, por exemplo `fmt.Errorf("krios: executar select: %w", err)`, preservando a causa para `errors.Is` e `errors.As`.
- Falhas posteriores à entrega das linhas são verificadas pelo consumidor em `Scan` e `Rows.Err()`; não alteram retroativamente o erro retornado por `Exec`. O consumidor também fecha `Rows`.

Referência: [API de database/sql](https://pkg.go.dev/database/sql).

5. **Transações (ocorrências de `ExecTx` e `MigrateTx`):** o argumento será `*sql.Tx`? Quem executará commit e rollback? Como tratar transação nula, encerrada ou incompatível com o cliente? Toda operação auxiliar de paginação ou migração utilizará a mesma transação?

- Resposta: o usuario deve abrir um contexto, sql.Tx e ser responsavel por fechar e tratar o commit. Aprovado a sugestão da IA.

**Resposta sugerida pela IA — complemento pendente de aprovação:**

- A aplicação cria a transação com `db.BeginTx(...)` e fica responsável por `Commit` ou `Rollback`, incluindo o tratamento de seus erros. O argumento recebido por `ExecTx` e `MigrateTx` é `*sql.Tx`.
- O Krios não confirma nem desfaz a transação recebida. Sucesso no método indica que a operação terminou, mas ainda depende do commit da aplicação.
- Usar exclusivamente a transação fornecida, inclusive para contagem da paginação, consulta principal e leitura/escrita do histórico de migrações. Não usar o pool como alternativa em caso de erro.
- Não armazenar a transação no cliente para chamadas futuras. Chamadas sem transação continuam usando o pool configurado.
- `tx == nil`: retornar erro de argumento antes de executar SQL.
- Transação encerrada: propagar o erro retornado por `database/sql`, normalmente `sql.ErrTxDone`, preservando sua identificação.
- Contexto da operação cancelado: retornar o erro e não iniciar novos comandos.
- A aplicação deve fornecer uma transação compatível com o banco e o dialeto do cliente. `*sql.Tx` não expõe publicamente seu pool de origem; não prometer validação geral de que pertence a `Config.Db`.
- Manter válido o contexto passado a `BeginTx` até encerrar a transação. Seu cancelamento pode provocar rollback pelo próprio `database/sql`, independentemente do contexto usado em uma consulta. Prazo/cancelamento de uma operação e ciclo de vida da transação são responsabilidades distintas.

Referências: [Execução de transações no Go](https://go.dev/doc/database/execute-transactions) e [API de sql.Tx](https://pkg.go.dev/database/sql#Tx).

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
