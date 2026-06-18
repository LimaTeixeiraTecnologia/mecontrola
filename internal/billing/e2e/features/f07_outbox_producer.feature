# language: pt

Funcionalidade: Billing — producer de eventos na outbox

  Cenário: Envelope de activated tem aggregate_type Subscription e campos obrigatórios no payload
    Dado que o produto billing está configurado
    Quando o webhook billing "order_approved" é enviado
    Então o envelope do evento "billing.subscription.activated" deve ter aggregate_type "Subscription"
    E o payload do evento "billing.subscription.activated" deve conter o campo "subscription_id"
    E o payload do evento "billing.subscription.activated" deve conter o campo "funnel_token"
    E o payload do evento "billing.subscription.activated" deve conter o campo "plan_code"
    E o payload do evento "billing.subscription.activated" deve conter o campo "period_end"
    E o payload do evento "billing.subscription.activated" deve conter o campo "occurred_at"

  Cenário: Envelope de renewed tem previous_period_end anterior ao period_end
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_renewed" é enviado
    Então o envelope do evento "billing.subscription.renewed" deve ter aggregate_type "Subscription"
    E o payload do evento "billing.subscription.renewed" deve ter previous_period_end anterior ao period_end

  Cenário: Envelope de past_due tem grace_end aproximadamente 3 dias após o period_end
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_late" é enviado
    Então o envelope do evento "billing.subscription.past_due" deve ter aggregate_type "Subscription"
    E o payload do evento "billing.subscription.past_due" deve ter grace_end aproximadamente 3 dias após occurred_at

  Cenário: Envelope de canceled tem period_end no payload
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_canceled" é enviado
    Então o envelope do evento "billing.subscription.canceled" deve ter aggregate_type "Subscription"
    E o payload do evento "billing.subscription.canceled" deve conter o campo "period_end"

  Cenário: Envelope de refunded tem subscription_id no payload
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "order_refunded" é enviado
    Então o envelope do evento "billing.subscription.refunded" deve ter aggregate_type "Subscription"
    E o payload do evento "billing.subscription.refunded" deve conter o campo "subscription_id"
