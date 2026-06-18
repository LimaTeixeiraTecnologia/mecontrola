# language: pt
Funcionalidade: Criação e atualização de despesa

  Cenário: criar despesa válida retorna 201 e persiste no banco
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-07"
    Quando o usuário autenticado cria uma despesa de "5000" centavos na competência "2025-07"
    Então a resposta HTTP deve ter status 201
    E o banco deve conter 1 despesa para o usuário

  Cenário: criar despesa sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado tenta criar uma despesa
    Então a resposta HTTP deve ter status 401
