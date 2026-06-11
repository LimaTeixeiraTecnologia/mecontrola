# Refactor `internal/categories` — DDD tático + funções puras

## Context

`internal/categories` é enxuto, mas o fluxo de busca espalha regra incidental entre handler HTTP, use case e DTO:

- **Normalização da query duplicada em 3 lugares** com semânticas divergentes:
  - `search_dictionary_handler.go:190-199` — trim + alfanumérico (não lowercase).
  - `search_dictionary_input.go:37-45` — trim + alfanumérico + lowercase.
  - `search_dictionary.go:48` — `strings.TrimSpace(in.Query)` direto na query do repositório.
- **Outcome classification ("matched"/"ambiguous"/"no_match") vive no handler** (`determineOutcome`, linhas 158-169), forçando o output a expor `SignalTypeTop` e `IsAmbiguous` como campos `json:"-"` puramente para o handler reconstruir o outcome.
- **Strings mágicas** (`"no_match"`, `"candidates"`, `"matched"`, `"ambiguous"`) repetidas em use case e handler sem fronteira semântica.
- `calcQLenBucket` é puro mas re-normaliza a query (terceira normalização) só para medir tamanho.

`candidate_resolver` já concentra bem o ranqueamento (`SignalType.Precedence`, ordenação, limite de 3, match reason), então **não há refactor de domínio amplo** — é refino tático: extrair regra de outcome para o domínio, consolidar normalização no input, transformar handler em adapter fino.

Skills/refs já carregadas conforme o prompt: `AGENTS.md`, `go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md`, `testing.md` (suites de handler/use case serão reescritas).

## Recommended Approach

### 1. Novo VO `SearchOutcome` em `domain/valueobjects/search_outcome.go`

Enum tipado representando o resultado semântico de uma busca:

```
type SearchOutcome int

const (
    SearchOutcomeUnknown SearchOutcome = iota
    SearchOutcomeNoMatch
    SearchOutcomeMatched
    SearchOutcomeAmbiguous
)
```

Métodos: `String()` (retorna `"no_match"`, `"matched"`, `"ambiguous"` — usado como label de métrica e como contrato semântico interno), `IsValid()`.

Inclui função pura `ClassifyOutcome(candidatesCount int) SearchOutcome` no mesmo arquivo:
- `0` → `SearchOutcomeNoMatch`
- `1` → `SearchOutcomeMatched`
- `>1` → `SearchOutcomeAmbiguous`

Test: `search_outcome_test.go` cobrindo as 3 transições + `IsValid` + `String`.

### 2. Consolidar normalização da query no input DTO

- Manter `SearchDictionaryInput.NormalizedQuery()` como **única** fonte de verdade.
- `search_dictionary.go:48` passa a usar `in.NormalizedQuery()` em vez de `strings.TrimSpace(in.Query)`. **Decisão**: o repositório receberá a query alfanumérica + lowercase. Verificar `dictionary_repository.go` para confirmar que a busca atual é case-insensitive (se já é `LOWER(...) LIKE LOWER(...)` ou similar, a mudança é semanticamente idempotente; caso contrário, manter `strings.TrimSpace` no use case e usar `NormalizedQuery()` apenas para validação/bucket).
- Remover `normalizeQuery` do handler (linhas 190-199).
- `calcQLenBucket` migra para função pura `qLenBucket(normalized string) string` em **novo arquivo `metric_buckets.go` do mesmo pacote `handlers`** — recebe a query já normalizada, sem re-normalizar. Não cria interface, não vira método.

### 3. Use case passa a expor `Outcome` e `SignalTypeTop` explícitos no output

Alterações em `DictionarySearchOutput` (`dictionary_search_output.go`):
- Remover `IsAmbiguous bool` (decorrência de `Outcome`).
- Manter `SignalTypeTop string` (`json:"-"`) — é dimensão de métrica ortogonal ao outcome.
- Adicionar `Outcome valueobjects.SearchOutcome` (`json:"-"`).
- Campo `Result string `json:"result"`` **permanece inalterado no payload** (`"no_match"` ou `"candidates"`) para preservar compat de API.

Em `search_dictionary.go`:
- Após obter candidatos, computar `outcome := valueobjects.ClassifyOutcome(len(candidates))`.
- Preencher `result.Outcome = outcome`, `result.SignalTypeTop = candidates[0].SignalType.String()` quando `len > 0`.
- Manter `Result: "candidates"` / `Result: "no_match"` no payload conforme hoje.

### 4. Handler vira adapter fino

`search_dictionary_handler.go` perde:
- `normalizeQuery` (movido para input).
- `determineOutcome` (vem do use case via `out.Outcome.String()`).
- `calcQLenBucket` método → consome `qLenBucket(in.NormalizedQuery())` helper.

`recordMetrics` continua local — é tradução HTTP→métrica, escopo legítimo do adapter. ETag, status, parse de `kind`, problem responses continuam no handler (são detalhes HTTP). Não criar interface nova para o resolver, factory ou helper — tipos concretos bastam (R-INTF-001).

### Arquivos críticos a modificar

- `internal/categories/domain/valueobjects/search_outcome.go` (novo) + `_test.go`.
- `internal/categories/application/usecases/search_dictionary.go` — usar `NormalizedQuery()`, preencher `Outcome` e `SignalTypeTop`, eliminar `IsAmbiguous` set.
- `internal/categories/application/usecases/search_dictionary_test.go` — substituir asserts de `IsAmbiguous` por asserts de `Outcome`.
- `internal/categories/application/dtos/output/dictionary_search_output.go` — adicionar `Outcome`, remover `IsAmbiguous`.
- `internal/categories/infrastructure/http/server/handlers/search_dictionary_handler.go` — remover `normalizeQuery`/`determineOutcome`/`calcQLenBucket` (método), consumir helper + `out.Outcome.String()`.
- `internal/categories/infrastructure/http/server/handlers/metric_buckets.go` (novo) + atualizar `search_dictionary_metrics_test.go` para chamar helper de pacote em vez de método de `handler`.
- Pesquisar e atualizar outros consumidores de `IsAmbiguous` (apenas o canonical_scenarios e error_envelope integration tests podem tocar — confirmar no diff).

### Restrições respeitadas

- **R-ADAPTER-001.1**: nenhum comentário Go novo nos arquivos de produção (handler, use case, VO).
- **R-ADAPTER-001.2**: handler continua sem SQL/branching de domínio; lift de outcome reduz branching incidental.
- **R-INTF-001**: nenhuma interface nova; tipos concretos (`*services.CandidateResolver`, helpers de pacote como funções) bastam.
- **R6.4**: sem `var _ Interface = (*Type)(nil)`.
- **R5.12**: sem `panic` introduzido.
- Compat de API: `Result` permanece "no_match"/"candidates" no JSON; `Candidates`, `HasMore`, `Version` inalterados; ETag/headers inalterados.

### Verification

1. **Build**: `go build ./internal/categories/...`
2. **Unit tests** (atualizados):
   - `go test ./internal/categories/domain/valueobjects/... -run SearchOutcome`
   - `go test ./internal/categories/application/usecases/... -run SearchDictionary`
   - `go test ./internal/categories/infrastructure/http/server/handlers/... -run SearchDictionary`
3. **Integration tests existentes** (sem alteração esperada):
   - `go test ./internal/categories/infrastructure/http/server/handlers/... -run "Canonical|RF34|ErrorEnvelope"`
4. **R-ADAPTER-001.1 gate** (zero comentários):
   ```
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
     "^[[:space:]]*//" internal/categories/ \
     | grep -Ev "(//go:|//nolint:|// Code generated)"
   ```
   Deve retornar vazio.
5. **R-ADAPTER-001.2 gate** (sem SQL em handlers):
   ```
   grep -rn --include="*.go" --exclude="*_test.go" "QueryContext\|ExecContext" \
     internal/categories/infrastructure/http/server/handlers/
   ```
   Deve retornar vazio.
6. **Checklist R0–R7** (`references/build.md`):
   - R0 sem `init()` novo; R6 context propagado nas fronteiras; R7.6 `fmt.Errorf("...: %w", err)` mantido nos wrappers existentes.

### O que NÃO está no escopo

- N+1 em `buildCategoryMap` (linha 96-114 do use case) — otimização orthogonal ao prompt; deixar para refactor dedicado.
- Reescrita do `candidate_resolver` — já está bem encapsulado.
- Mudança de wire ou nova interface para resolver/helpers — proibido pelo prompt ("não criar interfaces novas se os tipos concretos atuais já bastam").
- Mudança de payload JSON externo.
