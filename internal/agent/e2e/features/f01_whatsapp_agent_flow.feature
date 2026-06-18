# language: pt
Funcionalidade: Fluxo WhatsApp do agente financeiro

  Cenário: Registrar despesa persiste transação
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    E o gateway respondeu ao usuário
    E o evento "transactions.transaction.created.v1" deve estar no outbox do usuário
    E o evento "agent.intent.executed.v1" deve estar no outbox do usuário

  Cenário: Consumo do evento de transação atualiza o resumo mensal
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o evento "transactions.transaction.created.v1" deve estar no outbox do usuário
    Quando o outbox é processado
    Então o resumo mensal do usuário reflete a despesa
    E a despesa do orçamento do usuário foi registrada

  Cenário: Registrar receita persiste transação
    Dado que o usuário está ativo
    Quando o usuário envia "recebi salário de 3000" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    E o gateway respondeu ao usuário

  Cenário: Intenção desconhecida não escreve nada
    Dado que o usuário está ativo
    Quando o usuário envia "ação não suportada" via webhook
    Então a resposta HTTP deve ter status 200
    E nenhuma transação foi persistida
    E o gateway respondeu ao usuário
    E a resposta do gateway não contém "registrei"
    E a resposta do gateway não contém "anotei"
    E o evento "agent.intent.rejected.v1" deve estar no outbox do usuário

  Cenário: Isolamento entre usuários
    Dado que o usuário está ativo
    E existe um segundo usuário "+5511977776666" ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o segundo usuário não vê transações novas

  Cenário: Idempotência por WAMID
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    Quando o usuário reenvia a última mensagem com o mesmo identificador
    Então a resposta HTTP deve ter status 200
    E nenhuma transação foi persistida
