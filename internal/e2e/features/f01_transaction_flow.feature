# language: pt
Funcionalidade: Criação de transação via HTTP

  Cenário: Criar transação de despesa com categoria válida e validar no banco
    Dado que a categoria "prazeres" está disponível no sistema
    Quando o usuário cria uma transação de 5800 centavos no método "pix"
    Então a resposta HTTP deve ter status 201
    E a transação deve estar salva no banco com valor 5800
    E o corpo da resposta deve conter o campo "id"
