# Considerações sobre `.spec/DEFINITIONS.md`

O documento define o escopo inicial da biblioteca, mas alguns contratos da API precisam ficar mais claros antes da implementação. As considerações abaixo são propostas para discussão, não decisões já aprovadas nem substituições dos requisitos originais.

1. **Separar contexto de transação.** `context.Context` controla cancelamento e prazos; `Begin`, `Commit` e `Rollback` controlam uma transação. A proposta combina os dois. A sugestão é que `Begin(ctx)` retorne um objeto de transação e que os comandos sejam executados por esse objeto.

2. **Evitar uma conexão ou transação global.** Chamadas como `krios.Begin()` e `krios.Commit()` deixam ambíguo o comportamento com múltiplas goroutines ou bancos. Uma instância por configuração, como `db`, e uma instância por transação, como `tx`, tornariam esse uso mais seguro e explícito.

3. **Definir quem fornece a conexão.** A responsabilidade do utilizador pode significar fornecer o driver ou fornecer um `*sql.DB` já aberto. É preciso escolher esse contrato, inclusive para definir quem deve fechar a conexão.

4. **Ajustar os exemplos à sintaxe de Go.** Os exemplos funcionam como esboço, mas Go não permite omitir argumentos posicionais comuns nem variar a quantidade de retornos de um mesmo método. Em particular, `Query(...).Exec()` não pode retornar três valores para SELECT e dois para INSERT. A sugestão é separar consultas que retornam linhas de comandos que retornam um resultado de execução. Para ignorar um retorno, usa-se `_`, não `nil`.

5. **Rever os retornos de INSERT e UPDATE.** Retornar automaticamente o ID inserido exige tratamento por banco e uma definição da coluna identificadora. Em UPDATE, o ID do último registro alterado fica ambíguo quando várias linhas são alteradas. A sugestão é retornar a quantidade de linhas afetadas para UPDATE e oferecer suporte explícito ao retorno de IDs quando necessário. Essa contagem também seria útil no DELETE.

6. **Definir ordenação e paginação.** `Paginate(1, 10)` está descrito como offset e limite, mas o nome pode sugerir página e tamanho. Convém escolher uma dessas interpretações e definir o conteúdo de `page`. `First()` e `Last()` também precisam de uma ordenação definida para terem resultados previsíveis.

7. **Especificar a semântica das condições.** Faltam decisões para `In()` vazio, `Eq("campo", nil)` e comparação entre duas colunas. Esta última é essencial nos JOINs: comparar `usuarios.id` com `pedidos.usuario_id` deve ser diferente de comparar uma coluna com um valor de texto. `Exists` e `NotExists` deveriam receber uma subconsulta.

8. **Tratar segurança como parte do contrato.** Valores devem ser enviados como parâmetros. Nomes de tabelas, colunas, schemas e aliases precisam de tratamento próprio conforme o dialeto. A exigência de WHERE em UPDATE e DELETE é boa, mas deve rejeitar também condições vazias que não produzam uma restrição efetiva.

9. **Delimitar as diferenças entre bancos.** O dialeto precisa cobrir placeholders, identificadores, schemas, paginação, JOINs e retorno de dados alterados. Algumas operações não terão suporte equivalente nos três bancos; nesses casos, é melhor definir um erro explícito do que prometer comportamento uniforme.

10. **Reformular a regra de migração.** A comparação deveria ser entre a versão registrada no banco e a versão desejada na configuração. `CurrentVersion` parece representar uma versão de destino; `TargetVersion` seria mais claro. Também faltam regras para ordem dos arquivos, versões duplicadas, falhas e execuções simultâneas. A garantia de rollback precisa considerar o suporte transacional dos comandos em cada banco.

11. **Tornar o acesso a arquivos embed explícito.** A biblioteca precisa receber acesso aos arquivos incorporados pela aplicação; apenas o nome do arquivo não basta. Receber um `fs.FS`, compatível com `embed.FS`, seria uma boa opção. A sugestão também é evitar identificar automaticamente paginação ou IDs analisando SQL arbitrário, algo que aumenta bastante a complexidade.

12. **Documentar o ciclo de vida dos resultados e erros.** Se SELECT retorna `*sql.Rows`, o usuário deve saber que precisa fechar as linhas e verificar erros de iteração. Também vale definir o comportamento de timeout zero, operações sem transação, chamadas após `Close()` e reutilização dos builders.

## Prioridades sugeridas

Priorizar as definições de instâncias e transações, assinaturas e retornos, e migrações. Essas decisões influenciam quase toda a biblioteca; resolvê-las primeiro evita retrabalho nas demais funcionalidades.
