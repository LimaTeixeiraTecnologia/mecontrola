# Tarefa 7.0: whatsapp.ratelimit.Limiter (Start/Shutdown via module.go) + race + bench

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o rate-limiter por `user_id` em `internal/platform/whatsapp/ratelimit`. Token bucket com `sync.Map[uuid.UUID]*bucket` e `atomic.Int64` em hot path (lock-free), goroutine de cleanup TTL 5min cancelável via `Start(ctx)`/`Shutdown(ctx)`. Race detector obrigatório. Wiring no `module.go` segue padrão de lifecycle de `outbox.Dispatcher`.

<requirements>
- RF-07: `Limiter.Allow(userID) bool` com bucket 60 tokens + refill 1/s + burst 60; constantes Go exportadas.
- RF-08: `Limiter.Start(ctx)` + `Shutdown(ctx)`; goroutine cleanup TTL 5min remove buckets inativos; métrica `whatsapp_ratelimit_buckets_count`.
- RF-32: registro via `module.go` no padrão lifecycle hook; `cmd/api/main.go` chama `Start` antes de servir HTTP e `Shutdown` em SIGTERM; race detector obrigatório.
</requirements>

## Subtarefas

- [ ] 7.1 Criar `internal/platform/whatsapp/ratelimit/limiter.go` com `Limiter` struct, constantes `DefaultBucketCapacity=60`, `DefaultRefillPerSecond=1`, `DefaultInactivityTTL=5*time.Minute`, `DefaultCleanupPeriod=60*time.Second`, `DefaultShutdownTimeout=5*time.Second`.
- [ ] 7.2 Implementar `bucket` struct com `atomic.Int64` para `tokens`, `lastRefill`, `lastSeen` (todos em nanos).
- [ ] 7.3 Implementar `Allow(userID uuid.UUID) bool`: `LoadOrStore` bucket; atualizar `lastSeen`; `refill(b, now)`; `tryConsume(b)` em CAS loop.
- [ ] 7.4 Implementar `Start(ctx) error`: dispara `cleanupLoop` goroutine; retorna após inicialização.
- [ ] 7.5 Implementar `Shutdown(ctx) error`: fecha `shutdownCh`; aguarda `doneCh` ou `ctx.Done()`.
- [ ] 7.6 Implementar `cleanupLoop(ctx)`: a cada `cleanupPeriod`, itera `buckets.Range` removendo buckets com `now - lastSeen > inactivityTTL`; emite métricas `whatsapp_ratelimit_cleanup_duration_seconds` e `whatsapp_ratelimit_buckets_count`.
- [ ] 7.7 Criar `limiter_test.go` table-driven (cenários A–D do techspec) + `TestLimiter_RaceAllow_*` (1000 goroutines × 100 Allow paralelos com `t.Parallel` + `-race`).
- [ ] 7.8 Criar `BenchmarkLimiter_Allow` com pool de 5000 buckets — alvo < 200 ns/op.
- [ ] 7.9 Criar `TestLimiter_Shutdown` validando que goroutine encerra em < timeout.
- [ ] 7.10 Atualizar `internal/identity/module.go` (ou `internal/platform/whatsapp/ratelimit/module.go`) para expor `Limiter` no padrão lifecycle (Start/Stop callbacks compatíveis com `cmd/api/main.go`).

## Detalhes de Implementação

Ver techspec `## Design de Implementação > Interfaces Chave > Limiter` para esqueleto. Ver `.agents/skills/go-implementation/references/concurrency.md` para padrão de goroutines canceláveis (R6). Zero alocação em hot path: bucket é struct embedded; `LoadOrStore` reusa; CAS loop em `tryConsume` evita perda em race.

## Critérios de Sucesso

- `go test -race ./internal/platform/whatsapp/ratelimit/...` verde com 0 reports.
- `go test -bench=BenchmarkLimiter_Allow -benchmem` reporta < 200 ns/op + 0 allocs/op.
- `Shutdown(ctx)` retorna `nil` em < 5s; goroutine de cleanup encerra (verificado via `runtime.NumGoroutine` antes/depois).
- Métrica `whatsapp_ratelimit_buckets_count` reflete contagem real após Allow + cleanup.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários table-driven
- [ ] Testes de concorrência com `-race`
- [ ] Microbenchmark `BenchmarkLimiter_Allow`
- [ ] Test de shutdown cooperativo

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/ratelimit/limiter.go` + `_test.go` (criar)
- `internal/platform/whatsapp/ratelimit/bucket.go` (opcional — se preferir separar) + `_test.go`
- `internal/identity/module.go` ou `internal/platform/whatsapp/ratelimit/module.go` (atualizar/criar)
- `cmd/api/main.go` (atualizar em 9.0 — não nesta tarefa)
