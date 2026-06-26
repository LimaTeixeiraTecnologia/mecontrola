Feature: Onboarding conversacional do agente

  Scenario: Jornada feliz completa das 8 etapas
    Given que existe um usuario com sessao de onboarding iniciada
    When o usuario enviar "oi" com message_id "m1"
    Then o agente deve responder contendo "Bem-vindo"
    When o usuario enviar "sim" com message_id "m2"
    Then o agente deve responder contendo "objetivo"
    When o usuario enviar "quitar dividas" com message_id "m3"
    Then o agente deve responder contendo "recebe"
    When o usuario enviar "5000" com message_id "m4"
    Then o agente deve responder contendo "cartao"
    When o usuario enviar "Nubank 15" com message_id "m5"
    Then o agente deve responder contendo "Outro"
    When o usuario enviar "nao" com message_id "m6"
    Then o agente deve responder contendo "categorias"
    When o usuario enviar "ok" com message_id "m7"
    Then o agente deve responder contendo "fixed_cost"
    When o usuario enviar "2000" com message_id "m8"
    Then o agente deve responder contendo "knowledge"
    When o usuario enviar "500" com message_id "m9"
    Then o agente deve responder contendo "pleasures"
    When o usuario enviar "500" com message_id "m10"
    Then o agente deve responder contendo "goals"
    When o usuario enviar "1000" com message_id "m11"
    Then o agente deve responder contendo "financial_freedom"
    When o usuario enviar "1000" com message_id "m12"
    Then o agente deve responder contendo "Resumo"
    When o usuario enviar "sim" com message_id "m13"
    Then o agente deve responder contendo "Pronto"
    And o estado da sessao de onboarding deve ser "active"
    And deve existir 1 evento(s) outbox do tipo "onboarding.completed"

  Scenario: Comando diario durante o onboarding redireciona sem registrar objetivo
    Given que existe um usuario com sessao de onboarding iniciada
    When o usuario enviar "oi" com message_id "m1"
    And o usuario enviar "gastei 50 mercado" com message_id "m2"
    Then o agente deve responder contendo "Termine"
    And o objetivo persistido deve ser ""

  Scenario: Retomada apos interrupcao do runtime
    Given que existe um usuario com sessao de onboarding iniciada
    When o usuario enviar "oi" com message_id "m1"
    Then o agente deve responder contendo "Bem-vindo"
    When o runtime do agente for reiniciado
    And o usuario enviar "sim" com message_id "m2"
    Then o agente deve responder contendo "objetivo"

  Scenario: Correcao de objetivo no resumo
    Given que existe um usuario com sessao de onboarding iniciada
    When o usuario enviar "oi" com message_id "m1"
    And o usuario enviar "sim" com message_id "m2"
    And o usuario enviar "viajar" com message_id "m3"
    And o usuario enviar "5000" com message_id "m4"
    And o usuario enviar "nao" com message_id "m5"
    And o usuario enviar "ok" com message_id "m6"
    And o usuario enviar "2000" com message_id "m7"
    And o usuario enviar "500" com message_id "m8"
    And o usuario enviar "500" com message_id "m9"
    And o usuario enviar "1000" com message_id "m10"
    And o usuario enviar "1000" com message_id "m11"
    And o usuario enviar "corrigir objetivo para comprar casa" com message_id "m12"
    Then o agente deve responder contendo "comprar casa"
    And o objetivo persistido deve ser "comprar casa"
