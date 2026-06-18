# language: pt
Funcionalidade: Busca no dicionário de categorias

  Cenário: Retornar match inequívoco
    Quando o cliente busca no dicionario por "13º salário" em "income"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E o corpo da resposta deve conter "candidates" no campo "result"
    E o primeiro candidato deve conter signal_type "alias"
    E o primeiro candidato deve conter confidence "high"
    E o primeiro candidato deve conter match_reason

  Cenário: Retornar candidatos ambíguos
    Dado que existe um termo ambiguo "termounicocategoriese2e" no dicionario
    Quando o cliente busca no dicionario por "termounicocategoriese2e" em "expense"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter "candidates" no campo "result"
    E a resposta deve conter candidatos ambíguos

  Cenário: Retornar no_match para termo inexistente
    Quando o cliente busca no dicionario por "xyz123naoexiste" em "expense"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter "no_match" no campo "result"

  Cenário: Retornar no_match para kind incompatível
    Quando o cliente busca no dicionario por "energia" em "income"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter "no_match" no campo "result"

  Cenário: Rejeitar query vazia
    Quando o cliente busca no dicionario por query vazia
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_query"

  Cenário: Exigir autenticação para busca
    Quando o cliente nao autenticado busca no dicionario por "energia" em "expense"
    Então a resposta HTTP deve ter status 401
    E o corpo da resposta deve ser "{\"message\":\"unauthorized\"}"

  Cenário: Rejeitar query curta
    Quando o cliente busca no dicionario por query curta
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_query"

  Cenário: Rejeitar query com espaços
    Quando o cliente busca no dicionario por query com espacos
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_query"

  Cenário: Rejeitar ausência de kind
    Quando o cliente busca no dicionario sem informar kind
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_kind"

  Cenário: Rejeitar kind inválido
    Quando o cliente busca no dicionario com kind invalido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_kind"

  Cenário: Responder 304 com If-None-Match
    Quando o cliente busca no dicionario por "energia" em "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E uma nova consulta com o mesmo ETag deve retornar status 304
