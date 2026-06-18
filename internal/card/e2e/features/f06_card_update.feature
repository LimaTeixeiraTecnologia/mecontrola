# language: pt
Funcionalidade: Atualização de cartão e controle de versão do limite

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Atualizar nome via PUT retorna 200 com nome novo
    Dado que o usuário possui um cartão criado com nome "Antes Update", fechamento 5, vencimento 12 e limite 100000
    Quando o usuário atualiza o cartão informando apenas o nome "Depois Update"
    Então a resposta HTTP deve ter status 200
    E o campo texto "name" da resposta deve ser "Depois Update"

  Cenário: Atualizar closing_day e due_day via PUT retorna 200
    Dado que o usuário possui um cartão criado com nome "Ciclo Update", fechamento 5, vencimento 12 e limite 100000
    Quando o usuário atualiza o cartão informando fechamento 15 e vencimento 25
    Então a resposta HTTP deve ter status 200
    E o campo numérico "closing_day" da resposta deve ser 15
    E o campo numérico "due_day" da resposta deve ser 25

  Cenário: Atualizar cartão inexistente via PUT retorna 404
    Quando o usuário tenta atualizar um cartão com ID inexistente informando o nome "Qualquer"
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Atualizar cartão com nome de 65 caracteres retorna 400
    Dado que o usuário possui um cartão criado com nome "Nome Antes PUT", fechamento 5, vencimento 12 e limite 100000
    Quando o usuário tenta atualizar o cartão com nome de 65 caracteres
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_card_name"

  Cenário: Atualizar limite via PATCH sem expected_version retorna 200
    Dado que o usuário possui um cartão criado com nome "Limite PATCH", fechamento 5, vencimento 12 e limite 200000
    Quando o usuário atualiza o limite do cartão para 400000 centavos
    Então a resposta HTTP deve ter status 200
    E o campo numérico "limit_cents" da resposta deve ser 400000

  Cenário: Atualizar limite com expected_version correto retorna 200 e incrementa versão
    Dado que o usuário possui um cartão criado com nome "Versão OK", fechamento 5, vencimento 12 e limite 200000
    Quando o usuário atualiza o limite do cartão para 500000 centavos com expected_version 1
    Então a resposta HTTP deve ter status 200
    E o campo numérico "limit_cents" da resposta deve ser 500000
    E a versão do cartão no banco deve ser 2

  Cenário: Rejeitar atualização de limite com expected_version desatualizado retorna 409
    Dado que o usuário possui um cartão criado com nome "Versão Conflito", fechamento 5, vencimento 12 e limite 200000
    Quando o usuário atualiza o limite do cartão para 500000 centavos com expected_version 0
    Então a resposta HTTP deve ter status 409
    E o campo de erro deve ser "card_limit_version_conflict"

  Cenário: Segundo conflito de versão após atualização bem-sucedida
    Dado que o usuário possui um cartão criado com nome "Duplo Conflito", fechamento 5, vencimento 12 e limite 200000
    Quando o usuário atualiza o limite do cartão para 300000 centavos com expected_version 1
    Então a resposta HTTP deve ter status 200
    Quando o usuário atualiza o limite do cartão para 400000 centavos com expected_version 1
    Então a resposta HTTP deve ter status 409
    E o campo de erro deve ser "card_limit_version_conflict"

  Cenário: Atualizar limite com valor negativo retorna 400
    Dado que o usuário possui um cartão criado com nome "Limite Negativo", fechamento 5, vencimento 12 e limite 100000
    Quando o usuário tenta atualizar o limite do cartão para -1 centavos
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_card_limit"

  Cenário: Atualizar limite acima do máximo retorna 400
    Dado que o usuário possui um cartão criado com nome "Limite Max", fechamento 5, vencimento 12 e limite 100000
    Quando o usuário tenta atualizar o limite do cartão para 100000001 centavos
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "card_limit_too_large"

  Cenário: Atualizar limite de cartão inexistente retorna 404
    Quando o usuário tenta atualizar o limite de um cartão com ID inexistente para 100000 centavos
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"
