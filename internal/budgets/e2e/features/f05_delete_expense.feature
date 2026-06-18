# language: pt
Funcionalidade: Exclusão de despesa

  Cenário: excluir despesa existente retorna 204 e realiza soft-delete
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa para o usuário com versão 1
    Quando o usuário autenticado exclui a despesa com versão esperada 1
    Então a resposta HTTP deve ter status 204
    E o banco deve conter a despesa com deleted_at preenchido
    E o banco deve conter tombstone para a despesa

  Cenário: excluir despesa inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta excluir uma despesa inexistente
    Então a resposta HTTP deve ter status 404
