# language: pt

Funcionalidade: Billing via webhook — subscription_canceled

  Cenário: Cancelar assinatura ativa
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_canceled" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "CANCELED_PENDING"
    E o period_end da assinatura deve ser preservado
    E o evento "billing.subscription.canceled" deve estar na outbox

  Cenário: Replay idempotente de subscription_canceled
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_canceled" é enviado
    E o webhook billing "subscription_canceled" é reenviado com o mesmo order_id
    Então a resposta HTTP deve ter status 202
    E a outbox não deve ter duplicata para o evento "billing.subscription.canceled"
