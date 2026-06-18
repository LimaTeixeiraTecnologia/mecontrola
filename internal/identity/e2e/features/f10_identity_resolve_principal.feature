# language: pt
Funcionalidade: Resolução de principal via canal de identidade

  Cenário: Resolver principal via canal Telegram vinculado retorna UserID correto
    Dado que existe um usuário com whatsapp "+5511988880060" cadastrado no sistema
    E o canal "telegram" com external_id "tg123456e2e" é vinculado ao usuário
    Quando o principal é resolvido pelo canal "telegram" e external_id "tg123456e2e"
    Então o principal resolvido deve ter o UserID do usuário cadastrado

  Cenário: Canal desconhecido na resolução de principal retorna erro
    Quando o principal é resolvido pelo canal "telegram" e external_id "tg_nao_existe_e2e"
    Então a resolução de principal deve retornar erro
