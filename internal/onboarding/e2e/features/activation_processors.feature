# language: pt
Funcionalidade: Processors de ativação

  Cenário: Ativar via WhatsApp consome token e publica subscription_bound
    Dado existe um token pago com assinatura e dados do cliente
    Quando o processor de WhatsApp recebe um comando de ativação com o token atual
    Então o token atual deve estar consumido pelo usuário corrente
    E deve existir 1 evento(s) outbox do tipo "onboarding.subscription_bound"
