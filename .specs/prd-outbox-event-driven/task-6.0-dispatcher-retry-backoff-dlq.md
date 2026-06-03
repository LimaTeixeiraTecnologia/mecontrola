# Tarefa 6.0: Dispatcher com retry, backoff, DLQ e timeout

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o motor do Outbox: o `Dispatcher` que acorda no `time.Ticker`, faz claim de um lote via `Storage.ClaimReady`, executa cada `Handler` com timeout, classifica o erro e marca o resultado (`MarkProcessed` | `MarkFailed` com `NextRetryAt` calculado pela `BackoffPolicy` | `MarkDLQ` em `ErrPermanent` ou exhaustão de attempts). Drena handlers in-flight no `Stop(ctx)` via `sync.WaitGroup`.

<requirements>
- RF-11: timeout configurável de handler (default `OUTBOX_DISPATCHER_HANDLER_TIMEOUT=10s`); timeout tratado como falha transitória.
- RF-12: backoff exponencial com jitter (base 2s, cap 5min, até 15 tentativas); após exhaustão, transitar para DLQ.
- RF-13: respeitar `errors.Is(err, ErrPermanent)` → DLQ imediato sem consumir tentativas.
- RF-15: gravar `attempts`, `next_retry_at`, `last_error`, `processed_at`, `dead_letter_at` por delivery via Storage.
- RF-33: suíte unitária do Dispatcher cobrindo regras de retry, backoff, DLQ, timeout com mock de Storage.
</requirements>

## Subtarefas

- [ ] 6.1 Criar `dispatcher.go` com struct `Dispatcher` (campos: `storage Storage`, `registry Registry`, `policy BackoffPolicy`, `maxAttempts Attempt`, `handlerTimeout time.Duration`, `tickInterval time.Duration`, `batchSize int`, `instanceID string`, `clock Clock`, `metrics Metrics`, `logger *slog.Logger`, `wg sync.WaitGroup`).
- [ ] 6.2 Implementar `Start(ctx)`: respeita `OUTBOX_DISPATCHER_ENABLED=false` (não entra no loop); inicia goroutine com `time.NewTicker(tickInterval)`; em cada tick chama `tickOnce(ctx)`.
- [ ] 6.3 Implementar `tickOnce(ctx)`: `claims, err := storage.ClaimReady(ctx, batchSize, instanceID)`; itera; para cada claim, executa `s.deliver(ctx, claim)` em goroutine controlada por `wg`.
- [ ] 6.4 Implementar `deliver(ctx, claim)`: hidrata `Handler` via `Registry.SubscriptionsFor`; cria span filho `outbox.deliver` extraindo `traceparent` do `claim.Event.Headers`; envelopa em `context.WithTimeout(ctx, handlerTimeout)`; envelopa em `defer recover` para tratar panic como `ErrPermanent`; chama `handler(ctx, claim.Event)`; classifica resultado via `s.markResult`.
- [ ] 6.5 Implementar `markResult`: switch sobre tipo de erro — nil → `MarkProcessed`; `errors.Is(ErrPermanent)` → `MarkDLQ`; `context.DeadlineExceeded` → `MarkFailed` (transient); attempts esgotados → `MarkDLQ`; demais → `MarkFailed` com `nextRetryAt = policy.NextRetryAt(attempt, now)`.
- [ ] 6.6 Implementar `Stop(ctx)`: cancela ticker, aguarda `wg.Wait()` respeitando `ctx` (via `select` com `ctx.Done()`).
- [ ] 6.7 Criar `dispatcher_test.go` com `testify/suite` + table-driven + `mocks.Storage` + `Clock` fake + `Handler` fakes em `fakes/handler.go` (success, transient, permanent, panic, timeout).
- [ ] 6.8 Criar `internal/infrastructure/outbox/fakes/handler.go` com construtores de handlers determinísticos para uso em testes.

## Detalhes de Implementação

Ver techspec.md seções **Arquitetura do Sistema → Componentes → `outbox.Dispatcher`**, **Abordagem de Testes → Testes Unitários → Dispatcher** (8 cenários obrigatórios), **Considerações Técnicas → Aplicação Explícita de Object Calisthenics → #1, #2, #7** (early-return + sem else + ≤ 4 colaboradores) e **Riscos Conhecidos** (`rand.Rand` não thread-safe — cada Dispatcher cria o próprio).

## Critérios de Sucesso

- `go test ./internal/infrastructure/outbox/...` verde sem `time.Sleep` para sincronização (usar `Clock` injetável + `chan` para coordenar testes).
- 8 cenários do dispatcher_test.go verdes:
  1. Sucesso → `MarkProcessed` chamado com `processedAt` definido.
  2. Erro transitório → `MarkFailed` com `nextRetryAt > now` e `attempts+=1`.
  3. `ErrPermanent` → `MarkDLQ` imediato sem incrementar attempts.
  4. Exhaustão (`attempt >= 15`) com erro transitório → `MarkDLQ` com `lastErr`.
  5. Timeout do handler → `MarkFailed` (classificado como transient).
  6. Panic do handler → `MarkDLQ` (classificado como permanent via recover).
  7. `OUTBOX_DISPATCHER_ENABLED=false` → goroutine não inicia, ticker não cria, `Storage.ClaimReady` nunca chamado.
  8. `Stop(ctx)` com handler in-flight: handler termina antes do `Stop` retornar (até `ctx.Done()`); `wg.Wait()` cumprido.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `dispatcher_test.go` com os 8 cenários listados, usando `mocks.Storage`, fakes manuais de `Handler` e `Clock` (interface `Clock { Now() time.Time }`).
- [ ] Testes de integração: cobertos em 8.0 (`subsystem_integration_test.go` + `concurrency_integration_test.go`).

**Definition of Done**:
- [ ] Nenhum `time.Sleep` em código de produção do Dispatcher (apenas `time.NewTicker`).
- [ ] Nenhum `time.Sleep` em testes (R-TEST-001 — usar `Clock` fake + canais).
- [ ] `BackoffPolicy.rng` é local ao Dispatcher (não compartilhado entre goroutines).
- [ ] Span `outbox.deliver` criado por delivery extraindo `traceparent` do header (RF-22).
- [ ] `Stop(ctx)` retorna em ≤ `handlerTimeout + 1s` mesmo com 10 handlers in-flight (verificar em teste com asserts de tempo).
- [ ] Cobertura ≥ 90% no `dispatcher.go`.
- [ ] `gofmt -w .` + `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/dispatcher.go` (novo)
- `internal/infrastructure/outbox/dispatcher_test.go` (novo)
- `internal/infrastructure/outbox/fakes/handler.go` (novo)
- `internal/infrastructure/outbox/storage.go` (consumido — criado em 3.0)
- `internal/infrastructure/outbox/registry.go` (consumido — criado em 4.0)
- `internal/infrastructure/outbox/backoff_policy.go` (consumido — criado em 2.0)
- `internal/infrastructure/outbox/errors.go` (consumido — criado em 2.0)
