# Plano de Cobertura 100% — Módulo `internal/categories`

**Data:** 2026-06-18
**Skill obrigatória:** go-implementation
**Scope:** `internal/categories/` — módulo read-only (sem outbox, producers, consumers, jobs)

---

## 1. Inventário Real do Módulo

### 1.1 Endpoints HTTP (todos GET, read-only)

| Método | Rota | Handler | Status esperados |
|--------|------|---------|-----------------|
| GET | `/categories` | `ListCategoriesHandler` | 200, 304, 401, 422, 500 |
| GET | `/categories/{id}` | `GetCategoryHandler` | 200, 304, 401, 404, 422, 500 |
| GET | `/category-dictionary` | `ListDictionaryHandler` | 200, 304, 401, 422, 500 |
| GET | `/category-dictionary/search` | `SearchDictionaryHandler` | 200, 304, 401, 422, 500 |

### 1.2 Use Cases

| Use Case | Arquivo | Erros retornados |
|----------|---------|-----------------|
| `ListCategories` | `list_categories.go` | repo error, version error |
| `GetCategory` | `get_category.go` | `ErrCategoryNotFound`, version error, repo error |
| `ListDictionary` | `list_dictionary.go` | repo error, version error |
| `SearchDictionary` | `search_dictionary.go` | `ErrInvalidKind`, `ErrInvalidQuery`, repo error, version error |
| `ValidateSubcategory` | `validate_subcategory.go` | `ErrCategoryNotFound`, `ErrSubcategoryNotRoot`, repo error |
| `ResolveBySlug` | `resolve_by_slug.go` | slug not found, repo error |

### 1.3 Domain

**Value Objects (todos com smart constructors):**
- `Kind` — `ParseKind(s)` → `ErrInvalidKind`
- `AllocationType` — `ParseAllocationType(s)` → `ErrInvalidAllocationType`
- `SignalType` — `ParseSignalType(s)` + `Precedence()` → `ErrInvalidSignalType`
- `Confidence` — `ParseConfidence(s)` → `ErrInvalidConfidence`
- `Slug` — `NewSlug(s)` → 6 erros distintos (empty, too short, too long, invalid chars, edge hyphen, double hyphen)
- `SearchQuery` — `NewSearchQuery(s)` → `ErrInvalidQuery` (< 3 chars normalized)
- `SearchOutcome` — `ClassifyOutcome(count)` (puro, sem erro)

**Entities:**
- `Category` — `IsRoot()`, `IsActive()`
- `DictionaryEntry`

**Domain Services:**
- `PTBRCollator` — `Less(a, b string) bool`
- `CandidateResolver` — `Resolve(entries, categories) ([]Candidate, bool)` — top 3, hasMore

### 1.4 Repositories (interfaces)

| Interface | Métodos |
|-----------|---------|
| `CategoryRepository` | `List(ctx, query)`, `ListByIDs(ctx, ids)`, `GetByID(ctx, id)` |
| `DictionaryRepository` | `List(ctx, query) (entries, nextCursor, error)`, `Search(ctx, query)` |
| `VersionReader` | `Current(ctx)` |

### 1.5 Tabelas SQL

- `mecontrola.categories` — UUID PK, slug, name, kind, parent_id, allocation_type, deprecated_at
- `mecontrola.category_dictionary` — UUID PK, category_id, kind, term, term_normalized (generated), signal_type, confidence, is_ambiguous, deprecated_at
- `mecontrola.category_editorial_version` — version (int64)

### 1.6 Producers / Consumers / Jobs / Outbox

**Não existem.** O módulo é puramente read-only. Nenhum evento é publicado, nenhum consumer existe, nenhum job handler existe.

### 1.7 Mocks Gerados

- `application/interfaces/mocks/category_repository.go`
- `application/interfaces/mocks/dictionary_repository.go`
- `application/interfaces/mocks/version_reader.go`

---

## 2. Estado Atual dos Testes (43 arquivos)

### 2.1 E2E Godog — COMPLETO ✅

Todos os 4 feature files existem em `internal/categories/e2e/features/`:

| Feature | Cenários | Auth (401) | ETag (304) |
|---------|---------|-----------|-----------|
| `f01_categories_list.feature` | 8 | ✅ | ✅ |
| `f02_category_get.feature` | 8 | ✅ | ✅ |
| `f03_dictionary_list.feature` | 9 | ✅ | ✅ |
| `f04_dictionary_search.feature` | 10 | ✅ | ✅ |

**Os steps já estão implementados em:**
- `steps_categories_list_test.go`
- `steps_category_get_test.go`
- `steps_dictionary_list_test.go`
- `steps_dictionary_search_test.go`
- `steps_shared_test.go`

### 2.2 Unit — Domain (Value Objects)

| Arquivo | Status | Gaps |
|---------|--------|------|
| `kind_test.go` | ✅ | — |
| `allocation_type_test.go` | ✅ | — |
| `signal_type_test.go` | ✅ | Precedence ordering entre todos os tipos |
| `confidence_test.go` | ✅ | — |
| `slug_test.go` | ✅ | Erros acumulados (ambos edge cases na mesma chamada) |
| `search_query_test.go` | ✅ | Normalização de unicode complexo |
| `search_outcome_test.go` | ✅ | — |
| `candidate_resolver_test.go` | ✅ | Ordering por confidence dentro do mesmo SignalType |
| `ptbr_collator_test.go` | ✅ | — |

### 2.3 Unit — Use Cases

| Arquivo | Status | Gaps |
|---------|--------|------|
| `list_categories_test.go` | ⚠️ | Empty result; PT-BR ordering validation |
| `get_category_test.go` | ⚠️ | Leaf node (root sem filhos) |
| `list_dictionary_test.go` | ⚠️ | Cursor malformado; page_size bounds explícitos |
| `search_dictionary_test.go` | ✅ | Confidence ordering nos candidatos |
| `validate_subcategory_test.go` | ✅ | — |
| `resolve_by_slug_test.go` | ⚠️ | Deprecated categories ignoradas; slug collision |

### 2.4 Unit — HTTP Handlers

| Arquivo | Status | Gap CRÍTICO |
|---------|--------|------------|
| `list_categories_handler_test.go` | 🔴 | **401 Unauthorized ausente** |
| `get_category_handler_test.go` | 🔴 | **401 Unauthorized ausente** |
| `list_dictionary_handler_test.go` | 🔴 | **401 Unauthorized ausente** + cursor inválido |
| `search_dictionary_handler_test.go` | 🔴 | **401 Unauthorized ausente** |

### 2.5 Integração — Repositories

| Arquivo | Status | Gaps |
|---------|--------|------|
| `category_repository_integration_test.go` | ⚠️ | `ListByIDs` ausente; deprecated filtering; hierarchy 3+ níveis |
| `dictionary_repository_integration_test.go` | ⚠️ | Search ordering; filtros combinados simultâneos |
| `version_reader_integration_test.go` | ✅ | — |
| `schema_regression_integration_test.go` | ✅ | slug uniqueness constraint |
| `canonical_scenarios_integration_test.go` | ✅ | — |

---

## 3. Matriz de Cobertura Obrigatória — Gaps a Implementar

### Camada 1 — Domain Unit (Prioridade P1)

#### 1.1 `signal_type_test.go` — adicionar

```go
// Precedence ordering: canonical_name(5) > alias(4) > phrase(3) > merchant(2) > segment(1)
func TestSignalTypePrecedenceOrdering(t *testing.T) {
    types := []SignalType{
        SignalTypeSegment, SignalTypeMerchant, SignalTypePhrase,
        SignalTypeAlias, SignalTypeCanonicalName,
    }
    for i := 1; i < len(types); i++ {
        assert.Greater(t, types[i].Precedence(), types[i-1].Precedence())
    }
}
```

#### 1.2 `candidate_resolver_test.go` — adicionar

```go
// Dois candidatos com mesmo SignalType → ordenar por path alfabético
func TestCandidateResolverOrdersByPathWhenSameSignalType(t *testing.T) { ... }

// Mais de 3 candidatos → top 3 + hasMore=true
func TestCandidateResolverLimitsToThreeCandidates(t *testing.T) { ... }
```

---

### Camada 2 — Use Case Unit (Prioridade P0/P1)

#### 2.1 `list_categories_test.go` — adicionar

```go
// Empty result quando kind não tem categorias cadastradas
func TestListCategoriesReturnsEmptyWhenNoMatchingKind(t *testing.T) { ... }

// Validação de ordenação PT-BR na saída
func TestListCategoriesOrdersAlphabeticallyPTBR(t *testing.T) { ... }
```

#### 2.2 `get_category_test.go` — adicionar

```go
// Root category sem subcategorias → Subcategories vazio (não nil)
func TestGetCategoryRootWithNoChildren(t *testing.T) { ... }
```

#### 2.3 `list_dictionary_test.go` — adicionar

```go
// page_size 0 → usa default (50)
func TestListDictionaryDefaultPageSize(t *testing.T) { ... }

// page_size > 200 → clamp para 200
func TestListDictionaryMaxPageSize(t *testing.T) { ... }

// Sem cursor → começa do início
func TestListDictionaryFirstPageNoCursor(t *testing.T) { ... }
```

#### 2.4 `resolve_by_slug_test.go` — adicionar

```go
// Categoria deprecated não é retornada como raiz válida
func TestResolveBySlugIgnoresDeprecatedRootCategories(t *testing.T) { ... }
```

---

### Camada 3 — HTTP Handler Unit (Prioridade P0 — CRÍTICO)

#### 3.1 `list_categories_handler_test.go` — adicionar

```go
func TestListCategoriesHandlerReturnsUnauthorizedWhenNoAuthHeader(t *testing.T) {
    // Request sem header de autenticação → 401
}
```

#### 3.2 `get_category_handler_test.go` — adicionar

```go
func TestGetCategoryHandlerReturnsUnauthorizedWhenNoAuthHeader(t *testing.T) {
    // Request sem header → 401
}
```

#### 3.3 `list_dictionary_handler_test.go` — adicionar

```go
func TestListDictionaryHandlerReturnsUnauthorizedWhenNoAuthHeader(t *testing.T) { ... }

func TestListDictionaryHandlerReturns422ForMalformedCursor(t *testing.T) {
    // Cursor base64 malformado → 422
}
```

#### 3.4 `search_dictionary_handler_test.go` — adicionar

```go
func TestSearchDictionaryHandlerReturnsUnauthorizedWhenNoAuthHeader(t *testing.T) { ... }
```

**Padrão para os 4 testes de 401:**
- Criar `httptest.NewRequest` sem header `X-Gateway-User-ID` (ou o header que `gatewayAuth` exige)
- Verificar status 401 e body `application/problem+json`
- Verificar que o use case mock **não** foi chamado (`.Times(0)`)

---

### Camada 4 — Repositório Integração (Prioridade P1)

#### 4.1 `category_repository_integration_test.go` — adicionar

```go
//go:build integration

func TestCategoryRepositoryListByIDs(t *testing.T) {
    // Busca batch de 3 IDs → retorna exatamente 3 categorias
    // Verificação: SELECT id FROM mecontrola.categories WHERE id IN (...)
}

func TestCategoryRepositoryListExcludesDeprecatedByDefault(t *testing.T) {
    // Cria categoria com deprecated_at preenchido
    // List sem IncludeDeprecated → não retorna categoria
    // Verificação: SELECT COUNT(*) WHERE deprecated_at IS NOT NULL
}

func TestCategoryRepositoryListIncludesDeprecatedWhenFlagSet(t *testing.T) {
    // Mesma categoria deprecated → retorna quando IncludeDeprecated=true
}
```

#### 4.2 `dictionary_repository_integration_test.go` — adicionar

```go
//go:build integration

func TestDictionaryRepositorySearchOrdersBySignalTypePrecedence(t *testing.T) {
    // Mesmo termo indexado como canonical_name E como alias
    // Search → canonical_name aparece primeiro
    // Verificação: assert result[0].SignalType == SignalTypeCanonicalName
}

func TestDictionaryRepositoryListWithCombinedFilters(t *testing.T) {
    // kind=expense + signal_type=canonical_name + category_id específico
    // → retorna apenas entradas que satisfazem TODOS os filtros
}
```

---

### Camada 5 — E2E Godog (COMPLETO — nenhum gap)

Os 4 feature files existentes cobrem todos os cenários de negócio. Nenhum arquivo novo é necessário.

**Verificação de banco obrigatória nos steps existentes:**
Os steps de GET não fazem escrita, mas devem verificar que os dados seed estão presentes via `SELECT COUNT(*)` antes de executar os cenários que dependem de dados específicos do banco.

---

## 4. Estrutura de Pastas — Estado Final

```
internal/categories/
├── domain/
│   ├── entities/
│   │   ├── category.go
│   │   ├── category_test.go              ✅ existe
│   │   ├── dictionary_entry.go
│   │   └── dictionary_entry_test.go      ✅ existe
│   ├── services/
│   │   ├── candidate_resolver.go
│   │   ├── candidate_resolver_test.go    ✅ existe + gaps P1
│   │   ├── ptbr_collator.go
│   │   └── ptbr_collator_test.go         ✅ existe
│   └── valueobjects/
│       ├── *.go                          (7 VOs)
│       └── *_test.go                     ✅ existem + gaps P1
├── application/
│   ├── interfaces/
│   │   ├── *.go (3 interfaces)
│   │   └── mocks/*.go (3 mocks)          ✅ existem
│   ├── dtos/
│   │   ├── input/*.go
│   │   └── output/*.go
│   └── usecases/
│       ├── *.go (6 use cases)
│       └── *_test.go                     ✅ existem + gaps P0/P1
├── infrastructure/
│   ├── http/server/
│   │   ├── handlers/
│   │   │   ├── *.go (7 arquivos)
│   │   │   └── *_test.go                 ⚠️ falta 401 em 4 handlers
│   │   └── router.go + router_test.go   ✅ existe
│   └── repositories/postgres/
│       ├── *.go (3 repositórios)
│       └── *_integration_test.go         ⚠️ falta ListByIDs, deprecated, ordering
└── e2e/
    ├── features/
    │   ├── f01_categories_list.feature    ✅ COMPLETO
    │   ├── f02_category_get.feature       ✅ COMPLETO
    │   ├── f03_dictionary_list.feature    ✅ COMPLETO
    │   └── f04_dictionary_search.feature  ✅ COMPLETO
    ├── suite_test.go                      ✅ existe
    ├── ctx_test.go                        ✅ existe
    ├── helpers_test.go                    ✅ existe
    ├── steps_categories_list_test.go      ✅ existe
    ├── steps_category_get_test.go         ✅ existe
    ├── steps_dictionary_list_test.go      ✅ existe
    ├── steps_dictionary_search_test.go    ✅ existe
    └── steps_shared_test.go               ✅ existe
```

**Nenhuma pasta nova é necessária.** A estrutura existente é completa para o módulo read-only.

---

## 5. Estratégia de Evidência de Validação

### 5.1 Módulo read-only — Sem Outbox

Como o módulo não tem writes, não há `outbox_events` para validar. A estratégia de evidência foca em:

1. **Estado do banco pré-operação:** helpers `countCategories(t, db, kind)` e `countDictionaryEntries(t, db, kind)` confirmam que o seed está presente.
2. **Assertiva pós-GET:** verificar que o retorno do handler bate com o estado real do banco via `SELECT ... WHERE id = $1`.
3. **ETag consistency:** verificar que o `version` retornado no JSON bate com `SELECT version FROM mecontrola.category_editorial_version`.

### 5.2 Helpers de Banco a Criar

```go
// em integration tests ou helpers_test.go

func countActiveCategories(t testing.TB, db *sql.DB, kind string) int {
    t.Helper()
    var count int
    err := db.QueryRowContext(context.Background(),
        `SELECT COUNT(*) FROM mecontrola.categories WHERE kind = $1 AND deprecated_at IS NULL`,
        kind).Scan(&count)
    require.NoError(t, err)
    return count
}

func findCategoryByID(t testing.TB, db *sql.DB, id uuid.UUID) *dbCategory {
    t.Helper()
    var c dbCategory
    err := db.QueryRowContext(context.Background(),
        `SELECT id, slug, name, kind, parent_id, allocation_type, deprecated_at
         FROM mecontrola.categories WHERE id = $1`, id).
        Scan(&c.ID, &c.Slug, &c.Name, &c.Kind, &c.ParentID, &c.AllocationType, &c.DeprecatedAt)
    if errors.Is(err, sql.ErrNoRows) { return nil }
    require.NoError(t, err)
    return &c
}

func currentEditorialVersion(t testing.TB, db *sql.DB) int64 {
    t.Helper()
    var v int64
    err := db.QueryRowContext(context.Background(),
        `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`).Scan(&v)
    require.NoError(t, err)
    return v
}
```

### 5.3 Padrão de Asserção para 401

```go
func withoutAuthHeader(t testing.TB, router http.Handler, method, path string) *http.Response {
    t.Helper()
    req := httptest.NewRequest(method, path, nil)
    // Não adicionar header de autenticação
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    return rec.Result()
}

// Então nos testes:
resp := withoutAuthHeader(t, router, "GET", "/categories")
assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
// Verificar que use case mock não foi chamado
mockUseCase.AssertNotCalled(t, "Execute")
```

---

## 6. Definition of Done

### Gates de Código (todos devem retornar vazio)

```bash
# R-ADAPTER-001.1 — zero comentários em .go de produção
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/categories/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" && exit 1 || true

# R-ADAPTER-001.2 — sem SQL direto em adapters (handlers)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/categories/infrastructure/http/server/handlers/ \
  && echo "FAIL" && exit 1 || true
```

### Gates de Teste

```bash
# Unit tests (sem Docker)
task test:unit -- -run TestCategories

# Integration tests (requer Docker)
task test:integration -- -run TestCategories

# E2E tests (requer Docker + servidor)
task test:e2e -- --tags=categories
```

### Checklist de Evidência

- [ ] `task test:unit` verde com `-race`
- [ ] `task test:integration` verde (Testcontainers)
- [ ] `task test:e2e` verde (godog)
- [ ] `golangci-lint run ./internal/categories/...` limpo
- [ ] `go vet ./internal/categories/...` limpo
- [ ] Gate zero-comentários retorna vazio
- [ ] Gate sem-SQL-em-adapter retorna vazio
- [ ] Todos os 401 testados em handler unit tests
- [ ] `ListByIDs` coberto em integração
- [ ] `deprecated filtering` coberto em integração

---

## 7. Sequência de Execução (Orquestração Paralela)

A implementação dos gaps deve ser feita com **1 subagent por camada em paralelo**:

| Subagent | Responsabilidade | Arquivos alvo |
|----------|-----------------|---------------|
| `domain-unit-gaps` | Precedence ordering + candidate resolver | `signal_type_test.go`, `candidate_resolver_test.go` |
| `usecase-unit-gaps` | Empty result, PT-BR ordering, leaf node, deprecated | 4 `*_test.go` em `usecases/` |
| `handler-401-gaps` | 401 Unauthorized em 4 handlers + cursor inválido | 4 `*_handler_test.go` |
| `repo-integration-gaps` | `ListByIDs`, deprecated, search ordering | 2 `*_integration_test.go` |

O subagent E2E **não é necessário** — os 4 feature files e todos os steps já existem e estão completos.

---

## 8. Arquivos Gherkin Existentes — Resumo dos Cenários

Os arquivos já existem e não precisam ser recriados. Reprodução para referência:

### f01_categories_list.feature (PT-BR)
1. Listar categorias de despesa com autenticação
2. Listar categorias de receita
3. Listar subcategorias por parent_id
4. Incluir subcategorias deprecated
5. Rejeitar kind inválido (422)
6. Exigir autenticação (401)
7. Rejeitar parent_id inválido (422)
8. Responder 304 com If-None-Match

### f02_category_get.feature (PT-BR)
1. Obter categoria raiz com subcategorias
2. Obter subcategoria com path
3. Retornar 404 para id inexistente
4. Exigir autenticação (401)
5. Ocultar deprecated por padrão (404)
6. Mostrar deprecated com flag (200)
7. Rejeitar id UUID inválido (422)
8. Responder 304 com If-None-Match

### f03_dictionary_list.feature (PT-BR)
1. Listar primeira página
2. Filtrar por kind
3. Filtrar por category_id
4. Filtrar por signal_type (canonical_name)
5. Filtrar por signal_type (alias)
6. Paginar com cursor
7. Rejeitar kind inválido (422)
8. Exigir autenticação (401)
9. Rejeitar signal_type inválido (422)

### f04_dictionary_search.feature (PT-BR)
1. Match inequívoco com confidence high
2. Candidatos ambíguos
3. No match para termo inexistente
4. No match para kind incompatível
5. Rejeitar query vazia (422)
6. Exigir autenticação (401)
7. Rejeitar query curta (422)
8. Rejeitar query só espaços (422)
9. Rejeitar ausência de kind (422)
10. Rejeitar kind inválido (422)

---

## 9. Restrições Mandatórias

- **Zero comentários** em `.go` de produção (R-ADAPTER-001.1) — inegociável
- **Gherkin e regex:** PT-BR
- **Métodos/Steps Go:** inglês
- **Build tags:** `//go:build integration` em todo teste que sobe container
- **Sem `var _ Interface = (*Type)(nil)`** — proibido (feedback memory)
- **Sem `Clock` interface** — usar `time.Now().UTC()` inline
- **Sem falso positivo** — se um teste quebra, corrigir o código de produção, nunca o teste
