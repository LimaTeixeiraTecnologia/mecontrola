# language: pt
Funcionalidade: Resolução de principal via canal de identidade

  Cenário: Resolver principal via canal WhatsApp vinculado retorna UserID correto
    Dado que existe um usuário com whatsapp "+5511988880060" cadastrado no sistema
    E o canal "whatsapp" com external_id "+5511900000060" é vinculado ao usuário
    Quando o principal é resolvido pelo canal "whatsapp" e external_id "+5511900000060"
    Então o principal resolvido deve ter o UserID do usuário cadastrado

  Cenário: External_id sem vínculo na resolução de principal retorna erro
    Quando o principal é resolvido pelo canal "whatsapp" e external_id "+5511900099999"
    Então a resolução de principal deve retornar erro
