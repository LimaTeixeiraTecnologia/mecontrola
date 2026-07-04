# language: pt
Funcionalidade: Compra no crédito via CRUD unificado de transações

  Cenário: criar compra no crédito em 3 parcelas persiste itens de fatura e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário cria uma compra no crédito de 9000 centavos em 3 parcelas no cartão em "2026-06-15"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter 3 parcelas para a transação criada
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.created.v1"

  Cenário: criar compra no crédito à vista (1 parcela) vincula à fatura
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário cria uma compra no crédito de 3000 centavos em 1 parcelas no cartão em "2026-06-05"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter 1 parcelas para a transação criada

  Cenário: deletar compra no crédito faz soft-delete e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    E que existe uma compra no crédito de 6000 centavos em 2 parcelas no cartão em "2026-06-10"
    Quando o usuário deleta a transação no crédito
    Então a resposta HTTP deve ter status 204
    E a transação no crédito deve ter deleted_at preenchido no banco
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.deleted.v1"

  Cenário: rotas legadas de card-purchase foram removidas e retornam 404
    Dado que o ambiente E2E de transactions está pronto
    Então uma requisição POST para "/api/v1/card-purchases" retorna status 404
    E uma requisição GET para "/api/v1/card-purchases" retorna status 404
