Sim. As regras principais estão definidas, mas restam estas decisões de comportamento:

1. **Migrations concorrentes:** uma segunda execução deve esperar ou retornar erro imediatamente? Se esperar, qual prazo usar?
2. **Histórico após reparo:** como o utilizador informa ao Krios que corrigiu uma falha e que a execução pode continuar?
3. **Cadastro inválido:** após um erro em `Append`/`AppendFiles`, será necessário recriar o conjunto ou haverá uma operação para corrigir seu estado?

Também faltam detalhes técnicos que podemos propor nas respectivas tarefas:

- Mecanismo de exclusividade por banco, incluindo proteção até o encerramento da transação externa.
- Execução de scripts com vários comandos e requisitos dos drivers.
- Estrutura física e evolução da tabela `migrations`, incluindo registros revertidos.
- Tipos e assinaturas públicas: referências de colunas, joins, subconsultas, erros, paginação, modos de migration e inspeção do plano.
- Drivers e versões adicionais dos testes.

Esses pontos estão reunidos em [Detalhamentos ainda necessários](/home/mineot/Projetos/krios/.specs/DEFINITIONS.md:927). **Não é necessário reabrir as decisões anteriores; podemos resolver os três comportamentos acima e detalhar o restante no planejamento.**
