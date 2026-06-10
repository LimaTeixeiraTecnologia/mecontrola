# Tarefa 10.0: Module wiring + cmd integration + OpenAPI + observabilidade + smoke load

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o módulo: `module.go` com DI manual explícita (padrão `BillingModule`), wiring em `cmd/server/server.go` e `cmd/worker/worker.go`, `openapi.yaml`, instrumentação completa (Prometheus + slog + OTel traces), dashboards Grafana e smoke load test confirmando os SLOs M-05 (p95 ≤ 300ms) e M-06 (99,9%). Esta é a única tarefa que altera os entrypoints.

<requirements>
- `internal/budgets/module.go` segue padrão `BillingModule`: construtor `NewBudgetsModule(cfg, o11y, dbManager) (*BudgetsModule, error)`; struct exporta `BudgetsRouter`, `EventHandlers`, `AbandonedDraftReaper`, `PendingEventsReaper`, `RetentionPurge`, `ExpenseCommittedConsumer`, `ExternalExpenseConsumer`.
- DI manual: instancia repositórios, UoWs (`uow.New[T](mgr, uow.WithObservability(o11y))`), use cases, handlers, router, jobs, consumers, publisher.
- Boot resolve as 5 raízes oficiais; falha retorna erro do `NewBudgetsModule` e impede startup (RT-31).
- `cmd/server/server.go`: instancia `BudgetsModule`, registra `BudgetsRouter` quando não-nulo (padrão dos demais módulos).
- `cmd/worker/worker.go`: registra os 3 jobs (`AbandonedDraftReaper`, `PendingEventsReaper`, `RetentionPurge`) no `worker.Manager`; registra os 2 consumers (`ExpenseCommittedConsumer`, `ExternalExpenseConsumer`) no `events.Dispatcher`.
- `configs/`: adicionar `BudgetsConfig` consolidado com `PendingReaperInterval`, `PendingTTLHours`, `AbandonedDraftCron`, `RetentionPurgeCron`, `CategoriesCacheTTL`.
- `internal/budgets/openapi.yaml` cobrindo os 9 endpoints com schemas compartilhados (errors envelope igual a `internal/categories/openapi.yaml`).
- Métricas Prometheus listadas na seção **Monitoramento e Observabilidade** da techspec — cardinalidade RT-25 (proibido `user_id`/`external_transaction_id`/`subcategory_id` em labels).
- Dashboards Grafana: replicar layout do dashboard de billing substituindo séries.
- Smoke load test (`task test:load` ou script `k6`/`vegeta`): 100 RPS por 60s em `POST /api/v1/budgets/expenses` confirmando p95 ≤ 300ms.
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 10.1 `internal/budgets/module.go` com DI manual (≈80–120 linhas, espelhando `BillingModule`).
- [ ] 10.2 Edits em `cmd/server/server.go` (registrar router) e `cmd/worker/worker.go` (registrar jobs + consumers).
- [ ] 10.3 `configs/` atualizado com `BudgetsConfig`.
- [ ] 10.4 `internal/budgets/openapi.yaml`.
- [ ] 10.5 Instrumentação Prometheus + slog estruturado + tracing OTel em use cases, handlers, consumers e jobs.
- [ ] 10.6 Dashboards Grafana JSON exportados para `configs/grafana/budgets/` (ou pasta equivalente já em uso).
- [ ] 10.7 Smoke load test script + relatório p95 ≤ 300ms.
- [ ] 10.8 Atualizar `AGENTS.md` (seção Módulos ativos) listando `internal/budgets`.

## Detalhes de Implementação

Ver **Visão Geral dos Componentes**, **Monitoramento e Observabilidade**, **Arquivos Relevantes e Dependentes** na `techspec.md`.

Endpoint admin HTTP **NÃO** está no MVP (OUT-16). RF-39c (métricas) e RF-69 (logs sem PII financeira) são satisfeitos por esta tarefa.

`AGENTS.md` lista `internal/identity`, `internal/billing`, `internal/platform` como módulos ativos. Atualizar para incluir `internal/budgets` e `internal/categories`.

## Critérios de Sucesso

- `task run:server` sobe com `internal/budgets` registrado; log estruturado confirma "budgets module wired".
- `task run:worker` registra os 3 jobs e 2 consumers; log confirma registro.
- Smoke load: p95 medido ≤ 300ms para escrita de despesa em perfil RT-08.
- `golangci-lint run ./...` limpo.
- `task vulncheck` sem novos avisos atribuíveis ao módulo.
- Boot com 1 raiz oficial removida do seed editorial **falha** com erro claro (proteção RT-31).
- Dashboards Grafana mostram séries para todas as métricas listadas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `otel-grafana-dashboards` — gerar/atualizar dashboards Grafana com séries OTel para o módulo budgets, incluindo lag de avaliação de alerta, idade do pendente mais antigo e p95 do summary.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/budgets/module.go` (novo)
- `internal/budgets/openapi.yaml` (novo)
- `cmd/server/server.go` (alterar)
- `cmd/worker/worker.go` (alterar)
- `configs/` (alterar para incluir `BudgetsConfig`)
- `configs/grafana/budgets/` (novo, JSON dos dashboards)
- `AGENTS.md` (alterar seção Módulos ativos)
- `Taskfile.yml` (alterar se preciso adicionar `task test:load:budgets`)
- Referência: `internal/billing/module.go`, `cmd/worker/worker.go` (existente)
