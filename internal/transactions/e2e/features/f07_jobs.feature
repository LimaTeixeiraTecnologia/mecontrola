# language: pt
Funcionalidade: Jobs de materialização e reconciliação

  Cenário: job de materialização cria transação para recurring-template ativo no dia
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template ativo de 2000 centavos com frequência "monthly" para o dia de hoje
    Quando o job recurring-materializer é executado para hoje
    Então o banco deve conter pelo menos 1 materialização para o recurring-template no mês atual

  Cenário: job de materialização é idempotente ao ser executado duas vezes no mesmo dia
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template ativo de 1500 centavos com frequência "monthly" para o dia de hoje
    E que o job recurring-materializer já foi executado para hoje
    Quando o job recurring-materializer é executado para hoje
    Então o banco deve conter exatamente 1 materialização para o recurring-template no mês atual

  Cenário: job de reconciliação monthly-summary executa sem erro
    Dado que o ambiente E2E de transactions está pronto
    Quando o job monthly-summary-reconciler é executado
    Então a resposta do job deve ser sucesso
