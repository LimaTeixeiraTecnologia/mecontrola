# Tarefa 7.0: Cron (housekeeping + reaper), métricas OTel e traces

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar dois jobs periódicos via `robfig/cron/v3` e a fachada OTel completa do pacote. `housekeeping @daily` chama `Storage.PurgeOlderThan(retention)`; `reaper @every 1m` chama `Storage.ReleaseStuck(stuckAfter)`. A fachada `Metrics` instrumenta 10 medidas (counters/gauges/histograms) com label `subscription_name` e expõe `tracer` para os spans `outbox.publish` / `outbox.deliver` / `outbox.handle.<name>`.

<requirements>
- RF-18: housekeeping `@daily` apaga deliveries em `processed`/`dead_letter` com idade > `OUTBOX_HOUSEKEEPING_RETENTION_DAYS` e propaga delete para eventos órfãos.
- RF-19: reaper `@every 1m` libera deliveries em `claimed` há > `OUTBOX_REAPER_STUCK_AFTER`.
- RF-20: métricas das execuções de housekeeping e reaper (contadores de linhas afetadas) consumíveis em dashboard.
- RF-21: 10 métricas OTel com labels conforme techspec.
- RF-22: propagação de tracing via `headers.traceparent`.
</requirements>

## Subtarefas

- [ ] 7.1 Criar `cron.go` com struct `Cron` que envolve `*cron.Cron` (v3) e construtor `NewCron(deps CronDeps)` aceitando `Storage`, `Metrics`, `Logger`, `Clock`, `HousekeepingSchedule string`, `ReaperInterval string`, `RetentionDays int`, `ReaperStuckAfter time.Duration`.
- [ ] 7.2 Implementar `Start(ctx)` registrando 2 entries via `c.AddFunc(schedule, fn)` e chamando `c.Start()`. Falhar com erro claro se `schedule` inválido (parse-check já fez em 1.0 mas validar de novo defensive).
- [ ] 7.3 Implementar `Stop(ctx)` com `select { case <-c.Stop().Done(): case <-ctx.Done(): }` para respeitar o deadline.
- [ ] 7.4 Implementar `runHousekeeping(ctx)`: chama `Storage.PurgeOlderThan(ctx, now - retention)`, registra `outbox.housekeeping.deleted.total += N` e log estruturado `INFO outbox.housekeeping.purged`.
- [ ] 7.5 Implementar `runReaper(ctx)`: chama `Storage.ReleaseStuck(ctx, now - stuckAfter)`, registra `outbox.reaper.released.total += N` e log `WARN outbox.reaper.released` (se N > 0).
- [ ] 7.6 Criar `metrics.go` com struct `Metrics` envolvendo `observability.Observability` (Provider já existente) e os 10 instrumentos OTel da tabela techspec.
- [ ] 7.7 Implementar `Metrics.RecordPublished(eventType)`, `RecordProcessed(subscriptionName, latencyMs)`, `RecordFailed(subscriptionName, errorClass)`, `RecordDLQ(subscriptionName)`, `RecordPoll(durationMs, batchSize)`, `RecordReaperReleased(n)`, `RecordHousekeepingDeleted(n)`, `SetPending(subscriptionName, count)` — todos como métodos da struct.
- [ ] 7.8 Implementar bucketização de `error_class` em `RecordFailed`: `transient | timeout | permanent | panic | unknown` (controle de cardinalidade R-OBS-001).
- [ ] 7.9 Expor `Metrics.Tracer()` para uso pelo Publisher (5.0) e Dispatcher (6.0). Documentar que `outbox.publish`/`outbox.deliver`/`outbox.handle.<name>` são criados pelos consumidores, não pelo `Metrics`.
- [ ] 7.10 Criar `cron_test.go` (unitário) cobrindo dispatch de jobs com `cron.Schedule` fake.
- [ ] 7.11 Criar `metrics_test.go` cobrindo bucketização de `error_class` e que `payload` literalmente nunca aparece em chamadas registradas (verificar via inspeção dos atributos).

## Detalhes de Implementação

Ver techspec.md seções **Arquitetura do Sistema → Componentes → `outbox.Cron`/`outbox.Metrics`**, **Monitoramento e Observabilidade → Métricas OTel (RF-21)** (tabela completa) e **→ Traces (RF-22)**. Considerar **Riscos Conhecidos → `cron.Stop(ctx)`** (usar `select` com `ctx.Done()`).

## Critérios de Sucesso

- `go test ./internal/infrastructure/outbox/...` verde para `cron_test.go` e `metrics_test.go`.
- Cenário 1: `Cron.Start` registra exatamente 2 entries (housekeeping + reaper).
- Cenário 2: invocar `runHousekeeping` chama `Storage.PurgeOlderThan` com `now - retention` e incrementa o contador OTel pelo valor retornado.
- Cenário 3: invocar `runReaper` chama `Storage.ReleaseStuck` com `now - stuckAfter` e loga apenas se N > 0.
- Cenário 4: `Cron.Stop(ctx)` retorna em ≤ deadline mesmo se nenhum job estiver in-flight.
- Cenário 5: `Metrics.RecordFailed(..., errors.New("connection reset"))` mapeia para bucket `"transient"`; `RecordFailed(..., ErrPermanent)` → `"permanent"`; `RecordFailed(..., context.DeadlineExceeded)` → `"timeout"`.
- Cenário 6: nenhum dos métodos de `Metrics` aceita `payload` como parâmetro (assinatura inspecionada via reflection ou ausência de tipo `json.RawMessage`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `cron_test.go` e `metrics_test.go` com os 6 cenários listados; usar `mocks.Storage` para verificar argumentos passados.
- [ ] Testes de integração: cobertura ponta-a-ponta do reaper em Postgres real fica na task 8.0 (`subsystem_integration_test.go` → cenário `reaper`).

**Definition of Done**:
- [ ] Schedules `@daily` e `@every 1m` parsedos por `cron.ParseStandard` sem erro no startup.
- [ ] `Cron.Stop(ctx)` respeita `ctx.Done()` (assert em teste com deadline curto + job longo).
- [ ] 10 métricas OTel da techspec todas instrumentadas e nomeadas exatamente como `outbox.events.published.total`, `outbox.deliveries.pending`, etc.
- [ ] `error_class` bucketizado em 5 valores fixos — assert por table-driven.
- [ ] Nenhum import de `pgx` em `cron.go`/`metrics.go` (esses arquivos podem importar `otel` por exceção documentada em techspec; SQL fica em `storage_pgx.go`).
- [ ] `gofmt -w .` + `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/cron.go` (novo)
- `internal/infrastructure/outbox/cron_test.go` (novo)
- `internal/infrastructure/outbox/metrics.go` (novo)
- `internal/infrastructure/outbox/metrics_test.go` (novo)
- `internal/infrastructure/outbox/storage.go` (consumido — criado em 3.0)
- `internal/infrastructure/observability/` (dependência — `Observability`/`Provider`)
