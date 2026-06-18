# language: pt

Funcionalidade: Billing via webhook — subscription_renewed

  Cenário: Renovar assinatura ativa
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_renewed" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"
    E o periodo period_end deve ser estendido
    E o evento "billing.subscription.renewed" deve estar na outbox

  Cenário: Evento retroativo não modifica status da assinatura
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook "subscription_late" é reenviado com data anterior ao primeiro evento
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"

  Cenário: Replay idempotente de subscription_renewed
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_renewed" é enviado
    E o webhook billing "subscription_renewed" é reenviado com o mesmo timestamp
    Então a resposta HTTP deve ter status 202
    E deve existir exatamente 1 evento "billing.subscription.renewed" na outbox
