# language: pt
Funcionalidade: Resumo mensal de orçamento

  Cenário: obter resumo de competência ativa retorna 200
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-08"
    Quando o usuário autenticado solicita o resumo da competência "2025-08"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o campo "competence" com valor "2025-08"

  Cenário: obter resumo de competência inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado solicita o resumo da competência "2099-01"
    Então a resposta HTTP deve ter status 404
