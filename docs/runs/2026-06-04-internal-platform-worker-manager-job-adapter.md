# Plano — `internal/platform/worker` (WorkerManager + JobAdapter + ConsumerAdapter + Registry agnóstico)

## Contexto

O prompt em `docs/prompts/internal-platform-worker-manager-job-adapter.md` exige implementar uma capacidade compartilhada de orquestração de workers, cron jobs e consumers em `internal/platform`, com:

- um único `WorkerManager` como ponto central de lifecycle (start/stop coordenado, shutdown gracioso, consolidação de erros);
- `JobAdapter` como único caminho válido para registrar cron jobs;
- `ConsumerAdapter` como único caminho válido para registrar consumers;
- um `consumer.Registry` agnóstico que recebe handlers do módulo uma única vez e é reutilizado por adapters de tecnologia (nesta entrega: apenas `database`; `kafka`, `rabbitmq`, `sqs`, `azureservicebus` ficam previstos no desenho mas não são criados agora);
- contrato canônico de handler `Handler(ctx, params map[string]string, body []byte) error`;
- semântica de `database` (o "memory" do prompt) como transporte local persistido via outbox/banco — sem reimplementar o outbox.

Estado atual do worktree (fonte de verdade, mandatório e inegociável):

- `internal/platform/` contém apenas `httpclient/`, `id/`, `errors/`. Os pacotes `worker`, `events`, `database`, `observability`, `outbox` foram removidos no refactor em andamento (vide `git status`).
- `cmd/worker/worker.go` ainda importa módulos deletados (`internal/billing`, `internal/identity`, `internal/platform/events`, `internal/platform/database`, `internal/platform/observability`, `internal/platform/worker`) e referencia `platformworker.NewManager(logger, ...runners)` — assinatura variádica que será substituída pela assinatura tipada `(Config, []Job, []Consumer, *slog.Logger)` definida no prompt. A reescrita de `cmd/worker/worker.go` e dos módulos é trabalho separado (out of scope aqui).
- `mockery.yml` declara `internal/platform/outbox.{Storage,Registry}` — pacote ainda inexistente; **não está no escopo deste plano** (não vamos criar `internal/platform/outbox`; o prompt proíbe alterar/reimplementar outbox e o `consumer/memory` da plataforma deve receber uma `Source` injetada).
- `go.mod`: Go 1.26.2; `github.com/robfig/cron/v3 v3.0.1` disponível para o scheduler. Nenhuma lib de Kafka/RabbitMQ/SQS/ASB presente — adapters por tecnologia ficarão como wrappers finos sobre uma `Source` injetada (sem importar clients reais).
- `configs/config.go` define `OutboxConfig` com `HousekeepingSchedule`/`ReaperInterval` validados via `cron.ParseStandard` — referência de padrão para validação de schedule.

Por que este trabalho agora: o `cmd/worker` precisa de uma plataforma de orquestração robusta para que os módulos (a serem reconstruídos depois) tenham um único contrato de bootstrap para jobs cron e consumers de mensageria. Sem isso, cada módulo improvisaria seu próprio loop, com risco de leak, shutdown silenciosamente abortado e bypass do outbox.

## Diretrizes inegociáveis aplicadas

- Sem `uber/fx`, `dig`, `fx.Lifecycle` ou DI runtime.
- Sem comentários no código entregue (AGENTS.md + R5 Uber).
- Sem `init()` (R0), sem `var _ Interface = (*Type)(nil)` (R6.4), sem prefixo `_` em globais (memória de feedback), sem abstração de tempo (memória `feedback_no_time_abstraction`).
- `context.Context` propagado em todo IO e em toda goroutine, com `errors.Join` para consolidar erros de start/stop.
- Logging estruturado com `log/slog`.
- Sem regra de negócio em `internal/platform`; sem importar `internal/<módulo>/...`.
- Sem fallback silencioso: erros de startup/shutdown sempre retornados/agregados.
- Sem blank identifier para mascarar dependência (R6.3 / AGENTS.md).

## Estrutura a criar

```
internal/platform/worker/
  types.go             # Managed, Job, Consumer interfaces
  config.go            # Config (ShutdownTimeout, Location, opcional StartTimeout)
  errors.go            # sentinel errors: errStartFailed, errStopTimeout, errDuplicateName
  manager.go           # Manager: NewManager(Config, []Job, []Consumer, *slog.Logger), Start, Stop, Wait
  manager_test.go      # tabela: start ok, start falha, stop ordenado, shutdown timeout, no-leak (goleak)
  job/
    types.go           # OverlapPolicy (Skip|Allow), Options
    adapter.go         # Adapter implementando worker.Job (Name, Schedule, Run, OverlapPolicy)
    adapter_test.go
    scheduler.go       # scheduler interno usando robfig/cron/v3 com Location, política de overlap, cancellation
    scheduler_test.go  # overlap=Skip não dispara concorrente; ctx cancela; erro propagado via slog
  consumer/
    types.go           # Handler interface, Message struct, Source interface, Runner interface
    registration.go    # Registration struct
    registry.go        # Registry impl: Register (dup error), Dispatch (unknown event error explícito)
    registry_test.go
    runner.go          # NewRunner(Source, Registry, *slog.Logger) Runner — bind Source -> Dispatch
    runner_test.go
    adapter.go         # Adapter implementando worker.Consumer (Name, Technology, Start, Stop)
    adapter_test.go
    database/
      adapter.go       # NewAdapter(name string, runner consumer.Runner) worker.Consumer (tecnologia="database")
```

Apenas o subpacote `database/` é criado agora. Ele é um wrapper fino: recebe uma `consumer.Source` construída pelo módulo (que conhece o outbox/banco real) e devolve um `worker.Consumer` com `Technology() = "database"`. Isso mantém `internal/platform` livre de dependências de broker e preserva a semântica do prompt — transporte local persistido via outbox/banco, sem reimplementar o outbox.

Os adapters de tecnologias adicionais (`kafka`, `rabbitmq`, `sqs`, `azureservicebus`) **não serão criados neste momento**. O desenho do `Registry` + `Source` + `Adapter` já está preparado para que essas tecnologias entrem no futuro como novos subpacotes (cada um expondo um `NewAdapter(name, runner)` análogo) sem qualquer alteração nos handlers ou no registry. Esse trabalho fica para o momento em que a tecnologia for efetivamente adotada.

## Contratos principais (a implementar literalmente)

```go
// worker/types.go
type Managed interface {
    Name() string
    Start(context.Context) error
    Stop(context.Context) error
}

type Job interface {
    Name() string
    Schedule() string
    Run(context.Context) error
    OverlapPolicy() job.OverlapPolicy
}

type Consumer interface {
    Name() string
    Technology() string
    Start(context.Context) error
    Stop(context.Context) error
}
```

```go
// worker/consumer/types.go
type Handler interface {
    Handle(ctx context.Context, params map[string]string, body []byte) error
}

type Message struct {
    EventType string
    Params    map[string]string
    Body      []byte
}

type Source interface {
    Start(ctx context.Context, deliver func(context.Context, Message) error) error
    Stop(ctx context.Context) error
}

type Runner interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

```go
// worker/consumer/registry.go
type Registry interface {
    Register(reg Registration) error
    Dispatch(ctx context.Context, eventType string, params map[string]string, body []byte) error
}
```

Observação sobre `Handler`: o prompt mostra dois formatos (`Handler(ctx, params, body) error` como método único e `Register(eventType, handler) error`). Vou unificar em **`type Handler interface { Handle(ctx, params, body) error }`** + **`Register(Registration{Name, EventType, Handler}) error`**. Isso satisfaz o critério "equivalente semanticamente idêntico" do prompt e evita armadilha de função-tipo (que pode ser convertida em qualquer assinatura compatível e perde nome diagnóstico). Também permite adaptar `HandlerFunc(func) Handler` para conveniência.

## Lifecycle do `WorkerManager`

`Start(ctx)`:

1. valida unicidade de `Name()` em jobs e consumers (`errDuplicateName`);
2. cria `runCtx, cancel := context.WithCancel(ctx)` armazenado no manager (raiz para todas as goroutines);
3. inicia o `job.scheduler` em uma goroutine (com `errgroup` de jobs e respeito a `OverlapPolicy`);
4. inicia cada `Consumer.Start(runCtx)` em sua própria goroutine (cada source roda no seu ritmo; manager mantém `errgroup` paralelo);
5. retorna assim que todos sinalizaram "iniciado" (channel `ready` por componente) ou retorna o primeiro erro consolidado (`errors.Join`).

`Stop(ctx)`:

1. cancela `runCtx` (interrompe admissão);
2. para o scheduler (`scheduler.Stop()` do `robfig/cron/v3` devolve um `context.Context` que sinaliza fim das execuções em voo — usaremos para aguardar drain);
3. chama `Consumer.Stop(stopCtx)` em paralelo onde `stopCtx, _ := context.WithTimeout(ctx, cfg.ShutdownTimeout)`;
4. aguarda `wg.Wait()` das goroutines com timeout do `stopCtx`;
5. consolida erros com `errors.Join` e devolve. Se o timeout estourou com trabalho em voo, devolve `errStopTimeout` agregado (sem fingir sucesso).

Goroutines distintas (obrigatório): 1 para coordenador de cron + N para cada consumer (cada `Consumer.Start` é bloqueante até `Stop`). Todas atadas ao `runCtx` e ao `wg` interno do manager.

## Política de overlap dos jobs

`OverlapPolicy`:

- `OverlapSkip` (default): se a execução anterior ainda está rodando, registra `slog.Warn` com `job.name` e `skipped=true` e não dispara;
- `OverlapAllow`: dispara concorrente (responsabilidade do job).

Implementação via `atomic.Bool` por job (`running.CompareAndSwap(false,true)`); o tick do cron consulta antes de chamar `Run`.

## Reuso de utilitários existentes

- `github.com/robfig/cron/v3` já presente em `go.mod` — usar `cron.New(cron.WithLocation(cfg.Location))` para o scheduler; validar specstring via `cron.ParseStandard` antes de registrar (mesma técnica de `configs/config.go:428-438`).
- `log/slog` (R7) já é padrão de logger no `cmd/worker/worker.go:37`.
- `errors.Join` (Go 1.20+) — disponível e já idiomático no `cmd/worker/worker.go:58,70`.
- Não criar novo pacote de clock. `time.Now()`/`time.Since` inline quando precisar medir latência (memória `feedback_no_time_abstraction`).

## Testes

- `manager_test.go`: tabela cobrindo (a) start sucesso com 2 jobs + 2 consumers fake; (b) start falha em consumer e erro propagado/agregado; (c) stop ordenado dentro do timeout; (d) shutdown timeout estourado retorna `errStopTimeout` agregado; (e) verificação de não-leak usando `runtime.NumGoroutine()` antes/depois (sem dep externa `goleak` — manter zero dependências novas).
- `job/scheduler_test.go`: registro com schedule inválido falha; OverlapSkip não dispara concorrente; ctx cancelado interrompe execução em voo; erro do `Run` é logado.
- `consumer/registry_test.go`: `Register` retorna erro em `EventType` duplicado e em `Handler` nil; `Dispatch` em evento desconhecido retorna erro explícito (`errUnknownEventType`); `Dispatch` propaga erro do handler; params/body chegam intactos.
- `consumer/runner_test.go`: erro de `Source.Start` propagado; `Stop` chama `Source.Stop` mesmo após erro de Dispatch; cancelamento do ctx encerra deliver loop.
- `consumer/adapter_test.go`: `Name()` e `Technology()` preservados; Start/Stop delegam.

Todos os testes table-driven com `testify/suite` quando houver estado compartilhado (R4). Sem mocks gerados (mockery) para tipos internos do `worker` — uso de fakes locais inline em `_test.go` (R1 exceção). Adicionar entrada no `mockery.yml`? **Não** — o prompt não pede e os contratos do worker são consumidos por testes simples; manter `mockery.yml` enxuto.

## Out of scope (explícito)

- Reescrever `cmd/worker/worker.go` para a nova assinatura (`worker.NewManager(Config, jobs, consumers, logger)`): o arquivo seguirá quebrado até os módulos `billing`/`identity` serem reconstruídos. Sinalizarei isso no relatório final.
- Criar `internal/platform/outbox`, `internal/platform/events`, `internal/platform/database`, `internal/platform/observability`.
- Implementar a `Source` concreta backed por outbox (responsabilidade do módulo, conforme prompt).
- Criar subpacotes `consumer/kafka`, `consumer/rabbitmq`, `consumer/sqs`, `consumer/azureservicebus` — adiados; só serão criados quando a respectiva tecnologia for adotada.
- Importar libs de Kafka/RabbitMQ/SQS/ASB (cada adapter futuro será wrapper que aceita `Source` injetada).
- Alterar `mockery.yml`, `Taskfile.yml`, migrations.

## Verificação ponta-a-ponta

```bash
# build do pacote-alvo
go build ./internal/platform/worker/...

# vet + race + cobertura local
go vet ./internal/platform/worker/...
go test -race -count=1 ./internal/platform/worker/...
go test -race -count=1 -cover ./internal/platform/worker/...

# lint do pacote
task lint:run -- ./internal/platform/worker/... || golangci-lint run ./internal/platform/worker/...

# checagem de proibições (devem retornar vazio)
grep -RnE "^func init\(\)" internal/platform/worker
grep -RnE "var _ [A-Z][A-Za-z0-9_]* = " internal/platform/worker
grep -RnE "interface\s*\{\s*\}" internal/platform/worker   # deve usar any
grep -RnE "internal/(billing|identity|platform/(events|database|observability|outbox))" internal/platform/worker
grep -Rn "//" internal/platform/worker | grep -v "_test.go" | grep -v "//go:" # sem comentários em código de produção
```

Smoke manual: instanciar `worker.NewManager` em um `main_test.go` fake com 1 job (`@every 1s`) e 1 consumer com fake-source que entrega 3 mensagens, verificar que `Start` retorna sem erro, jobs disparam, dispatch chega aos handlers e `Stop(ctx)` encerra dentro do timeout.

## Critérios de aceitação espelhados ao prompt

1. `WorkerManager` único, com `Start`/`Stop`, consolida lifecycle.
2. Todo cron job passa por `job.Adapter`.
3. Todo consumer passa por `consumer.Adapter`.
4. Jobs e consumers sobem no `Start`.
5. Goroutines distintas, com cancelamento cooperativo e shutdown coordenado.
6. Sem regra de negócio em `internal/platform`.
7. Sem dependência de broker; `database` (substitui o "memory" do prompt) será composição via `Source` injetada (sem alterar outbox); adapters de Kafka/RabbitMQ/SQS/ASB ficam adiados.
8. Erros nunca silenciados; logging estruturado.
