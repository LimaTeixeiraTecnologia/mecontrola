# Tarefa 6.0: Use cases: ListCategories, GetCategory, ListDictionary, SearchDictionary

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os quatro use cases de leitura: `ListCategories`, `GetCategory`, `ListDictionary`, `SearchDictionary`. Cada use case orquestra chamada ao repositório, montagem de DTOs de output, inclusão de `version` no output e validação de input. `SearchDictionary` integra o `CandidateResolver`.

<requirements>
- RF-08–RF-13: listagem e consulta de categorias
- RF-09: filtros opcionais `kind`, `parent_id`, `include_deprecated`
- RF-10: listagem sem `parent_id` retorna árvore hierárquica
- RF-12: `GetCategory` retorna item, caminho completo e subcategorias
- RF-14–RF-18a: listagem e busca do dicionário com versionamento
- RF-16: `kind` obrigatório na busca
- RF-16a: `q` normalizado < 3 caracteres → `invalid_query`
- RF-17: todo candidato retorna `category_id`, `root_category_id`, `path`, `matched_term`, `signal_type`, `confidence`, `is_ambiguous`, `match_reason`
- RF-18: resposta da busca é `candidates` ou `no_match`
- RF-27: deduplicação e ambiguidade via `CandidateResolver`
- ADR-002: version em todo output de leitura
</requirements>

## Subtarefas

- [ ] 6.1 Implementar `ListCategories` com filtros e montagem de árvore
- [ ] 6.2 Implementar `GetCategory` com caminho completo e subcategorias
- [ ] 6.3 Implementar `ListDictionary` com paginação cursor
- [ ] 6.4 Implementar `SearchDictionary` com validação de input e integração com `CandidateResolver`
- [ ] 6.5 DTOs de input e output em `application/dtos/`
- [ ] 6.6 Unit tests com mocks para todos os use cases

## Detalhes de Implementação

Ver techspec.md seções **Interfaces Chave** e **Endpoints de API**.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar `references/interfaces.md` se houver redesign de interfaces
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- `ListCategories` sem `parent_id`: query todas as categorias do `kind` e monta árvore em memória (raiz → subcategorias).
- `GetCategory`: se raiz, busca subcategorias via `List` com `parent_id`; se subcategoria, busca raiz para montar `path`.
- `SearchDictionary`: validar `kind` primeiro (422 `invalid_kind` se ausente/inválido); validar `q` depois (422 `invalid_query` se normalizado < 3).
- `SearchDictionary`: chamar `repo.Search`, depois `resolver.Resolve`, limitar a 3, calcular `has_more`.
- Todo DTO de output inclui campo `version` (lido via `VersionReader`).
- Use cases não tratam HTTP; apenas retornam DTOs e erros de domínio/aplicação.

## Critérios de Sucesso

- [ ] `ListCategories` retorna árvore correta para `kind=expense` (5 raízes, subcategorias ordenadas)
- [ ] `GetCategory` retorna 404 para ID inexistente ou descontinuado sem `include_deprecated`
- [ ] `SearchDictionary` retorna 422 para `kind` ausente
- [ ] `SearchDictionary` retorna 422 para `q=ab` (normalizado < 3)
- [ ] `SearchDictionary` retorna `no_match` para termo inexistente
- [ ] Unit tests com `testify/suite` e mocks via `mockery.yml`
- [ ] Gate R0-R7 passa

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit test: `ListCategories` — happy path, filtro por `kind`, filtro por `parent_id`, inclusão de deprecated
- [ ] Unit test: `GetCategory` — raiz com subcategorias, subcategoria com path, not found
- [ ] Unit test: `ListDictionary` — paginação, filtros
- [ ] Unit test: `SearchDictionary` — match, no_match, invalid_kind, invalid_query, ambiguous

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/application/usecases/list_categories.go`
- `internal/categories/application/usecases/get_category.go`
- `internal/categories/application/usecases/list_dictionary.go`
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/dtos/input/*.go`
- `internal/categories/application/dtos/output/*.go`
- Arquivos `_test.go` correspondentes
- `mockery.yml`
