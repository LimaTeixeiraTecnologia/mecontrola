# language: pt
Funcionalidade: Leitura e listagem de cartões

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Buscar cartão existente retorna 200 com todos os campos
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário busca o cartão pelo ID cadastrado
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"
    E o corpo da resposta deve conter o campo "user_id"
    E o corpo da resposta deve conter o campo "bank"
    E o corpo da resposta deve conter o campo "nickname"
    E o corpo da resposta deve conter o campo "closing_day"
    E o corpo da resposta deve conter o campo "due_day"
    E o corpo da resposta deve conter o campo "best_purchase_day"
    E o campo texto "bank" da resposta deve ser "nubank"

  Cenário: Buscar cartão com ID inexistente retorna 404
    Quando o usuário busca um cartão com ID aleatório inexistente
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Listar cartões inclui o cartão recém-criado
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário lista todos os seus cartões
    Então a resposta HTTP deve ter status 200
    E a lista de cartões deve conter ao menos 1 item
    E o campo "items" da lista deve estar presente

  Cenário: Listar com limit=1 e 2 cartões cadastrados retorna next_cursor
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    E que o usuário possui um cartão criado com banco "itau" e vencimento 15
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
