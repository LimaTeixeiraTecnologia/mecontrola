# MVP Production-Readiness — Status Final

> Data: 2026-06-16
> Histórico: continuação do `2026-06-15-mvp-local-end-to-end.md` após audit de production-proof.

## Resumo executivo

Build + vet + race tests verdes no projeto inteiro. 10 dos 11 gaps identificados na auditoria foram fechados com gates verificáveis. 1 gap (#3 alerta de cartão) está bloqueado por decisão de produto (PRD não existe — discovery doc emitido para PO decidir entre 2 opções modeladas).

**Veredito**: pronto para o **primeiro deploy local com smoke E2E**. Pronto para Hostinger após exercício manual do smoke + decisão do PO sobre limite de cartão.

## Gaps fechados (com verificação)

| Gap | Entregável | Verificação |
|-----|-----------|-------------|
| #1 IntentRouter | `internal/agent/application/services/intent_router.go` + adapters | 13 testes table-driven |
| #2 LogExpense persiste | `internal/agent/application/usecases/log_expense_from_agent.go` + wiring | 7 testes suite + build/vet |
| #4 Duplo alerta | `BUDGETS_THRESHOLD_ALERTS_MODE=legacy/job/both` em `configs/config.go` + wiring condicional em `internal/budgets/module.go` + `cmd/worker/worker.go` | Build verde, default `legacy` preserva comportamento |
| #5 Integration tests | `threshold_alert_sent_repository_integration_test.go` + `onboarding_session_repository_integration_test.go` | Ambos `-tags=integration` passam em Postgres real via testcontainer |
| #6 Migrations smoke | `up→down→up` em testcontainer com `budget_alerts_sent` + `onboarding_sessions` | `task test:integration` |
| #7 Smoke E2E | `scripts/smoke/mvp_e2e.sh` + `task mvp:smoke` (Kiwify→Mailpit→state) | `bash -n` syntax OK; rodar com stack local |
| #8 OpenRouter schema | `JSONSchemaSpec` por request em `interfaces.LLMRequest` + `ParseIntentJSONSchema` no `ParseInbound` + nova métrica `agent_intent_parse_decode_failed_total{reason}` | Testes openrouter+parse_inbound verdes |
| #9 Security audit | Auditoria de 6 vetores. 0 críticos/altos. 2 MÉDIOS fechados inline (CORS no `/state`, rate limit no Kiwify webhook). 2 BAIXOS documentados (jitter cosmético, ordem escape Telegram). | Relatório em task_a819ffca0661982a2 |
| #2.5 Suite test LogExpense | `log_expense_from_agent_test.go` com 7 casos (invalid intent, no hint, ambiguous, not found, upsert fail, happy path, fallback merchant) | `go test ./internal/agent/application/usecases -run TestLogExpense` |
| #3.0 Discovery doc | `docs/discovery/2026-06-16-card-limit-cents.md` com Opção A + Opção B | PO escolhe → PRD → execução |

## Gates de governança (R0–R7 + R-ADAPTER-001)

```
$ go build ./...        → clean
$ go vet ./...          → clean
$ go test ./...         → clean
$ grep -rn "^func init()" --include="*.go" internal/ configs/   → 0
$ grep -rn "panic(" --include="*.go" --exclude="*_test.go" --exclude-dir=mocks internal/   → 0
$ grep -rn "interface{}" --include="*.go" --exclude="*_test.go" --exclude-dir=mocks internal/   → 0
$ Mockery regen   → clean (sem interface não encontrada nos novos pacotes)
```

## Achados de segurança (do audit #9)

### Fechados inline neste turno (MÉDIOS)

- **SEC-M-001 — CORS faltando em `/api/v1/onboarding/tokens/{token}/state`**: corrigido em `internal/onboarding/infrastructure/http/server/router.go` — middleware CORS aplicado também na rota GET, `Allow-Methods` agora cobre GET+POST+OPTIONS, header `Vary: Origin` adicionado.
- **SEC-M-002 — Rate limit ausente no webhook Kiwify**: criado `internal/billing/infrastructure/http/server/middleware/rate_limit.go` + 3 envs novas (`KIWIFY_WEBHOOK_RATE_LIMIT_PER_MIN=60`, `KIWIFY_WEBHOOK_RATE_LIMIT_BURST=30`, `KIWIFY_WEBHOOK_TRUSTED_PROXIES`) + wiring em `internal/billing/module.go` + `internal/billing/infrastructure/http/server/router.go`.

### Diferidos (BAIXOS)

- **SEC-L-001 — Jitter `math/rand/v2` em `TokenStateHandler`**: cosmético; ofusca pouco contra timing-attack remoto. Não-bloqueante. Próxima sprint.
- **SEC-L-002 — Ordem `escape → truncate` no Telegram gateway**: pode cortar `&amp;` no meio. Telegram tolera. Próxima sprint.

## Gaps pendentes (não fechados, com motivo explícito)

| Gap | Estado | Motivo | Bloqueador |
|-----|--------|--------|------------|
| #3 `card_limit_near` | BLOQUEADO | Schema `mecontrola.cards` não tem `limit_cents`. Inventar campo viola anti-alucinação. | PRD `prd-card-limit-cents` aprovado por PO |

## Próximos passos para "production-proof real"

1. **PO decide Opção A vs B** do `docs/discovery/2026-06-16-card-limit-cents.md` → PRD → exec.
2. **Rodar `task local:up` + `task mvp:smoke`** ao menos uma vez. Confirma o ciclo:
   - Kiwify webhook → outbox event → consumer dispara `SendActivationEmail`
   - Email aterrissa em Mailpit (`localhost:8025`)
   - Link `/activate?token=...` na landing chama `/tokens/.../state`
   - Resposta JSON traz `wa_me_url` e `telegram_deep_link` válidos
3. **Exercício manual** com `task ngrok:server` + 1 webhook real do Meta WhatsApp Business sandbox para confirmar HMAC + dispatcher inbound.
4. **Deploy Hostinger** após (1)+(2)+(3) verdes.

## Documentos relevantes

- Plano original: `~/.claude/plans/dado-essa-imagem-users-jailtonjunior-dow-misty-avalanche.md`
- Runbook E2E local: `docs/runbooks/2026-06-15-mvp-local-end-to-end.md`
- Discovery limit_cents: `docs/discovery/2026-06-16-card-limit-cents.md`
- Security audit: subagent task_a819ffca0661982a2 (output JSONL salvo na transcript)
