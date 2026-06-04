# Tarefa 10.0: Integration tests + depguard + drift cleanup + cobertura final

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Validar via integration tests com `testcontainers-go/postgres` que o pipeline completo funciona (CA-01 a CA-12). Adicionar regra `billing-no-identity-infrastructure` em `.golangci.yml`. Reescrever `internal/billing/{AGENTS.md, README.md, doc.go}` removendo qualquer drift e documentando contratos finais. Verificar PII masking em logs (RF-42), spans OTel emitidos (RF-43), cobertura consolidada. Garantir gates do PRD verdes.

<requirements>
- Integration tests com testcontainers-go cobrindo CA-02 (idempotência 5x replay), CA-03 (eventos fora de ordem), CA-06 (reconciliation detecta divergência), CA-07 (smoke E2E completo), CA-11 (1000 webhooks/1h zero DLQ), CA-12 (anonimização após 366d)
- `.golangci.yml` adiciona `billing-no-identity-infrastructure` (billing pode importar identity/domain e identity/application, nunca identity/infrastructure)
- Reforço de `domain-no-infrastructure` cobrindo `internal/billing/domain/`
- `internal/billing/AGENTS.md` documenta: política PII redactor paths, política grace period 7d, política trust provider para period_end, padrão de locking, máquina de estados (RF-45)
- `internal/billing/README.md` documenta: como adicionar nova rota via `chiserver.Router`, como ajustar schedule, runbook básico
- `internal/billing/doc.go` (`domain/`, `application/`, `infrastructure/`) descrevem responsabilidades
- Grep por `whatsapp_number` ou `email` em claro nos logs capturados em integration retorna zero (CA-08, RF-42)
- Verificação de spans emitidos via OTel exporter em integration (CA via assertion no tracetest exporter)
- `ai-spec check-spec-drift .specs/prd-billing-pipeline/tasks.md` retorna verde (CA-10)
- Cobertura final consolidada: `go test -cover ./internal/billing/...` reporta ≥ 90% (target CA-04)
- Sem comentários supérfluos no código produzido (R-AGENTS-001 + governança)
</requirements>

## Subtarefas

- [ ] 10.1 `internal/billing/infrastructure/repositories/postgres/subscription_repository_integration_test.go` (build tag `//go:build integration`): SetupSuite sobe postgres:16-alpine + aplica migrations 0001..0010; suite cobre `Upsert` (insert + update + UNIQUE violation), `FindActiveByUserID` (com soft delete invisível), `FindActiveByUserIDForUpdate` (lock verificável via segunda goroutine bloqueando), `ListByStatusInBatch` (cursor estável, paginação).
- [ ] 10.2 `webhook_event_repository_integration_test.go`: cobre `InsertIfNew` com dedup, `RecordApplication` idempotente, `ListPendingAnonymization` com `received_at` antigo, `Anonymize` substitui PII e preenche `anonymized_at` (CA-12).
- [ ] 10.3 `internal/billing/application/usecases/ingest_kiwify_webhook_integration_test.go`: envia payload novo → row em `webhook_events` + 1 row em `outbox_events`; replay mesmo payload → 0 novas rows.
- [ ] 10.4 `process_billing_event_integration_test.go`: registra handler no `outbox.Registry`, popula `webhook_events` com 5 cópias do mesmo external_event_id, dispara Dispatcher manualmente, valida 1 row em `billing_event_applications` e estado final correto (CA-02).
- [ ] 10.5 Test "fora de ordem" (CA-03): publica `subscription_renewed` (period_end=2026-07-01) então `compra_aprovada` (period_end=2026-06-01); valida que estado final reflete o evento de maior `occurred_at`.
- [ ] 10.6 Test "smoke E2E" (CA-07): compra_aprovada → ACTIVE → CheckEntitlement granted → subscription_canceled → CANCELED_PENDING → CheckEntitlement granted até period_end → após period_end, CheckEntitlement denied.
- [ ] 10.7 Test reconciliation (CA-06): mock `BillingProvider.FetchSubscription` retorna estado divergente para 1 sub; executa job; valida que evento sintético foi publicado em outbox e processor o aplicou, convergindo o estado.
- [ ] 10.8 Test stress (CA-11): publica 1000 webhooks em 60s; afirmação `count(billing_event_applications) >= 999` e `count(outbox_deliveries WHERE delivery_status='dlq') == 0`.
- [ ] 10.9 Test PII masking (CA-08): captura output do logger em buffer durante happy path; grep regex `"whatsapp_number":"[+0-9]{4,}"` retorna zero.
- [ ] 10.10 Test spans OTel (RF-43): registra exporter de teste, executa happy path, valida que cada nome de span esperado aparece pelo menos uma vez.
- [ ] 10.11 Estender `.golangci.yml` com regra `billing-no-identity-infrastructure` (negar imports de `internal/identity/infrastructure` em `internal/billing/...`) e reforçar `domain-no-infrastructure` cobrindo `internal/billing/domain/`.
- [ ] 10.12 Criar `internal/billing/AGENTS.md`, `internal/billing/README.md`, `internal/billing/doc.go` + docs em `domain/`, `application/`, `infrastructure/`. Sem comentários supérfluos.
- [ ] 10.13 Rodar `ai-spec sync-spec-hash .specs/prd-billing-pipeline/tasks.md` e `ai-spec check-spec-drift .specs/prd-billing-pipeline` — corrigir até verde.
- [ ] 10.14 Rodar cobertura agregada `go test -coverpkg=./internal/billing/... -coverprofile=coverage.out ./...` e reportar resultado. Investigar gaps < 90% e adicionar testes pontuais.

## Detalhes de Implementação

Ver techspec §Abordagem de Testes, §Plano de Rollout, e CA-01..12 do PRD. Estrutura segue `internal/platform/outbox/storage_pgx_integration_test.go` (SetupSuite/SetupTest) e `internal/identity/.../*_integration_test.go` (quando entregue por E1).

## Critérios de Sucesso

- `go test -tags=integration ./internal/billing/... -race -count=1` verde em < 10min.
- `golangci-lint run ./...` verde com novas regras `billing-no-identity-infrastructure` e `domain-no-infrastructure` cobrindo `internal/billing/domain/`.
- `ai-spec check-spec-drift .specs/prd-billing-pipeline` retorna sem drift.
- `go test -coverpkg=./internal/billing/... -coverprofile=coverage.out ./...` reporta ≥ 90% global (com 100% em VOs, state machine, entities).
- Grep em `internal/billing/**/*.go` por `// ` (comentários) retorna apenas: godocs concisos (1-2 linhas no topo de tipos/funções), notas de invariante não-óbvia, referência a ADR. Sem `// TODO`, `// FIXME`, comentários redundantes que repetem nome.
- AGENTS.md/README.md/doc.go contêm: política de redactor (lista de paths), grace period 7d, trust provider, pessimist lock pattern, máquina de estados (diagrama Mermaid).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Subset principal já listado em Subtarefas 10.1–10.10 (suites integration por arquivo).
- [ ] Teste de regressão CA-02 (idempotência 5x): obrigatório.
- [ ] Teste CA-12 (anonimização aos 366d): obrigatório.
- [ ] Teste de drift: `ai-spec check-spec-drift .specs/prd-billing-pipeline` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/repositories/postgres/subscription_repository_integration_test.go` (novo)
- `internal/billing/infrastructure/repositories/postgres/webhook_event_repository_integration_test.go` (novo)
- `internal/billing/application/usecases/ingest_kiwify_webhook_integration_test.go` (novo)
- `internal/billing/application/usecases/process_billing_event_integration_test.go` (novo)
- `internal/billing/application/usecases/reconcile_subscriptions_integration_test.go` (novo)
- `internal/billing/application/usecases/anonymize_webhook_events_integration_test.go` (novo)
- `internal/billing/application/usecases/check_entitlement_integration_test.go` (novo)
- `internal/billing/observability_integration_test.go` (novo — PII masking + spans)
- `.golangci.yml` (alterado — depguard rules)
- `internal/billing/AGENTS.md` (novo)
- `internal/billing/README.md` (novo)
- `internal/billing/doc.go` (novo)
- `internal/billing/domain/doc.go` (novo)
- `internal/billing/application/doc.go` (alterado/novo)
- `internal/billing/infrastructure/doc.go` (novo)
- Depende de: todas as tasks anteriores (1.0..9.0)
- Validação final: `ai-spec check-spec-drift .specs/prd-billing-pipeline`
