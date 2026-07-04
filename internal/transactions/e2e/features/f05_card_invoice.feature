# language: pt
Funcionalidade: Fatura do cartão

  Cenário: obter fatura do cartão após criação de compra no crédito retorna 200
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário com fechamento no dia 10
    E que existe uma compra no crédito de 4000 centavos em 1 parcela no cartão em "2026-03-05"
    Quando o usuário obtém a fatura do cartão para "2026-03"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "card_id"

  Cenário: obter fatura de mês sem compras retorna 404
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário com fechamento no dia 10
    Quando o usuário obtém a fatura do cartão para "2020-01"
    Então a resposta HTTP deve ter status 404
