# language: pt
Funcionalidade: Robustez de onboarding

  Cenário: Fallback por WhatsApp consome token e abre sessão
    Dado existe um token pago elegível para fallback por WhatsApp
    Quando o processor de WhatsApp recebe uma tentativa de fallback do número atual
    Então a última mensagem WhatsApp enviada deve ser "wa-welcome"
    E deve existir 1 evento(s) outbox do tipo "onboarding.subscription_bound"
    Quando o dispatcher processa o evento onboarding.subscription_bound
    Então o token atual deve estar consumido pelo usuário corrente
    E deve existir uma sessão de onboarding em estado "awaiting_income"

  Cenário: Reutilização de token consumido cria sinal de suporte
    Dado existe um token já consumido por outro usuário
    Quando o processor de WhatsApp recebe uma tentativa de ativação com reutilização do token
    Então a última mensagem WhatsApp enviada deve ser "wa-code-used"
    E deve existir um support signal do tipo "token_reuse_attempt"
    E o token atual deve permanecer com status "CONSUMED"

  Cenário: Ativação direta no Telegram bloqueada por falta de dados
    Dado existe um token pago sem dados suficientes para ativação direta no Telegram
    Quando o processor do Telegram recebe uma ativação sem dados suficientes
    Então o processor retorna a mensagem "telegram-requires-whatsapp"
    E deve existir 0 evento(s) outbox do tipo "onboarding.subscription_bound"
    E o token atual deve permanecer com status "PAID"

  Cenário: Evento sem handlers vai para failed
    Quando o evento "billing.subscription.activated_without_token" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado sem handlers registrados
    Então o último evento outbox "billing.subscription.activated_without_token" deve estar com status 4
    E o último evento outbox "billing.subscription.activated_without_token" deve conter erro "no handlers registered"

  Cenário: Handler falhando mantém evento pendente para retry
    Quando o evento "billing.subscription.activated_without_token" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handler que falha
    Então o último evento outbox "billing.subscription.activated_without_token" deve estar com status 1
    E o último evento outbox "billing.subscription.activated_without_token" deve ter attempts 1
    E o último evento outbox "billing.subscription.activated_without_token" deve conter erro "forced dispatch failure"

  Cenário: Outreach 4xx mantém marcação sem reset
    Dado existe um token pago elegível para outreach via WhatsApp
    E o gateway de outreach responde erro 4xx
    Quando o job de outreach é executado
    Então deve ter sido enviado 0 template(s) de outreach
    E o token atual deve ter outreach_sent_at preenchido

  Cenário: Outreach 5xx reseta marcação para retry
    Dado existe um token pago elegível para outreach via WhatsApp
    E o gateway de outreach responde erro 5xx
    Quando o job de outreach é executado
    Então deve ter sido enviado 0 template(s) de outreach
    E o token atual deve ter outreach_sent_at nulo

  Cenário: Conversa de onboarding publica evento de renda
    Dado existe um token pago com assinatura e dados do cliente
    Quando o processor de WhatsApp recebe um comando de ativação com o token atual
    E o dispatcher processa o evento onboarding.subscription_bound
    Então deve existir uma sessão de onboarding em estado "awaiting_income"
    Quando o usuário informa renda "3500" via WhatsApp
    Então a última mensagem WhatsApp enviada deve ser "Otimo! Quer cadastrar um cartao de credito agora? Responda sim ou nao."
    E deve existir 1 evento(s) outbox do tipo "onboarding.income_registered"
    E deve existir uma sessão de onboarding em estado "awaiting_card_decision"

  Cenário: excesso de requisições simultâneas ao checkout retorna 429
    Dado que o ambiente de teste para onboarding está pronto
    Quando o cliente envia requisições em excesso para o endpoint de checkout
    Então pelo menos uma resposta deve ter status 429
