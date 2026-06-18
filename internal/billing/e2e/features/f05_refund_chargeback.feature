# language: pt

Funcionalidade: Billing via webhook — reembolso e estorno

  Cenário: Reembolsar assinatura via order_refunded
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "order_refunded" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "REFUNDED"
    E o evento "billing.subscription.refunded" deve estar na outbox

  Cenário: Estorno via chargeback
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "chargeback" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "REFUNDED"
    E o evento "billing.subscription.refunded" deve estar na outbox

  Cenário: Replay idempotente de order_refunded
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "order_refunded" é enviado
    E o webhook billing "order_refunded" é reenviado com o mesmo order_id
    Então a resposta HTTP deve ter status 202
    E a outbox não deve ter duplicata para o evento "billing.subscription.refunded"
