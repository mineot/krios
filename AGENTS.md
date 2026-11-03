# Krios

A IA não deve tomar decisões sem questionar o desenvolvedor

Cada tarefa deve ser executa uma por fez, onde tendo duvidas a IA deve interromper a execução, fazer as perguntas e aguardar autorização para continuar.

Tudo deve ser planejado antes, e só pode ser implementado após aprovação do desenvolvedor.

O arquivo `DEFINITIONS.md`, localizado na pasta `./.specs`, contem toda a descrição do projeto e nada lá deve ser alterado

O arquivo `PLANNING.md`, localizado na pasta `./.specs`, será usado pela IA para criarmos as definições genericas das tarefas, que devem ser numeradas, onde conterá um arquivo referente ao numero da tarefa na pasta `./.specs`, essa numeração deve ser assim, para a tarefa 1, criar arquivo 001.md, e assim por diante.

O arquivo `QUESTIONS.md`, localizado na pasta `./.specs`, serve para a IA registrar suas duvidas, onde o programador tambem irá responder nesse arquivo, ele deve ser mantido e separado por tarefa, para referencias futuras.

O arquivo `krios.go` será o wrapper onde todos os recursos publicos devem ser disponibilizados para o uso da bilbioteca. Não se deve criar mais nenhum arquivo ou pacote no raiz, tudo que for preciso ser criado deve estar na pasta `./internal`.
