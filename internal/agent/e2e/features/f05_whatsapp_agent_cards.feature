# language: pt
Funcionalidade: Gestão de cartões do agente financeiro via WhatsApp

  Cenário: Cadastrar cartão persiste e confirma ao usuário
    Dado que o usuário está ativo
    Quando o usuário envia "cadastra meu cartão inter" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de cartões do usuário aumentou em 1
    E o gateway respondeu ao usuário
    E a resposta do gateway contém "Cartão cadastrado"
    E o número de transações do usuário permaneceu igual

  Cenário: Cadastrar cartão é idempotente por WAMID
    Dado que o usuário está ativo
    Quando o usuário envia "cadastra meu cartão c6" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de cartões do usuário aumentou em 1
    Quando o usuário reenvia a última mensagem com o mesmo identificador
    Então a resposta HTTP deve ter status 200
    E o número de cartões do usuário permaneceu igual

  Cenário: Contar cartões é read-only e reflete a quantidade persistida
    Dado que o usuário está ativo
    Quando o usuário envia "quantos cartões eu tenho?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway informa a quantidade real de cartões do usuário
    E o número de cartões do usuário permaneceu igual
    E o número de transações do usuário permaneceu igual

  Cenário: Contar cartões reflete novo cadastro
    Dado que o usuário está ativo
    Quando o usuário envia "cadastra meu cartão will" via webhook
    Então a resposta HTTP deve ter status 200
    E o número de cartões do usuário aumentou em 1
    Quando o usuário envia "quantos cartões eu tenho?" via webhook
    Então a resposta HTTP deve ter status 200
    E o gateway respondeu ao usuário
    E a resposta do gateway informa a quantidade real de cartões do usuário
    E o número de transações do usuário permaneceu igual
