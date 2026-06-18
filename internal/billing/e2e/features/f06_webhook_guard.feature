# language: pt
Funcionalidade: Billing via webhook — validações e guards

  Cenário: Assinatura HMAC inválida retorna 401
    Dado que o produto billing está configurado
    Quando o webhook é enviado sem assinatura HMAC
    Então a resposta HTTP deve ter status 401

  Cenário: Content-Type incorreto retorna 415
    Dado que o produto billing está configurado
    Quando o webhook é enviado com Content-Type incorreto
    Então a resposta HTTP deve ter status 415

  Cenário: Corpo JSON malformado retorna 422 com código invalid_json
    Dado que o produto billing está configurado
    Quando o webhook é enviado com corpo JSON inválido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_json"

  Cenário: Trigger desconhecido retorna 422 com código unknown_trigger
    Dado que o produto billing está configurado
    Quando o webhook é enviado com trigger desconhecido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "unknown_trigger"

  Cenário: subscription_id inválido retorna 422 com código invalid_kiwify_subscription_id
    Dado que o produto billing está configurado
    Quando o webhook é enviado com subscription_id inválido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_kiwify_subscription_id"

  Cenário: sck com apenas espaços retorna 422 com código funnel_token_missing
    Dado que o produto billing está configurado
    Quando o webhook é enviado com sck contendo apenas espaços
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "funnel_token_missing"
