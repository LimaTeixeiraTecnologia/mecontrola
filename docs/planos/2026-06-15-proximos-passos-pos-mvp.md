# Próximos Passos Pós-MVP — Multi-canal + LLM Intent

> **Nota de 2026-06-15:** as seções abaixo que citam migrations `000018`-`000022`, backfill incremental e observação de tabelas legadas foram superadas pela adoção da baseline única `V0` em `migrations/000001_initial_baseline.*`. Use este documento apenas como histórico de planejamento.

> **Skill obrigatória ao retomar:** `go-implementation` carregada antes de qualquer edição em `.go`. Padrão canônico: `internal/transactions` (DMMF seletivo, ADR-006).
>
> **Estado de partida (2026-06-15):**
> - 13 milestones do MVP entregues (A1, A1.2, A2, A3, B1, B1.5, B2.a, B2.b, B3, Hardening, Legacy cleanup, D1, budgets.list)
> - 117 packages com testes passando
> - 12 falsos positivos eliminados em 6 rodadas de auditoria crítica
> - Build limpo + triple-run race + integration test (Postgres testcontainer)
> - Runbook `docs/runbooks/agent-llm.md` documentando trust model honesto

---

## 1. Rodada 7 de auditoria crítica (CONTINUAR antes do canary)

A curva de captura de falsos positivos **ainda não estabilizou**:
1→3→1→2→3→2. Cada rodada encontrou algo. Precisamos pelo menos 2 rodadas consecutivas com 0 FPs antes de declarar convergência genuína.

### Itens da rodada 7

| # | Área | Hipótese a falsificar | Como auditar |
|---|---|---|---|
| 7.1 | PromptBuilder edge cases | Padding ≥1024 tok não é atingido com seeds vazias; ou cresce sem teto | Test que mede `len([]rune(prompt))` para combinações: 0 cats/0 cards, 100 cats/100 cards, max strings |
| 7.2 | FallbackChain concurrency | Breaker `sync.Map` pode race em provider state cross-goroutines | Test concurrent com `t.Parallel()` + 50 goroutines chamando `Interpret` |
| 7.3 | Nil dep injection | Use cases com construtores `New*(deps...)` aceitando `nil` silenciosamente | Grep `if .* == nil` em construtores; verificar se faz validação |
| 7.4 | Acoplamento agent→identity | Validar que agent não importa código de produção do identity (só interface) | `go list -deps` no agent/llm e verificar imports |
| 7.5 | Context cancellation | Use cases com IO em loop respeitam `ctx.Done()`? | Test que cancela ctx no meio de `FallbackChain.Interpret` |
| 7.6 | Outbox event ordering | Eventos agent.intent.* publicados em ordem cronológica? | Verificar `occurred_at` strictly increasing em sequência rápida |
| 7.7 | Telegram bot identity hijack | TG webhook com `bot_id` diferente do configurado é aceito? | Dispatcher valida que update vem do bot esperado |
| 7.8 | Migration 022 disabled-state | Build/lint trata `.disabled` files corretamente? | `find migrations/ -name "*.sql.disabled"` deve não ser carregado por embed.go |
| 7.9 | i18n strings — XSS via Telegram | Reply text contém `<script>` ou `&` sem escape via HTML parse_mode? | Test com texto contendo `<`, `>`, `&` indo via TG gateway |
| 7.10 | Métricas com cardinality bomb | Algum counter recebe label dinâmico (user input)? | Grep `observability.String\(.*err\.Error\(\)` ou `.*payload\.` em counters |

**Critério de "pronto" pós-rodada 7:** se 0 FPs nesta rodada, executar rodada 8. Convergência = 2 rodadas consecutivas com 0 FPs.

---

## 2. Operacional pré-canary (paralelo à rodada 7)

### 2.1 Migrations em staging

```bash
# Ordem obrigatória:
task migrate-up   # aplica 18→19→20→21 (sequencial garantido pelo numbering)

# Validar:
psql -d mecontrola_staging -c "SELECT count(*) FROM mecontrola.user_identities;"
psql -d mecontrola_staging -c "SELECT count(*) FROM mecontrola.channel_processed_messages WHERE channel='whatsapp';"
```

Esperado:
- `user_identities`: N rows (backfill de WhatsApp users existentes)
- `channel_processed_messages`: M rows (backfill de meta_processed_messages)
- Migration 022 (drop legacy) **permanece `.disabled`** por +30 dias

### 2.2 Smoke staging — fase 1 (Telegram pipeline, sem LLM)

```bash
# Env vars staging:
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=<staging-bot-token>
TELEGRAM_BOT_ID=<int64>
TELEGRAM_SECRET_TOKEN=<random-256-chars>
AGENT_MODE=stub  # ← ainda em stub
```

Smoke:
```bash
# Verificar webhook autentica:
go run ./scripts/smoke/telegram_webhook --url http://staging/api/v1/channels/telegram/webhook --secret "$SECRET" --text "test"
# Esperado: 200, reply stub recebida no chat

# Verificar 401 sem header:
go run ./scripts/smoke/telegram_webhook --missing-header
# Esperado: 401
```

**Duração mínima:** 7 dias antes de prosseguir.

**Métricas a monitorar:**
- `telegram_dispatcher_route_total{outcome}` — distribuição esperada: ~95% agent, ~5% onboarding
- `telegram_stale_webhook_total` — deve ser ~0
- `auth_resolve_path_total{path=identity}` — deve subir até ~100% (dual-read estabilizado)
- WhatsApp metrics — **nenhuma regressão** vs baseline

### 2.3 Smoke staging — fase 2 (LLM canary 10%)

Quando fase 1 estável:

```bash
AGENT_MODE=openrouter
OPENROUTER_API_KEY=<staging-key>
AGENT_LLM_PRIMARY_MODEL=google/gemini-2.5-flash-lite
AGENT_LLM_FALLBACK_MODELS=openai/gpt-5-nano,mistralai/mistral-small-3.2-24b-instruct,anthropic/claude-haiku-4.5
AGENT_LLM_MAX_TOKENS=256
AGENT_LLM_REQUEST_TIMEOUT=8s
AGENT_LLM_CIRCUIT_FAILURES=5
AGENT_LLM_CIRCUIT_WINDOW=30s
AGENT_LLM_CIRCUIT_COOLDOWN=60s
```

**Duração mínima:** 48h em 10% do tráfego (via feature flag no dispatcher se houver, ou rate limit).

**Critérios de continuidade para ramp 100%:**
- `agent_llm_outcome_total{outcome=routed}` > 70% das interações com NL
- `agent_llm_outcome_total{outcome=provider_exhausted}` < 1%
- `agent_llm_provider_latency_seconds p95` < 1.5s
- Custo extrapolado < $50/mês a 10k req/dia
- Zero alerta crítico no Grafana

---

## 3. Backlog conhecido (não bloqueante para canary)

### 3.1 D1.next — new-user-via-Telegram (sem WhatsApp pré-ativado)

**Escopo atual D1:** cross-link apenas. User precisa ter ativado WhatsApp primeiro.

**Limitação:** se cliente paga via Kiwify, recebe magic-token por e-mail, e tenta ATIVAR direto no Telegram sem nunca ter usado WhatsApp:
- Status do token: `PAID` (não `CONSUMED`)
- Outcome: `ActivateTelegramOutcomeRequiresWhatsAppActivation`
- Mensagem: "Ative sua conta no WhatsApp primeiro."

**Feature pendente:** permitir ativação direta via Telegram criando User com `whatsapp_number = NULL`. Requer:
- Refator de `UpsertUserByWhatsApp` ou criar `UpsertUserByEmail` paralelo
- Migration aceitando `whatsapp_number NULL` na tabela `users` (ou NULLable via outra coluna)
- Bind subscription a User sem phone
- Backfill `tenant_identities` no novo fluxo
- Testes E2E cobrindo TG-first onboarding

**Estimativa:** ~3-5 dias de trabalho. PR dedicado pós-canary do D1 atual.

### 3.2 Migration 022 — drop legacy processed tables

**Pré-requisitos:**
- 30 dias mínimos pós-deploy de 21 sem rollback
- Métricas confirmando zero writes nas tabelas antigas:
  ```sql
  SELECT max(processed_at) FROM mecontrola.meta_processed_messages;
  SELECT max(processed_at) FROM mecontrola.telegram_processed_updates;
  ```
  Ambos devem ser anteriores ao deploy de 21.

**Ativação:**
```bash
mv migrations/000022_drop_legacy_processed_tables.up.sql.disabled \
   migrations/000022_drop_legacy_processed_tables.up.sql
mv migrations/000022_drop_legacy_processed_tables.down.sql.disabled \
   migrations/000022_drop_legacy_processed_tables.down.sql
# Commit + PR review obrigatório
```

### 3.3 Grafana dashboards

Métricas novas disponíveis em Prometheus; falta dashboard. Painéis recomendados (`docs/dashboards/agent-llm.json` futuro):

1. **Outcomes** — stacked area de `agent_llm_outcome_total{outcome}`
2. **Provider chain** — `agent_llm_fallback_attempts_total{model, outcome}` + `agent_llm_fallback_skipped_total`
3. **Latência por modelo** — heatmap de `agent_llm_provider_latency_seconds`
4. **Custo estimado** — `sum(rate(prompt_tokens))` × preço/1M + `sum(rate(completion_tokens))` × preço/1M
5. **Circuit breaker state** — gauge por modelo
6. **Dispatch outcomes** — `agent_llm_dispatch_total{module, action, outcome}`
7. **Telegram pipeline** — `telegram_dispatcher_route_total{outcome}` + `telegram_payload_rejection_total{reason}`
8. **Identity resolution path** — `auth_resolve_path_total{path}` (identity vs legacy ratio)
9. **TG onboarding** — `onboarding_activate_telegram_outcome_total{outcome}`

### 3.4 Alertas Prometheus

Conforme runbook, falta materializar em `docs/alerts/agent-llm.yaml`:

```yaml
groups:
  - name: agent_llm
    rules:
      - alert: AgentLLMProviderExhausted
        expr: rate(agent_llm_fallback_exhausted_total[5m]) > 0.05
        for: 10m
        labels: { severity: warning }
      - alert: AgentLLMCircuitOpenGemini
        expr: agent_llm_circuit_state{model="google/gemini-2.5-flash-lite"} == 2
        for: 5m
        labels: { severity: critical }
      - alert: AgentLLMHighLatency
        expr: histogram_quantile(0.95, rate(agent_llm_provider_latency_seconds_bucket[5m])) > 2
        for: 5m
        labels: { severity: warning }
      - alert: TelegramHighRejectionRate
        expr: rate(telegram_payload_rejection_total[5m]) / rate(telegram_dispatcher_route_total[5m]) > 0.3
        for: 10m
        labels: { severity: warning }
```

### 3.5 Outras melhorias de longo prazo

- **B2.c get-by-id intents** — quando intent contém `{"id": "..."}` específico (categories.get, cards.get, transactions.get)
- **update intents** — categories.update, cards.update, transactions.update (com confirmação explícita)
- **Prompt context caching** — Redis cache de PromptSeed por user_id (TTL 5min) para reduzir queries em pico
- **Circuit breaker persistente** — atualmente em memória por processo; em deploy rolling perde estado. Considerar Redis.
- **Multi-region** — quando crescer, separar circuit breakers por região para evitar tempestades cross-region

---

## 4. Riscos conhecidos e mitigações

| Risco | Probabilidade | Impacto | Mitigação atual |
|---|---|---|---|
| Cache Gemini não dispara (system <1024 tok) | Alta | Custo 2× | Padding via glossário PT-BR `PromptPadTokens=1100` (já implementado) |
| LLM emite JSON em markdown fences | Baixa | Parser falha | `response_format: json_schema strict` + validator com strip-fences fallback |
| Telegram update_id reset após 7d inativo | Baixa | Dedup quebra brevemente | TTL ≥24h cobre; após reset, dedup ainda funciona por igualdade |
| Refator agent quebra rota atual | Média (já mitigado) | Indisponibilidade | Toggle `AGENT_MODE=stub` permite rollback instantâneo |
| OpenRouter quota / credit exhaustion | Média | LLM caminho exausto | Circuit breaker + fallback chain (4 modelos diversificados) |
| LLM custo escala além do esperado | Média | Bill shock | Métricas de tokens consumidos via outbox events; alarme em $50/mês excedido |
| User envia mesma mensagem 2× legitimamente | Alta | 2 transações | Documentado como comportamento desejado (não dá pra distinguir replay de gasto idêntico) |

---

## 5. Sequência de execução recomendada

```
DIA 0     → Rodada 7 de auditoria (paralela ao smoke)
            Aplicar migrations 18-21 em staging
DIA 0-7   → Fase 1 smoke staging (TG=stub)
            Rodadas 8-N até 2 rodadas consecutivas com 0 FPs
DIA 7-9   → Fase 2 smoke staging (AGENT_MODE=openrouter canary 10%)
DIA 9-11  → Ramp para 100% se métricas OK
DIA 11+   → Production deploy
DIA 11-41 → Observação 30 dias do channel_processed_messages
DIA 41+   → Ativar migration 022 (drop legacy)
```

---

## 6. Critérios de "pronto pra produção" — definição de done

- [ ] Rodada 7 (e subsequentes) com 0 FPs em 2 execuções consecutivas
- [ ] Migrations 18-21 aplicadas em staging sem erro
- [ ] Fase 1 staging (TG=stub) por ≥7 dias sem regressão WhatsApp
- [ ] Fase 2 canary (AGENT_MODE=openrouter 10%) por ≥48h
- [ ] Métricas dentro dos envelopes do runbook
- [ ] Dashboards Grafana materializados
- [ ] Alertas Prometheus configurados e testados (fake-fire)
- [ ] On-call ciente do runbook + acesso ao OpenRouter dashboard
- [ ] `OPENROUTER_API_KEY` rotation procedure documentado

---

## 7. Skills declaradas para retomada

Quando retomar este trabalho, carregar antes de qualquer edição:

- `go-implementation` — para qualquer `.go` (R0–R7, R-ADAPTER-001)
- `agent-governance` — regras transversais
- `review` — antes de merge

Padrão canônico arquitetural a espelhar: **`internal/transactions`** (DMMF seletivo conforme ADR-006).
