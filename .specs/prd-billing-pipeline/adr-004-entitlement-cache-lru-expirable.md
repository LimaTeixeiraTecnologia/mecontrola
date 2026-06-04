# ADR-004 — Cache de entitlement com `hashicorp/golang-lru/v2/expirable`

## Metadados

- **Título:** Implementação de cache in-memory do `EntitlementService`
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-32, RF-33, RF-35, RF-36, D-02), `techspec.md` §Cache LRU, `go.mod`

## Contexto

PRD decidiu cache in-memory sem Redis (D-02), com TTL = 5min e capacidade limitada (RF-36 default 50k entries). Implementações possíveis:
- `hashicorp/golang-lru/v2/expirable` — LRU+TTL nativo, 0 indiretas, Hashicorp.
- `jellydator/ttlcache/v3` — TTL+LRU com hooks ricos.
- `sync.Map` + struct entry + goroutine cleanup custom — zero novas deps mas ~150 linhas custom.

`go.mod` atual não tem nenhuma das duas; confronto: precisamos da menor superfície e maior estabilidade.

## Decisão

Adotar `github.com/hashicorp/golang-lru/v2 v2.x` como dependência direta nova; usar `expirable.NewLRU[K, V](size, onEvict, ttl)` como motor do `EntitlementLRU`.

**TTL fixo de 5 minutos** (constante por construção) — `expirable.LRU` não suporta TTL per-entry. RF-33 do PRD pede `TTL = min(period_end - now, 5min)`. Aceita-se TTL fixo de 5min como aproximação superior conservadora: para qualquer caso onde `period_end - now > 5min`, o TTL é exatamente o desejado; para casos onde `period_end - now < 5min`, o cache pode servir uma decisão por até `period_end - now` segundos a mais que o ideal, mas a decisão (granted/denied) está correta — só a expiração visível ao cliente atrasa segundos.

**Capacidade default 50_000 entries** (cobre 10× a meta de 5k subs ativas, deixando folga para usuários inativos cacheados temporariamente).

## Alternativas Consideradas

### `jellydator/ttlcache/v3`

- Vantagem: TTL per-entry nativo + hooks (`OnInsertion`, `OnEviction`); mais flexibilidade.
- Desvantagem: API maior; superfície adicional para manter; popularidade menor que Hashicorp LRU.
- Rejeitada por adicionar features que o MVP não usa.

### `sync.Map` + cleanup goroutine custom

- Vantagem: zero dependência nova.
- Desvantagem: ~150 linhas (LRU manual via lista duplamente ligada + TTL + cleanup ticker) + testes de concorrência; risco de bug sutil.
- Rejeitada por trade-off não favorecer custom code.

## Consequências

### Benefícios Esperados

- API mínima e estável (`Get`, `Add`, `Remove`).
- Manutenção zero — biblioteca Hashicorp.
- Implementação <30 linhas no wrapper `EntitlementLRU`.
- Atende RF-32/33/35/36 com perda funcional aceitável (TTL fixo).

### Trade-offs e Custos

- Nova dependência direta — adiciona ~5k linhas Go ao supply chain (mas é Hashicorp, audit risk baixo).
- TTL fixo de 5min em vez de dinâmico — limitação documentada; ganho marginal de dinâmico não justifica complexidade.

### Riscos e Mitigações

- **Risco:** memory pressure se capacidade for muito alta. **Mitigação:** default 50k; alerta em métrica de uso de heap por processo.
- **Risco:** Hashicorp arquivar a lib (improvável). **Mitigação:** wrapper isola dependência; troca de impl é localizada se necessário.

## Plano de Implementação

1. `go get github.com/hashicorp/golang-lru/v2@latest`.
2. Criar `internal/billing/infrastructure/cache/entitlement_lru.go` com wrapper minimal.
3. Construtor recebe `capacity int` e `ttl time.Duration` para parametrização.
4. Wire em `billing_subsystem.go` lendo `BillingConfig.EntitlementCacheCapacity` e `BillingConfig.EntitlementCacheTTL`.
5. Testes unitários: hit, miss, eviction por capacidade (insere capacity+1 e verifica oldest evicted), TTL expira após sleep curto (com `clock.FakeClock` se exposto pela lib; senão, testar com TTL=10ms).

## Monitoramento e Validação

- Métrica `entitlement_cache_hit_ratio` calculada via `Get` retornando `ok` (cache hit) ou não (miss).
- Métrica `entitlement_cache_size` via `Len()` da LRU em loop periódico (a cada 30s).
- Alerta em hit_ratio < 0.85 em janela de 15min.

## Impacto em Documentação e Operação

- AGENTS.md billing documenta TTL fixo.
- Runbook: forçar invalidação geral via SIGUSR1 ou comando admin (TBD em PRD futuro).

## Revisão Futura

- Se métricas mostrarem `period_end - now < 5min` materializando em > 0.5% das decisões, considerar TTL dinâmico via `ttlcache/v3` ou impl custom.
- Se introduzir Redis (gatilho em D-02), substituir esta camada por cliente Redis com mesmo wrapper interface.
