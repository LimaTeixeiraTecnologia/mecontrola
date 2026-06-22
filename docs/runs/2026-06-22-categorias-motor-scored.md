# 2026-06-22 — Motor de categorização determinístico, scored e production-proof (`internal/categories`)

> Skill obrigatória declarada para todo código Go desta run: `.agents/skills/go-implementation` (Etapas 1–5).
> Premissa inegociável do usuário: robusto / production-proof / eficiente / econômico, **sem falso positivo**.
> "Agentes Mastra" = serviços Go determinísticos (não micro-agentes LLM). Inclui seed inicial + gate aditivo no agente.

## Contexto

A categorização do `internal/categories` fazia **match exato de string normalizada** e nada mais — gargalo de
precisão (falso negativo sistemático em "mercado", "netflix", "iptu"; tokens destruídos na normalização; sem
score composto; gate do agente medindo só confiança do LLM, não a qualidade do match). Esta run evolui o módulo
de forma incremental e não-quebra-compatibilidade: tokenização, expansão de alias/sinônimos já existentes, score
numérico composto, classificação por threshold, tolerância a typo (fuzzy) e gate de confirmação no agente.

## Pipeline determinístico (mapa "agentes" → serviços Go)

`normalize → tokenize → exact → token → fuzzy → score → resolve → classify` (estágios curto-circuitados).

| "Agente" | Artefato Go determinístico |
|---|---|
| Normalization | `SearchQuery` + `tokenizeSearchQuery` (stopwords PT-BR) |
| Dictionary | `DictionaryRepository.Search` (exato) + novo `SearchTokens` |
| Alias | `SearchTokens` reusando `signal_type ∈ {alias,phrase,merchant,segment}` |
| Category/Subcategory Resolver | `CandidateResolver.findRootID` / `buildPath` |
| Confidence | `valueobjects.NewMatchScore` (VO puro) |
| Validation | `valueobjects.ClassifyByScore` |

## Modelo de score (`domain/valueobjects/match_score.go`)

`score = 0.45*(precedence/5) + 0.30*Confidence.Weight() + 0.25*MatchQuality.Weight()`, clamp [0,1].
- Confidence.Weight: high=1.0, medium=0.66, low=0.33. Quality.Weight: exact=1.0, token=0.8, fuzzy=0.5.
- Thresholds: `ScoreAutoThreshold=0.80`, `ScoreConfirmThreshold=0.55`.
- **Garantia sem falso positivo:** fuzzy nunca atinge auto (teto explícito `fuzzyScoreCeiling=0.79` em
  `NewMatchScore`) → typo sempre cai em confirmação, nunca auto-loga. Coberto por
  `TestFuzzyNeverReachesAutoThreshold`.
- `ClassifyByScore`: top<confirm→no_match; top∈[confirm,auto) ou top-2 dentro de delta(0.10)→ambiguous; senão matched.

## Arquivos

**Criados:** `valueobjects/match_quality.go`(+test), `valueobjects/match_score.go`(+test),
`migrations/000015_dictionary_token_search.{up,down}.sql` (pg_trgm + índice GIN trgm),
`migrations/000016_seed_dictionary_synonyms.{up,down}.sql` (55 sinônimos curados, guarded
`INSERT…SELECT…WHERE NOT EXISTS`), teste `services/category_clarification_internal_test.go`.

**Modificados:** `valueobjects/search_query.go` (Tokens()/stopwords), `valueobjects/search_outcome.go`
(mantém `ClassifyOutcome`), `valueobjects/confidence.go` (Weight()), `domain/services/candidate_resolver.go`
(Score/Quality + `ResolveScored`, ranqueio por score, `Resolve` wrapper exact), `application/interfaces/dictionary_repository.go`
(+SearchTokens/+SearchFuzzy, aditivo), `infrastructure/repositories/postgres/dictionary_repository.go`
(impl + spans), `application/usecases/search_dictionary.go` (pipeline staged + ClassifyByScore),
`application/dtos/output/dictionary_search_output.go` (+Score/+MatchQuality, aditivo).
Agente: `usecases/log_transaction_from_agent.go` + `usecases/category_resolution.go` (gate por score +
`CategoryNeedsConfirmationError`), `infrastructure/binding/category_error.go`,
`services/intent_router.go` (mirror + `formatCategoryNeedsConfirmation`), `services/daily_ledger_agent.go`
(catch em `categoryClarification`). Mocks regenerados via `.mockery.yml`.

## Gate aditivo no agente

`top.Score ≥ Auto` → auto-loga (comportamento atual); `[Confirm,Auto)` → `CategoryNeedsConfirmationError`
(WhatsApp pede confirmação: "Acho que isso entra em *X*. Posso registrar assim?"); `<Confirm` → não encontrado.
Métrica `agent_log_transaction_resolve_failed_total{reason="needs_confirmation"|"low_score"}` (baixa cardinalidade).
Ortogonal a `AGENT_POLICY_MIN_CONFIDENCE` (confiança do LLM no intent).

## Validação executada

- `go build ./...` ✅ · `go vet ./internal/categories/... ./internal/agent/...` ✅ · `gofmt -l` vazio ✅
- `go test ./...` (unit, todo o repo) ✅ · `go test -race ./internal/categories/... ./internal/agent/application/...` ✅
- Gate zero-comentário (R-ADAPTER-001.1) em categories+agent produção: vazio ✅
- **Integração (Docker/testcontainers):**
  - `go test -tags integration ./migrations/` ✅ (up+down de 000015 pg_trgm e 000016 seed)
  - `go test -tags integration ./internal/categories/infrastructure/repositories/postgres/` ✅
    incluindo `TestSearchTokens` (mercado/netflix), `TestSearchFuzzyToleratesTypo` ("netflyx"→"netflix"),
    `TestSearchFuzzyEmptyForGibberish`.
- **Limitação:** `golangci-lint` local é v1 e o config do repo é v2 (gap de ambiente). gofmt+vet limpos;
  lint v2 deve rodar em CI (`task lint`).

## Hardening (fechamento de lacunas — 2ª rodada)

- **Garantia endurecida — só EXATO auto-loga:** tetos `tokenScoreCeiling=0.79` e `fuzzyScoreCeiling=0.70`
  (< auto 0.80). Agora **auto-log ⟺ `category_hint` é termo exato do dicionário**; token/fuzzy sempre
  confirmam ou caem em não-encontrado. Fecha o vetor de falso-positivo por token canônico fora de
  contexto ("aluguel do carro"). Coberto por `TestOnlyExactCanAutoLog`.
- **Tokenização accent-insensitive:** tokens unaccentados em Go (`x/text/unicode/norm`) → stopwords
  acentuadas removidas e tokens canônicos ("saúde"→"saude").
- **Métrica de calibração:** histograma `agent_category_match_score{outcome}` nos 3 caminhos de escrita.
  Runbook `docs/runbooks/categorization-scoring.md` com PromQL e procedimento de tuning.
- **Seed expandido:** 37→55 termos curados (telefonia/transporte/combustível/saúde/streaming/games/
  investimentos), `WHERE NOT EXISTS` mantém anti-colisão.
- **Assert HTTP:** handler test afirma `"score"`/`"match_quality"` no corpo JSON.
- **Cobertura extra:** testes de confirmação/low-score no caminho compartilhado (cartão+recorrente),
  unit tests dos builders SQL, `BenchmarkResolveScored` (~3.6µs/resolução).
- **Lint:** `golangci-lint v2` (instalado local) `run ./...` = **0 issues**; `task lint:run` completo
  (lint + auth-bypass + outbox-user-id) = pass.

## Ressalva eliminada — policy total e verificada (sem calibração pendente)

A "ressalva de calibração" foi **removida**: o score vem de um domínio **finito** de `5×3×3 = 45`
combinações, não de uma distribuição contínua. A policy é **total e provada** sobre as 45
(`TestScoreLatticeIsTotalAndOnlyExactAutoLogs`): exatamente **5 auto-logam, todas EXATO; zero token/fuzzy**.
Logo os thresholds **não dependem de dado de produção para correção** — `auto-log ⟺ termo exato` é
invariante de código, não parâmetro a tunar. `fuzzyMinSimilarity` é o único valor contínuo e **não afeta
correção** (fuzzy nunca auto-loga). Tabela completa da treliça em `docs/runbooks/categorization-scoring.md`.

O que ainda evolui com tráfego é **recall** (frequência de ajudar sem perguntar) — **não correção**:
dicionário não-coberto → pergunta/rejeita, nunca registra errado. Métrica `agent_category_match_score` +
runbook servem para expandir dicionário, não para mexer em threshold. `WHERE NOT EXISTS` mantém
anti-colisão. Sem mudança de comportamento no caminho exato; campos de output são aditivos.
