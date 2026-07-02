# language: pt
Funcionalidade: Consumo de eventos do módulo de cartão

  Contexto:
    Dado existe um usuário autenticado

  Cenário: InvoiceDueNotifier envia notificação ao receber evento de vencimento
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    E existe um registro de alerta pendente para o cartão com vencimento em 2 dias
    Quando o consumer recebe o evento "card.invoice_due.v1" para o cartão criado com vencimento em 2 dias
    Então a gateway de canal deve ter recebido ao menos 1 mensagem para o usuário

  Cenário: Consumer InvoiceDueNotifier é idempotente para vencimento já notificado
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    E existe um registro de alerta pendente para o cartão com vencimento em 2 dias
    Quando o consumer recebe o evento "card.invoice_due.v1" para o cartão criado com vencimento em 2 dias
    E o mesmo evento de vencimento é reprocessado
    Então a gateway de canal deve ter recebido exatamente 1 mensagem para o usuário
