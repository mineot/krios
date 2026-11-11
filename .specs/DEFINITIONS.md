# Definições de Implementação do Krios

## Sobre o Krios

Krios será uma biblioteca Typescript para aplicativos node backend e cli para a execução de queries, e controle de migração do banco de dados. Neste momento a biblioteca suportará 3 bancos apenas, `PostreSql`, `SQLite3` e `Mysql`/`MariaDB`.

## Regras de implementação

- Criar a estrutura da biblioteca;
- Escolher as melhories bibliotecas opensource para conversar com cada banco;
- Implementação da Configuração Inicial;
- Implementação dos Dialetos;
- Implementar Execução padrão e execução usando contexto
- Implementação do QueryBuilder;
- Implementação do File Executor
- Implementação da Migração;
- Criação dos testes unitários;
- Documentação da biblioteca;
- Preparar e publicar no npmjs;

## Orientações de Implementação

### Configuração

- Vamos nos basear no `AdonisJS`, no arquivo `env` dele, porem não teremos um arquivo `env`.
- O `client` da configuração, vai conter a instancia de tudo

```ts
const client = await new Krios.Config({
    // informar o dialeto do banco
    dialect: Krios.DIALECT.POSTGRESQL,
    // case seja SQLite3
    fileDb: './..*.db',
    // para os postresql e mysql/mariadb
    host: 'localhost',
    port: 0000,
    username: 'user',
    password: 'password',
    database: 'database',
    // quando for testadp
    timeout: 3000,
});
```

```ts
// abrir o banco de dados
await client.open();
```

```ts
// fechar o banco de dados
await client.close();
```

### Dialetos

- O dialeto deve ser estruturado para auxiliar o query builder criar consultas usando a sintaxe correta que data instrução sql para cada um dos bancos.

### Execução

### QueryBuilder

### FileExecutor

### Migração
