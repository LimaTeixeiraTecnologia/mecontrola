# language: pt
Funcionalidade: Escritas determinísticas do agente financeiro via WhatsApp

  Cenário: Compra parcelada persiste e recomputa o resumo mensal
    Dado que o usuário está ativo
    Quando o usuário envia "parcelei 1200 em 6x no nubank" via webhook
    Então a resposta HTTP deve ter status 200
    E deve existir 1 compra parcelada de 120000 em 6x
    E o evento "transactions.card_purchase.created.v1" deve estar no outbox do usuário
    Quando o outbox é processado
    Então o resumo mensal do usuário reflete a despesa
    E o orçamento do usuário registrou 6 parcelas da compra

  Cenário: Recorrência de receita persiste e emite evento
    Dado que o usuário está ativo
    Quando o usuário envia "todo mês recebo 5000 no dia 5" via webhook
    Então a resposta HTTP deve ter status 200
    E deve existir 1 recorrência de 500000 no dia 5
    E o evento "transactions.recurring_template.created.v1" deve estar no outbox do usuário

  Cenário: Edição da última transação atualiza valor e versão
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    Quando o usuário envia "na verdade foi 80" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Você deseja atualizar"
    Quando o usuário envia "sim" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E o valor da última transação do usuário deve ser 8000
    E a versão da última transação do usuário deve ser pelo menos 1
    E o evento "transactions.transaction.updated.v1" deve estar no outbox do usuário
    Quando o outbox é processado
    Então o resumo mensal do usuário reflete a despesa

  Cenário: Exclusão da última transação faz soft-delete e remove do orçamento
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    Quando o usuário envia "apaga o último" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Você deseja apagar"
    Quando o usuário envia "sim" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a última transação do usuário deve estar excluída
    E o número de transações ativas do usuário diminuiu em 1
    E o evento "transactions.transaction.deleted.v1" deve estar no outbox do usuário
    Quando o outbox é processado
    Então a despesa do orçamento do usuário foi removida
