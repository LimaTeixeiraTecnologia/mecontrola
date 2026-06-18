# language: pt
Funcionalidade: Leituras e casos de erro determinísticos do agente financeiro via WhatsApp

  Cenário: Listar lançamentos é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de transações do usuário aumentou em 1
    Quando o usuário envia "lista meus lançamentos" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Lançamentos"
    E o número de transações do usuário permaneceu igual

  Cenário: Resumo mensal é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "resumo do mês" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Resumo"
    E o número de transações do usuário permaneceu igual

  Cenário: Como estou indo é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "como estou indo?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E o número de transações do usuário permaneceu igual

  Cenário: Consultar categoria é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "quanto gastei em prazeres?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E o número de transações do usuário permaneceu igual

  Cenário: Consultar meta é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "como está minha meta?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E o número de transações do usuário permaneceu igual

  Cenário: Consultar fatura do cartão é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "qual a fatura do nubank?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Fatura"
    E o número de transações do usuário permaneceu igual

  Cenário: Listar cartões é read-only e lista o cartão cadastrado
    Dado que o usuário está ativo
    Quando o usuário envia "meus cartões" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "cartões"
    E o número de transações do usuário permaneceu igual

  Cenário: Listar recorrências é read-only e responde ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "todo mês recebo 5000 no dia 5" via webhook
    Então a resposta HTTP deve ter status 200
    Quando o usuário envia "quais minhas recorrências?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Recorrências"
    E o número de transações do usuário permaneceu igual

  Cenário: Assinatura inválida é rejeitada e não persiste transação
    Dado que o usuário está ativo
    Quando o usuário envia "gastei 50 no mercado" via webhook com assinatura inválida
    Então a resposta HTTP deve ter status 401
    E o número de transações do usuário permaneceu igual

  Cenário: Compra parcelada em cartão inexistente não persiste e responde com honestidade
    Dado que o usuário está ativo
    Quando o usuário envia "comprei 300 no cartão fantasma" via webhook
    Então a resposta HTTP deve ter status 200
    E nenhuma compra parcelada foi persistida
    E o gateway respondeu ao usuário
    E a resposta do gateway não contém "registrada"
