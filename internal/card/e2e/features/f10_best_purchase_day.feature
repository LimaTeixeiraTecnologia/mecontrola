# language: pt
Funcionalidade: Consulta de melhor dia de compra

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Nubank com vencimento 20 retorna fechamento 13 e melhor dia 14
    Quando o usuário consulta o melhor dia de compra para banco "nubank" e vencimento 20
    Então a resposta HTTP deve ter status 200
    E o campo numérico "closing_day" da resposta deve ser 13
    E o campo numérico "best_purchase_day" da resposta deve ser 14

  Cenário: Banco desconhecido usa fallback de 7 dias
    Quando o usuário consulta o melhor dia de compra para banco "banco-inexistente" e vencimento 20
    Então a resposta HTTP deve ter status 200
    E o campo numérico "closing_day" da resposta deve ser 13
    E o campo numérico "best_purchase_day" da resposta deve ser 14

  Cenário: Consulta sem banco retorna 400
    Quando o usuário consulta o melhor dia de compra sem informar banco e vencimento 20
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "bank_required"

  Cenário: Consulta com due_day inválido retorna 400
    Quando o usuário consulta o melhor dia de compra para banco "nubank" e vencimento 0
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_due_day"
