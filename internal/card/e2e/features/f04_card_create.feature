# language: pt
Funcionalidade: Criação de cartão via HTTP

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Criar cartão com todos os campos válidos retorna 201
    Quando o usuário cria um cartão com nome "Nubank Pessoal", apelido único, fechamento 5, vencimento 12 e limite de 300000 centavos
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o corpo da resposta deve conter o campo "name"
    E o corpo da resposta deve conter o campo "limit_cents"
    E o cartão deve estar persistido no banco com nome "Nubank Pessoal" e limite de 300000 centavos

  Cenário: Criar cartão com limite zero é permitido
    Quando o usuário cria um cartão com nome "Zero Limit", apelido único, fechamento 1, vencimento 15 e limite de 0 centavos
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"

  Cenário: Criar cartão com nickname duplicado retorna 409
    Dado que já existe um cartão com o apelido "nick-dup-e2e"
    Quando o usuário tenta criar um cartão com o mesmo apelido "nick-dup-e2e"
    Então a resposta HTTP deve ter status 409
    E o campo de erro deve ser "nickname_in_use"

  Cenário: Criar cartão com nome vazio retorna 400
    Quando o usuário tenta criar um cartão com nome "", apelido único, fechamento 5 e vencimento 12
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_card_name"

  Cenário: Criar cartão com nome acima de 64 caracteres retorna 400
    Quando o usuário tenta criar um cartão com nome de 65 caracteres
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_card_name"

  Cenário: Criar cartão com apelido vazio retorna 400
    Quando o usuário tenta criar um cartão com apelido ""
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_nickname"

  Cenário: Criar cartão com apelido acima de 32 caracteres retorna 400
    Quando o usuário tenta criar um cartão com apelido de 33 caracteres
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_nickname"

  Cenário: Criar cartão com closing_day zero retorna 400
    Quando o usuário tenta criar um cartão com fechamento 0 e vencimento 10
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_closing_day"

  Cenário: Criar cartão com closing_day 32 retorna 400
    Quando o usuário tenta criar um cartão com fechamento 32 e vencimento 10
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_closing_day"

  Cenário: Criar cartão com due_day zero retorna 400
    Quando o usuário tenta criar um cartão com fechamento 5 e vencimento 0
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_due_day"

  Cenário: Criar cartão com due_day 32 retorna 400
    Quando o usuário tenta criar um cartão com fechamento 5 e vencimento 32
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_due_day"

  Cenário: Criar cartão com limite negativo retorna 400
    Quando o usuário tenta criar um cartão com limite de -1 centavos
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_card_limit"

  Cenário: Criar cartão com limite acima do máximo retorna 400
    Quando o usuário tenta criar um cartão com limite de 100000001 centavos
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "card_limit_too_large"

  Cenário: Requisição idempotente com a mesma chave não duplica registro
    Quando o usuário inicia a criação do cartão "Idempotente" com limite 100000, fechamento 10, vencimento 20 e captura a chave de idempotência
    E o usuário reenvia a mesma requisição com a chave capturada
    Então a resposta HTTP deve ter status 201
    E deve existir exatamente 1 cartão com nome "Idempotente" no banco para o usuário
