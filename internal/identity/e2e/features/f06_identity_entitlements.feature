# language: pt
Funcionalidade: Projeção de entitlements a partir de eventos de assinatura

  Cenário: Projetar entitlement ativo ao receber evento de ativação de assinatura
    Dado que existe um usuário com whatsapp "+5511988880020" cadastrado no sistema
    Quando o evento de assinatura "billing.subscription.activated" é projetado para o usuário com status "ACTIVE"
    Então o entitlement do usuário deve estar salvo no banco com status "ACTIVE"

  Cenário: Atualizar entitlement para PAST_DUE ao receber evento de inadimplência
    Dado que existe um usuário com whatsapp "+5511988880021" cadastrado no sistema
    E o evento de assinatura "billing.subscription.activated" foi projetado para o usuário com status "ACTIVE"
    Quando o evento de assinatura "billing.subscription.past_due" é projetado para o usuário com status "PAST_DUE"
    Então o entitlement do usuário deve estar salvo no banco com status "PAST_DUE"

  Cenário: Marcar entitlement como EXPIRED ao receber evento de cancelamento
    Dado que existe um usuário com whatsapp "+5511988880022" cadastrado no sistema
    E o evento de assinatura "billing.subscription.activated" foi projetado para o usuário com status "ACTIVE"
    Quando o evento de assinatura "billing.subscription.canceled" é projetado para o usuário com status "EXPIRED"
    Então o entitlement do usuário deve estar salvo no banco com status "EXPIRED"

  Cenário: Projetar entitlement REFUNDED ao receber evento de reembolso
    Dado que existe um usuário com whatsapp "+5511988880023" cadastrado no sistema
    E o evento de assinatura "billing.subscription.activated" foi projetado para o usuário com status "ACTIVE"
    Quando o evento de assinatura "billing.subscription.refunded" é projetado para o usuário com status "REFUNDED"
    Então o entitlement do usuário deve estar salvo no banco com status "REFUNDED"
