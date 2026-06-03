# Tarefa 2.0: Value Objects e tipos de domínio do pacote outbox

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar os tipos imutáveis que sustentam invariantes do domínio Outbox (R-DDD-001, ADR-003): `Event`, `Headers`, `SubscriptionName`, `DeliveryStatus` (State Pattern), `Attempt`, `BackoffPolicy` (ADR-004, `rand.Rand` injetável), `Claim`, `Stats`, além dos sentinels de erro e do `Handler`/`InstanceID`. Nenhum import de `pgx`, `cron` ou `otel` é permitido nesta camada — só Go stdlib + `internal/infrastructure/events` (para `EventID`/`EventName` canônicos).

<requirements>
- RF-03: reutilizar `events.EventID` (ULID) e `events.EventName` no `Event`; metadados mínimos no struct.
- RF-13: sentinels `ErrPermanent`, `ErrHandlerNotRegistered`, `ErrDispatcherDisabled` consumíveis via `errors.Is`/`errors.As`; adicionar `ErrDuplicateSubscription` e `ErrInvalidEvent`.
- D-08: caller fornece `EventID`; construtor `NewEvent` apenas valida unicidade-de-campo (não gera ULID).
- D-11: `InstanceID = fmt.Sprintf("%s-%d", hostname, pid)`.
- D-13: `BackoffPolicy` com `rand.Rand` injetável para testes determinísticos.
- ADR-003: `DeliveryStatus` como VO com State Pattern (`CanTransitionTo`) — sem enum-string solto.
</requirements>

## Subtarefas

- [ ] 2.1 Criar pacote `internal/infrastructure/outbox/` com `doc.go` documentando idempotência obrigatória por `event_id` e critério de quando usar `outbox.Publisher` vs `events.Bus`.
- [ ] 2.2 Criar `event.go` com struct `Event` (campos não exportados), `NewEvent(NewEventParams)` validando `ID`, `AggregateType`, `Payload` (json.Valid), versionamento default 1, `OccurredAt` default `time.Now().UTC()`; getters de leitura por intenção.
- [ ] 2.3 Criar `headers.go` com `type Headers map[string]string` + métodos `WithTrace(traceparent)`, `Get(key)`, `Validate()` (chaves canônicas).
- [ ] 2.4 Criar `subscription.go` com `SubscriptionName` encapsulando `string` (regex `^[a-z][a-z0-9_-]{2,63}$`) e struct `Subscription{Name, EventType, Handler}`.
- [ ] 2.5 Criar `delivery_status.go` com VO + constantes `StatusPending/Claimed/Processed/DeadLetter` + método `CanTransitionTo(next)` cobrindo as transições documentadas na techspec.
- [ ] 2.6 Criar `attempt.go` com VO `Attempt` (uint8) + `Next()` + `IsExhausted(max Attempt) bool`.
- [ ] 2.7 Criar `backoff_policy.go` com `NewBackoffPolicy(base, cap, rng)` e `NextRetryAt(attempt, now) time.Time` aplicando `min(base * 2^attempt * (0.5 + rng), cap)`.
- [ ] 2.8 Criar `handler.go` com `type Handler func(ctx, evt) error` e godoc explicando a regra obrigatória de idempotência por `event.ID`.
- [ ] 2.9 Criar `errors.go` com os 5 sentinels (`ErrPermanent`, `ErrHandlerNotRegistered`, `ErrDispatcherDisabled`, `ErrDuplicateSubscription`, `ErrInvalidEvent`).
- [ ] 2.10 Criar `instance_id.go` com função `NewInstanceID()` chamando `os.Hostname()` + `os.Getpid()` e retornando `string` `"host-pid"`.
- [ ] 2.11 Criar `claim.go` (`Claim{ID, Event, SubscriptionName, Attempt, ClaimedAt}`, `ClaimID int64`) e `stats.go` (`Stats{Pending, DeadLetter, OldestPendingAt}`).
- [ ] 2.12 Criar `_test.go` por VO usando `testify/suite` + table-driven (R4 da go-implementation), com sementes fixas em `BackoffPolicy` para asserts determinísticos.

## Detalhes de Implementação

Ver techspec.md seções **Design de Implementação → Modelos de Dados → Value Objects (domain)** (código completo dos construtores `NewEvent`, `NewBackoffPolicy`, `DeliveryStatus.CanTransitionTo`), **Considerações Técnicas → Aplicação Explícita de Object Calisthenics** (mapeamento OC #1–#9) e **Estratégia de Erros (RF-13 + R-ERR-001)** (definição dos sentinels).

## Critérios de Sucesso

- `go test ./internal/infrastructure/outbox/...` verde rodando **só os VOs**.
- `gofmt -w .` + `golangci-lint run` verde para o pacote.
- Nenhum arquivo do pacote (exceto futuros `*_pgx.go`/`metrics.go`/`storage_pgx_integration_test.go`) importa `pgx`, `robfig/cron` ou `go.opentelemetry.io/otel`.
- Cobertura ≥ 90% nos VOs (medir com `go test -cover`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: um `_test.go` por VO (`event_test.go`, `headers_test.go`, `subscription_test.go`, `delivery_status_test.go`, `attempt_test.go`, `backoff_policy_test.go`, `instance_id_test.go`) com cenários de construção válida, falha de invariante, transição inválida (state machine), exhaustão de attempts e backoff em attempts 0 / N / max.
- [ ] Testes de integração: não aplicável nesta tarefa (camada de domínio pura).

**Definition of Done**:
- [ ] Sentinels publicamente exportados e cobertos por teste `errors.Is`.
- [ ] Construtor `NewEvent` rejeita `ID` vazio, `AggregateType` vazio, `Payload` não-JSON com mensagens claras.
- [ ] `DeliveryStatus.CanTransitionTo` cobre as 4 origens × 4 destinos via table-driven.
- [ ] `BackoffPolicy` com `rng` semeado em teste produz delays determinísticos (asserts numéricos exatos para attempts 0, 5, 15).
- [ ] `NewInstanceID()` retorna string não-vazia e contém o pid corrente.
- [ ] Lint passa sem warning de unused-fields/unused-export.
- [ ] Cobertura ≥ 90% no pacote.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/doc.go` (novo)
- `internal/infrastructure/outbox/event.go` + `event_test.go` (novos)
- `internal/infrastructure/outbox/headers.go` + `headers_test.go` (novos)
- `internal/infrastructure/outbox/subscription.go` + `subscription_test.go` (novos)
- `internal/infrastructure/outbox/delivery_status.go` + `delivery_status_test.go` (novos)
- `internal/infrastructure/outbox/attempt.go` + `attempt_test.go` (novos)
- `internal/infrastructure/outbox/backoff_policy.go` + `backoff_policy_test.go` (novos)
- `internal/infrastructure/outbox/handler.go` (novo)
- `internal/infrastructure/outbox/errors.go` (novo)
- `internal/infrastructure/outbox/instance_id.go` + `instance_id_test.go` (novos)
- `internal/infrastructure/outbox/claim.go` (novo)
- `internal/infrastructure/outbox/stats.go` (novo)
