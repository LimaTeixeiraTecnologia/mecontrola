# language: pt
Funcionalidade: Checkout público de onboarding

  Cenário: Criar checkout session válida
    Quando o cliente cria uma checkout session com plan_id "monthly"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "checkout_url"
    E o token de onboarding deve estar persistido

  Cenário: Rejeitar JSON inválido
    Quando o cliente envia um payload inválido para checkout
    Então a resposta HTTP deve ter status 400

  Cenário: Rejeitar plano desconhecido
    Quando o cliente cria uma checkout session com plan_id desconhecido "unknown-plan"
    Então a resposta HTTP deve ter status 400

  Cenário: Responder preflight com origin permitida
    Quando o cliente faz preflight de checkout com origin permitida
    Então a resposta HTTP deve ter status 204
