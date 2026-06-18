# language: pt
Funcionalidade: Exclusão de orçamento rascunho

  Cenário: excluir rascunho retorna 204 e remove do banco
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento rascunho para o usuário na competência "2025-05"
    Quando o usuário autenticado exclui o rascunho da competência "2025-05"
    Então a resposta HTTP deve ter status 204
    E o banco não deve conter orçamento para o usuário na competência "2025-05"

  Cenário: excluir orçamento inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta excluir o orçamento da competência "2099-11"
    Então a resposta HTTP deve ter status 404

  Cenário: excluir orçamento ativo retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-06"
    Quando o usuário autenticado tenta excluir o orçamento da competência "2025-06"
    Então a resposta HTTP deve ter status 409
