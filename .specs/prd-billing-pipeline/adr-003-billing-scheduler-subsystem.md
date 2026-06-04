# ADR-003 — Scheduler dedicado em `infrastructure/scheduler` com `robfig/cron/v3`

## Metadados

- **Título:** Subsystem de scheduler do módulo billing
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-37, RF-49), `techspec.md` §Scheduler Subsystem, `internal/platform/outbox/cron.go`, `go.mod`

## Contexto

Billing precisa de dois jobs periódicos:
- Reconciliação horária (RF-37) — pull contra Kiwify para detectar webhook perdido.
- Anonimização diária de `webhook_events` antigos (RF-49) — D-08 do PRD.

`internal/platform/outbox/cron.go` já usa `robfig/cron/v3` mas é **dedicado a housekeeping do outbox** (purga `outbox_deliveries` antigas e reaper de stuck deliveries). Não é scheduler genérico.

Confronto com `go.mod`: `github.com/robfig/cron/v3 v3.0.1` já direta.

## Decisão

Criar `BillingScheduler` como `Subsystem` próprio em `internal/billing/infrastructure/scheduler/subsystem.go`, instanciando próprio `robfigcron.Cron` independente do outbox. Registra dois cron jobs:

```
@hourly  → ReconcileSubscriptionsUseCase.Execute(ctx)
@daily   → AnonymizeWebhookEventsUseCase.Execute(ctx)
```

Schedule expressions configuráveis via env (`KIWIFY_RECONCILIATION_INTERVAL`, `BILLING_ANONYMIZATION_SCHEDULE`). Lifecycle compatível com `runtime.Subsystem` (`Start(ctx) error`, `Stop(ctx) error`).

## Alternativas Consideradas

### Generalizar `outbox.Cron` para `platform.Cron` com `JobRegistrar`

- Vantagem: scheduler compartilhado.
- Desvantagem: refator do código estável do outbox; risco de regressão no housekeeping; aumenta escopo da techspec billing.
- Rejeitada por escopo e risco.

### `time.Ticker` em goroutine própria

- Vantagem: zero dependência nova.
- Desvantagem: sem expressão cron; converter `@hourly`/`@daily` manualmente; perde alinhamento com padrão outbox.
- Rejeitada por reduzir consistência operacional.

## Consequências

### Benefícios Esperados

- Lifecycle isolado de outbox housekeeping.
- Configuração via env por job (`KIWIFY_RECONCILIATION_INTERVAL` separado de `OUTBOX_HOUSEKEEPING_SCHEDULE`).
- Reusa lib já presente.
- Padrão repetível para futuros jobs de billing (e.g., relatórios MRR em E4).

### Trade-offs e Custos

- Mais um `Subsystem` no runtime — overhead < 1 goroutine + cron parser.
- Duas instâncias de `robfigcron.Cron` no processo (uma do outbox, outra do billing). Recurso negligenciável.

### Riscos e Mitigações

- **Risco:** schedule incorreto (`@every 1z`) crasha o boot. **Mitigação:** `cron.AddFunc` retorna erro de parse — Subsystem.Start retorna esse erro abortando boot com mensagem clara.
- **Risco:** job overlapping (reconciliation > 1h). **Mitigação:** semáforo interno (`sync.Mutex.TryLock`); se job ainda rodando ao tick, pula com log warn e métrica `billing_reconciliation_skipped_overlapping_total`.

## Plano de Implementação

1. Criar `internal/billing/infrastructure/scheduler/subsystem.go` com `BillingScheduler` struct e `Deps`.
2. `reconciliation_job.go` e `anonymization_job.go` encapsulam handler do tick com semáforo TryLock.
3. `runtime/billing_subsystem.go` registra o scheduler no `runtime.Application.Subsystems`.
4. Defaults em `configs/config.go`: `KIWIFY_RECONCILIATION_INTERVAL=@hourly`, `BILLING_ANONYMIZATION_SCHEDULE=@daily`.
5. Stop graceful: `cron.Stop()` retorna context que sinaliza quando jobs em andamento terminaram; respeitar `ctx.Done()` para timeout duro.

## Monitoramento e Validação

- Métricas: `billing_reconciliation_run_total{outcome}`, `billing_webhook_events_anonymized_total`, `billing_*_skipped_overlapping_total`.
- Logs: tick start/end com duration.

## Impacto em Documentação e Operação

- README do billing documenta como ajustar schedule via env.
- Runbook: forçar execução fora do schedule via comando admin (TBD em PRD futuro).

## Revisão Futura

- Se mais de 3 jobs periódicos em billing/onboarding/notifications, considerar `platform.Cron` genérico (substitui esta ADR e ADR equivalente do outbox).
