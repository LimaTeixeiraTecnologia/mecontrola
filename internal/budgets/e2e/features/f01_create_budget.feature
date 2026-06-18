# language: pt
Funcionalidade: Criação de orçamento

  Cenário: criar orçamento válido retorna 201 e persiste no banco
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado cria um orçamento para a competência "2025-01" com total de "100000" centavos
    Então a resposta HTTP deve ter status 201
    E o banco deve conter 1 orçamento para o usuário na competência "2025-01"

  Cenário: criar orçamento sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado tenta criar um orçamento
    Então a resposta HTTP deve ter status 401

  Cenário: criar orçamento duplicado retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que já existe um orçamento para o usuário na competência "2025-02"
    Quando o usuário autenticado cria um orçamento para a competência "2025-02" com total de "50000" centavos
    Então a resposta HTTP deve ter status 409
