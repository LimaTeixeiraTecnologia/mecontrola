# language: pt
Funcionalidade: Editar e apagar lançamento por referência via WhatsApp

  Cenário: Apagar o Uber com resultado único confirma e exclui
    Dado que o usuário está ativo
    E o usuário possui um lançamento "Uber" de 3500 centavos
    Quando o usuário envia "apaga o uber" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Você deseja apagar"
    E a resposta do gateway contém "Uber"
    Quando o usuário envia "sim" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Lançamento excluído!"
    E o número de transações ativas do usuário diminuiu em 1

  Cenário: Apagar o mercado com múltiplos resultados pede escolha antes de excluir
    Dado que o usuário está ativo
    E o usuário possui um lançamento "Mercado" de 12000 centavos
    E o usuário possui um lançamento "Mercado Extra" de 8500 centavos
    E o usuário possui um lançamento "Mercadinho" de 4300 centavos
    Quando o usuário envia "apaga o mercado" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Encontrei mais de um lançamento"
    E a resposta do gateway contém "Responda com o número."
    E o número de transações do usuário permaneceu igual
    Quando o usuário envia "2" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Você deseja apagar"
    Quando o usuário envia "sim" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Lançamento excluído!"
    E o número de transações ativas do usuário diminuiu em 1

  Cenário: Editar o Uber por referência atualiza o valor
    Dado que o usuário está ativo
    E o usuário possui um lançamento "Uber" de 3500 centavos
    Quando o usuário envia "o uber foi 42 e não 35" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Você deseja atualizar"
    Quando o usuário envia "sim" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Lançamento atualizado!"
