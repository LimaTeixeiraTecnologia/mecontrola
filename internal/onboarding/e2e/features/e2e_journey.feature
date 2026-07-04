# language: pt
Funcionalidade: Jornada completa de ativação via WhatsApp

  Cenário: Jornada feliz — billing PAID → state retorna mensagem de ativação → ativação via inbound → subscription_bound sem boas-vindas do consumer
    Dado que o ambiente de teste para onboarding está pronto
    E existe um token pendente com assinatura billing associada
    Quando o evento "billing.subscription.activated" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então o token atual deve estar marcado como pago
    E o cliente consulta o estado do token atual
    E o corpo da resposta deve ter status 200
    E a resposta deve conter wa_me_url com texto "Ativar+o+meu+plano"
    Quando o evento de tentativa de ativação é enfileirado para o número do cliente
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então o token atual deve estar consumido pelo usuário corrente
    E deve ter sido enviadas 0 mensagens de boas-vindas para o número do cliente
    E deve existir 1 evento(s) outbox do tipo "onboarding.subscription_bound"

  Cenário: Idempotência — reentrega do mesmo WAMID não gera boas-vindas do consumer
    Dado que o ambiente de teste para onboarding está pronto
    E existe um token pendente com assinatura billing associada
    Quando o evento "billing.subscription.activated" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então o token atual deve estar marcado como pago
    Quando o evento de tentativa de ativação é enfileirado para o número do cliente
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então o token atual deve estar consumido pelo usuário corrente
    Quando o mesmo evento de tentativa de ativação é reenviado
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então deve ter sido enviadas 0 mensagens de boas-vindas para o número do cliente

  Cenário: No-match throttle — número sem sessão PAID recebe resposta única
    Dado que o ambiente de teste para onboarding está pronto
    Quando o evento de tentativa de ativação é enfileirado para um número sem sessão PAID
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então deve ter sido enviada 1 mensagem de no-match para o número sem sessão
    Quando o evento de tentativa de ativação é enfileirado para um número sem sessão PAID
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então deve ter sido enviada 1 mensagem de no-match para o número sem sessão

  Cenário: Janela expirada — paidAt com mais de 24h não ativa
    Dado que o ambiente de teste para onboarding está pronto
    E existe um token com pagamento expirado há mais de 24 horas
    Quando o evento de tentativa de ativação é enfileirado para o número do token expirado
    E o dispatcher do outbox é executado com handlers reais e handlers da jornada completa
    Então o token deve permanecer com status "PAID"
    E nenhuma mensagem de boas-vindas deve ter sido enviada
