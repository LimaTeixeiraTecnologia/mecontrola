# language: pt
Funcionalidade: Consulta de categoria

  Cenário: Obter categoria raiz com subcategorias
    Quando o cliente consulta a categoria "prazeres" em "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E o corpo da resposta deve conter "Prazeres" no campo "name"
    E o corpo da resposta deve conter o campo "subcategories"
    E a resposta deve conter o path "Prazeres"

  Cenário: Obter subcategoria com path completo
    Quando o cliente consulta a categoria "delivery" em "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E o corpo da resposta deve conter "Delivery" no campo "name"
    E a resposta deve conter o path "Prazeres > Delivery"

  Cenário: Retornar 404 para categoria inexistente
    Quando o cliente consulta uma categoria inexistente
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "not_found"

  Cenário: Exigir autenticação para consulta por id
    Quando o cliente nao autenticado consulta a categoria "prazeres" em "expense"
    Então a resposta HTTP deve ter status 401
    E o corpo da resposta deve ser "{\"message\":\"unauthorized\"}"

  Cenário: Ocultar categoria deprecated por padrão
    Quando o cliente consulta a categoria deprecated "categoria-deprecated-e2e-get" em "prazeres"
    Então a resposta HTTP deve ter status 404
    E o campo de erro deve ser "not_found"

  Cenário: Permitir categoria deprecated com include_deprecated
    Quando o cliente consulta a categoria deprecated "categoria-deprecated-e2e-get" em "prazeres" incluindo deprecated
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter "Categoria Deprecated E2E" no campo "name"

  Cenário: Rejeitar id inválido
    Quando o cliente consulta uma categoria com id invalido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_query"

  Cenário: Responder 304 com If-None-Match
    Quando o cliente consulta a categoria "prazeres" em "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E uma nova consulta com o mesmo ETag deve retornar status 304
