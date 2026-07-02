# language: pt
Funcionalidade: Cálculo de fatura e alertas de vencimento

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Compra antes do fechamento usa o ciclo corrente
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário consulta a fatura do cartão para a data "2025-06-05"
    Então a resposta HTTP deve ter status 200
    E o campo "closing_date" da fatura deve ser "2025-06-13"
    E o campo "due_date" da fatura deve ser "2025-06-20"

  Cenário: Compra no dia do fechamento usa o ciclo corrente
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário consulta a fatura do cartão para a data "2025-06-13"
    Então a resposta HTTP deve ter status 200
    E o campo "closing_date" da fatura deve ser "2025-06-13"
    E o campo "due_date" da fatura deve ser "2025-06-20"

  Cenário: Compra após o fechamento usa o próximo ciclo
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário consulta a fatura do cartão para a data "2025-06-14"
    Então a resposta HTTP deve ter status 200
    E o campo "closing_date" da fatura deve ser "2025-07-13"
    E o campo "due_date" da fatura deve ser "2025-07-20"

  Cenário: Data inválida retorna 400
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário consulta a fatura do cartão para a data "01-01-2025"
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "invalid_purchase_date"

  Cenário: Sem parâmetro "for" retorna 400
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário consulta a fatura do cartão sem informar a data
    Então a resposta HTTP deve ter status 400
    E o campo de erro deve ser "missing_for_param"

  Cenário: Cartão inexistente retorna 404
    Quando o usuário consulta a fatura de um cartão com ID aleatório inexistente para a data "2025-06-01"
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Cartão excluído retorna 404
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário exclui o cartão cadastrado
    E o usuário consulta a fatura do cartão pelo ID cadastrado para a data "2025-06-01"
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Worker de alertas dispara evento no outbox e notifica via canal
    Dado o usuário possui um cartão com fatura vencendo em 2 dias
    Quando o worker de alertas de fatura é executado
    Então deve existir 1 evento do tipo "card.invoice_due.v1" no outbox para o cartão
    E o payload do evento deve referenciar o cartão e o vencimento em 2 dias
