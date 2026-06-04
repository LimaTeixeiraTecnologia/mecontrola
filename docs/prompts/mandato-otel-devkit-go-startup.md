# Prompt enriquecido - mandato de observabilidade com devkit-go/otel

## Prompt original

Quero usar `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` de forma mandatoria, obrigatoria e inegociavel no projeto. No startup de `cmd/server/server.go` e `cmd/worker/worker.go`, usar obrigatoriamente o padrao abaixo:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

o11yConfig := &otel.Config{
    Environment:     cfg.Environment,
    ServiceName:     cfg.HTTPConfig.ServiceName,
    ServiceVersion:  cfg.O11yConfig.ServiceVersion,
    TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
    OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
    Insecure:        cfg.O11yConfig.ExporterInsecure,
    LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
    OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
    LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
}

o11y, err := otel.NewProvider(context.Background(), o11yConfig)
if err != nil {
    return fmt.Errorf("run: failed to create observability provider: %v", err)
}
```

Tambem quero as seguintes configs:

```go
O11yConfig struct {
    ServiceVersion   string  `mapstructure:"OTEL_SERVICE_VERSION"`
    ExporterEndpoint string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
    ExporterProtocol string  `mapstructure:"OTEL_EXPORTER_OTLP_PROTOCOL"`
    ExporterInsecure bool    `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
    TraceSampleRate  float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
    LogLevel         string  `mapstructure:"LOG_LEVEL"`
    LogFormat        string  `mapstructure:"LOG_FORMAT"`
}
```

E inegociavel e mandatorio carregar a skill `go-implementation` e seus exemplos, com foco em robustez, eficiencia, production-ready, production-proof e zero comentarios.

## Ambiguidades resolvidas no enriquecimento

1. O workspace atual usa `internal/platform/observability.NewProvider` em `cmd/server/server.go` e `cmd/worker/worker.go`, entao o prompt enriquecido exige substituicao explicita pela integracao com `github.com/JailtonJunior94/devkit-go/pkg/observability/otel`.
2. O contexto raiz com `signal.NotifyContext(...)` hoje nasce em `cmd/main.go`; o prompt enriquecido deixa explicito que a mudanca deve ser aplicada diretamente nos entrypoints `cmd/server/server.go` e `cmd/worker/worker.go`, conforme sua exigencia.
3. O `configs.O11yConfig` atual ainda nao contem `ExporterProtocol` nem `ExporterInsecure`, e `ServiceVersion` usa `mapstructure:"SERVICE_VERSION"`; o prompt enriquecido torna a migracao de configuracao um requisito objetivo e verificavel.
4. O prompt enriquecido fixa a nomenclatura `o11y`, proibe comentarios adicionados e exige aderencia ao `AGENTS.md`, `agent-governance` e `go-implementation`.

## Prompt enriquecido

````text
Use o estado atual do working tree como fonte de verdade e siga obrigatoriamente `AGENTS.md` como instrucao canonica deste repositorio.

Antes de editar qualquer codigo:
1. Carregue obrigatoriamente `.agents/skills/agent-governance/SKILL.md`.
2. Carregue obrigatoriamente `.agents/skills/go-implementation/SKILL.md`.
3. Carregue obrigatoriamente os exemplos pertinentes da skill `go-implementation`, especialmente os exemplos de lifecycle/startup, observabilidade e testes aplicaveis ao fluxo real.
4. Nao carregue referencias desnecessarias fora do escopo.

Objetivo inegociavel:
- Tornar `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` a unica forma permitida de bootstrap de observabilidade no projeto.
- Remover a dependencia do bootstrap atual baseado em `internal/platform/observability.NewProvider` nos entrypoints.
- Aplicar obrigatoriamente o bootstrap de observabilidade em `cmd/server/server.go` e `cmd/worker/worker.go`.
- Usar a nomenclatura `o11y` em vez de `provider`.
- Nao adicionar comentarios no codigo.

Ponto de partida obrigatorio:
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- Ler tambem os arquivos reais de configuracao e wiring afetados antes de editar, sem inventar adapters, routers, modules ou providers inexistentes.
- Nao partir de `internal/platform/runtime`.

Contexto atual que deve ser respeitado:
- O projeto e um monolito modular Go.
- O startup raiz hoje usa `signal.NotifyContext(...)` em `cmd/main.go`, mas esta tarefa exige que `cmd/server/server.go` e `cmd/worker/worker.go` passem a controlar explicitamente seu contexto de shutdown.
- O `configs.O11yConfig` atual ainda nao possui todos os campos necessarios para o contrato desejado.
- O bootstrap atual de observabilidade em `server` e `worker` usa `internal/platform/observability.NewProvider(cfg)`, o que deve deixar de ser a abordagem de inicializacao.

Implementacao obrigatoria:
1. Ajustar `configs.O11yConfig` para refletir exatamente este contrato:

   ```go
   O11yConfig struct {
       ServiceVersion   string  `mapstructure:"OTEL_SERVICE_VERSION"`
       ExporterEndpoint string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
       ExporterProtocol string  `mapstructure:"OTEL_EXPORTER_OTLP_PROTOCOL"`
       ExporterInsecure bool    `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
       TraceSampleRate  float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
       LogLevel         string  `mapstructure:"LOG_LEVEL"`
       LogFormat        string  `mapstructure:"LOG_FORMAT"`
   }
   ```

2. Em `cmd/server/server.go` e `cmd/worker/worker.go`, inicializar observabilidade obrigatoriamente com este formato estrutural, adaptando apenas nomes de campos para o contexto real quando estritamente necessario:

   ```go
   ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
   defer stop()

   o11yConfig := &otel.Config{
       Environment:     cfg.Environment,
       ServiceName:     cfg.HTTPConfig.ServiceName,
       ServiceVersion:  cfg.O11yConfig.ServiceVersion,
       TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
       OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
       Insecure:        cfg.O11yConfig.ExporterInsecure,
       LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
       OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
       LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
   }

   o11y, err := otel.NewProvider(context.Background(), o11yConfig)
   if err != nil {
       return fmt.Errorf("run: failed to create observability provider: %v", err)
   }
   ```

3. Usar os exemplos abaixo como few-shot obrigatorio para a implementacao. Eles sao mandatórios como referencia estrutural de startup, bootstrap de observabilidade, conexao com dependencias, wiring e shutdown. Adapte-os ao codebase atual sem copiar imports, nomes de modulos, routers, jobs ou contratos inexistentes.

   Exemplo obrigatorio de `server.go`:

   ```go
   package server

   import (
      "context"
      "fmt"
      "os"
      "os/signal"
      "syscall"
      "time"

      httpserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"
      "github.com/JailtonJunior94/devkit-go/pkg/observability"
      "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

      "github.com/jailtonjunior94/financial/configs"
      "github.com/jailtonjunior94/financial/internal/budget"
      "github.com/jailtonjunior94/financial/internal/card"
      "github.com/jailtonjunior94/financial/internal/category"
      "github.com/jailtonjunior94/financial/internal/invoice"
      invoiceRepositories "github.com/jailtonjunior94/financial/internal/invoice/infrastructure/repositories"
      "github.com/jailtonjunior94/financial/internal/payment_method"
      "github.com/jailtonjunior94/financial/internal/transaction"
      "github.com/jailtonjunior94/financial/internal/user"
      "github.com/jailtonjunior94/financial/pkg/api/middlewares"
      "github.com/jailtonjunior94/financial/pkg/auth"
      "github.com/jailtonjunior94/financial/pkg/database"
      "github.com/jailtonjunior94/financial/pkg/observability/metrics"
      "github.com/jailtonjunior94/financial/pkg/outbox"
   )

   func Run() error {
      cfg, err := configs.LoadConfig(".")
      if err != nil {
          return fmt.Errorf("run: failed to load config: %v", err)
      }

      ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
      defer stop()

      o11yConfig := &otel.Config{
          Environment:     cfg.Environment,
          ServiceName:     cfg.HTTPConfig.ServiceName,
          ServiceVersion:  cfg.O11yConfig.ServiceVersion,
          TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
          OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
          Insecure:        cfg.O11yConfig.ExporterInsecure,
          LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
          OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
          LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
      }

      o11y, err := otel.NewProvider(context.Background(), o11yConfig)
      if err != nil {
          return fmt.Errorf("run: failed to create observability provider: %v", err)
      }

      dbManager, err := database.NewDatabaseManager(
          ctx,
          database.WithMetrics(true),
          database.WithDSN(cfg.DBConfig.DSN()),
          database.WithConnMaxLifetime(5*time.Minute),
          database.WithConnMaxIdleTime(2*time.Minute),
          database.WithServiceName(cfg.HTTPConfig.ServiceName),
          database.WithMaxOpenConns(cfg.DBConfig.DBMaxOpenConns),
          database.WithMaxIdleConns(cfg.DBConfig.DBMaxIdleConns),
      )
      if err != nil {
          return fmt.Errorf("run: failed to connect to database: %v", err)
      }
      o11y.Logger().Info(ctx, "database connection established with OpenTelemetry instrumentation")

      metricsMiddleware := middlewares.NewMetricsMiddleware(o11y)

      jwtAdapter := auth.NewJwtAdapter(cfg, o11y)
      userModule := user.NewUserModule(dbManager.DB(), cfg, o11y, jwtAdapter, jwtAdapter)

      invoiceFinancialMetrics := metrics.NewFinancialMetrics(o11y)
      invoiceRepository := invoiceRepositories.NewInvoiceRepository(dbManager.DB(), o11y, invoiceFinancialMetrics)
      cardModule := card.NewCardModule(dbManager.DB(), o11y, jwtAdapter, invoiceRepository)

      categoryModule, err := category.NewCategoryModule(dbManager.DB(), o11y, jwtAdapter)
      if err != nil {
          return fmt.Errorf("run: failed to create category module: %v", err)
      }
      paymentMethodModule := payment_method.NewPaymentMethodModule(dbManager.DB(), o11y)

      outboxRepository := outbox.NewRepository(dbManager.DB(), o11y)
      outboxService := outbox.NewService(outboxRepository, o11y)

      invoiceModule := invoice.NewInvoiceModule(dbManager.DB(), o11y, jwtAdapter)

      transactionModule, err := transaction.NewTransactionModule(dbManager.DB(), o11y, jwtAdapter, invoiceModule.InvoiceProviderAdapter, cardModule.CardProvider, outboxService)
      if err != nil {
          return fmt.Errorf("run: failed to create transaction module: %v", err)
      }

      budgetModule, err := budget.NewBudgetModule(dbManager.DB(), o11y, jwtAdapter, invoiceModule.InvoiceCategoryTotalProvider)
      if err != nil {
          return fmt.Errorf("run: failed to create budget module: %v", err)
      }

      srv, err := httpserver.New(
          o11y,
          httpserver.WithMetrics(),
          httpserver.WithCORS("*"),
          httpserver.WithPort(cfg.HTTPConfig.Port),
          httpserver.WithServiceName(cfg.HTTPConfig.ServiceName),
          httpserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
          httpserver.WithHealthChecks(map[string]httpserver.HealthCheckFunc{"database": dbManager.Ping}),
          httpserver.WithMiddleware(metricsMiddleware.Handler),
      )
      if err != nil {
          return fmt.Errorf("run: failed to create http server: %v", err)
      }

      srv.RegisterRouters(userModule.UserRouter)
      srv.RegisterRouters(categoryModule.CategoryRouter)
      srv.RegisterRouters(cardModule.CardRouter)
      srv.RegisterRouters(transactionModule.TransactionRouter)
      srv.RegisterRouters(paymentMethodModule.PaymentMethodRouter)
      srv.RegisterRouters(budgetModule.BudgetRouter)
      srv.RegisterRouters(invoiceModule.InvoiceRouter)

      go func() {
          <-ctx.Done()

          shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
          defer cancel()

          o11y.Logger().Info(shutdownCtx, "shutting down gracefully...")

          if err := o11y.Shutdown(shutdownCtx); err != nil {
              o11y.Logger().Error(shutdownCtx, "error during o11y shutdown", observability.Error(err))
          }

          if err := dbManager.Shutdown(shutdownCtx); err != nil {
              o11y.Logger().Error(shutdownCtx, "error during database shutdown", observability.Error(err))
          }
      }()

      return srv.Start(ctx)
   }
   ```

   Exemplo obrigatorio de `worker.go`:

   ```go
   package worker

   import (
      "context"
      "fmt"
      "os"
      "os/signal"
      "syscall"
      "time"

      "github.com/jailtonjunior94/financial/configs"
      "github.com/jailtonjunior94/financial/pkg/database"
      pkgjobs "github.com/jailtonjunior94/financial/pkg/jobs"
      "github.com/jailtonjunior94/financial/pkg/outbox"
      "github.com/jailtonjunior94/financial/pkg/scheduler"

      "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
      "github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
      "github.com/JailtonJunior94/devkit-go/pkg/observability"
      "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
   )

   func Run() error {
      cfg, err := configs.LoadConfig(".")
      if err != nil {
          return fmt.Errorf("worker: failed to load config: %v", err)
      }

      ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
      defer stop()

      o11yConfig := &otel.Config{
          Environment:     cfg.Environment,
          ServiceName:     cfg.WorkerConfig.ServiceName,
          ServiceVersion:  cfg.O11yConfig.ServiceVersion,
          TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
          OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
          Insecure:        cfg.O11yConfig.ExporterInsecure,
          LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
          LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
          OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
      }

      o11y, err := otel.NewProvider(context.Background(), o11yConfig)
      if err != nil {
          return fmt.Errorf("worker: failed to create observability provider: %v", err)
      }

      dbManager, err := database.NewDatabaseManager(
          ctx,
          database.WithMetrics(true),
          database.WithDSN(cfg.DBConfig.DSN()),
          database.WithConnMaxLifetime(5*time.Minute),
          database.WithConnMaxIdleTime(2*time.Minute),
          database.WithServiceName(cfg.WorkerConfig.ServiceName),
          database.WithMaxOpenConns(cfg.DBConfig.DBMaxOpenConns),
          database.WithMaxIdleConns(cfg.DBConfig.DBMaxIdleConns),
      )
      if err != nil {
          return fmt.Errorf("run: failed to connect to database: %v", err)
      }
      o11y.Logger().Info(ctx, "database connection established with OpenTelemetry instrumentation")

      rabbitClient, err := rabbitmq.New(
          o11y,
          rabbitmq.WithAutoReconnect(true),
          rabbitmq.WithPublisherConfirms(true),
          rabbitmq.WithCloudConnection(cfg.RabbitMQConfig.URL),
      )
      if err != nil {
          return fmt.Errorf("worker: failed to create rabbitmq client: %v", err)
      }

      if err := rabbitClient.DeclareExchange(ctx, cfg.RabbitMQConfig.Exchange, "topic", true, false, nil); err != nil {
          return fmt.Errorf("worker: failed to declare exchange: %v", err)
      }

      o11y.Logger().Info(ctx, "rabbitmq initialized",
          observability.String("exchange", cfg.RabbitMQConfig.Exchange),
          observability.String("url", cfg.RabbitMQConfig.URL),
      )

      uow, err := uow.NewUnitOfWork(dbManager.DB())
      if err != nil {
          return fmt.Errorf("worker: failed to create unit of work: %v", err)
      }
      outboxDispatcher := outbox.NewDispatcher(dbManager.DB(), uow, rabbitClient, outbox.DefaultDispatcherConfig(cfg.RabbitMQConfig.Exchange), o11y)
      outboxCleanup := outbox.NewCleaner(uow, outbox.DefaultCleanupConfig(), o11y)

      jobsToRegister := []pkgjobs.Job{
          outbox.NewDispatcherJob(outboxDispatcher, "@every 5s", o11y),
          outbox.NewCleanupJob(outboxCleanup, "@daily", o11y),
      }

      scheduler := scheduler.New(ctx, o11y, pkgjobs.DefaultConfig())

      for _, job := range jobsToRegister {
          if err := scheduler.Register(job); err != nil {
              return fmt.Errorf("worker: failed to register job %s: %v", job.Name(), err)
          }
      }

      scheduler.Start()

      o11y.Logger().Info(ctx, "worker started successfully", observability.Int("jobs_registered", len(jobsToRegister)))

      <-ctx.Done()

      o11y.Logger().Info(context.Background(), "shutdown signal received, initiating graceful shutdown...")

      shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
      defer cancel()

      if err := scheduler.Shutdown(shutdownCtx); err != nil {
          o11y.Logger().Error(context.Background(), "error during scheduler shutdown", observability.Error(err))
      }

      if err := rabbitClient.Shutdown(shutdownCtx); err != nil {
          o11y.Logger().Error(context.Background(), "error during rabbitmq shutdown", observability.Error(err))
      }

      if err := dbManager.Shutdown(shutdownCtx); err != nil {
          o11y.Logger().Error(context.Background(), "error during database shutdown", observability.Error(err))
      }

      if err := o11y.Shutdown(shutdownCtx); err != nil {
          o11y.Logger().Error(context.Background(), "error during o11y shutdown", observability.Error(err))
      }

      o11y.Logger().Info(context.Background(), "worker shutdown completed")
      return nil
   }
   ```

4. Garantir que `server` e `worker` usem o provider `o11y` no restante do wiring real, sem criar abstrações artificiais e sem manter caminhos antigos de bootstrap competindo com o novo.
5. Preservar shutdown coordenado, propagacao de contexto, `errors.Join` quando apropriado e compatibilidade com os componentes reais que consomem observabilidade no bootstrap.
6. Nao usar `panic`, `init()`, globais novas, `interface{}`, `_ = dependencia` nem fallback silencioso.
7. Nao inventar APIs. Se algum consumidor atual exigir contrato diferente do oferecido por `otel.NewProvider`, adapte de forma minima, segura e baseada no codigo existente.
8. Manter foco em robustez, eficiencia, production-ready e production-proof como requisitos obrigatorios, nao como aspiracao.

Restricoes obrigatorias:
- Zero comentarios adicionados.
- Nao reverter mudancas do usuario.
- Nao criar markdown de planejamento.
- Nao mudar comportamento publico alem do necessario para cumprir a migracao de observabilidade.
- Seguir DI e fronteiras existentes do repositorio.
- Se houver divergencia entre prompt historico e codigo real, prevalece o working tree atual com a opcao mais segura.

Criterios de aceitacao:
1. `cmd/server/server.go` inicia observabilidade via `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` com variavel `o11y`.
2. `cmd/worker/worker.go` inicia observabilidade via `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` com variavel `o11y`.
3. `configs.O11yConfig` contem exatamente os campos `ServiceVersion`, `ExporterEndpoint`, `ExporterProtocol`, `ExporterInsecure`, `TraceSampleRate`, `LogLevel` e `LogFormat` com os `mapstructure` exigidos.
4. O bootstrap antigo via `internal/platform/observability.NewProvider` deixa de ser o caminho de inicializacao em `server` e `worker`.
5. O startup continua com shutdown gracioso, tratamento de erro explicito e sem comentarios adicionados.
6. A implementacao carrega `agent-governance`, `go-implementation` e os exemplos relevantes antes de editar.
7. Os entrypoints seguem obrigatoriamente os exemplos fornecidos de `server.go` e `worker.go`, adaptados ao codebase atual sem copiar elementos inexistentes de outro repositorio.
8. A validacao final executa, no minimo, `gofmt -w` nos arquivos alterados, `go build`, `go vet` e `go test -race -count=1` no escopo alterado, alem de `golangci-lint run` quando disponivel no repositorio.

Formato da resposta esperado:
- Responder em portugues do Brasil.
- Informar objetivamente quais arquivos foram alterados.
- Explicar qualquer drift encontrado entre o requisito e o codigo atual.
- Nao incluir plano; executar a mudanca.
- Nao propor alternativas: implementar exatamente o mandato acima.
````

## Justificativa das adicoes

| Adicao | Motivo |
| --- | --- |
| Ponto de partida obrigatorio em `cmd/server/server.go` e `cmd/worker/worker.go` | Alinha o prompt com a regra do repositorio e com sua exigencia explicita de startup. |
| Referencia ao working tree atual | Evita alucinacao e impede implementacao baseada em estado historico ou exemplos cegos. |
| Contraste entre bootstrap atual e bootstrap desejado | Torna o objetivo mensuravel e reduz risco de uma resposta parcial. |
| Few-shot obrigatorio com `server.go` e `worker.go` | Obriga o agente executor a seguir exatamente o padrao de startup, wiring e shutdown exigido, adaptando apenas o necessario ao repositorio atual. |
| Criterios de aceitacao objetivos | Permite validar se a migracao ficou completa, production-ready e sem loopholes. |
| Restricao de zero comentarios e uso de `o11y` | Preserva suas preferencias explicitas de estilo e nomenclatura. |

## Variante enxuta

Use a secao `Prompt enriquecido` isoladamente quando quiser colar o prompt direto em outro agente sem levar contexto adicional.
