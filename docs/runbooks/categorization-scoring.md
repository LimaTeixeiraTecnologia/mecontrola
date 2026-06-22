# Runbook — Categorização: pipeline scored, gate de confirmação e calibração

Escopo: caminho `category_hint → SearchDictionary (exato → token → fuzzy) → MatchScore → gate do agente`
dos módulos `internal/categories` e `internal/agent`. Cobre o motor determinístico de match, o score
composto, o gate de confirmação por score e o procedimento de calibração de thresholds.

## Arquitetura resumida

- **Pipeline staged** (`internal/categories/application/usecases/search_dictionary.go`): estágios
  curto-circuitados — `Search` (exato) → `SearchTokens` (token) → `SearchFuzzy` (pg_trgm). Token só
  roda se exato vazio; fuzzy só se exato+token vazios. Eficiente e econômico (1 query por estágio
  necessário; fuzzy usa índice GIN trigram `dictionary_term_trgm_idx`).
- **Normalização/tokenização** (`domain/valueobjects/search_query.go`): tokens minúsculos,
  **unaccentados** (via `x/text/unicode/norm`), sem stopwords PT-BR, deduplicados. Min 2 runes.
- **Score composto** (`domain/valueobjects/match_score.go`):
  `score = 0.45·(precedence/5) + 0.30·Confidence.Weight() + 0.25·MatchQuality.Weight()`, clamp [0,1].
  - Confidence: high=1.0, medium=0.66, low=0.33. Quality: exact=1.0, token=0.8, fuzzy=0.5.
- **Garantia inegociável (sem falso positivo):** **só match EXATO pode auto-logar.** Token tem teto
  `0.79` e fuzzy teto `0.70`, ambos abaixo de `ScoreAutoThreshold=0.80`. Logo: **auto-log ⟺ o
  `category_hint` é exatamente um termo do dicionário** (canonical/alias/phrase/merchant/segment).
  Token e fuzzy sempre caem em confirmação ou não-encontrado.
- **Gate do agente** (`application/usecases/category_resolution.go` + `log_transaction_from_agent.go`):
  para os 3 caminhos de escrita (transação, compra de cartão, recorrência), via
  `resolveCategoryCandidate`:
  - `score ≥ 0.80` → **auto-loga** (só exato chega aqui).
  - `0.55 ≤ score < 0.80` → `CategoryNeedsConfirmationError` → WhatsApp pede confirmação.
  - `score < 0.55` → `ErrLogTransactionCategoryNotFound` → pede reformular.
  - ambiguidade (≥2 categorias, top-2 dentro de `delta=0.10`) → `CategoryAmbiguousError` → pergunta qual.

## Constantes de policy (calibráveis)

Em `internal/categories/domain/valueobjects/match_score.go` e `search_dictionary.go`:

| Constante | Valor | Efeito |
|-----------|-------|--------|
| `ScoreAutoThreshold` | 0.80 | acima → auto-loga (só exato alcança) |
| `ScoreConfirmThreshold` | 0.55 | abaixo → não-encontrado |
| `scoreAmbiguityDelta` | 0.10 | top-2 mais próximos que isso → ambíguo |
| `tokenScoreCeiling` | 0.79 | teto de qualquer match por token |
| `fuzzyScoreCeiling` | 0.70 | teto de qualquer match fuzzy |
| `fuzzyMinSimilarity` | 0.4 | similaridade trigram mínima para fuzzy retornar |

## Métricas

- `agent_category_match_score{outcome}` — **histograma** do score do top candidato por outcome
  (`auto_logged`, `needs_confirmation`, `low_score`, `ambiguous`). Fonte primária de calibração.
- `agent_log_transaction_resolve_failed_total{reason}` / `agent_log_card_purchase_failed_total{reason}`
  / `agent_create_recurring_failed_total{reason}` — contadores por `reason`
  (`no_match`, `needs_confirmation`, `low_score`, `ambiguous`, `resolver_failed`, `create_failed`).
- `agent_log_transaction_persisted_total{direction}` — auto-logs efetivados.

Cardinalidade controlada: nenhum label carrega `user_id`/`category_id`.

## Totalidade da policy (não há calibração de correção pendente)

O score **não é contínuo**: vem de um domínio **finito** de `5 signal × 3 confidence × 3 quality = 45`
combinações. A policy é **total e verificada** sobre todas elas (teste
`TestScoreLatticeIsTotalAndOnlyExactAutoLogs`): exatamente **5 combinações auto-logam, todas EXATO**;
**zero** combinações token/fuzzy auto-logam. Portanto não há espaço contínuo a "aprender" e os thresholds
**não dependem de dado de produção para garantir correção** — `auto-log ⟺ termo exato` é invariante.

Treliça completa (score após teto; balde):

| quality | canonical | alias | phrase | merchant | segment |
|---------|-----------|-------|--------|----------|---------|
| exact (high/med/low) | **A/A**/c | **A/A**/c | **A**/c/c | c/c/r | c/r/r |
| token (high/med/low) | c/c/c | c/c/c | c/c/c | c/c/r | c/r/r |
| fuzzy (high/med/low) | c/c/c | c/c/c | c/c/r | c/r/r | r/r/r |

`A`=auto-log, `c`=confirma, `r`=rejeita (não-encontrado). Únicos `A`: exato {canonical-high,
canonical-med, alias-high, alias-med, phrase-high}. `fuzzyMinSimilarity` (0.4) é o único parâmetro
contínuo e **não afeta correção** — fuzzy nunca auto-loga; mexer nele só muda o que é *sugerido* numa
confirmação.

## Observabilidade de recall (expandir dicionário, não calibrar thresholds)

Com a correção garantida pela totalidade acima, o que melhora com tráfego é apenas **recall** (com que
frequência ajudamos sem perguntar) — nunca correção. Métrica e procedimento abaixo servem para **expandir
o dicionário**, não para mexer em threshold:

1. **Taxa de confirmação vs auto** (atrito vs cobertura):
   ```promql
   sum(rate(agent_category_match_score_count{outcome="needs_confirmation"}[1d]))
   / sum(rate(agent_category_match_score_count[1d]))
   ```
   Alvo saudável: confirmação < ~30%. Se muito alto → faltam sinônimos no dicionário (ver passo 4),
   **não** baixar `ScoreAutoThreshold` (isso reabriria risco de falso positivo).

2. **Distribuição de score na faixa de confirmação** (onde mora a dúvida):
   ```promql
   histogram_quantile(0.5, sum(rate(agent_category_match_score_bucket{outcome="needs_confirmation"}[1d])) by (le))
   ```

3. **Taxa de não-encontrado** (cobertura insuficiente do dicionário):
   ```promql
   sum(rate(agent_log_transaction_resolve_failed_total{reason=~"no_match|low_score"}[1d]))
   ```
   Subindo → expandir seed de sinônimos.

4. **Expandir dicionário guiado por dado:** revisar (via logs/trace) os `hint` que caíram em
   `no_match`/`low_score` e adicionar como `alias`/`merchant` numa nova migração de seed
   (modelo: `migrations/000016_seed_dictionary_synonyms.up.sql`, com `WHERE NOT EXISTS` para
   evitar colisão e ambiguidade cruzada). Termos inequívocos → `alias`/`high` (auto no caminho exato);
   marcas ambíguas (ifood, rappi) → `merchant`/`medium` (sempre confirmam).

5. **Tuning de fuzzy:** se fuzzy estiver sugerindo categorias erradas demais em confirmação, subir
   `fuzzyMinSimilarity` (0.4 → 0.5). Fuzzy nunca auto-loga, então o risco é só de sugestão ruim, não
   de registro errado.

**Regra de ouro:** nunca relaxar `ScoreAutoThreshold` nem permitir token/fuzzy auto-logar. A garantia
de não-falso-positivo depende de "auto-log ⟺ termo exato". Aumentar cobertura é problema de dicionário,
não de threshold.

## Verificação

- Unit: `go test ./internal/categories/... ./internal/agent/application/...`
- Garantia: `TestOnlyExactCanAutoLog`, `TestFuzzyNeverReachesAutoThreshold` em
  `domain/valueobjects/match_score_test.go`.
- Integração (Docker): `go test -tags integration ./migrations/` e
  `go test -tags integration ./internal/categories/infrastructure/repositories/postgres/`
  (cobre `SearchTokens`, `SearchFuzzy` com typo real e gibberish).
