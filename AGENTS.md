# krios

- A IA não deve alterar nada, nem criar arquivos, sem autorização do utilizador;
- A ordenização do codigo, deve ser a mais separada possivel, para uma melhor visualização do utilizador. Verifique como ele escreve o codigo e faça da mesma forma;

## Pendências de documentação

- Documentar o significado do campo `Table.Schema` em cada banco, com exemplos de uso:
  - PostgreSQL: identifica o schema que contém a tabela, como `public`;
  - MySQL: identifica o banco de dados que contém a tabela;
  - SQLite3: identifica o banco `main`, `temp` ou um banco anexado via `ATTACH DATABASE`;
- Explicar que `public` não é um prefixo universal e precisa existir no destino conforme o significado acima;
- Documentar que deixar `Schema` vazio omite o prefixo e usa as regras de resolução de tabelas de cada banco.
