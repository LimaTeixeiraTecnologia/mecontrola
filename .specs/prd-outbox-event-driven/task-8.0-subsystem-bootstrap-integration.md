# Tarefa 8.0: Subsystem agregador, bootstrap no cmd/worker e suites integration/concorrência

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o ciclo: criar `outbox.Subsystem` que implementa `runtime.Subsystem` (`Start`/`Stop`/`Name`) compondo Dispatcher + Cron via `errgroup.WithContext` + `sync.WaitGroup`, registrá-lo em `buildSubsystems(ModeWorker)` em `internal/infrastructure/runtime/bootstrap.go`, criar o handler dummy + subscription (FC-10) e provar o ciclo end-to-end com integration ponta-a-ponta e concorrência (3 dispatchers paralelos / 1000 deliveries / 0 double-processing).

<requirements>
- RF-09: Dispatcher e jobs do Cron orquestrados por um único `outbox.Subsystem` (`Start`/`Stop`/`Name() == "outbox"`), registrado em `buildSubsystems(ModeWorker)`; `Stop(ctx)` drena handlers em execução.
- RF-34: suite de integração com testcontainers Postgres cobrindo `publish → claim → handler → mark` end-to-end.
- RF-35: teste de concorrência com pelo menos 3 dispatchers paralelos no mesmo Postgres provando empiricamente zero double-processing.
- RF-39: `Subsystem.Stop(ctx)` cancela ticker do Dispatcher e scheduler do Cron, aguarda handlers via `WaitGroup`, retorna erros via `errors.Join`; respeita `OUTBOX_DISPATCHER_ENABLED=false` não iniciando loop (Cron continua para housekeeping/reaper).
</requirements>

## Subtarefas

- [ ] 8.1 Criar `subsystem.go` com struct `Subsystem` (campos: `dispatcher *Dispatcher`, `cron *Cron`, `registry Registry`, `metrics *Metrics`, `logger *slog.Logger`, `wg sync.WaitGroup`, `enabled bool`).
- [ ] 8.2 Criar `NewSubsystem(deps SubsystemDeps) (*Subsystem, error)` recebendo `Config OutboxConfig`, `Storage`, `Registry`, `Metrics`, `Logger`, `Clock`, `InstanceID`; valida `registry.Validate()`; constrói Dispatcher e Cron internamente.
- [ ] 8.3 Implementar `Start(ctx) error`: chama `cron.Start(ctx)` sempre; chama `dispatcher.Start(ctx)` somente se `enabled`; log estruturado `outbox.subsystem.started` com `dispatcher_enabled`, `instance_id`.
- [ ] 8.4 Implementar `Stop(ctx) error`: chama `dispatcher.Stop(ctx)` e `cron.Stop(ctx)` em paralelo, agrega erros com `errors.Join`.
- [ ] 8.5 Implementar `Name() string { return "outbox" }`.
- [ ] 8.6 Criar `dummy_handler.go` com `DummyHandler` (handler trivial que registra métrica e log; idempotente trivialmente).
- [ ] 8.7 Criar `internal/infrastructure/runtime/outbox_subsystem.go` com `lazyOutboxSubsystem` exatamente como esqueleto da techspec (`Start` constrói `Manager`/`Provider`/`Registry`/`Storage`/`Metrics`/`Subsystem`; `Stop` reverte na ordem inversa via `closers`).
- [ ] 8.8 Criar função privada `registerSubscriptions(registry Registry) error` em `runtime/outbox_subsystem.go` registrando a subscription dummy (`Name: "outbox.dummy"`, `EventType: events.MustEventName("platform.outbox-dummy")`, `Handler: outbox.DummyHandler`).
- [ ] 8.9 Modificar `internal/infrastructure/runtime/bootstrap.go` para `buildSubsystems(ModeWorker)` retornar `[]Subsystem{b.newOutboxSubsystem(cfg, foundation)}`.
- [ ] 8.10 Criar `outbox_subsystem_test.go` em `runtime/` cobrindo: construção bem-sucedida, falha em `registerSubscriptions` propaga erro, `Stop` chama closers em ordem reversa.
- [ ] 8.11 Criar `subsystem_integration_test.go` (build tag `integration`): sobe Postgres via testcontainers, registra `DummyHandler`, publica 1 evento via `Publisher`, espera processed em < 2s, valida log/metric.
- [ ] 8.12 Criar `concurrency_integration_test.go` (build tag `integration`): pré-popula 1000 deliveries `pending`, sobe 3 Subsystems com `instanceID` distinto contra o mesmo Postgres; aguarda drain; valida `SELECT event_id, subscription_name, COUNT(*) FROM outbox_deliveries WHERE status='processed' GROUP BY 1,2 HAVING COUNT(*) > 1` retorna vazio; throughput agregado ≥ 100 deliveries/s.
- [ ] 8.13 No `subsystem_integration_test.go`, adicionar cenário `reaper`: insere delivery `status='claimed'` com `claimed_at = now()-10m`; aguarda 1 tick do reaper (forçar com clock + entry de cron de teste); verifica volta para `pending` e Dispatcher seguinte a processa.
- [ ] 8.14 No mesmo arquivo, adicionar cenário `flag-off`: sobe Subsystem com `OUTBOX_DISPATCHER_ENABLED=false`, publica evento, verifica que `outbox_deliveries.status` permanece `pending` (Dispatcher não claimou).

## Detalhes de Implementação

Ver techspec.md seções **Arquitetura do Sistema → Componentes → `outbox.Subsystem`**, **Pontos de Integração → `runtime.Subsystem` (RF-09 / RF-39)** (esqueleto completo do `lazyOutboxSubsystem`), **Abordagem de Testes → Testes de Integração** (cenários obrigatórios `Storage` / `Subsystem` / `Concorrência` / `Reaper`).

## Critérios de Sucesso

- `go test -tags=integration ./internal/infrastructure/outbox/...` verde, < 60s total.
- `go test ./internal/infrastructure/runtime/...` verde (unit).
- Cenário ponta-a-ponta: latência `publish → processed` < 2s para 1 evento + 1 handler dummy.
- Cenário concorrência: 1000 deliveries drenadas por 3 dispatchers; 0 deliveries `processed` duplicadas; throughput ≥ 100/s.
- Cenário reaper: delivery `claimed_at = now() - 10m` volta para `pending` em ≤ 2 ticks do reaper de teste e é re-claimada na sequência.
- Cenário flag-off: `OUTBOX_DISPATCHER_ENABLED=false` mantém Publisher escrevendo e Dispatcher silencioso; Cron (reaper/housekeeping) continua executando.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `subsystem_test.go` cobrindo agregação Dispatcher+Cron, `Stop` com `errors.Join`, respeito ao flag off; `outbox_subsystem_test.go` em `runtime/` cobrindo construção e ordem de fechamento.
- [ ] Testes de integração: `subsystem_integration_test.go` (ponta-a-ponta + reaper + flag-off) e `concurrency_integration_test.go` (3 dispatchers / 1000 deliveries / 0 double-processing).

**Definition of Done**:
- [ ] `cmd/worker` boot completo: build, log "outbox.subsystem.started", Postgres acessado, Registry validado sem erro.
- [ ] `Stop(ctx)` retorna em ≤ `handlerTimeout + 2s` mesmo com 10 handlers in-flight (assert de tempo).
- [ ] Concurrency test valida ausência de double-processing via SQL `GROUP BY HAVING COUNT(*) > 1` retornando 0 linhas.
- [ ] Throughput de 100 deliveries/s sustentado documentado no resultado do test (impressão via `t.Logf`).
- [ ] Reaper test usa entry de cron com schedule curto (`@every 100ms`) injetada via `Cron.Override(...)` ou config de teste — não esperar por `@every 1m` real.
- [ ] `gofmt -w .` + `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/subsystem.go` (novo)
- `internal/infrastructure/outbox/subsystem_test.go` (novo)
- `internal/infrastructure/outbox/dummy_handler.go` (novo)
- `internal/infrastructure/outbox/subsystem_integration_test.go` (novo, build tag integration)
- `internal/infrastructure/outbox/concurrency_integration_test.go` (novo, build tag integration)
- `internal/infrastructure/runtime/outbox_subsystem.go` (novo)
- `internal/infrastructure/runtime/outbox_subsystem_test.go` (novo)
- `internal/infrastructure/runtime/bootstrap.go` (modificado — `buildSubsystems(ModeWorker)`)
- `cmd/worker/main.go` (consumido — sem modificação se já chama `runtime.Bootstrap`)
