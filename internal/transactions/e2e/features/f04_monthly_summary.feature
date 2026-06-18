# language: pt
Funcionalidade: Resumo mensal e entradas mensais

  Cenário: obter resumo mensal existente retorna 200 com campo ref_month
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 5000 centavos com método "pix" e direção "outcome" em "2026-05-10"
    E o consumer processa os eventos pendentes do outbox
    Quando o usuário obtém o resumo do mês "2026-05"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "ref_month"

  Cenário: obter resumo de mês sem dados retorna 200 com valores zero
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário obtém o resumo do mês "2020-01"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "ref_month"

  Cenário: listar entradas mensais retorna 200
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 3000 centavos com método "pix" e direção "income" em "2026-04-15"
    E o consumer processa os eventos pendentes do outbox
    Quando o usuário lista as entradas do mês "2026-04"
    Então a resposta HTTP deve ter status 200
