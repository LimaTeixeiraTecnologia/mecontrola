# language: pt
Funcionalidade: Gestão de cartão e alertas de fatura

  Cenário: Criar novo cartão com limite definido
    Dado existe um usuário autenticado
    Quando o usuário cria um cartão "Nubank" com limite de R$ 5000,00, fechamento 5 e vencimento 12
    Então a resposta HTTP deve ter status 201
    E o cartão deve estar salvo no banco com limite de R$ 5000,00
    E o corpo da resposta deve conter o campo "id"

  Cenário: Aumentar o limite de um cartão existente
    Dado o usuário já possui um cartão com limite de R$ 1000,00
    Quando o usuário solicita o aumento para R$ 2000,00
    Então a resposta HTTP deve ter status 200
    E a leitura do cartão deve refletir o limite de R$ 2000,00

  Cenário: Gerar alerta de vencimento de fatura sem duplicidade
    Dado o usuário possui um cartão com fatura vencendo em 2 dias
    Quando o worker de alertas de fatura é executado
    Então deve existir 1 evento do tipo "card.invoice_due.v1" no outbox para o cartão
    E o payload do evento deve referenciar o cartão e o vencimento em 2 dias
    Quando o worker de alertas de fatura é executado novamente
    Então deve existir apenas 1 registro de alerta para o cartão
    E deve existir 1 evento do tipo "card.invoice_due.v1" no outbox para o cartão
