# Relatório de Execução — Tarefa 8.0: Observabilidade e job de abandono

<!-- Generated: 2026-06-26T15:16:44Z -->

## Resumo

Implementação da observabilidade do funil de onboarding (métricas de funil, Run auditável) e do job periódico de abandono, seguindo as restrições de cardinalidade (R-WF-KERNEL-001.4) e o padrão de módulo do `internal/agent`.

## RF Cobertos

- RF-29: Run auditável por execução (`thread_id`, `run_id`, `workflow=onboarding`, `step`, `status`, `duration_ms`, `error`).
- RF-30: Job de abandono emitindo `onboarding_step_abandoned_total{step}` com TTL configurável e métricas de funil por etapa.

## Subtarefas Atendidas

- [x] 8.1 Métricas de funil (`onboarding_step_total` existente; adicionados `onboarding_completed_total`, `onboarding_run_duration_seconds`, `onboarding_step_abandoned_total`).
- [x] 8.2 Run auditável com campos mínimos no `OnboardingAgent`.
- [x] 8.3 Job de abandono (`OnboardingAbandonmentJob`) com scan de `workflow_runs`, emissão de métrica e marcação idempotente via `AbandonedAt` no estado.

## Arquivos Alterados

### Produção

- `configs/config.go` — `OnboardingConfig` com `AbandonmentTTLHours`, `AbandonmentJobSchedule`, `AbandonmentBatchSize`; defaults; validação.
- `cmd/worker/worker.go` — registra `agentModule.OnboardingAbandonmentJob` no `WorkerManager`.
- `internal/agent/module.go` — expõe `OnboardingAbandonmentJob`; wiring via `attachOnboardingAbandonmentJob`.
- `internal/agent/application/services/onboarding_agent.go` — adiciona métricas `onboarding_completed_total`/`onboarding_run_duration_seconds` e log estruturado de Run auditável.
- `internal/agent/application/workflow/onboarding_state.go` — adiciona `AbandonedAt` para idempotência do job.
- `internal/agent/infrastructure/jobs/handlers/onboarding_abandonment_job.go` — job de abandono (scan, métrica, marcação idempotente).
- `internal/platform/workflow/store.go` — adiciona `ListSuspended` à interface `Store`.
- `internal/platform/workflow/infrastructure/postgres/store.go` — implementa `ListSuspended` com query paginada.
- `.env.example` — documenta as novas variáveis de abandono.

### Testes

- `configs/config_test.go` — ajusta `newBaseConfig`/`newProductionConfig` com valores válidos de abandono.
- `internal/agent/application/services/onboarding_agent_test.go` — testes de métricas de conclusão/duração, log de Run e detecção de replay.
- `internal/agent/infrastructure/jobs/handlers/onboarding_abandonment_job_test.go` — testes do job (seleção, métrica com label, idempotência, workflows estranhos, validação de config).
- `internal/platform/workflow/engine_test.go` — `FakeStore` implementa `ListSuspended`.
- `internal/platform/workflow/housekeeping_test.go` — `fakeHousekeepingStore` implementa `ListSuspended`.

## Comandos Executados

```bash
# build
$ go build ./...
ok

# testes direcionados
$ go test -race -count=1 ./internal/agent/infrastructure/jobs/handlers/...
ok      github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/jobs/handlers  1.409s

$ go test -race -count=1 ./internal/agent/application/services/...
ok      github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services  1.483s

$ go test -race -count=1 ./configs/...
ok      github.com/LimaTeixeiraTecnologia/mecontrola/configs  1.452s

$ go test -race -count=1 ./internal/platform/workflow/... ./internal/agent/application/workflow/...
ok      github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow  1.641s
ok      github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow  1.304s
ok      github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps  1.862s

# lint
$ golangci-lint run ./internal/agent/... ./configs/... ./cmd/worker/... ./internal/platform/workflow/...
0 issues.

# build via task
$ task build:build
ok

# testes unitários via task
$ task test:unit
ok (todos os pacotes passaram)

# segurança
$ task security:vulncheck
No vulnerabilities found.

# gates obrigatórios
$ grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/ configs/ cmd/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL comentários" || echo OK
OK

$ grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ && echo "FAIL import domínio no kernel" || echo OK
OK

$ grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|openai\|anthropic\|ParseInbound" \
  internal/platform/workflow/ | grep -v "infrastructure/postgres" && echo "FAIL kernel" || echo OK
OK

$ f=$(find internal/agent -name daily_ledger_agent.go ! -name "*_test.go"); \
  [ -n "$f" ] && c=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f"); \
  [ "${c:-0}" -gt 1 ] && echo "FAIL switch cresceu" || echo OK
OK

$ grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  && echo "FAIL SQL em tool/workflow" || echo OK
OK

# spec drift
$ ai-spec check-spec-drift .specs/prd-onboarding-conversacional
OK: sem drift detectado.
```

## Notas

- `task check` falha no gate `lint:deadcode` por código morto **pré-existente** em `internal/agent` (tool_catalog.go, onboardingv2draft, etc.), não introduzido por esta tarefa. O drift foi detectado antes da execução e está fora do escopo da 8.0.
- A integração com scan real de `workflow_runs` em PostgreSQL está prevista para a tarefa 9.0; o job possui adapter Postgres pronto e cobertura unitária com fake store.
- A métrica `onboarding_step_total{step,outcome}` já existia no `BuildOnboardingDefinition`; reforçamos a cardinalidade controlada (sem `user_id`/`correlation_key`/`category_id`).

## Riscos Residuais

- O job de abandono mantém o run no status `suspended` para permitir retomada, apenas marcando `AbandonedAt` no estado. Se o estado for corrompido entre `ListSuspended` e `Save`, a métrica pode ser emitida sem marcação; o tratamento de `ErrVersionConflict` evita duplicação na maioria dos casos.
- `task check` depende de limpeza de código morto legado para passar integralmente.

## Suposições

- A emissão de `onboarding.step_abandoned` no RF-30 é satisfeita pela métrica `onboarding_step_abandoned_total{step}`; não foi criado evento de domínio/outbox adicional, pois a techspec descreve o sinal como métrica de funil.
- O TTL padrão de 48h e schedule `@hourly` são valores iniciais; ajustáveis via `.env`/`config`.
