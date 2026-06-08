# Tarefa 10.0: Observabilidade + runbooks + Grafana dashboard + load test k6 + task auth:smoke

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Finaliza o épico entregando a camada operacional: métricas OTel completas, spans, logs estruturados, alertas, dashboard Grafana "Auth Module", runbooks de rotação e incident response, load test k6 como acceptance criteria (500 msg/min × 10 min, p99 < 300 ms), e `task auth:smoke` como gate de release automatizado.

<requirements>
- RF-15: runbook `docs/runbooks/auth-meta-secret-rotation.md` com janela CURRENT+NEXT.
- RF-16: runbook `docs/runbooks/auth-incident-response.md`.
- RF-17: instrumentação das 11 métricas OTel listadas na techspec; cardinalidade controlada (zero `user_id`/`wa_id` em labels).
- RF-18: spans OTel `auth.resolve_principal`, `auth.require_user`, `whatsapp.dispatcher.route` com atributos especificados.
- RF-19: `slog` estruturado com `trace_id`/`span_id`/`user_id` (quando Principal); teste de regressão inspeciona output do logger e falha se identificar `wa_id` cru, `Authorization` ou `META_APP_SECRET`.
- RF-20: dashboard Grafana "Auth Module" + alertas.
- RF-29: load test k6 sustentando 500 msg/min × 10 min com p99 < 300 ms e zero erro 5xx.
- RF-30: `task auth:smoke` automatizado bloqueando release.
</requirements>

## Subtarefas

- [ ] 10.1 Instrumentar métricas OTel em todos os componentes: `auth_principal_established_total{source}`, `auth_failed_total{reason}`, `auth_unknown_wa_id_total`, `auth_rate_limit_hits_total`, `auth_resolve_wa_duration_seconds`, `whatsapp_dispatcher_route_total{outcome}`, `whatsapp_ratelimit_buckets_count`, `whatsapp_ratelimit_cleanup_duration_seconds`, `auth_events_housekeeping_deleted_total`, `auth_events_housekeeping_duration_seconds` (reusar `meta_signature_status_total{status}` migrada em 6.0).
- [ ] 10.2 Instrumentar spans `auth.resolve_principal`, `auth.require_user`, `whatsapp.dispatcher.route`, `whatsapp.ratelimit.cleanup` com atributos especificados.
- [ ] 10.3 Garantir handler `slog` injeta `trace_id`/`span_id` (já vem do devkit) e que campos sensíveis nunca aparecem.
- [ ] 10.4 Criar `internal/identity/infrastructure/observability/logging_regression_test.go` ou similar — captura output do logger em buffer, executa fluxo completo de auth, faz assertion que buffer não contém `wa_id` cru, `Authorization`, `META_APP_SECRET`.
- [ ] 10.5 Criar `docs/runbooks/auth-meta-secret-rotation.md` com procedimento CURRENT+NEXT detalhado.
- [ ] 10.6 Criar `docs/runbooks/auth-incident-response.md` cobrindo 3 cenários de alerta principais.
- [ ] 10.7 Criar dashboard Grafana "Auth Module" (JSON em `docs/grafana/auth-module-dashboard.json`) com painéis para cada métrica + alertas (`auth_failed_total{reason='invalid_signature'} > 0/5min`, `auth_failed_total{reason='db_unavailable'} > 1/1min`, `whatsapp_ratelimit_buckets_count > 10000`, p99 `auth_resolve_wa_duration_seconds > 100ms` sustentado 3d, `outbox_publish_failed_total{kind=~"auth\\..*"} > 0`).
- [ ] 10.8 Criar `scripts/load-test/auth-webhook.k6.js` exatamente como na techspec; documentar como executar (`k6 run --env WEBHOOK_URL=… --env META_APP_SECRET=… auth-webhook.k6.js`).
- [ ] 10.9 Criar `scripts/smoke/auth_webhook/main.go` que envia payload válido com HMAC + consulta `auth_events` via SQL.
- [ ] 10.10 Adicionar recipe `auth:smoke` em `Taskfile.yml` (descrição + cmds conforme techspec).
- [ ] 10.11 Integrar `task auth:smoke` no CI: roda no merge para `main`; pipeline de deploy roda após staging deploy; falha aborta deploy em prod.
- [ ] 10.12 Executar load test k6 em staging; anexar relatório (`k6-report.html` ou stdout) ao PR final do épico; critério: p99 < 300 ms + erro < 0.1%.

## Detalhes de Implementação

Ver techspec `## Monitoramento e Observabilidade` para detalhes de cada métrica/span/log/alerta. Ver `assets/k6` na techspec para script base. Ver runbooks já delineados no PRD (Subtarefas 10.5-10.6).

## Critérios de Sucesso

- Todas as 11 métricas + 4 spans expostas em `/metrics` e visíveis no dashboard.
- Teste de regressão de logging passa.
- Dashboard Grafana importado com sucesso; alertas configurados.
- Load test reporta p99 < 300 ms e zero erros 5xx em 10 min sustentados de 500 msg/min.
- `task auth:smoke` retorna exit 0 quando ambiente sano; exit ≠ 0 quando webhook quebrado ou linha em `auth_events` ausente.
- CI fail simulado quando smoke quebra.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `taskfile-production` — recipe `auth:smoke` em `Taskfile.yml` e integração em CI seguem padrão go-task production-ready.
- `otel-grafana-dashboards` — dashboard "Auth Module" e alertas para serviço Go instrumentado com OpenTelemetry.

## Testes da Tarefa

- [ ] Teste de regressão de logging (RF-19)
- [ ] Smoke test executável (`task auth:smoke`)
- [ ] Load test k6 com critério verificável
- [ ] Validação manual do dashboard (importar em staging Grafana)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `docs/runbooks/auth-meta-secret-rotation.md` (criar)
- `docs/runbooks/auth-incident-response.md` (criar)
- `docs/grafana/auth-module-dashboard.json` (criar)
- `scripts/load-test/auth-webhook.k6.js` (criar)
- `scripts/smoke/auth_webhook/main.go` (criar)
- `Taskfile.yml` (atualizar — recipe `auth:smoke`)
- `.github/workflows/*.yml` ou pipeline equivalente (atualizar — invocação smoke)
- `internal/identity/infrastructure/observability/logging_regression_test.go` (criar)
- Diversos arquivos de 2.0–9.0 (atualizar — adicionar métricas/spans onde já existem stubs)
