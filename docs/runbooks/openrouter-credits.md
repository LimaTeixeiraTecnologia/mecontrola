# Runbook — Créditos OpenRouter

Alertas cobertos: `mc-openrouter-credit-low` (warning), `mc-openrouter-credit-critical` (critical), `mc-openrouter-credits-stale` (warning). Todos roteados ao Telegram (rota `alertname =~ "mc-openrouter-.*"` em `contact-points.yaml`).

## Fonte dos dados

O worker (`cmd/worker/worker.go`, `startOpenRouterCreditsMonitor`) consulta a cada 15 minutos:

- `GET {OPENROUTER_BASE_URL}/api/v1/credits` → `total_credits`, `total_usage` (saldo = diferença, calculado em código).
- `GET {OPENROUTER_BASE_URL}/api/v1/auth/key` → `usage_daily`, `usage_weekly`, `usage_monthly`.

Gauges publicados: `openrouter_credits_total_usd`, `openrouter_credits_used_usd` (monotônico), `openrouter_credits_remaining_usd`, `openrouter_usage_daily_usd`, `openrouter_usage_weekly_usd`, `openrouter_usage_monthly_usd`, `openrouter_credits_last_success_timestamp_seconds`, e o counter `openrouter_credits_scrape_errors_total`.

## mc-openrouter-credit-low / mc-openrouter-credit-critical

1. Confirmar o saldo real no painel: https://openrouter.ai/credits (o valor do alerta vem da mesma API, então divergência indica métrica stale — ver seção abaixo).
2. Verificar o ritmo de gasto no dashboard **MeControla — OpenRouter Credits** (pasta MeControla no Grafana): gasto/dia real (`increase(openrouter_credits_used_usd[24h])`) e dias restantes projetados (ritmo médio semanal).
3. Se o gasto estiver anormalmente alto, investigar `agent_llm_provider_call_total` e `agent_stt_cost_microusd_total` por modelo no dashboard **MeControla — Agents Runtime** — um loop de agente ou volume de áudio inesperado é a causa mais provável.
4. Recarregar em https://openrouter.ai/credits. O alerta resolve sozinho em até ~20 minutos (poll de 15 min + avaliação).

## mc-openrouter-credits-stale

Significa que os alertas de saldo estão **cegos** — tratar antes de confiar em "está tudo bem".

1. `openrouter_credits_scrape_errors_total` crescendo → a API do OpenRouter está falhando ou a chave expirou/foi revogada. Logs do worker: `openrouter credits poll failed`.
2. Sem erros de scrape mas sem série → worker sem o monitor (versão antiga em produção) ou worker down (ver `mc-worker-down`).
3. Testar a chave manualmente: `curl -H "Authorization: Bearer $OPENROUTER_API_KEY" https://openrouter.ai/api/v1/credits` na VPS.
