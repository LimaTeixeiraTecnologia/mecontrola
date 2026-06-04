# Plano — Event Dispatcher em `internal/platform/events`

## Contexto

O prompt `docs/prompts/internal-platform-event-dispatcher.md` pede um *event dispatcher* in-process, com a API `Event` / `EventHandler` / `EventDispatcher` (`Register`/`Dispatch`/`Remove`/`Has`/`Clear`), production-ready, thread-safe, zero comentários, alinhado ao `go-implementation` e ao codebase real.

Análise do working tree (não inventando contexto ausente):

- `internal/platform/worker/consumer/registry.go` é semanticamente diferente:
  - mapeia `eventType → 1 Handler` (não N);
  - payload é `params map[string]string, body []byte` (envelope de broker), não `Event` tipado;
  - é a cola entre `Source` externa (broker/queue) e handler de consumo, não pub-sub de eventos de domínio in-process.
  → **Reuso/extracção de `consumer.Registry` produz acoplamento errado**. A solução correta é um pacote novo `internal/platform/events`, justificável tecnicamente:
  - cardinalidade diferente (N handlers por tipo);
  - contrato `Event` tipado vs envelope byte;
  - concern diferente (domain pub-sub vs broker consumer registration).
- Modules `internal/billing/module.go` e `internal/identity/module.go` estão como **stubs vazios**; `cmd/server/server.go` e `cmd/worker/worker.go` ainda fazem drift referenciando `NewModule`/`Runners()`/`Routers()` que não existem localmente. Por instrução explícita do prompt, **não inventar `module.go`** — wiring de módulos fica fora do escopo desta task.
- Padrão de composição do repo: **Functional Options** (não há `bundle.Container`). Observabilidade é externa (`devkit-go`), exposta via `provider.Observability()`.
- Testes: `testify/suite` + table-driven + mockery; arquivo `*_test.go` whitebox no mesmo pacote.
- Go 1.26.2 — `slices.Contains`, `clear()`, `errors.Join` disponíveis e idiomáticos.
- `consumer.Registry` é interface exportada retornada por `NewRegistry()` — **convenção local** de pacote de plataforma compartilhada. O dispatcher seguirá o mesmo padrão.

## Escolha de design

**Variante aceitável** do prompt: novo pacote `internal/platform/events`, separado de `consumer`. Justificativa registrada acima.

### API pública (adaptada do exemplo, com adaptações objetivas)

```go
package events

type Event interface {
    GetEventType() string
    GetPayload() any
}

type Handler interface {
    Handle(ctx context.Context, event Event) error
}

type Dispatcher interface {
    Register(eventType string, handler Handler) error
    Dispatch(ctx context.Context, event Event) error
    Remove(eventType string, handler Handler) error
    Has(eventType string, handler Handler) bool
    Clear()
}

type Option func(*dispatcher)

func WithCapacity(capacity int) Option
func NewDispatcher(opts ...Option) Dispatcher
```

Adaptações relativas ao exemplo do prompt (todas justificadas por conflito objetivo com o codebase ou idiomático Go):

1. `EventHandler` → `Handler` e `EventDispatcher` → `Dispatcher`. Espelha o padrão `consumer.Registry`/`consumer.Handler` (pacote já carrega o sufixo semântico). Não prefixar com `Event` em pacote `events` é idiomático Go.
2. `DispatcherOption` → `Option` (mesma justificativa).
3. Sentinels **exportados** (`ErrHandlerAlreadyRegistered`, `ErrEventNil`, `ErrHandlerNil`, `ErrEventTypeEmpty`) — diferente de `consumer.registry` que mantém sentinels privados. Justificativa: módulos consumidores precisam discriminar duplicidade em testes/lógica (`errors.Is`).
4. Mensagens com prefixo `events:` para tracing rápido em logs.

### Comportamento

- `Register`: valida `eventType != ""` e `handler != nil`; recusa duplicidade com `ErrHandlerAlreadyRegistered` (igualdade de interface por header `==`, equivalente ao exemplo).
- `Dispatch`: valida `event != nil` e `event.GetEventType() != ""`; se não houver handlers para o tipo → **no-op** (consistente com o exemplo e com pub-sub idiomático); snapshot dos handlers sob `RLock`, libera, executa **sequencialmente** preservando ordem; antes de cada handler checa `ctx.Done()` retornando `ctx.Err()`; **curto-circuita** no primeiro erro do handler (sem `errors.Join`, alinhado ao exemplo e ao requisito de "ordem").
- `Remove`: tolerante (eventType/handler vazios ou inexistentes → no-op silencioso, retorna `nil`), igual ao exemplo. Remove **uma única** ocorrência preservando a ordem das restantes.
- `Has`: retorna `false` para entradas inválidas; `RLock` puro.
- `Clear`: `clear(map)` sob write lock.

### Concorrência e robustez

- `sync.RWMutex`: read lock em `Dispatch`/`Has`, write lock em `Register`/`Remove`/`Clear`.
- Snapshot defensivo do slice em `Dispatch` antes de liberar o lock, evita corrida com `Remove`/`Clear`.
- Sem goroutines internas → sem leak, sem necessidade de shutdown.
- Sem `init()`, sem `panic`, sem `_ = x`, sem `var _ Interface = (*Type)(nil)`.
- Zero comentários no código final (linter pode reclamar de godoc ausente; o usuário aceita).

## Arquivos a criar

| Arquivo | Conteúdo |
|---|---|
| `internal/platform/events/events.go` | Interfaces `Event`, `Handler`, `Dispatcher`; sentinels `ErrHandlerAlreadyRegistered`, `ErrEventNil`, `ErrHandlerNil`, `ErrEventTypeEmpty`. |
| `internal/platform/events/dispatcher.go` | Struct `dispatcher` (privada), `Option`, `WithCapacity`, `NewDispatcher`, implementações de `Register`/`Dispatch`/`Remove`/`Has`/`Clear`. Usa `slices.Contains`. |
| `internal/platform/events/dispatcher_test.go` | Suite `testify/suite` whitebox com cenários table-driven: nil event, eventType vazio, handler nil, registro válido, duplicidade, dispatch sem handlers (no-op), dispatch com sucesso, propagação de erro do handler, ordem de execução, context cancelado antes do primeiro handler, context cancelado no meio, remoção (existente, inexistente, eventType inexistente, último handler limpa o bucket), `Has` true/false/inputs inválidos, `Clear`, `WithCapacity`, e teste de concorrência (`-race`) com `goroutines` paralelas chamando `Register`/`Dispatch`/`Remove`/`Has` simultaneamente. |

## Arquivos a NÃO modificar

- `cmd/server/server.go`, `cmd/worker/worker.go`: o dispatcher não tem consumidor real ainda (modules são stub). Wiring fica para quando os modules existirem — fora do escopo do prompt.
- `internal/platform/worker/**`: dispatcher não compartilha contrato com `consumer.Registry`.
- `internal/{billing,identity}/module.go`: stubs vazios; não inventar conteúdo.

## Verificação

```
gofmt -l internal/platform/events
go vet ./internal/platform/events/...
go test -race -count=1 ./internal/platform/events/...
```

Critérios de aceite verificáveis:
- `go test -race` passa em todos os cenários, incluindo o teste de concorrência paralela.
- Cobertura cobre: validação de inputs, duplicidade, ordem, snapshot, cancelamento de contexto antes/durante, remoção idempotente, `Has`, `Clear`, `WithCapacity`.
- `grep -nE '(^|[^:])//|/\*' internal/platform/events/*.go | grep -v _test` deve retornar vazio (zero comentários no código de produção; arquivos de teste podem ter `// Setup` se a skill exigir — verificar exemplos da skill antes).
- Nenhuma `init()`, nenhum `panic`, nenhum `var _ Dispatcher = (*dispatcher)(nil)`.

## Resumo final (a ser entregue após implementação)

1. Onde o dispatcher ficou: `internal/platform/events`.
2. Relação com `consumer.Registry`: pacotes ortogonais — `consumer.Registry` é cola broker→handler (1:1, byte payload); `events.Dispatcher` é pub-sub in-process (1:N, `Event` tipado). Sem duplicação semântica.
3. Registro em módulo: não aplicado nesta task — modules estão como stub vazio no working tree; o usuário pediu explicitamente para **não inventar** `module.go`. Wiring acontece quando os modules existirem (`identityModule.NewModule(events.WithDispatcher(...))` seguindo o padrão functional options já estabelecido).
4. Por que é production-ready: thread-safe com lock mínimo, snapshot defensivo, respeita `context.Context`, sem leaks/goroutines/estado global/init, sentinels exportados para `errors.Is`, testes com `-race` cobrindo concorrência, validação de todos os edge cases do contrato.
