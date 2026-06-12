# Plano Advisory — Refactor `internal/categories`

## Contexto

Refactor advisory (sem execução por padrão) de `internal/categories`, módulo predominantemente read-oriented que expõe 4 endpoints GET com ETag (`"v<int>"`) para listar/buscar/lookup de categorias e dicionário. Origem do pedido: `docs/refactors/internal-categories.md`. Aplicar DMMF (Wlaschin) **seletivamente** onde traz ganho real de modelagem (smart constructors, erros semanticamente claros, factories explícitas), evitando workflows, eventos de domínio e state-as-type que não cabem em módulo de leitura. Objetivo: melhorar legibilidade, separação de responsabilidades e robustez **preservando 100% do comportamento observável** (payloads, status, ETag, ordenação PTBR, cap de 3 candidatos, semântica de outcomes).

## Skills obrigatórias

- `.agents/skills/refactor/SKILL.md`
- `.agents/skills/go-implementation/SKILL.md` (R0–R7 obrigatórias; matriz R-ADAPTER-001.3 sob demanda)
- `.agents/skills/review/SKILL.md` (ao final de cada batch)

## Diagnóstico (estado atual)

| Aspecto | Status |
|--------|--------|
| R-ADAPTER-001.1 (zero comentários) | PASS |
| R-ADAPTER-001.2 (adapters finos, sem SQL/branching de domínio) | PASS |
| Vazamento de observability para domínio | PASS (limpo) |
| Cobertura de testes (unit + integration ETag) | OK |

## Hotspots (com decisão)

| # | Local | Decisão | Razão |
|---|-------|---------|------|
| H1 | `domain/entities/category.go::GetDeprecatedAt() string` formata ISO no domínio | **FIX** | Apresentação vaza para entity; trocar para `*time.Time`, formatar no DTO/handler. |
| H2 | `application/usecases/search_dictionary.go` magic number `searchCandidateLimit = 100` | **FIX** | Promover a `domain/policy.go::DefaultSearchCandidateLimit`. |
| H3 | `domain/services/candidate_resolver.go` cap `3` hardcoded | **FIX leve** | Extrair `MaxResolvedCandidates = 3` no próprio service. **Não** parametrizar (quebra contrato). |
| H4 | `Slug`/`Name` como `string` cru em `Category` | **FIX** | Smart constructors `NewSlug`/`NewName` em `domain/valueobjects/` espelhando `internal/billing/domain/valueobjects/plan.go::NewPlan` e `internal/card/domain/valueobjects/card_name.go::NewCardName`. |
| H5 | `list_categories.go` filtro `s.Kind != root.Kind continue` não documentado | **FIX** | Encapsular em `Category.BelongsToTreeOf(root Category) bool`. |
| H6 | `ErrCategoryNotFound` genérico | **FIX seletivo** | Adicionar `ErrSlugNotResolvable` (wrap `interfaces.ErrNotFound` — mantém mapping HTTP). |
| H7 | VOs existentes (`Kind`, `SignalType`, `Confidence`, `AllocationType`, `SearchOutcome`) | **LEAVE** | Já têm `Parse*` + `IsValid`. |
| H8 | Reconstrução de `DictionaryEntry` no repo sem separar validar/hidratar | **FIX** | Espelhar `internal/card/domain/entities/card.go::HydrateCard`. Mesmo para `Category`. |

## DMMF — IN vs OUT

**IN (ganho real):**
- Smart constructors para `Slug` e `Name` — invariante na construção.
- Erros semanticamente ricos (`ErrSlugNotResolvable`).
- Separação `New*` (valida) vs `Hydrate*` (assume DB confiável) — fronteira domínio/persistência.
- Constantes de política nomeadas substituindo magic numbers.

**OUT (declinado explicitamente):**
- **Domain events** — não há fato de domínio mutável.
- **State-as-type / state machines** — `DeprecatedAt` é flag, não estado.
- **Workflow / railway-oriented pipelines** — usecases lineares de leitura.
- **Result/Either customizado** — proibido por `.agents/skills/agent-governance/references/domain-modeling.md`.

## Plano em batches (ordem do menor risco para o maior)

### Batch 1 — Constantes de política (zero impacto observável)
- Criar `internal/categories/domain/policy.go` com `DefaultSearchCandidateLimit = 100`, `MaxResolvedCandidates = 3`.
- Substituir `100` em `application/usecases/search_dictionary.go`.
- Substituir `3` em `domain/services/candidate_resolver.go`.

### Batch 2 — Erros de domínio
- Adicionar `ErrSlugNotResolvable` em `domain/errors.go` (wrap de `interfaces.ErrNotFound` para preservar HTTP mapping).
- Ajustar `application/usecases/resolve_by_slug.go` para retornar o sentinel.

### Batch 3 — Presentation leak (H1)
- `entities/category.go`: trocar `GetDeprecatedAt() string` por `DeprecatedAt() *time.Time` (substituição direta — módulo interno, sem backward-compat).
- Mover formatação ISO para o DTO/handler de resposta.
- Garantir snapshot de payload byte-idêntico.

### Batch 4 — Smart constructors `Slug` e `Name` (H4)
- Criar `domain/valueobjects/slug.go`: `Slug` struct com `value string` privado, `NewSlug(s) (Slug, error)` validando `^[a-z0-9-]+$`, len 1..64, UTF-8 válido, não-vazio. Sentinel `ErrInvalidSlug`.
- Criar `domain/valueobjects/name.go`: `NewName` — trim, não-vazio, UTF-8, len 1..120. Sentinel `ErrInvalidName`.
- Atualizar `Category` para usar VOs internamente com getters.
- Adicionar `HydrateCategory(...)` que reconstrói sem revalidar (espelha `HydrateCard`).
- `factories/category_id.go::NewCategoryID` passa a receber `Slug` (mesmo hash; mais seguro).
- Repositório postgres usa `HydrateCategory`.

### Batch 5 — Invariante de árvore + hidratação de dicionário (H5, H8)
- `Category.BelongsToTreeOf(root Category) bool` encapsula `c.Kind == root.Kind`.
- `application/usecases/list_categories.go` usa o método.
- `entities/dictionary_entry.go`: separar `NewDictionaryEntry` (valida) de `HydrateDictionaryEntry` (sem validar).

## Critical files

- `internal/categories/domain/entities/category.go`
- `internal/categories/domain/entities/dictionary_entry.go`
- `internal/categories/domain/services/candidate_resolver.go`
- `internal/categories/domain/errors.go`
- `internal/categories/domain/factories/category_id.go`
- `internal/categories/domain/policy.go` (novo)
- `internal/categories/domain/valueobjects/slug.go` (novo)
- `internal/categories/domain/valueobjects/name.go` (novo)
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/usecases/list_categories.go`
- `internal/categories/application/usecases/resolve_by_slug.go`
- `internal/categories/infrastructure/repositories/postgres/category_repository.go`
- `internal/categories/infrastructure/repositories/postgres/dictionary_repository.go`

## Restrições obrigatórias

- Sem alteração de contrato HTTP, payloads, status, ETag, ordenação PTBR ou cap de 3 candidatos.
- Zero comentários em código Go (R-ADAPTER-001.1) — inclusive em arquivos novos.
- Sem regra de negócio em handlers; sem SQL em adapters; fluxo `adapter → usecase` preservado.
- Máximo de 4 referências `go-implementation` simultâneas (matriz R-ADAPTER-001.3); para domínio carregar `architecture.md`, `domain-modeling.md`, `error-handling`, `testing-unit.md` sob demanda.

## Verificação (por batch + final)

1. `task lint && task vuln && task test` após cada batch.
2. Integration `canonical_scenarios_integration_test.go`: ETag `v<n>` byte-idêntico antes/depois.
3. Snapshot de payload `GET /api/v1/categories` e `/api/v1/category-dictionary/search` igual.
4. Novos testes table-driven:
   - `domain/valueobjects/slug_test.go` — casos: vazio, espaço, maiúsculo, unicode inválido, len>64, hyphen leading, válido.
   - `domain/valueobjects/name_test.go` — vazio após trim, len>120, UTF-8 inválido, válido.
   - `domain/entities/category_belongs_to_tree_test.go` — same-kind, diff-kind, root vs leaf.
5. Skill `review` no diff final de cada batch.
6. Relatório consolidado (advisory) com: arquivos alterados, magic numbers eliminados, campos string→VO, validações executadas, riscos residuais. Status final: `done`, `needs_input`, `blocked` ou `failed`.

## Execução

Modo advisory. **Nenhuma alteração será feita até aprovação explícita do usuário** indicando quais batches executar e em que ordem (recomendado: 1 → 5 sequencialmente, branch dedicada por batch, commit semantic `refactor(categories): ...`).
