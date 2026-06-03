# Tarefa 9.0: Logs slog sem payload com allowlist e benchmarks de publish/dispatcher

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cobrir o lado operacional do pacote: logs estruturados `slog` em todas as transições relevantes (startup, processed, failed, dlq, reaper, housekeeping) com allowlist explícita de campos, garantindo que `payload` nunca aparece em chamada `slog.*Context`; e benchmarks formais que medem `Publisher.Publish` p95 e throughput sustentado do `Dispatcher` drenando massa pré-populada — documentados em `docs/benchmarks/outbox-baseline.md` para confronto pós-deploy (RF-36).

<requirements>
- RF-23: logs estruturados via `slog` em transições relevantes contendo `event_id`, `event_type`, `subscription_name`, `attempt`, `correlation_id`.
- RF-24: payload bruto do evento NUNCA aparece em logs.
- RF-31: campos de auditoria/log limitados a allowlist canônico (`event_id`, `event_type`, `subscription_name`, `attempt`, `correlation_id`, `error_class`).
- RF-36: benchmark medindo p95 do publish com 1, 10, 100 e 1000 eventos/s + throughput sustentado do Dispatcher drenando massa pré-populada.
</requirements>

## Subtarefas

- [ ] 9.1 Criar tipo privado `logFields` em `internal/infrastructure/outbox/log_fields.go` modelando os 6 campos do allowlist (`EventID`, `EventType`, `SubscriptionName`, `Attempt`, `CorrelationID`, `ErrorClass`) com método `slog.Attrs() []slog.Attr` retornando apenas atributos não-vazios.
- [ ] 9.2 Refatorar Publisher (5.0), Dispatcher (6.0), Cron (7.0) e Subsystem (8.0) para emitir logs via `slog.LogAttrs(ctx, level, msg, fields.Attrs()...)` em 6 transições:
  - `INFO outbox.subsystem.started` (boot) — `tick_interval`, `batch_size`, `instance_id` (campos de boot, fora do allowlist por design — documentar).
  - `INFO outbox.delivery.processed` (sampled 1:100) — allowlist completo + `latency_ms`.
  - `WARN outbox.delivery.failed` — allowlist + `next_retry_at`.
  - `ERROR outbox.delivery.dlq` — allowlist + `total_attempts`.
  - `WARN outbox.reaper.released` — `count`, `older_than`.
  - `INFO outbox.housekeeping.purged` — `count`, `retention_days`.
- [ ] 9.3 Implementar sampler 1:100 em `outbox.delivery.processed` via contador atômico no Dispatcher (`atomic.AddUint64`); `failed`/`dlq` sempre logados sem sampler.
- [ ] 9.4 Adicionar validação estática (test) em `log_payload_safety_test.go`: percorre o pacote `outbox` via `go/ast` ou regex sobre arquivos e assegura que nenhuma chamada `slog.*` recebe `payload`, `Payload`, `json.RawMessage` ou `evt.Payload` como argumento.
- [ ] 9.5 Criar `benchmark_test.go` no pacote outbox com:
  - `BenchmarkPublisher_Publish_1Handler`, `_3Handlers`, `_5Handlers` medindo p95 do `Publish` em transação aberta com mock-fast de Storage.
  - `BenchmarkPublisher_Publish_Postgres` (build tag `integration`) medindo p95 em transação real do testcontainer.
  - `BenchmarkDispatcher_DrainBacklog` (build tag `integration`): pré-popula 10k deliveries com handler dummy de 10ms, mede tempo total de drain e throughput sustentado.
- [ ] 9.6 Criar `docs/benchmarks/outbox-baseline.md` com tabela de resultados executados na máquina de referência (deixar template + comando `task bench:outbox` que produz o arquivo).
- [ ] 9.7 Adicionar receita `task bench:outbox` no Taskfile.yml rodando `go test -tags=integration -bench=BenchmarkPublisher -benchmem -count=5 -run=^$ ./internal/infrastructure/outbox/...` com captura em `docs/benchmarks/outbox-baseline.md`.

## Detalhes de Implementação

Ver techspec.md seções **Monitoramento e Observabilidade → Logs `slog` (RF-23 + RF-24)** (lista de 6 transições + política de campos), **Considerações Técnicas → Riscos Conhecidos → `+1 INSERT` por handler** e **Abordagem de Testes → Benchmarks (RF-36)**.

## Critérios de Sucesso

- `go test ./internal/infrastructure/outbox/...` verde incluindo `log_payload_safety_test.go`.
- `go test -tags=integration -bench=BenchmarkPublisher -run=^$ ./internal/infrastructure/outbox/...` produz números reproduzíveis (variância < 20% entre runs).
- Cenário 1: chamada de `Dispatcher.markResult(processed)` produz exatamente 1 log `INFO outbox.delivery.processed` com `event_id`/`event_type`/`subscription_name`/`attempt`/`latency_ms` (e a cada 100 chamadas, capturado por contador atômico).
- Cenário 2: chamada com erro transitório produz `WARN outbox.delivery.failed` com `next_retry_at` ISO-8601.
- Cenário 3: `log_payload_safety_test.go` falha se algum arquivo do pacote chamar `slog.*` passando `payload`/`Payload` (proteção contra regressão).
- Cenário 4: `BenchmarkPublisher_Publish_1Handler` reporta ns/op estável; `BenchmarkDispatcher_DrainBacklog` reporta throughput ≥ 100 deliveries/s na máquina de referência.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `log_payload_safety_test.go` (proteção estática) + testes de sampler 1:100 + testes verificando campos do allowlist em cada uma das 6 transições.
- [ ] Testes de integração / benchmarks: `benchmark_test.go` com cenários do Publisher e do Dispatcher (build tag `integration` para os que dependem de Postgres).

**Definition of Done**:
- [ ] Nenhum arquivo `.go` do pacote contém literal `slog.*("...", "payload"` ou equivalente (assert estático).
- [ ] Allowlist `logFields` cobre exatamente os 6 campos canônicos + extensões documentadas (`latency_ms`, `next_retry_at`, `total_attempts`).
- [ ] Sampler 1:100 em `processed` verificável via contador atômico (test assert que após 200 chamadas saíram exatamente 2 logs).
- [ ] `docs/benchmarks/outbox-baseline.md` contém linhas com `ns/op` por cenário, data de execução e SHA do binário.
- [ ] Receita `task bench:outbox` executa sem erro local.
- [ ] `gofmt -w .` + `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/log_fields.go` (novo)
- `internal/infrastructure/outbox/log_payload_safety_test.go` (novo)
- `internal/infrastructure/outbox/benchmark_test.go` (novo)
- `internal/infrastructure/outbox/dispatcher.go` (modificado — emissão de logs via `logFields`)
- `internal/infrastructure/outbox/publisher.go` (modificado — log de boot/erros)
- `internal/infrastructure/outbox/cron.go` (modificado — logs de reaper/housekeeping)
- `internal/infrastructure/outbox/subsystem.go` (modificado — log de startup)
- `docs/benchmarks/outbox-baseline.md` (novo)
- `Taskfile.yml` (modificado — `task bench:outbox`)
