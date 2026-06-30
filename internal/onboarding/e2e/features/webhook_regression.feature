# language: pt
Funcionalidade: Regressão do webhook de billing (RF-01..05)

  Cenário: Evento billing.subscription.activated sem token não ativa conta
    Dado que o ambiente de teste para onboarding está pronto
    Quando o evento "billing.subscription.activated_without_token" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handlers reais
    Então deve existir um support signal do tipo "paid_without_token"
    E nenhum token deve estar com status PAID

  Cenário: Reentrega idempotente do billing event não duplica email
    Dado que o ambiente de teste para onboarding está pronto
    E existe um token pendente com assinatura billing associada
    Quando o evento "billing.subscription.activated" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handlers reais
    Então o token atual deve estar marcado como pago
    E deve ter sido enviado 1 email(s) de ativação
    Quando o evento "billing.subscription.activated" é enfileirado novamente com o mesmo ID
    E o dispatcher do outbox é executado com handlers reais
    Então deve ter sido enviado 1 email(s) de ativação
