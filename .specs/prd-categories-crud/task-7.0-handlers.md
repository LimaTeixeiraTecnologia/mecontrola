# Tarefa 7.0: Handlers HTTP, router, RequireUser, ETag/304 e envelope de erro

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar handlers HTTP finos, `CategoryRouter` com aplicação de `RequireUser`, suporte a ETag/304 (`If-None-Match`), e envelope de erro reutilizando `responses.ErrorWithDetails` do devkit-go. Seguir R-ADAPTER-001: zero comentários, adaptadores finos, fluxo `handler → usecase`.

<requirements>
- RF-07: não existem endpoints de create, update, delete, clone, restore ou hide
- RF-08–RF-18a: endpoints de leitura
- RF-18a: ETag + version em toda resposta de leitura (inclusive erros 404/422)
- RT-11: envelope de erro reutilizado de billing/identity
- ADR-001: RequireUser em todos os endpoints
- R-ADAPTER-001: handlers finos, sem lógica de negócio, SQL ou branching de domínio
</requirements>

## Subtarefas

- [ ] 7.1 Implementar `ListCategoriesHandler`
- [ ] 7.2 Implementar `GetCategoryHandler`
- [ ] 7.3 Implementar `ListDictionaryHandler`
- [ ] 7.4 Implementar `SearchDictionaryHandler`
- [ ] 7.5 Implementar `CategoryRouter` com `RequireUser` e rotas
- [ ] 7.6 Implementar middleware/helper de ETag/304 (pode ser função reutilizável)
- [ ] 7.7 Unit tests para handlers com mocks de use cases

## Detalhes de Implementação

Ver techspec.md seção **Endpoints de API** e ADR-001.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar `references/api.md` e `references/architecture.md` (R-ADAPTER-001.3)
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`
- R-ADAPTER-001: handler chama use case; nunca repositório, SQL ou branching de domínio

Pontos críticos:
- Handlers são apenas porta de entrada: decodificam input, chamam use case, codificam response.
- ETag: em toda resposta de leitura (sucesso ou erro), incluir header `ETag: "v<N>"` e `version` no corpo.
- 304: se `If-None-Match` corresponder à versão atual, retornar `304 Not Modified` sem corpo.
- Erros: usar `responses.ErrorWithDetails(w, status, message, map[string]string{"code": "..."})`.
- Códigos: `invalid_query`, `invalid_kind`, `not_found`.
- `RequireUser`: aplicar via `.With(middleware.RequireUser)` nas rotas do `CategoryRouter`.
- DTOs de request/response separados de entidades de domínio.

## Critérios de Sucesso

- [ ] Todos os 4 endpoints respondem com `ETag` e `version`
- [ ] `If-None-Match` correto retorna 304
- [ ] Request sem `Principal` no contexto retorna 401
- [ ] `invalid_query`, `invalid_kind`, `not_found` retornam 422/422/404 com envelope padrão
- [ ] Handlers não contêm lógica de negócio, SQL ou branching de domínio
- [ ] Unit tests com `testify/suite` e mocks
- [ ] Gate R0-R7 passa; gate R-ADAPTER-001 passa

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit test: cada handler — happy path, erro de use case, 304
- [ ] Unit test: router aplica `RequireUser`
- [ ] Unit test: ETag e version presentes em resposta de erro (404, 422)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/infrastructure/http/server/handlers/list_categories_handler.go`
- `internal/categories/infrastructure/http/server/handlers/get_category_handler.go`
- `internal/categories/infrastructure/http/server/handlers/list_dictionary_handler.go`
- `internal/categories/infrastructure/http/server/handlers/search_dictionary_handler.go`
- `internal/categories/infrastructure/http/server/router.go`
- Arquivos `_test.go` correspondentes
- `mockery.yml`
