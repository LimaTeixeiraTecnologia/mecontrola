# language: pt
Funcionalidade: Idempotência no consumo de eventos de identity

  Cenário: Reprocessar auth.principal_established com mesmo event_id não duplica auth_event
    Dado que existe um usuário com whatsapp "+5511988880050" cadastrado no sistema
    Quando o evento de auth "auth.principal_established" é projetado para o usuário com envelope fixo
    E o mesmo envelope de auth é reprocessado para o usuário
    Então deve existir exatamente 1 auth_event do tipo "principal_established" para o usuário

  Cenário: Reprocessar billing.subscription.activated não duplica entitlement
    Dado que existe um usuário com whatsapp "+5511988880051" cadastrado no sistema
    Quando o evento de assinatura "billing.subscription.activated" é projetado para o usuário com status "ACTIVE"
    E o evento de assinatura "billing.subscription.activated" é projetado novamente para o usuário com status "ACTIVE"
    Então o entitlement do usuário deve estar salvo no banco com status "ACTIVE"
    E deve existir exatamente 1 entitlement para o usuário
