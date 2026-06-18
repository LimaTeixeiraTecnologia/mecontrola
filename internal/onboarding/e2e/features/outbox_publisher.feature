# language: pt
Funcionalidade: Publicação de eventos onboarding no outbox

  Cenário: consumo de token PAID via WhatsApp publica subscription_bound no outbox
    Dado que o ambiente de teste para onboarding está pronto
    E que existe um token PAID válido com mobile "+5511900000099" e email "outbox@test.com"
    Quando o número "+5511900000099" envia a mensagem de ativação com o token
    Então a tabela outbox_events deve conter 1 evento "onboarding.subscription_bound"
    E o aggregate_type do evento deve ser "onboarding_token"

  Cenário: token já consumido não gera segundo evento no outbox
    Dado que o ambiente de teste para onboarding está pronto
    E que existe um token já consumido com activation_path "direct"
    Quando o número "+5511900000099" tenta reativar com o mesmo token
    Então a tabela outbox_events deve conter 0 eventos "onboarding.subscription_bound"
