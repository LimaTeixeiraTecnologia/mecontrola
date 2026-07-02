# language: pt
Funcionalidade: Criação de cartão via HTTP

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Criar cartão com banco e vencimento válidos retorna 201
    Quando o usuário cria um cartão com banco "nubank" e vencimento 20
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o corpo da resposta deve conter o campo "bank"
    E o corpo da resposta deve conter o campo "best_purchase_day"
    E o cartão deve estar persistido no banco com banco "nubank"

  Cenário: Criar cartão com banco desconhecido usa fallback de 7 dias
    Quando o usuário cria um cartão com banco "meu-banco-xpto" e vencimento 15
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "closing_day"

  Cenário: Criar cartão com nickname duplicado retorna 409
    Dado que já existe um cartão com o apelido "nick-dup-e2e"
    Quando o usuário tenta criar um cartão com o mesmo apelido "nick-dup-e2e"
    Então a resposta HTTP deve ter status 409
    E o campo de erro deve ser "nickname_in_use"

  Cenário: Criar cartão com apelido vazio retorna 400
    Quando o usuário tenta criar um cartão com apelido ""
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_nickname"

  Cenário: Criar cartão com apelido acima de 32 caracteres retorna 400
    Quando o usuário tenta criar um cartão com apelido de 33 caracteres
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_nickname"

  Cenário: Criar cartão com due_day zero retorna 400
    Quando o usuário tenta criar um cartão com banco "nubank" e vencimento 0
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_due_day"

  Cenário: Criar cartão com due_day 32 retorna 400
    Quando o usuário tenta criar um cartão com banco "nubank" e vencimento 32
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_due_day"

  Cenário: Requisição idempotente com a mesma chave não duplica registro
    Quando o usuário inicia a criação do cartão com banco "nubank" e vencimento 20 e captura a chave de idempotência
    E o usuário reenvia a mesma requisição com a chave capturada
    Então a resposta HTTP deve ter status 201
    E deve existir exatamente 1 cartão com o apelido capturado no banco para o usuário
