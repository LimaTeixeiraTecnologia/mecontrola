# language: pt

Funcionalidade: Billing via webhook — subscription_late

  Cenário: Marcar assinatura como PAST_DUE
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_late" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "PAST_DUE"
    E o evento "billing.subscription.past_due" deve estar na outbox

  Cenário: Replay idempotente de subscription_late
    Dado que o produto billing está configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_late" é enviado
    E o webhook billing "subscription_late" é reenviado com o mesmo timestamp
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "PAST_DUE"
    E deve existir exatamente 1 evento "billing.subscription.past_due" na outbox
