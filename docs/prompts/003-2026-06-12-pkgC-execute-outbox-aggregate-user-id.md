# Prompt 003 — 2026-06-12 — Pacote C — Executar PRD Outbox Aggregate User ID Top-Level

**PRD:** `.specs/prd-outbox-aggregate-user-id/`
**Skill orquestradora:** `execute-all-tasks`
**Total de tarefas:** 8 (cobertura 20/20 RFs)
**Paralelismo seguro:** 1.0 ‖ 2.0; 3.0 ‖ 4.0 ‖ 5.0 ‖ 6.0
**Severidade:** padronizacao arquitetural pos go-live (pre-requisito de RLS Postgres)
**Esforco estimado:** ~4 dias uteis com paralelismo

---

## Invocacao

```
/execute-all-tasks .specs/prd-outbox-aggregate-user-id/
```

---

## Mandatorios inegociaveis (aplicar a TODAS as 8 tarefas)

1. **Skill obrigatoria**: carregar `.claude/skills/go-implementation/SKILL.md` (Etapas 1-5, R0-R7, R-ADAPTER-001) em toda subtarefa que toque codigo Go.
2. **ZERO COMENTARIOS em codigo Go de producao** (R-ADAPTER-001.1). Excecoes apenas: `//go:build`, `//nolint:` com justificativa, `// Code generated`. Cabecalho de politica em `internal/platform/outbox/system_event_allowlist.go` e excecao explicita ja documentada em ADR-004.
3. **Domain Modeling Made Functional** (Wlaschin) onde fizer sentido:
   - `AggregateUserID string` como campo top-level no envelope `outbox.Event` (tipo string mantem consistencia com `AggregateID string` existente; uuid.UUID seria mais tipado mas quebra consistencia interna).
   - `isSystemEvent(eventType string) bool` em allowlist explicita (sem heuristica por nome) — exemplo de "make illegal states unrepresentable" adaptado.
   - Validacao opcional na v1 (warning + metrica); obrigatoria em v2 (ADR-001).
4. **Padronizar com `internal/transactions`** (baseline canonico):
   - Producers em `infrastructure/messaging/database/producers/` apenas serializam (zero calculo de dominio).
   - Eventos tipados em `domain/entities/events.go` carregam `UserID uuid.UUID` — producers leem `evt.UserID.String()` sem reparseio de payload.
   - Adapter fino R-ADAPTER-001.2 nos producers.
5. **Regra de memoria — sem abstracao de tempo**: `OccurredAt` ja passado por argumento; manter padrao.
6. **Foco**: MVP robusto, eficiente, economico, production-ready/proof, sem falso positivo, inegociavel.

## Decisoes pre-cravadas em 4 ADRs (NAO reabrir)

- ADR-001: coluna `aggregate_user_id UUID NULL` na v1; validacao opcional via warning + metrica. v2 (NOT NULL + erro em `NewEvent`) e PRD futuro apos 30 dias de cobertura ≥ 99.99%.
- ADR-002: rollout atomico single-deploy. Sem dual-write. Sem feature flag. Migration + codigo + 12 callers no mesmo PR (ou PRs adjacentes mergeados na mesma janela).
- ADR-003: registros antigos permanecem NULL. Sem backfill SQL/aplicacao. Housekeeping (`DeletePublishedBatch`) limpa naturalmente.
- ADR-004: allowlist explicita em `system_event_allowlist.go` (vazia no MVP). Adicionar entrada requer ADR de superseder ou PR com revisao humana.

## Sequenciamento obrigatorio

- 1.0 (migration + storage) e 2.0 (struct + validacao + envelope) podem rodar em paralelo (gates independentes).
- 3.0, 4.0, 5.0, 6.0 (atualizacao dos 12 callers em 5 modulos) dependem de 1.0 + 2.0, mas sao paralelos entre si (modulos isolados).
- 7.0 (gate `lint:outbox-user-id`) depende de 3.0+4.0+5.0+6.0 (caso contrario gera falso positivo nos PRs intermediarios).
- 8.0 (smoke + dashboard) depende de 7.0.

## Gates obrigatorios pos cada tarefa

- `task lint && task test && task vulncheck` PASS.
- `task lint:user-isolation` PASS (defesa em profundidade per-user nao regrediu).
- `task mocks` regenera sem erro apos 2.0 (mocks de `outbox.Publisher` atualizados).
- `grep R-ADAPTER-001.1` vazio nos `.go` produção.
- Sem nova dep em `go.mod`.

## Validacao final do pacote

- `task lint:outbox-user-id` PASS (gate inegociavel — todos os 12 callers populam `AggregateUserID`, exceto allowlist).
- Simulacao adversarial do gate: revert temporario de 1 producer + FAIL + restore + PASS documentado.
- Smoke staging: disparar 1 evento de cada modulo (transaction, expense, subscription, principal); validar `SELECT count(*) FROM outbox_events WHERE aggregate_user_id IS NOT NULL` aumenta.
- Metrica `outbox_events_inserted_total{has_user_id="true"}` ≥ 99% em staging por 24h.
- Painel Grafana "Outbox Adoption" com 2 paineis (% adoption + missing rate).
- Alerta `rate(... {has_user_id="false"}[5m]) / rate(... [5m]) > 0.01` por 10 min configurado.
- Runbook `docs/runbooks/outbox-aggregate-user-id.md` revisado sem TODOs.

## Tarefas-fonte

1. `task-1.0-migration-storage.md`
2. `task-2.0-struct-validacao-envelope.md`
3. `task-3.0-producers-transactions.md` (transactions: 3 publishers)
4. `task-4.0-producers-budgets-billing.md` (budgets + billing)
5. `task-5.0-identity-sites.md` (identity: 3 sites)
6. `task-6.0-onboarding-whatsapp-sites.md` (onboarding + whatsapp dispatcher)
7. `task-7.0-gate-lint-outbox-user-id.md`
8. `task-8.0-smoke-metrica-dashboard.md`
