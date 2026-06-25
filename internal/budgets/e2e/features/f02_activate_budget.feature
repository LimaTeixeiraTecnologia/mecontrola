# language: pt
Funcionalidade: Ativação de orçamento

  Cenário: ativar orçamento rascunho retorna 200 e publica no outbox
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento rascunho para o usuário na competência "2025-03"
    Quando o usuário autenticado ativa o orçamento da competência "2025-03"
    Então a resposta HTTP deve ter status 200
    E o banco deve conter o orçamento da competência "2025-03" com estado "active"

  Cenário: ativar orçamento inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta ativar o orçamento da competência "2099-12"
    Então a resposta HTTP deve ter status 404

  Cenário: ativar orçamento já ativo retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-04"
    Quando o usuário autenticado tenta ativar o orçamento da competência "2025-04"
    Então a resposta HTTP deve ter status 409
