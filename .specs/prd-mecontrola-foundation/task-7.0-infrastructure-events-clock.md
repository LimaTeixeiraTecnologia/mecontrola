# Tarefa 7.0: `internal/infrastructure/events` — typed eventbus via generics + `internal/infrastructure/clock`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o **eventbus in-process tipado via generics Go 1.26** em `internal/infrastructure/events` (D-08, ADR-003) com API mínima `Publish[E Event]`, `Subscribe[E Event]`, `Close()` + backpressure por buffer + emissão pós-`UoW.Commit` (contrato canônico para PRDs futuros). Materializar também o utilitário `Clock` em `internal/infrastructure/clock` (interface `Clock { Now() time.Time }` + `SystemClock` + `FakeClock` em `_test.go`). Cobre **RF-10** integralmente.

<requirements>
- API tipada com generics Go 1.26: `Publish[E Event](ctx, evt E) error` e `Subscribe[E Event](handler func(ctx, evt E) error) (unsubscribe func(), error)`.
- Tipo base `Event` interface com `Name() EventName`, `OccurredAt() time.Time`, `AggregateID() string`.
- VOs em `internal/infrastructure/events/event.go`: `EventID` (ULID via Clock injetado), `EventName` (kebab-case `<modulo>.<acao>`), `ModuleName` (enum dos 6 módulos de domínio).
- Backpressure: buffer configurável por subscriber (default 100); buffer cheio → log + drop com métrica `events_dropped_total{event_name,reason="buffer_full"}`.
- `Close(ctx)` idempotente; drena buffers até timeout do ctx; novos publishes após Close retornam erro `ErrBusClosed`.
- `Clock` interface em `internal/infrastructure/clock/clock.go`; `SystemClock` retorna `time.Now()`; `FakeClock` em `_test.go` (controlável para testes determinísticos).
- Integration test com 1000 eventos concorrentes validando ordem por subscriber + drop em close.
</requirements>

## Subtarefas

- [ ] 7.1 Criar `internal/infrastructure/events/event.go` com interface `Event` + VOs `EventID`, `EventName`, `ModuleName`.
- [ ] 7.2 Criar `internal/infrastructure/events/bus.go` com struct `Bus` (map[reflect.Type] → []handler com slice por tipo) + `Publish[E Event]` + `Subscribe[E Event]` + `Close`.
- [ ] 7.3 Implementar backpressure: cada subscriber tem `chan E` bufferizado (default 100); publish faz `select { case ch <- evt: default: dropAndMetric() }`.
- [ ] 7.4 Implementar `Close(ctx)` idempotente; sentinel `ErrBusClosed`.
- [ ] 7.5 Criar `internal/infrastructure/clock/clock.go` com interface `Clock` + `SystemClock`.
- [ ] 7.6 Criar `internal/infrastructure/clock/fake.go` com `FakeClock` (controlável) — pode viver em arquivo separado para reuso por outros testes.
- [ ] 7.7 Criar `internal/infrastructure/events/bus_test.go` (unit, table-driven; com `FakeClock`).
- [ ] 7.8 Criar `internal/infrastructure/events/bus_integration_test.go` com tag `//go:build integration` (in-process, sem testcontainers): 1000 eventos concorrentes; assert ordem por subscriber; assert drop em close.
- [ ] 7.9 Criar `internal/infrastructure/events/doc.go` com pattern de declaração de evento + exemplo `MessageReceived` (sem implementar, apenas documentar para os PRDs futuros).
- [ ] 7.10 Concretizar binding em `runtime.Bootstrap` (de 3.0) — `Bus` é singleton da foundation injetado em todos os subsistemas que precisarão publicar/subscrever.

## Detalhes de Implementação

Ver techspec §"Modelagem de Domínio" (Domain Events) + §"Interfaces Chave" + ADR-003.

## Critérios de Sucesso

- `go build ./internal/infrastructure/events/... ./internal/infrastructure/clock/...` compila com Go 1.26.3 (generics).
- `go test ./internal/infrastructure/events/...` verde com cobertura ≥ 90%.
- `go test -tags=integration -race -count=10 ./internal/infrastructure/events/...` verde — 1000 eventos concorrentes sem panic, sem race, ordem preservada por subscriber.
- `Close(ctx)` chamado 2x não panica e retorna nil na segunda.
- `Publish[E]` após `Close` retorna `ErrBusClosed`.
- Cobre RF-10 integralmente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `bus_test.go` cobrindo `Publish`+`Subscribe`+`Close`+drop em buffer cheio (com `FakeClock`); `clock_test.go` cobrindo `SystemClock` e `FakeClock`.
- [ ] Testes de integração: `bus_integration_test.go` com 1000 eventos concorrentes — assert ordem + drop em close (≥10 runs com `-race`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/infrastructure/events/event.go`
- `internal/infrastructure/events/bus.go`
- `internal/infrastructure/events/bus_test.go`
- `internal/infrastructure/events/bus_integration_test.go`
- `internal/infrastructure/events/doc.go`
- `internal/infrastructure/clock/clock.go`
- `internal/infrastructure/clock/fake.go`
- `internal/infrastructure/clock/clock_test.go`
- `internal/infrastructure/runtime/bootstrap.go` (binding concreto)
- `go.mod`, `go.sum`
