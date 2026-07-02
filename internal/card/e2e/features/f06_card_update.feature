# language: pt
Funcionalidade: Atualização de cartão

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Atualizar apelido via PUT retorna 200 com apelido novo
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário atualiza o cartão informando apenas o apelido "Meu Neon"
    Então a resposta HTTP deve ter status 200
    E o campo texto "nickname" da resposta deve ser "Meu Neon"

  Cenário: Atualizar banco e vencimento via PUT recalcula closing_day
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário atualiza o cartão informando banco "itau" e vencimento 10
    Então a resposta HTTP deve ter status 200
    E o campo numérico "due_day" da resposta deve ser 10
    E o corpo da resposta deve conter o campo "closing_day"
    E o corpo da resposta deve conter o campo "best_purchase_day"

  Cenário: Atualizar cartão inexistente via PUT retorna 404
    Quando o usuário tenta atualizar um cartão com ID inexistente informando o apelido "Qualquer"
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Atualizar cartão com apelido de 33 caracteres retorna 400
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário tenta atualizar o cartão com apelido de 33 caracteres
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_nickname"
