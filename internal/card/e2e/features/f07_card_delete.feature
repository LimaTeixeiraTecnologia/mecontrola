# language: pt
Funcionalidade: Exclusão de cartão

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Excluir cartão existente retorna 204
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário exclui o cartão cadastrado
    Então a resposta HTTP deve ter status 204
    E o cartão deve estar marcado como excluído no banco

  Cenário: Buscar cartão excluído retorna 404
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário exclui o cartão cadastrado
    E o usuário busca o cartão pelo ID cadastrado
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Excluir cartão inexistente retorna 404
    Quando o usuário tenta excluir um cartão com ID aleatório inexistente
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"

  Cenário: Cartão excluído não aparece na listagem
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário exclui o cartão cadastrado
    E o usuário lista todos os seus cartões
    Então o cartão excluído não deve constar na lista retornada

  Cenário: Excluir cartão já excluído retorna 404
    Dado que o usuário possui um cartão criado com banco "nubank" e vencimento 20
    Quando o usuário exclui o cartão cadastrado
    E o usuário tenta excluir o mesmo cartão novamente
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "card_not_found"
