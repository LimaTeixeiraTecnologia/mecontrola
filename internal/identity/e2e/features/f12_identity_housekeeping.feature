# language: pt
Funcionalidade: Limpeza periódica de auth_events antigos via job

  Cenário: Job remove auth_events mais antigos que o período de retenção
    Dado que existe um usuário com whatsapp "+5511988880070" cadastrado no sistema
    E existe um auth_event antigo com occurred_at superior ao período de retenção
    Quando o job de housekeeping de auth_events é executado
    Então o auth_event antigo não deve mais existir no banco

  Cenário: Job preserva auth_events dentro do período de retenção
    Dado que existe um usuário com whatsapp "+5511988880071" cadastrado no sistema
    E existe um auth_event recente no banco
    Quando o job de housekeeping de auth_events é executado
    Então o auth_event recente deve continuar existindo no banco
