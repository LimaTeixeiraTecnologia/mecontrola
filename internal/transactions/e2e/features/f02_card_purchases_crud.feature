# language: pt
Funcionalidade: CRUD de card-purchases via HTTP

  Cenário: criar card-purchase em 3 parcelas persiste itens e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário cria uma compra de 9000 centavos em 3 parcelas no cartão em "2026-06-15"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter 3 parcelas para a card-purchase criada
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.card_purchase.created.v1"

  Cenário: criar card-purchase com payload inválido retorna 400
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário envia uma requisição POST para "/api/v1/card-purchases" com payload inválido
    Então a resposta HTTP deve ter status 400

  Cenário: criar card-purchase sem autenticação retorna 401
    Dado que o ambiente E2E de transactions está pronto
    Quando uma requisição não autenticada envia POST para "/api/v1/card-purchases"
    Então a resposta HTTP deve ter status 401

  Cenário: obter card-purchase existente retorna 200 com dados corretos
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    E que existe uma card-purchase criada de 6000 centavos em 2 parcelas no cartão em "2026-06-10"
    Quando o usuário obtém a card-purchase pelo ID
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"

  Cenário: obter card-purchase inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta obter uma card-purchase com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: listar card-purchases retorna lista do usuário
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    E que existem 2 card-purchases criadas para o usuário
    Quando o usuário lista card-purchases
    Então a resposta HTTP deve ter status 200

  Cenário: atualizar card-purchase persiste mudança e enfileira evento de atualização
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    E que existe uma card-purchase criada de 6000 centavos em 2 parcelas no cartão em "2026-06-10"
    Quando o usuário atualiza a descrição da card-purchase
    Então a resposta HTTP deve ter status 200
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.card_purchase.updated.v1"

  Cenário: atualizar card-purchase inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta atualizar uma card-purchase com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: deletar card-purchase faz soft-delete e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    E que existe uma card-purchase criada de 3000 centavos em 1 parcela no cartão em "2026-06-05"
    Quando o usuário deleta a card-purchase
    Então a resposta HTTP deve ter status 204
    E a card-purchase deve ter deleted_at preenchido no banco
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.card_purchase.deleted.v1"

  Cenário: deletar card-purchase inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta deletar uma card-purchase com ID inexistente
    Então a resposta HTTP deve ter status 404
