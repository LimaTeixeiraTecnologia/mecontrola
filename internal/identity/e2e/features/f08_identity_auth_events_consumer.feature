# language: pt
Funcionalidade: Consumo e projeção de eventos de autenticação

  Cenário: Projetar evento auth.principal_established salva registro na tabela de auth_events
    Dado que existe um usuário com whatsapp "+5511988880040" cadastrado no sistema
    Quando o evento de auth "auth.principal_established" é projetado para o usuário
    Então um auth_event do tipo "principal_established" deve existir no banco para o usuário

  Cenário: Projetar evento auth.unknown_user salva registro sem user_id
    Quando o evento de auth "auth.unknown_user" é projetado sem usuário associado
    Então um auth_event do tipo "unknown_user" deve existir no banco sem user_id

  Cenário: Projetar evento user.deleted anonimiza auth_events do usuário
    Dado que existe um usuário com whatsapp "+5511988880041" cadastrado no sistema
    E o evento de auth "auth.principal_established" foi projetado para o usuário
    E um auth_event do tipo "principal_established" deve existir no banco para o usuário
    Quando o evento de auth "user.deleted" é projetado para o usuário
    Então os auth_events do usuário devem estar com user_id nulo
