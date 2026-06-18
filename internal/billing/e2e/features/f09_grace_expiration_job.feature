# language: pt

Funcionalidade: Billing — job de expiração de grace period

  Cenário: Assinatura PAST_DUE com grace expirado muda para EXPIRED e publica evento
    Dado que a assinatura está em PAST_DUE com grace expirado
    Quando o job de expiração de grace é executado
    Então a assinatura billing deve estar salva como "EXPIRED"
    E o evento "billing.subscription.expired_after_grace" deve estar na outbox
    E o envelope do evento "billing.subscription.expired_after_grace" deve ter aggregate_type "Subscription"
    E o payload do evento "billing.subscription.expired_after_grace" deve conter o campo "subscription_id"
    E o payload do evento "billing.subscription.expired_after_grace" deve conter o campo "period_end"
    E o payload do evento "billing.subscription.expired_after_grace" deve conter o campo "grace_end"
    E o payload do evento "billing.subscription.expired_after_grace" deve conter o campo "occurred_at"

  Cenário: Assinatura PAST_DUE com grace vigente permanece PAST_DUE sem evento
    Dado que a assinatura está em PAST_DUE com grace vigente
    Quando o job de expiração de grace é executado
    Então a assinatura billing deve estar salva como "PAST_DUE"
    E nenhum evento de expiração deve estar na outbox
