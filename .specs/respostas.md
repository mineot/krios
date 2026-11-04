1. **Criação do cliente:** `New` deve testar comunicação e consultar a versão do banco ou apenas validar a configuração?

- Deve testar a conexão, com ping e tudo mais se possivel.
- Não precisa testar a versão do banco, se ocorrer alguma incompatibilidade, vamos retornar um erro para o usuário, visto que ele deve estar ciente da versão do banco.
- Vamos validar a configuração.

2. **`IN` e `NOT IN`:** quais coleções aceitar e como tratar elementos `nil` dentro de listas não vazias?

- Eu imagino assim, eu posso aceitar qq coisa, inclusive nil, que devem ser tratados como null, pelo banco;
- Na verdade, o utilizador, deve ter o minimo de noção de como se usa uma query sql, não vamos entregar tudo.
- O Utilizador, deve saber que, um in de inteiros, devem receber apenas inteiros, o mesmo de strings;
- vamos deixar o banco tratar essa situação, se algum valor invalido for passado, e o banco retornar um erro, vamos retornar para o utilizador.

3. **Paginação:** `Clone()` preserva a posição atual? Como navegar quando inserções ou exclusões mudarem a última página?

- É de responsabilidade do utilizador atualizar a consulta, se ele não o fizer provavelmente a consulta retornará erro, que deve ser reportado ao utilizador.

4. **Exclusividade das migrations:** o Krios bloqueia execuções concorrentes pelo banco ou a aplicação garante um único executor?

- Na Migração, acredito que o krios deve bloquear execuções concorrenter, por ser um processo critico.

5. **Compatibilidade transacional:** como determinar se um script pode executar dentro de `sql.Tx`?

- Não sei, aceito sugestão da IA

6. **Recuperação de migrations:** como reparar e retomar versões parcialmente aplicadas ou execuções com resultado incerto?

- Eu passo essa responsabilidade para o utilizador, ele deve garantir que se ocorrer um erro possa ser revertido.
- Se ele passar um contexto, sql.Tx, e a migração der erro, então deve ser feito um rollback, mas não por parte do krios;
- O que podemos fazer, é remover a possilidade de executar uma migração sem sql.Tx, obrigando sempre a passar uma. o que a IA acha?

7. **Arquivos:** quando ler e validar os scripts, quais caminhos aceitar e como tratar arquivos ausentes?

- Eu acho que o utilizador deve informar o caminho dos scripts;
- Não é responsabilidade do krios validar os scripts, se eles estiverem errados, deve-se retornar o erro de banco para o utilizador
