# language: pt
Funcionalidade: Listagem de categorias

  Cenário: Listar categorias raiz de despesa
    Quando o cliente lista categorias de "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E a resposta deve conter version maior que zero
    E a resposta deve conter as categorias raiz "Conhecimento, Custo Fixo, Liberdade Financeira, Metas, Prazeres"
    E todas as categorias retornadas devem ter kind "expense"
    E o campo "categories" deve ser uma lista ordenada alfabeticamente por "name"

  Cenário: Listar categorias raiz de receita
    Quando o cliente lista categorias de "income"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E a resposta deve conter version maior que zero
    E a resposta deve retornar uma lista nao vazia no campo "categories"
    E todas as categorias retornadas devem ter kind "income"

  Cenário: Listar subcategorias por parent_id
    Quando o cliente lista subcategorias de "expense" em "prazeres"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter version maior que zero
    E a resposta deve conter a subcategoria "Delivery"
    E a resposta deve conter a subcategoria "Restaurantes"
    E todas as categorias retornadas devem ter kind "expense"

  Cenário: Listar subcategorias incluindo deprecated
    Dado que existe uma categoria deprecated em "prazeres" com slug "categoria-deprecated-e2e"
    Quando o cliente lista subcategorias incluindo deprecated de "expense" em "prazeres"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter a subcategoria "Categoria Deprecated E2E"

  Cenário: Rejeitar kind inválido
    Quando o cliente lista categorias com kind invalido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_kind"

  Cenário: Exigir autenticação para listagem
    Quando o cliente nao autenticado lista categorias de "expense"
    Então a resposta HTTP deve ter status 401
    E o corpo da resposta deve ser "{\"message\":\"unauthorized\"}"

  Cenário: Rejeitar parent_id inválido
    Quando o cliente lista categorias com parent_id invalido
    Então a resposta HTTP deve ter status 422
    E o campo de erro deve ser "invalid_query"

  Cenário: Responder 304 com If-None-Match
    Quando o cliente lista categorias de "expense"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o header ETag
    E uma nova consulta com o mesmo ETag deve retornar status 304
