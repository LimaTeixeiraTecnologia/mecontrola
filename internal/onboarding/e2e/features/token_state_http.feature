# language: pt
Funcionalidade: Estado público de token de onboarding

  Cenário: Consultar token pronto para ativação
    Dado existe um token pago pronto para ativação
    Quando o cliente consulta o estado do token atual
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve indicar ready_to_activate verdadeiro
    E o corpo da resposta deve conter o campo "wa_me_url"

  Cenário: Consultar token pendente
    Dado existe um token pendente
    Quando o cliente consulta o estado do token atual
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve indicar ready_to_activate falso

  Cenário: Consultar token expirado
    Dado existe um token expirado
    Quando o cliente consulta o estado do token atual
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve indicar ready_to_activate falso

  Cenário: Consultar token inexistente
    Quando o cliente consulta o estado de um token inexistente
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve indicar ready_to_activate falso
