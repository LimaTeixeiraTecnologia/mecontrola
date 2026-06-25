# language: pt
Funcionalidade: Edições conversacionais do agente financeiro via WhatsApp

  Cenário: Editar apelido do cartão persiste e confirma ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "muda o apelido do cartão nubank pra roxinho" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Cartão atualizado"
    E o apelido do cartão "Nubank" passou a ser "roxinho"
    E o número de cartões do usuário permaneceu igual
    E o número de transações do usuário permaneceu igual

  Cenário: Editar vencimento do cartão persiste e confirma ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "troca o vencimento do cartão roxinho pro dia 25" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Cartão atualizado"
    E o vencimento do cartão "roxinho" passou a ser dia 25
    E o número de cartões do usuário permaneceu igual

  Cenário: Editar cartão inexistente pede esclarecimento e não persiste
    Dado que o usuário está ativo
    Quando o usuário envia "muda o apelido do cartão fantasma pra qualquer" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway não contém "Cartão atualizado"
    E a resposta do gateway contém "Não encontrei"
    E o número de cartões do usuário permaneceu igual

  Cenário: Apagar cartão faz soft-delete e confirma ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "apaga o cartão roxinho" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Você deseja remover"
    Quando o usuário envia "sim" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Cartão removido"
    E o cartão "roxinho" não aparece mais na listagem
    E o número de transações do usuário permaneceu igual

  Cenário: Apagar cartão inexistente pede esclarecimento e não persiste
    Dado que o usuário está ativo
    Quando o usuário envia "apaga o cartão fantasma" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway não contém "Cartão apagado"
    E a resposta do gateway contém "Não encontrei"
    E o número de cartões do usuário permaneceu igual

  Cenário: Editar percentual de categoria rebalanceia e preserva a soma
    Dado que o usuário está ativo
    E o usuário possui um orçamento ativo
    Quando o usuário envia "coloca 30% em prazeres" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Orçamento ajustado"
    E o percentual da categoria "expense.prazeres" passou a ser 30%
    E a soma dos percentuais do orçamento permanece 100%

  Cenário: Editar percentual de categoria desconhecida pede esclarecimento
    Dado que o usuário está ativo
    E o usuário possui um orçamento ativo
    Quando o usuário envia "coloca 20% em viagens" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway não contém "Orçamento ajustado"
    E a soma dos percentuais do orçamento permanece 100%
