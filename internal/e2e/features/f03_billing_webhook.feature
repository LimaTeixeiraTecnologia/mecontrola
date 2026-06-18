# language: pt
Funcionalidade: Billing via webhook Kiwify

  Cenário: Ativar nova assinatura após pagamento aprovado
    Dado existe um produto billing configurado
    Quando o webhook billing "order_approved" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"
    E o evento de domínio "billing.subscription.activated" deve estar na outbox
    E o evento processado "order_approved" deve ter sido registrado

  Cenário: Cancelar assinatura ativa
    Dado existe um produto billing configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_canceled" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "CANCELED_PENDING"
    E o period_end da assinatura billing deve ser preservado
    E o evento de domínio "billing.subscription.canceled" deve estar na outbox

  Cenário: Renovar assinatura ativa
    Dado existe um produto billing configurado
    E que existe uma assinatura billing ativa
    Quando o webhook billing "subscription_renewed" é enviado
    Então a resposta HTTP deve ter status 202
    E a assinatura billing deve estar salva como "ACTIVE"
    E o period_end da assinatura billing deve ser estendido
    E o evento de domínio "billing.subscription.renewed" deve estar na outbox
