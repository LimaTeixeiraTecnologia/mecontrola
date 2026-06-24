# language: pt
Funcionalidade: Processors de ativação

  Cenário: Ativar via WhatsApp e abrir sessão de onboarding via outbox
    Dado existe um token pago com assinatura e dados do cliente
    Quando o processor de WhatsApp recebe um comando de ativação com o token atual
    E o dispatcher processa o evento onboarding.subscription_bound
    Então o token atual deve estar consumido pelo usuário corrente
    E deve existir 1 evento(s) outbox do tipo "onboarding.subscription_bound"
    E deve existir uma sessão de onboarding em estado "in_progress"

  Cenário: Ativar via Telegram direto
    Dado existe um token pago com assinatura e dados do cliente
    Quando o processor do Telegram recebe um comando de ativação com o token atual
    E o dispatcher processa o evento onboarding.subscription_bound
    Então o processor retorna a mensagem "telegram-welcome"
    E o token atual deve estar consumido pelo usuário corrente
    E deve existir uma sessão de onboarding em estado "in_progress"
