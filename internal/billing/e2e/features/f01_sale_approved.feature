# language: pt

Funcionalidade: Billing via webhook — order_approved

  Cenário: Ativar nova assinatura com funnel token
    Dado que o produto billing está configurado
    Quando o webhook billing "order_approved" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"
    E o evento "billing.subscription.activated" deve estar na outbox
    E o evento processado "order_approved" deve ter sido registrado

  Cenário: Ativar assinatura sem parâmetros de rastreamento
    Dado que o produto billing está configurado
    Quando o webhook "order_approved" é enviado sem parâmetros de rastreamento
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"
    E o evento "billing.subscription.activated_without_token" deve estar na outbox
    E o payload do evento "billing.subscription.activated_without_token" deve conter o campo "subscription_id"
    E o payload do evento "billing.subscription.activated_without_token" deve conter o campo "plan_code"
    E o payload do evento "billing.subscription.activated_without_token" deve conter o campo "occurred_at"

  Cenário: Replay idempotente com mesmo order_id
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "order_approved" é reenviado com o mesmo order_id
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"
    E deve existir exatamente 1 evento "billing.subscription.activated" na outbox
