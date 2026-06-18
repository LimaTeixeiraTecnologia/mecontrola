# language: pt
Funcionalidade: Consumo de eventos do módulo de cartão

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Consumer onboarding cria cartão a partir de evento
    Quando o consumer recebe o evento "onboarding.card_registered" com nome "Cartão Onboarding", limite 100000, fechamento 5 e vencimento 12
    Então o cartão deve estar persistido no banco para o usuário

  Cenário: InvoiceDueNotifier envia notificação ao receber evento de vencimento
    Dado que o usuário possui um cartão criado com nome "Notif Fatura", fechamento 5, vencimento 12 e limite 100000
    E existe um registro de alerta pendente para o cartão com vencimento em 2 dias
    Quando o consumer recebe o evento "card.invoice_due.v1" para o cartão criado com vencimento em 2 dias
    Então a gateway de canal deve ter recebido ao menos 1 mensagem para o usuário

  Cenário: Reprocessar evento de onboarding com mesmo event_id não cria cartão duplicado
    Quando o consumer recebe o evento "onboarding.card_registered" com nome "Cartão Idem", limite 50000, fechamento 3 e vencimento 10
    E o mesmo evento de onboarding é reprocessado com o mesmo event_id
    Então o banco deve conter exatamente 1 cartão com aquele nome para o usuário

  Cenário: Consumer InvoiceDueNotifier é idempotente para vencimento já notificado
    Dado que o usuário possui um cartão criado com nome "Notif Idem", fechamento 5, vencimento 12 e limite 100000
    E existe um registro de alerta pendente para o cartão com vencimento em 2 dias
    Quando o consumer recebe o evento "card.invoice_due.v1" para o cartão criado com vencimento em 2 dias
    E o mesmo evento de vencimento é reprocessado
    Então a gateway de canal deve ter recebido exatamente 1 mensagem para o usuário
