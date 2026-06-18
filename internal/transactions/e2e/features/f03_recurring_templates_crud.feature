# language: pt
Funcionalidade: CRUD de recurring-templates via HTTP

  Cenário: criar recurring-template persiste no banco e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário cria um recurring-template de 1500 centavos com frequência "monthly" no dia 5 e direção "outcome"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter 1 recurring-template novo para o usuário
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.recurring_template.created.v1"

  Cenário: criar recurring-template com payload inválido retorna 400
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário envia uma requisição POST para "/api/v1/recurring-templates" com payload inválido
    Então a resposta HTTP deve ter status 400

  Cenário: criar recurring-template sem autenticação retorna 401
    Dado que o ambiente E2E de transactions está pronto
    Quando uma requisição não autenticada envia POST para "/api/v1/recurring-templates"
    Então a resposta HTTP deve ter status 401

  Cenário: obter recurring-template existente retorna 200
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template criado de 2000 centavos com frequência "monthly" no dia 10 e direção "outcome"
    Quando o usuário obtém o recurring-template pelo ID
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"

  Cenário: obter recurring-template inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta obter um recurring-template com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: listar recurring-templates retorna lista do usuário
    Dado que o ambiente E2E de transactions está pronto
    E que existem 2 recurring-templates criados para o usuário
    Quando o usuário lista recurring-templates
    Então a resposta HTTP deve ter status 200

  Cenário: atualizar recurring-template persiste mudança e enfileira evento de atualização
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template criado de 1000 centavos com frequência "monthly" no dia 15 e direção "outcome"
    Quando o usuário atualiza o recurring-template para 1200 centavos
    Então a resposta HTTP deve ter status 200
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.recurring_template.updated.v1"

  Cenário: atualizar recurring-template inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta atualizar um recurring-template com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: deletar recurring-template faz soft-delete e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template criado de 800 centavos com frequência "monthly" no dia 20 e direção "outcome"
    Quando o usuário deleta o recurring-template
    Então a resposta HTTP deve ter status 204
    E o recurring-template deve ter deleted_at preenchido no banco
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.recurring_template.deleted.v1"

  Cenário: deletar recurring-template inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta deletar um recurring-template com ID inexistente
    Então a resposta HTTP deve ter status 404
