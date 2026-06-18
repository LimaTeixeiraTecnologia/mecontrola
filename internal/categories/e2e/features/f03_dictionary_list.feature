# language: pt
Funcionalidade: Listagem do dicionário de categorias

  Cenário: Listar primeira página do dicionário
    Quando o cliente lista o dicionario de categorias
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E a resposta deve conter version maior que zero
    E a resposta deve retornar uma lista nao vazia no campo "entries"

  Cenário: Filtrar o dicionário por kind
    Quando o cliente lista o dicionario de categorias com kind "expense"
    Então a resposta HTTP deve ter status 200
    E todas as entradas do dicionario devem ter kind "expense"

  Cenário: Filtrar o dicionário por category_id
    Quando o cliente lista o dicionario da categoria "delivery" em "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve retornar uma lista nao vazia no campo "entries"
    E todas as entradas do dicionario devem ter category_id da categoria "delivery" em "expense"

  Cenário: Filtrar o dicionário por signal_type
    Quando o cliente lista o dicionario com signal_type "canonical_name"
    Então a resposta HTTP deve ter status 200
    E todas as entradas do dicionario devem ter signal_type "canonical_name"

  Cenário: Paginar a listagem do dicionário
    Quando o cliente lista o dicionario com page_size 2
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter next_cursor
    Quando o cliente lista a proxima pagina do dicionario
    Então a resposta HTTP deve ter status 200
    E a resposta deve retornar uma lista nao vazia no campo "entries"

  Cenário: Rejeitar kind inválido
    Quando o cliente lista o dicionario com kind invalido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_kind"

  Cenário: Exigir autenticação para listagem do dicionário
    Quando o cliente nao autenticado lista o dicionario de categorias
    Então a resposta HTTP deve ter status 401
    E o corpo da resposta deve ser "{\"message\":\"unauthorized\"}"

  Cenário: Rejeitar signal_type inválido
    Quando o cliente lista o dicionario com signal_type invalido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_query"

  Cenário: Responder 304 com If-None-Match
    Quando o cliente lista o dicionario de categorias com kind "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E uma nova consulta com o mesmo ETag deve retornar status 304
