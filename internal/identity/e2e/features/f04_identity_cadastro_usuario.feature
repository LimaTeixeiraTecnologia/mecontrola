# language: pt
Funcionalidade: Cadastro e gerenciamento de usuários via HTTP

  Cenário: Criar novo usuário com WhatsApp e email válidos
    Quando o sistema recebe um cadastro com whatsapp "+5511988880001" e email "novo@example.com"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"
    E o usuário deve estar salvo no banco com whatsapp "+5511988880001" e status "ACTIVE"

  Cenário: Atualizar display_name de usuário existente mantendo o mesmo WhatsApp
    Dado que existe um usuário com whatsapp "+5511988880002" cadastrado no sistema
    Quando o sistema recebe um cadastro com whatsapp "+5511988880002" e display_name "Atualizado"
    Então a resposta HTTP deve ter status 200
    E o usuário deve estar salvo no banco com display_name "Atualizado"

  Cenário: Criar usuário sem email pois email é opcional
    Quando o sistema recebe um cadastro com whatsapp "+5511988880003" sem email
    Então a resposta HTTP deve ter status 200
    E o usuário deve estar salvo no banco com whatsapp "+5511988880003" e status "ACTIVE"

  Cenário: Rejeitar cadastro com WhatsApp de linha fixa sem indicador de celular
    Quando o sistema recebe um cadastro com whatsapp "+551133334444" e email "a@b.com"
    Então a resposta HTTP deve ter status 400

  Cenário: Rejeitar cadastro com email em formato inválido
    Quando o sistema recebe um cadastro com whatsapp "+5511988880005" e email inválido "nao-e-email"
    Então a resposta HTTP deve ter status 400

  Cenário: Reanimar usuário deletado dentro da janela de graça de 24 horas
    Dado que o usuário com whatsapp "+5511988880006" foi deletado há 2 horas
    Quando o sistema recebe um cadastro com whatsapp "+5511988880006"
    Então a resposta HTTP deve ter status 200
    E o usuário deve estar salvo no banco com whatsapp "+5511988880006" e status "ACTIVE"

  Cenário: Criar novo usuário quando deleted ultrapassou a janela de graça de 24 horas
    Dado que o usuário com whatsapp "+5511988880007" foi deletado há 48 horas
    Quando o sistema recebe um cadastro com whatsapp "+5511988880007"
    Então a resposta HTTP deve ter status 200
    E um novo registro de usuário deve existir no banco com whatsapp "+5511988880007"

  Cenário: Rejeitar cadastro com email já vinculado a outro usuário ativo
    Dado que existe um usuário com whatsapp "+5511988880008" cadastrado com email "ocupado@example.com"
    Quando o sistema recebe um cadastro com whatsapp "+5511988880009" e email "ocupado@example.com"
    Então a resposta HTTP deve ter status 409
