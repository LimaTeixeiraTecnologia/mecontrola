# language: pt
Funcionalidade: Leitura e listagem de cartões

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Buscar cartão existente retorna 200 com todos os campos
    Dado que o usuário possui um cartão criado com nome "Leitura Unitária", fechamento 5, vencimento 12 e limite 150000
    Quando o usuário busca o cartão pelo ID cadastrado
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"
    E o corpo da resposta deve conter o campo "user_id"
    E o corpo da resposta deve conter o campo "name"
    E o corpo da resposta deve conter o campo "nickname"
    E o corpo da resposta deve conter o campo "closing_day"
    E o corpo da resposta deve conter o campo "due_day"
    E o corpo da resposta deve conter o campo "limit_cents"
    E o campo texto "name" da resposta deve ser "Leitura Unitária"
    E o campo numérico "limit_cents" da resposta deve ser 150000

  Cenário: Buscar cartão com ID inexistente retorna 404
    Quando o usuário busca um cartão com ID aleatório inexistente
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Listar cartões inclui o cartão recém-criado
    Dado que o usuário possui um cartão criado com nome "Listagem Base", fechamento 3, vencimento 8 e limite 50000
    Quando o usuário lista todos os seus cartões
    Então a resposta HTTP deve ter status 200
    E a lista de cartões deve conter ao menos 1 item
    E o campo "items" da lista deve estar presente

  Cenário: Listar com limit=1 e 2 cartões cadastrados retorna next_cursor
    Dado que o usuário possui um cartão criado com nome "Página 1-A", fechamento 5, vencimento 12 e limite 50000
    E que o usuário possui um cartão criado com nome "Página 1-B", fechamento 5, vencimento 12 e limite 50000
    Quando o usuário lista os cartões com limit 1
    Então a resposta HTTP deve ter status 200
    E a lista de cartões deve conter 1 item
    E a resposta deve conter next_cursor
    Quando o usuário lista os cartões usando o cursor retornado
    Então a resposta HTTP deve ter status 200
    E a lista de cartões deve conter ao menos 1 item

  Cenário: Listar cartões com cursor inválido retorna 400
    Quando o usuário lista os cartões passando cursor inválido "isto-nao-e-cursor"
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_cursor"
