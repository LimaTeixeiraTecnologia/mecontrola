# ADR-008 — Cache de token OAuth Kiwify in-memory com re-auth em 401

## Metadados

- **Título:** Estratégia de gerenciamento de token OAuth Kiwify
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-31a, D-05), `techspec.md` §Adapter Kiwify, `docs.kiwify.com.br/api-reference/auth/oauth.md`

## Contexto

Documentação oficial Kiwify confirma:
- `POST https://public-api.kiwify.com/v1/oauth/token`
- Body `application/x-www-form-urlencoded` com `client_id` + `client_secret`
- Response: `access_token` (JWT Bearer), `expires_in: 86400` (24h), `scope` — **sem refresh_token**
- Renovação por re-autenticação no mesmo endpoint
- Rate limit global da API: 100 req/min; rate limit específico de `/oauth/token` não documentado

`FetchSubscription` (RF-31) é chamado pelo job de reconciliação horária. Em escala de 5k subs ativas, em 1h são até 5k chamadas — todas precisam de Bearer token válido.

## Decisão

`OAuthClient` em `internal/billing/infrastructure/http/client/kiwify/oauth.go` cacheia o token em memória com TTL = `expires_in − KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN` (default 5min, configurável). Estratégia:

1. **Lookup com lock leitura** (RWMutex.RLock): retorna token cacheado se `now < expiresAt`.
2. **Refresh com lock escrita** (RWMutex.Lock + double-check): se cache expirou, faz `POST /v1/oauth/token`, salva token + expiresAt.
3. **Retry em 401** no caller (adapter): em response 401 do `FetchSubscription`, força refresh (descarta cache) e retenta uma vez. Falha persistente → erro propagado.
4. **Sem refresh_token** — toda renovação é client credentials flow novamente.
5. **Token compartilhado por processo** — adapter Kiwify é singleton em billing_subsystem; um único `OAuthClient` instância serve processor + reconciliation.

## Alternativas Consideradas

### Re-auth a cada chamada (sem cache)

- Vantagem: simplicidade absoluta.
- Desvantagem: 5k re-auths/h durante reconciliation → estoura rate limit do `/oauth/token` (assumindo limite implícito de ~100 req/min).
- Rejeitada por escala.

### Cache em Redis para compartilhar entre processos

- Vantagem: dois processos (api + worker) compartilham token.
- Desvantagem: Redis não está no stack (PRD D-02). Token é cheap (refresh é só 1 POST a cada 24h por processo); custo de compartilhar > benefício.
- Rejeitada por excesso.

### Background refresh proativo (goroutine periódica)

- Vantagem: zero latência em refresh.
- Desvantagem: mais código + goroutine para gerenciar; benefício marginal (refresh on-demand é < 200ms).
- Rejeitada por overhead.

## Consequências

### Benefícios Esperados

- ≤ 2 refreshes/dia por processo (1 inicial + 1 antes da margem).
- Latência de `FetchSubscription` previsível: ~50ms p99 com cache quente, ~250ms p99 em refresh.
- Sem complexidade de bg goroutine.

### Trade-offs e Custos

- Cada processo mantém seu próprio token — 2 processos = 2 tokens. Negligível.
- Lock contention em high QPS — RWMutex permite múltiplas leituras concorrentes; refresh é raro.

### Riscos e Mitigações

- **Risco:** clock skew entre Kiwify e nosso processo causa token "ainda válido localmente, mas inválido na Kiwify". **Mitigação:** safety margin 5min cobre skew razoável; retry em 401 cobre casos extremos.
- **Risco:** thundering herd se 2 processos refrescam simultaneamente após restart coordenado. **Mitigação:** rate limit do `/oauth/token` é baixíssimo demanda; mesmo 2 concorrentes não causa problema.
- **Risco:** secrets em memória vazam via heap dump. **Mitigação:** padrão Go genérico; mitigação fora do escopo deste módulo (SO/container).

## Plano de Implementação

1. `oauth.go` implementa `OAuthClient` conforme techspec.
2. Construtor recebe `httpClient *http.Client` (compartilhado, OTel-instrumented), `baseURL`, `clientID`, `clientSecret`, `safetyMargin time.Duration`, `clock Clock`.
3. Método `Token(ctx) (string, error)` é a única API pública.
4. Adapter Kiwify (`adapter.go`) chama `oauthClient.Token(ctx)` antes de cada request; em 401 chama `oauthClient.ForceRefresh(ctx)` e retenta.
5. Teste: tabela cobrindo cache hit/miss/expirado, falha de auth (4xx), 5xx (retry transitório).

## Monitoramento e Validação

- Métrica `kiwify_oauth_refresh_total{outcome}` (counter) com `outcome ∈ {success, failure}`.
- Métrica `kiwify_oauth_token_age_seconds` (gauge) = `now - lastRefreshAt`.
- Span OTel `kiwify.oauth.fetch_token` em refresh.
- Alerta em `failure` > 0 em 5min consecutivos.

## Impacto em Documentação e Operação

- AGENTS.md billing documenta padrão de cache.
- Runbook: como rotacionar `KIWIFY_CLIENT_SECRET` (reset de processo invalida cache; novo secret pega na próxima request).

## Revisão Futura

- Se Kiwify publicar `refresh_token`, simplificar lógica.
- Se introduzirmos > 2 processos paralelos, considerar Redis para token compartilhado.
