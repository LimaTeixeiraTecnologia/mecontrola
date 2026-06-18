# language: pt
Funcionalidade: Consumers de billing via outbox

  Cenário: Processar subscription activated e marcar token como pago
    Dado existe um token pendente com assinatura billing associada
    Quando o evento "billing.subscription.activated" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handlers reais
    Então o token atual deve estar marcado como pago
    E deve ter sido enviado 1 email(s) de ativação
    E o último evento outbox "billing.subscription.activated" deve estar com status 3

  Cenário: Processar pagamento sem token
    Quando o evento "billing.subscription.activated_without_token" é enfileirado na outbox de integração
    E o dispatcher do outbox é executado com handlers reais
    Então deve existir um support signal do tipo "paid_without_token"
    E o último evento outbox "billing.subscription.activated_without_token" deve estar com status 3

  Cenário: consumer de email ignora evento sem customer_email
    Dado que o ambiente de teste para onboarding está pronto
    E que existe um token PENDING para o plano mensal
    E que um evento billing.subscription.activated sem customer_email é disparado para o token
    Quando o consumer de email processa o evento
    Então nenhum email deve ter sido enviado pelo gateway de email

  Cenário: consumer de email envia email quando token e email estão presentes
    Dado que o ambiente de teste para onboarding está pronto
    E que existe um token PENDING para o plano mensal
    E que um evento billing.subscription.activated com customer_email "user@e2e.test" é disparado para o token
    Quando o consumer de email processa o evento
    Então exatamente 1 email de ativação deve ter sido enviado
