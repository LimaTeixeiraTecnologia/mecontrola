# language: pt
Funcionalidade: Consumer recomputa monthly summary ao receber eventos do outbox

  Cenário: criar transação dispara evento e consumer atualiza o monthly summary
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário cria uma transação de 4000 centavos com método "pix" e direção "outcome" em "2026-07-20"
    E o consumer processa os eventos pendentes do outbox
    Então a tabela monthly_summary deve conter registro para o usuário em "2026-07"

  Cenário: reprocessar o mesmo evento é idempotente para o monthly summary
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário cria uma transação de 2500 centavos com método "pix" e direção "outcome" em "2026-08-10"
    E o consumer processa os eventos pendentes do outbox
    E o consumer processa os eventos pendentes do outbox novamente
    Então a tabela monthly_summary deve conter exatamente 1 registro para o usuário em "2026-08"

  Cenário: deletar transação dispara evento e consumer mantém monthly summary consistente
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 5000 centavos com método "pix" e direção "outcome" em "2026-09-01"
    E o consumer processa os eventos pendentes do outbox
    Quando o usuário deleta a transação
    E o consumer processa os eventos pendentes do outbox
    Então a tabela monthly_summary deve conter registro para o usuário em "2026-09"
