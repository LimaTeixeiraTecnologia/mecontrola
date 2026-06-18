# language: pt
Funcionalidade: Vinculação de canal e resolução de identidade multi-plataforma

  Cenário: Vincular canal Telegram a usuário existente e verificar no banco
    Dado que existe um usuário com whatsapp "+5511988880010" cadastrado no sistema
    Quando o canal "telegram" com external_id "123456789" é vinculado ao usuário
    Então a vinculação deve estar salva no banco com canal "telegram" e external_id "123456789"

  Cenário: Resolver canal preferido quando apenas Telegram está vinculado
    Dado que existe um usuário com whatsapp "+5511988880011" cadastrado no sistema
    E o canal "telegram" com external_id "987654321" foi vinculado ao usuário
    Quando o canal preferido do usuário é consultado
    Então o canal preferido resolvido deve ser "telegram"

  Cenário: Rejeitar vinculação duplicada do mesmo canal para o mesmo usuário
    Dado que existe um usuário com whatsapp "+5511988880012" cadastrado no sistema
    E o canal "telegram" com external_id "111111111" foi vinculado ao usuário
    Quando o canal "telegram" com external_id "111111111" é vinculado novamente ao mesmo usuário
    Então a operação de vinculação deve retornar erro de canal já vinculado

  Cenário: Rejeitar vinculação de external_id já associado a outro usuário
    Dado que existe um usuário com whatsapp "+5511988880013" cadastrado no sistema
    E o canal "telegram" com external_id "999999999" foi vinculado ao usuário
    E que existe um segundo usuário com whatsapp "+5511988880014" cadastrado no sistema
    Quando o canal "telegram" com external_id "999999999" é vinculado ao segundo usuário
    Então a operação de vinculação deve retornar erro de canal já vinculado
