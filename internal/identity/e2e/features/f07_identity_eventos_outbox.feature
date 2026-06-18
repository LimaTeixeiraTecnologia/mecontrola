# language: pt
Funcionalidade: Publicação de eventos de domínio via outbox

  Cenário: Deletar usuário publica evento user.deleted no outbox
    Dado que existe um usuário com whatsapp "+5511988880030" cadastrado no sistema
    Quando o usuário é deletado via use case
    Então o evento "user.deleted" deve estar registrado na outbox para o usuário

  Cenário: Estabelecer principal publica evento auth.principal_established no outbox
    Dado que existe um usuário com whatsapp "+5511988880031" cadastrado no sistema
    Quando o principal é estabelecido para o whatsapp "+5511988880031"
    Então o evento "auth.principal_established" deve estar registrado na outbox para o usuário

  Cenário: WhatsApp desconhecido publica evento auth.unknown_user no outbox
    Quando o principal é estabelecido para o whatsapp "+5511988880032" desconhecido
    Então o evento "auth.unknown_user" deve estar na outbox sem usuário associado

  Cenário: Falha de autenticação de gateway publica evento auth.failed no outbox
    Dado que existe um usuário com whatsapp "+5511988880033" cadastrado no sistema
    Quando uma falha de autenticação de gateway é registrada para o usuário
    Então o evento "auth.failed" deve estar registrado na outbox para o usuário
