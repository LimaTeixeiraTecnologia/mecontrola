# Tarefa 10.0: Observability + dashboard Grafana + 4 alertas + runbook + governance rule

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fecha o ciclo production-ready: instrumenta métricas Prometheus (16 séries com labels controlados), spans OTel `transactions.<layer>.<operation>`, logs `slog` sem PII, dashboard Grafana `transactions-overview`, 4 alertas críticos, runbook com 3 cenários, e a regra hard `.claude/rules/transactions-workflows.md` que codifica o gate "regra de domínio fora de `Decide*` bloqueia PR" (ADR-006).

<requirements>
- Métricas Prometheus em `infrastructure/observability/metrics.go` (16 séries):
  - `transactions_transactions_{created,updated,deleted}_total{direction,payment_method}`
  - `transactions_card_purchases_{created,updated,deleted}_total{installments_bucket}` (buckets `1`, `2-6`, `7-12`, `13-24`)
  - `transactions_recurring_template_created_total{frequency,payment_method}`
  - `transactions_recurring_materialize_{attempt_total,skipped_total{reason},duration_seconds}`
  - `transactions_{write,read}_duration_seconds{operation}` (Histogram)
  - `transactions_monthly_summary_{recompute_duration_seconds,coalesce_factor,drift_total{kind}}`
  - `transactions_outbox_{consumer_lag_seconds,dead_letter_total}`
  - `transactions_idempotency_replay_total{operation}`
  - `transactions_card_lookup_failure_total`
- **Cardinalidade controlada** (RT-15): nenhum label carrega `user_id` ou `category_id`.
- Spans OTel `transactions.<layer>.<operation>` em todo use case, repo, consumer, producer; `trace_id` propagado em `outbox.Event.Metadata` (RT-14).
- Logs `log/slog` com chaves obrigatórias `module="transactions"`, `operation`, `user_id`, `aggregate_id`, `ref_month`, `trace_id`. **Proibido logar** `description`, `amount_cents`, `category_name_snapshot` (RT-13).
- Dashboard Grafana `transactions-overview` (JSON exportado em `docs/dashboards/transactions-overview.json` ou local equivalente do repo) com painéis: RED por endpoint, consumer lag, drift, idempotency replay, recurring (attempt vs skipped), card_lookup failures.
- 4 alertas em `docs/alerts/transactions.yaml` (ou local equivalente):
  1. **Drift**: `increase(transactions_monthly_summary_drift_total{kind="detected"}[1d]) > 0` por 15min → warning.
  2. **Write p99**: `histogram_quantile(0.99, sum by (le) (rate(transactions_write_duration_seconds_bucket[5m]))) > 0.3` por 10min → critical.
  3. **Consumer lag**: `transactions_outbox_consumer_lag_seconds > 5` por 5min → critical.
  4. **Dead-letter**: `increase(transactions_outbox_dead_letter_total[15m]) > 0` → critical.
- `docs/runbooks/transactions.md` cobrindo 3 cenários mínimos (AS-12):
  - Consumer travado em loop de retry (drenar manual + reset de cursor).
  - Drift detectado pelo job diário > limiar (investigar evento perdido).
  - Dead-letter > 0 (replay manual ou descarte com auditoria).
- `.claude/rules/transactions-workflows.md` (ADR-006) — regra hard codificando: (a) lista dos 5 workflows com `Decide*` obrigatório, (b) proibição de validação fora de smart constructors, (c) producers só mapeiam domain event → envelope, (d) gate de revisão "regra de domínio fora de `Decide*` bloqueia PR".
- README curto em `internal/transactions/domain/services/README.md`: 1 parágrafo explicando "por que `Decide*` é puro e onde efeitos vivem".
</requirements>

## Subtarefas

- [ ] 10.1 `infrastructure/observability/metrics.go` com 16 séries registradas via `observability.Observability`.
- [ ] 10.2 Spans OTel adicionados em cada use case, repo, consumer, producer (sem nova abstração — usar `o11y.Tracer().Start(ctx, "transactions.<layer>.<operation>")`).
- [ ] 10.3 Auditoria de logs: garantir que nenhum `slog.Info`/`Warn`/`Error` cite `description`, `amount_cents` ou `category_name_snapshot`. Adicionar teste de regressão se possível.
- [ ] 10.4 Dashboard `transactions-overview.json` com painéis listados.
- [ ] 10.5 Alertas em `docs/alerts/transactions.yaml`.
- [ ] 10.6 Runbook `docs/runbooks/transactions.md` (3 cenários).
- [ ] 10.7 `.claude/rules/transactions-workflows.md` (gate hard de governance).
- [ ] 10.8 `internal/transactions/domain/services/README.md` (1 parágrafo).

## Detalhes de Implementação

Referência: techspec "Monitoramento e Observabilidade" (lista completa de métricas + alertas + spans + logs); ADR-004 (`coalesce_factor`), ADR-006 (governance rule). RT-13, RT-14, RT-15, AS-12.

## Critérios de Sucesso

- 16 métricas registradas com labels controlados; teste de cardinalidade (script ou grep) verifica que nenhum label é `user_id`/`category_id`.
- Dashboard Grafana importa sem erro em ambiente de homologação.
- 4 alertas validam contra Prometheus rule loader sem erro de sintaxe (`promtool check rules`).
- Runbook revisado com link para cada métrica/alerta relevante.
- Regra `.claude/rules/transactions-workflows.md` referenciada em `.claude/rules/governance.md` (precedência).
- Build + test final: `go test -race -count=1 ./...` no escopo do módulo passa.
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     `otel-grafana-dashboards` é processual extra — gatilho: criação de dashboard Grafana para serviço
     OTel-instrumentado (gera JSON dashboard, painéis, alertas). -->

- `otel-grafana-dashboards` — gera dashboard `transactions-overview` + alertas Prometheus para o módulo OTel-instrumentado, conforme RT-13 e AS-12.

## Testes da Tarefa

- [ ] Unit test de instrumentação (verificar que `Inc()`/`Observe()` é chamado em pontos críticos via mock do `Counter`/`Histogram`).
- [ ] Validação de sintaxe do dashboard JSON e dos alerts YAML (`promtool check rules`).
- [ ] Smoke test do runbook (review manual + revisão de pares).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/infrastructure/observability/metrics.go` (novo)
- `docs/dashboards/transactions-overview.json` (novo)
- `docs/alerts/transactions.yaml` (novo)
- `docs/runbooks/transactions.md` (novo)
- `.claude/rules/transactions-workflows.md` (novo)
- `internal/transactions/domain/services/README.md` (novo)
- `.claude/rules/governance.md` (modificado — referenciar a nova regra)
