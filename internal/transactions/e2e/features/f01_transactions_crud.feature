# language: pt
Funcionalidade: CRUD de transactions via HTTP

  Cenário: criar transação de despesa persiste no banco e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    E que não existe nenhuma transação para o usuário em "2026-06"
    Quando o usuário cria uma transação de 5800 centavos com método "pix" e direção "outcome" em "2026-06-15"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter exatamente 1 transação nova para o usuário em "2026-06"
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.created.v1"

  Cenário: criar transação com payload inválido retorna 400
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário envia uma requisição POST para "/api/v1/transactions" com payload inválido
    Então a resposta HTTP deve ter status 400

  Cenário: criar transação sem autenticação retorna 401
    Dado que o ambiente E2E de transactions está pronto
    Quando uma requisição não autenticada envia POST para "/api/v1/transactions"
    Então a resposta HTTP deve ter status 401

  Cenário: obter transação existente retorna 200 com dados corretos
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 3000 centavos com método "credit_card" e direção "outcome" em "2026-06-10"
    Quando o usuário obtém a transação pelo ID
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter a transação com valor 3000 centavos

  Cenário: obter transação inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta obter uma transação com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: listar transações retorna somente as do usuário autenticado em "2026-06"
    Dado que o ambiente E2E de transactions está pronto
    E que existem 2 transações criadas para o usuário em "2026-06"
    Quando o usuário lista transações do mês "2026-06"
    Então a resposta HTTP deve ter status 200

  Cenário: atualizar transação persiste mudança no banco e enfileira evento de atualização
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 5000 centavos com método "pix" e direção "outcome" em "2026-06-10"
    Quando o usuário atualiza a transação para 7000 centavos
    Então a resposta HTTP deve ter status 200
    E o banco deve conter a transação com valor 7000 centavos
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.updated.v1"

  Cenário: atualizar transação inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta atualizar uma transação com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: deletar transação faz soft-delete no banco e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 2000 centavos com método "pix" e direção "outcome" em "2026-06-01"
    Quando o usuário deleta a transação
    Então a resposta HTTP deve ter status 204
    E a transação deve ter deleted_at preenchido no banco
    E a transação não deve aparecer na listagem do mês "2026-06"
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.deleted.v1"

  Cenário: deletar transação inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta deletar uma transação com ID inexistente
    Então a resposta HTTP deve ter status 404
