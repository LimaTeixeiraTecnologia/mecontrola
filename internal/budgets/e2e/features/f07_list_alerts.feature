# language: pt
Funcionalidade: Listagem de alertas

  Cenário: listar alertas sem itens retorna lista vazia com status 200
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado lista os alertas
    Então a resposta HTTP deve ter status 200

  Cenário: listar alertas sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado lista os alertas
    Então a resposta HTTP deve ter status 401
