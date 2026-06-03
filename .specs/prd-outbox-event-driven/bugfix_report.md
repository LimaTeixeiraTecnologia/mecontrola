# Relatorio de Bugfix

- Total de bugs no escopo: 5
- Corrigidos: 5
- Testes de regressao adicionados ou corrigidos: 8
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-001
- Severidade: critical
- Origem: RF-12 / US-11 / finding de review - retry consumia duas tentativas por falha
- Estado: fixed
- Causa raiz: `ClaimReady` ja incrementava `attempts` ao reivindicar a delivery, mas `Dispatcher.markResult` chamava `claim.Attempt.Next()` antes de `MarkFailed`, gravando um segundo incremento na mesma falha.
- Arquivos alterados: internal/infrastructure/outbox/dispatcher.go, internal/infrastructure/outbox/dispatcher_test.go, internal/infrastructure/outbox/storage_pgx_integration_test.go
- Teste de regressao: `TestScenario2_TransientError_CallsMarkFailed`, `TestScenario5_HandlerTimeout_CallsMarkFailed`, `TestCenario2B_MarkFailedPreservesClaimAttemptSequence`
- Validacao: `go test ./internal/infrastructure/outbox/...`; `go test -tags=integration ./internal/infrastructure/outbox/...`; `go test ./...`; `go vet ./...`; `go test ./... -count=1 -race`

- ID: BUG-002
- Severidade: major
- Origem: RF-22 / finding de review - traceparent armazenado nao era extraido para contexto OTel
- Estado: fixed
- Causa raiz: o Dispatcher copiava `traceparent` para `context.WithValue`, mas o tracer OTel usa `trace.SpanContextFromContext`; portanto os spans consumidores nao herdavam o trace remoto.
- Arquivos alterados: internal/infrastructure/outbox/dispatcher.go, internal/infrastructure/outbox/dispatcher_trace_internal_test.go
- Teste de regressao: `TestContextWithEventTraceparentExtractsW3CContext`, `TestContextWithEventTraceparentIgnoresMissingHeader`
- Validacao: `go test ./internal/infrastructure/outbox/...`; `go test ./...`; `go vet ./...`

- ID: BUG-003
- Severidade: major
- Origem: RF-31 / finding de review - DummyHandler logava campos fora do allowlist
- Estado: fixed
- Causa raiz: `DummyHandler` logava `aggregate_type` e `aggregate_id`, embora RF-31 limite os campos canonicos de auditoria/log do Outbox.
- Arquivos alterados: internal/infrastructure/outbox/dummy_handler.go, internal/infrastructure/outbox/dummy_handler_test.go
- Teste de regressao: `TestDummyHandlerUsesLogAllowlist`
- Validacao: `go test ./internal/infrastructure/outbox/...`; `golangci-lint run --timeout=5m`

- ID: BUG-004
- Severidade: major
- Origem: RF-39 / finding de review - `Cron.Stop` suprimia erro de timeout/cancelamento
- Estado: fixed
- Causa raiz: `Cron.Stop` detectava `ctx.Done()` e logava warning, mas retornava `nil`; o runtime nao recebia evidencia de shutdown incompleto.
- Arquivos alterados: internal/infrastructure/outbox/cron.go, internal/infrastructure/outbox/cron_internal_test.go
- Teste de regressao: `TestStopReturnsContextErrorWhenJobIsStillRunning`, `TestStopReturnsNilWhenJobsDrainBeforeDeadline`
- Validacao: `go test ./internal/infrastructure/outbox/...`; `go test ./...`; `go vet ./...`

- ID: BUG-005
- Severidade: minor
- Origem: finding de review - `Event.Headers()` expunha mapa mutavel interno
- Estado: fixed
- Causa raiz: `NewEvent` armazenava o mapa recebido e `Headers()` retornava o mesmo mapa, permitindo mutacao externa de um value object que deveria ser imutavel.
- Arquivos alterados: internal/infrastructure/outbox/event.go, internal/infrastructure/outbox/event_test.go
- Teste de regressao: `TestHeaders_ReturnsDefensiveCopy`
- Validacao: `go test ./internal/infrastructure/outbox/...`; `go test ./...`

## Comandos Executados

- `gofmt -w internal/infrastructure/outbox/dispatcher.go internal/infrastructure/outbox/cron.go internal/infrastructure/outbox/event.go internal/infrastructure/outbox/dummy_handler.go internal/infrastructure/outbox/dispatcher_test.go internal/infrastructure/outbox/dispatcher_trace_internal_test.go internal/infrastructure/outbox/cron_internal_test.go internal/infrastructure/outbox/event_test.go internal/infrastructure/outbox/dummy_handler_test.go internal/infrastructure/outbox/storage_pgx_integration_test.go` -> passou sem saida
- `go test ./internal/infrastructure/outbox/...` -> ok `github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox 2.078s`; subpackages fakes/mocks sem testes
- `go test -tags=integration ./internal/infrastructure/outbox/...` -> ok `github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox 8.670s`; subpackages fakes/mocks sem testes
- `go test ./...` -> todos os packages passaram; packages sem testes reportaram `[no test files]`
- `go vet ./...` -> passou sem saida
- `go test ./... -count=1 -race` -> todos os packages passaram, incluindo `internal/infrastructure/outbox 4.770s`
- `go build ./...` -> passou sem saida
- `golangci-lint run --timeout=5m` -> `0 issues.`
- `mockery --config mockery.yml --dry-run` -> dry-run gerou `Registry` e `Storage` sem erro; apenas warnings de deprecacao do mockery v2.53.4
- `grep -rn "^func init()" --include="*.go" .` -> nenhuma ocorrencia
- `grep -rn "os\\.Exit\\|log\\.Fatal" --include="*.go" . | grep -v "^./cmd/"` -> nenhuma ocorrencia
- `grep -rn "\\bpanic(" --include="*.go" . | grep -v "_test.go" | grep -v "template\\.Must\\|regexp\\.MustCompile"` -> apenas mocks gerados por mockery e fake de teste `internal/infrastructure/outbox/fakes/handler.go`
- `grep -rn "interface{}" --include="*.go" . | grep -v "_test.go" | grep -v "vendor/"` -> apenas mocks gerados por mockery

## Riscos Residuais

- O diretorio `internal/infrastructure/outbox/` permanece nao rastreado no worktree atual, entao `git diff` nao exibe estes arquivos ate que sejam adicionados ao indice.
- O dry-run do mockery permanece com warnings de deprecacao da versao 2.53.4, sem falha funcional.
