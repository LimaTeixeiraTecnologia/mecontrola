# Runbook — Agent LLM (OpenRouter)

## Visão geral

O agente LLM (em `internal/agent/`) interpreta mensagens NL via OpenRouter, valida intent, e despacha para use cases existentes (`categories`/`cards`/`budgets`/`transactions`). Roda em dois modos:

- `AGENT_MODE=stub` (default seguro): responde template estático.
- `AGENT_MODE=openrouter`: chama LLM real com fallback chain e circuit breaker.

## Toggle e rollback

```bash
# Rollback imediato (sem deploy):
kubectl set env deploy/mecontrola-api AGENT_MODE=stub
kubectl rollout restart deploy/mecontrola-api

# Re-ativar:
kubectl set env deploy/mecontrola-api AGENT_MODE=openrouter
kubectl rollout restart deploy/mecontrola-api
```

`OPENROUTER_API_KEY` ausente em modo openrouter → fail-fast no boot com `ErrAPIKeyRequired`.

## Métricas Prometheus

### Outcomes

- `agent_llm_outcome_total{outcome}` — outcomes do agente:
  - `routed` — intent válido e despachado
  - `structured_error` — LLM retornou erro estruturado (not_found, out_of_scope, etc.)
  - `provider_exhausted` — todos providers falharam (fallback chain)
  - `unsupported_action` — module/action fora da allowlist

### Provider chain

- `agent_llm_provider_call_total{model, status}` — chamadas por modelo
- `agent_llm_provider_errors_total{model, reason}` — falhas por categoria (`transport`, `decode`, `upstream_5xx`, `client_4xx`, `rate_limited`, `unauthorized`, `no_credit`)
- `agent_llm_provider_latency_seconds{model}` — buckets `[0.1, 0.25, 0.5, 1, 2, 5, 10]`
- `agent_llm_fallback_attempts_total{model, outcome}` — tentativas da chain (`ok` ou `error`)
- `agent_llm_fallback_skipped_total{model, state}` — providers pulados (circuit open)
- `agent_llm_fallback_exhausted_total` — chain totalmente esgotada

### Dispatch

- `agent_llm_dispatch_total{module, action, outcome}` — applied / unsupported / error
- `agent_llm_dispatch_errors_total{module, action, reason}` — use_case / unsupported

### Prompt loader

- `agent_llm_prompt_loader_failures_total{source}` — categories / cards (graceful degradation)

## Alertas sugeridos

| Alerta | Threshold | Severidade | Ação |
|---|---|---|---|
| `provider_exhausted` taxa | > 5% por 10min | warning | Investigar fallback chain (todos modelos falhando) |
| `circuit_open` para `gemini-2.5-flash-lite` | > 5min | critical | Falha do primary; chain absorvendo; checar OpenRouter status page |
| `outcome=structured_error` | > 30% por 1h | warning | LLM rejeitando muitos pedidos válidos; revisar system prompt |
| `dispatch_errors{reason=use_case}` | > 10/min | critical | Use case quebrando — investigar logs do módulo afetado |
| `latency_seconds{model=gemini} p95` | > 2s por 5min | warning | LLM lento; usuários percebendo lag |
| `prompt_loader_failures{source=cards}` | > 5% por 1h | warning | Banco lento; degrada qualidade do prompt |

## Circuit breaker

State machine: `Closed → Open → HalfOpen → Closed`.

- Abre após **5 falhas em 30s** (`AGENT_LLM_CIRCUIT_FAILURES`, `AGENT_LLM_CIRCUIT_WINDOW`).
- Permanece **Open por 60s** (`AGENT_LLM_CIRCUIT_COOLDOWN`).
- Em HalfOpen, próxima request é testada; sucesso → Closed; falha → Open.

Quando circuito abre, fallback chain pula o provider. Se TODOS abertos → `provider_exhausted`.

## Outbox events

Eventos publicados em `outbox_events`:

- `agent.intent.executed.v1` — intent routed + use case aplicado
- `agent.intent.rejected.v1` — outros outcomes

Payload inclui `provider_used`, `latency_ms`, `prompt_tokens`, `completion_tokens` — auditoria de custo + qualidade.

Consumir via `outbox_events` para BI dashboards.

## Trust model em mutations NL

**Boundary de idempotência:** dedup do canal de entrada (`channel_processed_messages`).

Cada webhook delivery (WA wamid ou TG update_id) é processado **no máximo uma vez** via `INSERT ... ON CONFLICT DO NOTHING`. Retries do webhook (Meta/Telegram) retornam `OutcomeDuplicate` no dispatcher e NÃO chegam ao agente.

**Não há replay vector dentro do `HandleInboundMessage`:**
- Sem retry interno no dispatcher LLM
- Failures propagam para o webhook handler → 200/503
- Chi não tem retry built-in

**O que NÃO é proteção:**
Usuário enviar a MESMA mensagem 2× ("gastei 50") em 30s gera DUAS transações. Intencional — não dá pra distinguir replay de gasto legítimo idêntico (almoço + jantar de R$ 50). UX de "apaga o último" cobre engano via NL.

**Não foi implementado idempotency.Storage Get no `HandleInboundMessage`** porque:
1. Sem replay vector real (webhook dedup já cobre)
2. Key estável bloquearia gastos legítimos repetidos
3. Key fresh (uuid) é teatro — Get nunca acha
4. Adicionar dep sem caso de uso concreto viola "no premature defense in depth"

Se vetor surgir (ex: dispatcher ganhar retry), injetar `idempotency.Storage` com Key = hash(userID‖channel‖text‖module‖action‖bucket_5min).

## Custo

Custo nominal (10k req/dia, sem cache hit Gemini):
- Primary 95%: ~$31.50/mês
- Fallbacks 5% combinado: ~$3-5/mês
- **Total estimado: ~$34/mês**

Com padding ≥1024 tok ativando cache implícito Gemini: **~$17/mês**.

Métrica de custo: somar `agent_llm_provider_latency_seconds_count{model=*}` × preço/1M tokens (calculado off-line via `prompt_tokens` + `completion_tokens` no outbox).

## Troubleshooting

### Sintoma: usuários recebem "Tive uma instabilidade momentanea"

1. Checar `outcome=provider_exhausted` — toda chain falhou
2. Checar `agent_llm_provider_errors_total{reason}` — qual erro dominante?
   - `transport` → conectividade OpenRouter; checar `httpclient` métricas
   - `upstream_5xx` → OpenRouter ou providers em incidente; status page
   - `rate_limited` → quota OpenRouter excedida; revisar plan
   - `unauthorized` → key vazada/revogada; rotacionar
   - `no_credit` → conta sem crédito OpenRouter

### Sintoma: respostas inconsistentes / shape errado

1. Checar `outcome=structured_error` rate
2. Logs com `agent.llm.validator: forbidden field` → LLM tentou injetar `user_id`/`tenant_id` (prompt injection?). Investigar usuário.
3. Logs com `agent.llm.validator: module`/`action` invalid → LLM emitiu valor fora da allowlist; revisar prompt para reforçar enum.

### Sintoma: custo escalando

1. Métrica `agent_llm_provider_call_total{model=anthropic/claude-haiku-4.5, status=ok}` deve ser <1% — se >5%, fallback chain está chegando no caro mais que esperado.
2. Investigar `agent_llm_provider_errors_total` — primary com taxa alta de erro força fallback.
3. Verificar padding do system prompt: `len(systemPrompt) >= 4400 chars` (≥1100 tokens) para cache Gemini.

## Smoke manual

```bash
# Telegram webhook smoke (com TELEGRAM_ENABLED=true)
go run ./scripts/smoke/telegram_webhook \
  --url http://localhost:8080/api/v1/channels/telegram/webhook \
  --secret "$TELEGRAM_SECRET_TOKEN" \
  --from-id 987654321 \
  --text "Quanto gastei esse mes?"

# Sem header (deve 401)
go run ./scripts/smoke/telegram_webhook --missing-header
```

## Validação pré-canary

- [ ] Migrations 18, 19, 20, 21 aplicadas em prod
- [ ] Métrica `auth_resolve_path_total{path=identity}` >95% (dual-read estabilizado)
- [ ] `TELEGRAM_ENABLED=true` em staging por ≥7 dias sem regressão WhatsApp
- [ ] `AGENT_MODE=openrouter` em canary 10% por ≥48h
- [ ] `outcome=provider_exhausted` < 1%
- [ ] Custo observado <$50/mês extrapolado
- [ ] Latência p95 < 1.5s
