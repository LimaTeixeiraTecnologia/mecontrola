# Runbook — Agent: parser de intent, política de confiança e idempotência

Escopo: caminho `mensagem recebida → ParseInbound (LLM) → política → roteamento → tool → persistência`
do módulo `internal/agent`. Cobre o parser de structured output, a cadeia de fallback de LLM,
a política de confiança e a idempotência por `message_id`.

## Arquitetura resumida

- **Parser** (`internal/agent/application/usecases/parse_inbound.go`): envia `response_format: json_schema`
  (`Strict: false`) ao OpenRouter e decodifica o intent. O decoder é tolerante (strip de fences,
  defaults, fallback para `unknown`). Extrai `confidence` (0..1); ausência → default neutro `1.0`.
- **Cadeia de LLM** (`services/fallback_chain.go` + `circuit_breaker.go`): primário + fallbacks;
  circuit breaker por provider.
- **Política** (`domain/services/policy_evaluator.go`): para intents de **escrita**
  (`intent.Kind.IsWrite()`), `confidence < AGENT_POLICY_MIN_CONFIDENCE` (default 0.8) ⇒ pede
  esclarecimento (não executa). Leituras sempre passam.
- **Idempotência** (`services/decision_audit.go` + tabela `agent_decisions`, único
  `user_id+channel+message_id`): antes de executar escrita, `FindByMessage`; se já existe ⇒ replay
  (não reexecuta). Em corrida, o `Insert` perde a corrida e retorna conflito ⇒ replay.

## Modelos suportados no parser (CRÍTICO)

O parser **só** funciona com modelos que honram `response_format json_schema` no OpenRouter.
Validado via `RUN_REAL_LLM` (recognition matrix 100%):

| Modelo | Parser json_schema | Papel |
|--------|--------------------|-------|
| `google/gemini-2.5-flash-lite` | ✅ | primário |
| `mistralai/mistral-small-3.2-24b-instruct` | ✅ | fallback |
| `anthropic/claude-haiku-4.5` | ❌ (retorna unknown) | **só onboarding** (tool-calling) |
| `openai/gpt-5-nano` | ❌ (e lento) | não usar |

**Nunca** colocar haiku ou gpt-5-nano em `AGENT_LLM_PRIMARY_MODEL`/`AGENT_LLM_FALLBACK_MODELS`.
Antes de trocar primário/fallback, rodar o guard:
`RUN_REAL_LLM=1 LLM_TEST_MODEL=<modelo> go test -tags=integration ./internal/agent/e2e/ -run TestParseInbound_RealLLM_ProductionChain`.

## Métricas

- `agent_intent_parsed_total{kind,outcome}` — outcome `ok` vs `fallback_*`/`provider_error`.
- `agent_intent_confidence_histogram{kind}` — distribuição de confiança.
- `agent_intent_routed_total{kind,channel,outcome}` — `routed`/`fallback`/`policy_blocked`/`replay`/...
- `agent_policy_blocks_total{kind}` — escritas bloqueadas por baixa confiança.
- `agent_idempotency_replay_total{kind}` — replays idempotentes.
- `agent_llm_provider_errors_total{model,reason}`, `agent_llm_provider_call_total{model,status}`,
  `agent_llm_provider_latency_seconds`, `agent_llm_fallback_exhausted_total`.

Consultar por label `job` (stack otel-lgtm), não `service_name`.

## Resposta a alertas (`deployment/monitoring/prometheus-rules.yaml`, grupo `mecontrola.agent`)

### AgentParseUnknownRateHigh (critical)
Parser caindo em unknown/fallback > 30%. Causa nº 1: modelo parou de honrar json_schema.
1. `agent_intent_parsed_total{outcome}` — qual outcome domina (`provider_error` vs `fallback_*`).
2. Conferir `AGENT_LLM_PRIMARY_MODEL`/`AGENT_LLM_FALLBACK_MODELS` contra a tabela acima.
3. Se primário degradou, confirmar que o fallback (mistral) está assumindo (`agent_llm_provider_call_total{model}`).
4. Rodar o guard `TestParseInbound_RealLLM_ProductionChain`.

### AgentLLMFallbackExhausted (critical)
Todos os providers falharam. Verificar `OPENROUTER_API_KEY`, créditos (HTTP 402), rate limit (429),
circuit breaker aberto (`agent_llm_fallback_skipped_total{state}`) e status do OpenRouter.

### AgentLLMProviderErrorRateHigh (warning)
Erro de provider > 20%. Ver `reason` (timeout/rate_limited/upstream_5xx/no_credit) e ajustar
`AGENT_LLM_REQUEST_TIMEOUT` ou aguardar recuperação do upstream.

### AgentPolicyBlockRateHigh (warning)
Política bloqueando > 25% das escritas. Ver `agent_intent_confidence_histogram`: se a massa caiu
abaixo de 0.8, o modelo degradou (investigar como AgentParseUnknownRateHigh) ou o threshold
`AGENT_POLICY_MIN_CONFIDENCE` está agressivo demais para o modelo atual.

### AgentIdempotencyReplaySpike (warning)
Muitos replays. Normalmente reentrega de webhook (WhatsApp/Telegram reenviando). Confirmar que o
guard evita dupla execução (lançamentos não duplicados em `mecontrola.transactions`); investigar a
origem das mensagens repetidas.

## Flags relevantes

- `AGENT_POLICY_MIN_CONFIDENCE` (default 0.8) — threshold de escrita.
- `AGENT_LLM_PRIMARY_MODEL` / `AGENT_LLM_FALLBACK_MODELS`.
- `AGENT_LLM_CIRCUIT_FAILURES|WINDOW|COOLDOWN` — circuit breaker.
- `AGENT_LLM_REQUEST_TIMEOUT`.

## Notas

- Lançamentos do agent são para relatório (sem dinheiro real); não há gate de confirmação. A
  prioridade é idempotência/rastreabilidade.
- O replay devolve a resposta **redigida** (PII mascarada) quando disponível, senão um ack estável.
