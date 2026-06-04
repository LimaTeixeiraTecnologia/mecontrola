# Mandato: bootstrap de observabilidade via devkit-go/otel

## Context

O mandato em `docs/prompts/mandato-otel-devkit-go-startup.md` exige que `cmd/server/server.go` e `cmd/worker/worker.go` passem a inicializar observabilidade obrigatoriamente via `github.com/JailtonJunior94/devkit-go/pkg/observability/otel`, com a variável nomeada `o11y`, controlando seu próprio `signal.NotifyContext` no entrypoint, e que `configs.O11yConfig` adote exatamente o contrato de campos `ServiceVersion`, `ExporterEndpoint`, `ExporterProtocol`, `ExporterInsecure`, `TraceSampleRate`, `LogLevel`, `LogFormat`.

Estado atual relevante (working tree):

- `cmd/server/server.go:22` e `cmd/worker/worker.go:22` são stubs com erro de compilação (sem `return nil`, `cfg` declarado e não usado, `ctx` recebido e não usado). Nenhum bootstrap de observabilidade existe.
- `cmd/main.go:18` cria `signal.NotifyContext` e propaga via `cmd.Context()`. O `migrate` em `cmd/migrate/migrate.go:22` depende disso.
- `configs/config.go:114` define `O11yConfig` com campos `OTLPEndpoint`, `OTLPHeaders`, `TraceSampleRate`, `LogLevel`, `LogFormat`, `ServiceVersion` (mapstructure `SERVICE_VERSION`). Falta `ExporterProtocol`/`ExporterInsecure`, e `OTLPHeaders` precisa sair.
- Não existe `internal/platform/observability.NewProvider`. A “remoção do bootstrap antigo” do mandato é vacuosa nesse repo: a tarefa é introduzir o novo.
- Único consumidor atual de `observability.Observability` é `internal/platform/httpclient/client.go:24` (já tipado contra a interface devkit-go). Migração não quebra contratos a jusante.
- `devkit-go v0.4.0` (`go.mod:8`) expõe `otel.NewProvider(ctx, *Config) (*Provider, error)`; `*Provider` satisfaz `observability.Observability` com métodos `Logger()`, `Tracer()`, `Metrics()`, `Shutdown(ctx)`. Os tipos `OTLPProtocol`, `LogLevel`, `LogFormat` e helpers `observability.String/Int/Error` existem como descrito no mandato.
- Os exemplos few-shot do mandato referenciam pacotes (`pkg/database`, `pkg/outbox`, `pkg/scheduler`, módulos `internal/user`, etc.) que **não existem** neste repositório. A própria cláusula do mandato (“adaptados ao codebase atual sem copiar elementos inexistentes de outro repositorio”) autoriza eliminar todo o wiring inexistente.

Decisões confirmadas pelo usuário:

- `cmd/worker/worker.go` ganhará um novo campo de config `SERVICE_NAME_WORKER` (em `HTTPConfig`, ao lado de `SERVICE_NAME_API`) para alimentar o `ServiceName` do provider.
- `signal.NotifyContext` será removido de `cmd/main.go`; cada subcomando (server, worker, migrate) passa a criar seu próprio contexto cancelável. Migrate é atualizado para preservar cancelamento em SIGINT/SIGTERM.

## Arquivos a alterar

### 1. `configs/config.go`

- **`O11yConfig`** (linha 114): substituir struct inteira pelo contrato exigido, **na ordem do mandato**:

  ```go
  type O11yConfig struct {
      ServiceVersion   string  `mapstructure:"OTEL_SERVICE_VERSION"`
      ExporterEndpoint string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
      ExporterProtocol string  `mapstructure:"OTEL_EXPORTER_OTLP_PROTOCOL"`
      ExporterInsecure bool    `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
      TraceSampleRate  float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
      LogLevel         string  `mapstructure:"LOG_LEVEL"`
      LogFormat        string  `mapstructure:"LOG_FORMAT"`
  }
  ```

- **`HTTPConfig`** (linha 75): adicionar `ServiceNameWorker string` com mapstructure `SERVICE_NAME_WORKER`, ao lado de `ServiceNameAPI`.
- **`envKeys`** (linha 162): remover `"OTEL_EXPORTER_OTLP_HEADERS"`; renomear `"SERVICE_VERSION"` → `"OTEL_SERVICE_VERSION"`; adicionar `"OTEL_EXPORTER_OTLP_PROTOCOL"`, `"OTEL_EXPORTER_OTLP_INSECURE"`, `"SERVICE_NAME_WORKER"`.
- **Defaults** (linha 204): renomear `SetDefault("SERVICE_VERSION", "dev")` para `"OTEL_SERVICE_VERSION"`; adicionar `SetDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")`, `SetDefault("OTEL_EXPORTER_OTLP_INSECURE", true)`.
- **`validateProduction`** (linha 477): remover o bloco inteiro de validação de `OTLPHeaders` (placeholders + length ≥ 64). Esses checks deixam de existir junto com o campo.

### 2. `cmd/server/server.go`

- Mudar assinatura para `func Run() error` e ajustar `RunE` para `return Run()`.
- Corpo seguindo o padrão estrutural mandatório, adaptado ao codebase real (sem inventar wiring de módulos inexistentes):

  ```
  cfg ← configs.LoadConfig(".")
  ctx, stop ← signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer stop()

  o11yConfig := &otel.Config{
      Environment:     cfg.AppConfig.Environment,
      ServiceName:     cfg.HTTPConfig.ServiceNameAPI,
      ServiceVersion:  cfg.O11yConfig.ServiceVersion,
      TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
      OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
      Insecure:        cfg.O11yConfig.ExporterInsecure,
      LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
      OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
      LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
  }
  o11y, err := otel.NewProvider(context.Background(), o11yConfig)
  if err != nil { return fmt.Errorf("run: failed to create observability provider: %v", err) }

  o11y.Logger().Info(ctx, "server bootstrap completed", observability.String("service", cfg.HTTPConfig.ServiceNameAPI))
  <-ctx.Done()
  o11y.Logger().Info(context.Background(), "shutdown signal received, draining observability")

  shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()
  if err := o11y.Shutdown(shutdownCtx); err != nil {
      return fmt.Errorf("run: error during o11y shutdown: %w", err)
  }
  return nil
  ```

- Imports a adicionar: `context`, `fmt`, `os`, `os/signal`, `syscall`, `time`, `github.com/JailtonJunior94/devkit-go/pkg/observability`, `github.com/JailtonJunior94/devkit-go/pkg/observability/otel`.
- Zero comentários.

### 3. `cmd/worker/worker.go`

- Mesmo padrão estrutural do server, com:
  - `ServiceName: cfg.HTTPConfig.ServiceNameWorker`
  - mensagens de log com sufixo `"worker"`
  - prefixo de erro `"worker:"` em vez de `"run:"` (paridade com o exemplo do mandato)
- Imports idênticos ao de `server.go`.
- Sem `rabbitmq`, `uow`, `scheduler`, `outbox` — esses pacotes não existem no repositório atual e o mandato proíbe inventá-los. A vida do worker passa a ser apenas: bootstrap o11y, esperar sinal, shutdown.

### 4. `cmd/main.go`

- Remover `signal.NotifyContext` e o `import` de `context`/`os/signal`/`syscall`.
- `root.SetContext(ctx)` e `root.ExecuteContext(ctx)` viram `root.Execute()`.
- Cada subcomando passa a controlar seu próprio shutdown.

### 5. `cmd/migrate/migrate.go`

- `Run(ctx context.Context, writer io.Writer)` e `RunDown(...)`: criar `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` no início, substituindo o ctx vindo do cobra. Cancelamento manual de migrate continua funcionando.
- Mudança mínima — preserva todo o resto.

## Verificação

Executar a partir da raiz do repositório, na ordem:

1. `gofmt -w cmd/main.go cmd/server/server.go cmd/worker/worker.go cmd/migrate/migrate.go configs/config.go`
2. `go build ./...`
3. `go vet ./...`
4. `go test -race -count=1 ./configs/... ./cmd/...`
5. `golangci-lint run ./cmd/... ./configs/...` (se instalado no host; caso contrário, registrar como N/A nas evidências)
6. Testes de regressão em `configs/`: localizar testes existentes que asseguram defaults e validação (`configs/config_test.go` se houver) e confirmar que os renames `SERVICE_VERSION` → `OTEL_SERVICE_VERSION` e remoção de `OTLPHeaders` não quebram fixtures. Atualizar fixtures se necessário, sem expandir o escopo da Validate.
7. Smoke do bootstrap: `go run ./cmd server` por ~3s com `OTEL_EXPORTER_OTLP_INSECURE=true`, `ENVIRONMENT=local`, `SERVICE_NAME_API=mecontrola-server`, `SERVICE_NAME_WORKER=mecontrola-worker`, `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317`, validando que `o11y` inicializa e o SIGTERM enxerga o shutdown logger.

## Riscos / drifts

- **Fixtures de teste em `configs/`** podem referenciar `OTLPHeaders` ou `SERVICE_VERSION`; precisam ser atualizadas. Não foi auditado no plano — será tratado durante execução.
- **Endpoint vazio**: `otel.NewProvider` rejeita `OTLPEndpoint` vazio. Em `local` sem `.env` configurado o bootstrap falhará; comportamento intencional (fail-fast). Documentar nos defaults só se houver demanda — o mandato não pede default para endpoint.
- **Worker sem `OTLP_PROTOCOL` configurado**: default `"grpc"` cobre a maioria dos setups; produção `http/protobuf` segue configurável.
- **`cmd/migrate` ganha signal handler novo**: comportamento equivalente ao anterior, sem regressão funcional.
