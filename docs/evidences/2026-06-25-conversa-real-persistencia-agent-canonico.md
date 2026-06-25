# Evidência — Conversa Real + Persistência nas Tabelas (`internal/agent`)

> Iniciativa: `.specs/prd-refatoracao-agent-canonico/`
> Data da captura: 2026-06-25
> Tipo: prova end-to-end com **LLM real (OpenRouter)** + **Postgres real (testcontainer)**.

## 1. O que esta evidência prova

Uma conversa real no WhatsApp, interpretada pelo **parse LLM real** (Structured Output `Strict=true`),
executada de forma **determinística** pelo kernel de workflow e **persistida de fato** atravessando
três bounded contexts: `transactions` (T), kernel `internal/platform/workflow` (K) e `outbox` (P).

Pipeline exercitado (igual ao runbook `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md` §2.1):

```
WhatsApp → IntentRouter.RouteWhatsApp → ParseInbound (LLM real, Strict=true)
        → kernel Engine[ExpenseState] (transactions_write)
        → 7 passos: authorize → replay → policy → audit_begin → resolve_category → persist → format
        → transactions.CreateTransaction + outbox transaction.created.v1
```

## 2. Como reproduzir

```bash
set -a; source <(grep -E '^OPENROUTER_(API_KEY|BASE_URL)=' .env); set +a
export RUN_REAL_LLM=1
go test -count=1 -timeout 200s -tags "e2e integration" \
  -run "TestJourney_RealConversation_PersistsAcrossTables_E2E" -v ./internal/agent/e2e/
```

Teste: `internal/agent/e2e/journey_persistence_e2e_test.go`
(parser de produção: gemini-2.5-flash-lite primário + mistral-small retry — o mesmo contrato anti-flaky de produção).

## 3. Conversa real (saída verbatim do teste)

```
==================== CONVERSA REAL (WhatsApp → agent → OpenRouter) ====================

👤 USUÁRIO: gastei 58 no ifood
🤖 AGENTE: 💸 *Transação realizada!*

👤 USUÁRIO: recebi meu salário de 4000
🤖 AGENTE: 💰 *Recebimento registrado!*

👤 USUÁRIO: meus lançamentos
🤖 AGENTE: 📋 *Lançamentos de 2026-06* (2)
```

O LLM classificou: `gastei 58 no ifood` → `record_expense`; `recebi meu salário de 4000` → `record_income`;
`meus lançamentos` → `list_transactions` (leitura, refletindo os 2 lançamentos já persistidos).

## 4. Evidência de persistência (linhas reais, schema `mecontrola`)

### (T) `transactions`
`[direction, payment_method, amount_cents, description, category_name_snapshot, ref_month, version]`
```
[2  1  5800    ifood    expense.prazeres  2026-06  1]   ← despesa R$ 58,00 (direction=2 outcome)
[1  2  400000  salário                    2026-06  1]   ← receita R$ 4.000,00 (direction=1 income)
(2 linha(s))
```

### (K) `workflow_runs` — kernel de escrita durável
`[workflow, status, suspend_reason, cursor]`
```
[transactions_write  succeeded  unknown  0]
[transactions_write  succeeded  unknown  0]
(2 linha(s))
```

### (K) `workflow_steps` — 7 passos por escrita (14 no total)
`[step_id, status, seq]`
```
authorize→replay→policy→audit_begin→resolve_category→persist→format   (todos completed, ×2)
(14 linha(s))
```

### (P) `outbox_events`
`[event_type, status]`
```
[transactions.transaction.created.v1  1]
[transactions.transaction.created.v1  1]
(2 linha(s))
```

> `agent_decisions` (audit trail) só grava sob o runtime completo `cmd/server`/`cmd/worker`
> (gateway de decisão), fora deste harness mínimo de teste — o passo `audit_begin` roda
> (visível em `workflow_steps`), mas a linha em `agent_decisions` depende do wiring completo.

## 5. Asserções automáticas do teste (verdes)

- `transactions` = 2 (1 despesa + 1 receita) — `require.Equal(t, 2, txCount)`
- `outbox_events` com `transactions.transaction.created.v1` = 2 — `require.Equal(t, 2, events)`
- Resultado: `--- PASS: TestJourney_RealConversation_PersistsAcrossTables_E2E`

## 6. Guards real-LLM da iniciativa (aceitação RF-18/19) — todos verdes

| Guard | Resultado |
|-------|-----------|
| `TestParseInbound_RealLLM_ProductionChain` (gemini + mistral) | PASS |
| `TestParseInbound_RealLLM_RecognitionMatrix` | **28/28 (100%), core 8/8** PASS |
| `TestStructuredOutputGuard_ParseClass_StrictTrue` | PASS |
| `TestStructuredOutputGuard_OnboardingClass_StrictTrue` | PASS |
| `TestOnboardingRealLLM_*` (objetivo/renda/cartão/splits/off-topic) | 5/5 PASS |
| `TestConfigureBudget_RealLLM_*` (Extract + MultiTurn) | PASS |
| `TestParseInbound_RealLLM_Confidence` | PASS |
| `TestParseInbound_RealLLM_NewKinds_*` | PASS |

Correções de produção que viabilizaram estes resultados (descobertas só com LLM real):
`AGENT_LLM_MAX_TOKENS` 256→768 (truncação do schema strict de 28 campos); nomes de kind no prompt
`log_*`→`record_*`; exemplos terse + número PT-BR no prompt; `query_income_summary` adicionado ao prompt;
schema de onboarding sem `minimum`/`maximum` (incompatível com Anthropic); `ConfigureBudget` migrado
para o parse estruturado (RF-10).
