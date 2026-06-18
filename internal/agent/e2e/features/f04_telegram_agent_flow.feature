# language: pt
Funcionalidade: Fluxo Telegram do agente financeiro

  Cenário: Registrar despesa via Telegram persiste transação
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook do telegram
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    E o gateway do telegram respondeu ao usuário
    E o evento "transactions.transaction.created.v1" deve estar no outbox do usuário
    E o evento "agent.intent.executed.v1" deve estar no outbox do usuário

  Cenário: Intenção desconhecida via Telegram não escreve nada
    Dado que o usuário está ativo
    Quando o usuário envia "ação não suportada" via webhook do telegram
    Então a resposta HTTP deve ter status 200
    E nenhuma transação foi persistida
    E o gateway do telegram respondeu ao usuário
    E o evento "agent.intent.rejected.v1" deve estar no outbox do usuário
