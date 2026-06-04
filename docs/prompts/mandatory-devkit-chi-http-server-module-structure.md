# Prompt original

Quero implementar de forma mandatória e inegociável o uso de `httpserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"` em `cmd/server/server.go`, no espírito deste exemplo:

```go
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
```

Tambem quero que cada modulo que exponha rotas HTTP use a seguinte organizacao e wiring:
- `http/client`
- `http/server`
- `routes` na raiz da pasta HTTP
- `handlers/user_handler.go` como exemplo de naming

Quero que o registro de rotas siga este formato:

```go
package http

import (
    "github.com/go-chi/chi/v5"

    "github.com/jailtonjunior94/financial/pkg/api/middlewares"
)

type InvoiceRouter struct {
    handlers       *InvoiceHandler
    authMiddleware middlewares.Authorization
}

func NewInvoiceRouter(handlers *InvoiceHandler, authMiddleware middlewares.Authorization) *InvoiceRouter {
    return &InvoiceRouter{handlers: handlers, authMiddleware: authMiddleware}
}

func (r InvoiceRouter) Register(router chi.Router) {
    router.Group(func(protected chi.Router) {
        protected.Use(r.authMiddleware.Authorization)
        protected.Get("/api/v1/cards/{cardId}/invoices", r.handlers.ListByCard)
        protected.Get("/api/v1/cards/{cardId}/invoices/{invoiceId}", r.handlers.GetByCard)
    })
}
```

E no `module.go` de cada modulo, quero wiring no espírito deste exemplo:

```go
type InvoiceModule struct {
    InvoiceRouter                *http.InvoiceRouter
    InvoiceTotalProvider         pkginterfaces.InvoiceTotalProvider
    InvoiceCategoryTotalProvider pkginterfaces.InvoiceCategoryTotalProvider
    InvoiceProviderAdapter       *adapters.InvoiceProviderAdapter
}

func NewInvoiceModule(
    db database.DBTX,
    o11y observability.Observability,
    tokenValidator auth.TokenValidator,
) InvoiceModule {
    errorHandler := httperrors.NewErrorHandler(o11y, ErrorMappings())
    authMiddleware := middlewares.NewAuthorization(tokenValidator, o11y, errorHandler)

    financialMetrics := metrics.NewFinancialMetrics(o11y)
    invoiceRepository := repositories.NewInvoiceRepository(db, o11y, financialMetrics)

    getInvoiceUseCase := usecase.NewGetInvoiceUseCase(invoiceRepository, o11y)
    listInvoicesByCardPaginatedUseCase := usecase.NewListInvoicesByCardPaginatedUseCase(invoiceRepository, o11y)

    invoiceHandler := http.NewInvoiceHandler(
        o11y,
        errorHandler,
        listInvoicesByCardPaginatedUseCase,
        getInvoiceUseCase,
    )

    invoiceRouter := http.NewInvoiceRouter(invoiceHandler, authMiddleware)

    invoiceTotalProvider := adapters.NewInvoiceTotalProviderAdapter(invoiceRepository)
    invoiceCategoryTotalProvider := adapters.NewInvoiceCategoryTotalAdapter(invoiceRepository)
    invoiceProviderAdapter := adapters.NewInvoiceProviderAdapter(invoiceRepository, o11y)

    return InvoiceModule{
        InvoiceRouter:                invoiceRouter,
        InvoiceTotalProvider:         invoiceTotalProvider,
        InvoiceCategoryTotalProvider: invoiceCategoryTotalProvider,
        InvoiceProviderAdapter:       invoiceProviderAdapter,
    }
}
```

Uso obrigatório, mandatório e inegociável da skill `go-implementation`, de seus exemplos e dos exemplos deste próprio prompt como base normativa de implementação. Quero solução robusta, eficiente, com 0 comentários no código de forma mandatória e inegociável, em nível production-ready, production-proof, sem falso positivo. Não implemente nada fora do prompt.

# Contexto validado e ambiguidades reais

- `go.mod` declara Go `1.26.2` e `github.com/JailtonJunior94/devkit-go v0.4.0`.
- `cmd/server/server.go` ja usa explicitamente `chi_server`: cria `server, err := chiserver.New(...)`, registra routers com `server.RegisterRouters(...)`, sobe workers auxiliares com `internal/platform/worker.NewManager(...)` e inicia o servidor com `server.Start(ctx)`.
- `cmd/worker/worker.go` hoje tambem inicializa config, observabilidade e database manager, sobe os runners via `internal/platform/worker.NewManager(...)`.
- `internal/platform/worker/manager.go` hoje expõe `NewManager(cfg Config, jobs []Job, consumers []Consumer, logger *slog.Logger)`, com `Job` e `Consumer` definidos em `internal/platform/worker/types.go`.
- `AGENTS.md` e `CLAUDE.md` exigem, para `internal/identity` e `internal/billing`, o "Padrao Obrigatorio de Modulo" e proíbem `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` e `Runners()` como novo padrão de composição.
- `internal/billing/module.go` e `internal/identity/module.go` visíveis hoje nao fornecem wiring verificável além da declaração de package; portanto, qualquer exemplo de `module.go` deve ser tratado como alvo arquitetural e nao como espelho fiel do estado atual implementado.
- O estado atual do workspace deve ser tratado como fonte da verdade para qualquer refinamento, sem recorrer a historico, paths antigos ou suposicoes fora do que estiver visivel agora.
- O exemplo original cita módulos inexistentes neste repositório (`user`, `category`, `card`, `transaction`, `paymentMethod`, `budget`, `invoice`). No estado real atual, o prompt não pode exigir routers ou handlers para módulos que ainda não expõem HTTP.
- Há tensão entre o pedido por `http/client`, `http/server`, `routes` e `handlers/...` e a governança canônica do repositório, que exige preservar `internal/<modulo>/infrastructure/http/server` e `internal/<modulo>/infrastructure/http/client`.
- O pedido exige `0 comentarios`, então o prompt final precisa proibir comentários novos em código Go produzido, mesmo quando existirem comentários legados no codebase atual.

# Prompt refinado

```text
Quero que voce evolua, de forma obrigatoria, mandatoria e inegociavel, o bootstrap HTTP deste repositorio Go a partir do estado real atual, no qual `cmd/server/server.go` ja usa explicitamente `chi_server`. O ponto de partida obrigatorio deve continuar sendo `cmd/server/server.go` e/ou `cmd/worker/worker.go`, sem criar camadas intermediarias de bootstrap fora desses entrypoints. A solucao final precisa ser robusta, eficiente, escalavel, production-ready, production-proof e sem falso positivo.

Tambem e obrigatorio, mandatorio e inegociavel:
1. usar a skill `go-implementation`, seus exemplos e os exemplos deste proprio prompt como base normativa de implementacao;
2. adaptar todo exemplo ao estado real do repositorio, nunca copiar literalmente snippet de outro contexto;
3. entregar codigo Go final com `0 comentarios`, sem comentarios de linha, bloco, doc comments ou observacoes inline novas.

Antes de qualquer alteracao, carregue obrigatoriamente:
1. `AGENTS.md`
2. `.github/skills/agent-governance/SKILL.md`
3. `.github/skills/go-implementation/SKILL.md`
4. As referencias da skill Go realmente pertinentes para esta tarefa:
   - `.github/skills/go-implementation/references/architecture.md`
   - `.github/skills/go-implementation/references/interfaces.md`
   - `.github/skills/go-implementation/references/api.md`
   - `.github/skills/go-implementation/references/observability.md`
   - `.github/skills/go-implementation/references/configuration.md`
   - `.github/skills/go-implementation/references/graceful-lifecycle.md`
   - `.github/skills/go-implementation/references/security.md`
   - `.github/skills/go-implementation/references/testing.md`
   - `.github/skills/go-implementation/references/examples-infrastructure.md`
   - `.github/skills/go-implementation/references/examples-domain-flow.md`
5. `go.mod`
6. `cmd/server/server.go`
7. `cmd/worker/worker.go`, porque o ponto de partida obrigatorio do bootstrap deve ser sempre `cmd/server/server.go` e/ou `cmd/worker/worker.go`
8. Os imports e dependencias reais de `cmd/server/server.go` e `cmd/worker/worker.go`
9. Os paths de modulo e HTTP que estiverem efetivamente acessiveis no workspace atual

Contexto real que deve orientar a implementacao:
- `go.mod` declara Go `1.26.2` e `github.com/JailtonJunior94/devkit-go v0.4.0`.
- `cmd/server/server.go` ja concentra explicitamente o bootstrap HTTP: cria o servidor via `chiserver.New(...)`, registra routers e chama `server.Start(ctx)`.
- `cmd/worker/worker.go` ja concentra explicitamente o bootstrap de processamento em background via `platformworker.NewManager(...)`.
- O estado atual do workspace e a unica fonte de verdade para bootstrap, imports, wiring e estrutura de modulo.
- Portanto, qualquer evolucao do bootstrap ou do wiring deve partir exclusivamente do que estiver visivel agora em `cmd/server/server.go`, `cmd/worker/worker.go` e nos packages acessiveis a partir deles.
- O projeto segue orientacao modular em Go e precisa preservar fronteiras arquiteturais.
- A fronteira obrigatoria para HTTP continua sendo:
  - inbound: `internal/<modulo>/infrastructure/http/server`
  - outbound: `internal/<modulo>/infrastructure/http/client`
- Toda chamada HTTP outbound visivel no estado atual deve continuar usando `internal/platform/httpclient`.
- Nunca logar PII, segredos, payloads sensiveis, dados financeiros, email, telefone, identificadores pessoais ou credenciais em claro.

Ambiguidades e conflitos que voce deve resolver com seguranca antes de codar:
- O pedido quer bootstrap obrigatorio a partir dos entrypoints. Como `cmd/server/server.go` ja usa `chi_server` explicitamente, a tarefa nao e reintroduzir esse uso, e sim preservar e evoluir esse bootstrap sem regressao nem duplicidade.
- O exemplo original vem de outro contexto e cita routers/handlers/modulos inexistentes. Use o snippet apenas como referencia de composicao e naming, nunca como copia literal.
- O pedido fala em `routes` e `handlers/...`, mas `AGENTS.md` exige preservar `internal/<modulo>/infrastructure/http/server` e `internal/<modulo>/infrastructure/http/client`. Resolva essa tensao mantendo toda organizacao dentro de `infrastructure/http/...`.
- O estado visível hoje contém um drift entre o bootstrap dos entrypoints e o padrão obrigatório de módulo definido em `AGENTS.md`/`CLAUDE.md`: o prompt deve explicitar essa divergência em vez de cristalizar o padrão antigo como alvo desejado.
- O estado visível hoje também contém um drift entre o uso de `platformworker.NewManager(...)` nos entrypoints e a assinatura real exposta por `internal/platform/worker/manager.go`; o prompt deve exigir aderência à API real visível, sem inventar overloads ou helpers intermediários.
- Assuma o estado atual como fonte da verdade. Nao invente `billing`, `identity`, `events`, `platform/worker` ou qualquer outro package fora do que estiver efetivamente representado no workspace atual.
- O pedido exige `0 comentarios`; portanto, nao adicione comentarios novos em arquivos Go e nao use comentarios para justificar a implementacao.

Objetivo principal:
1. Preservar `cmd/server/server.go` como ponto de partida obrigatorio do bootstrap HTTP explicito com `chi_server`, evoluindo o desenho sem reintroduzir bootstrap paralelo ou indireto.
2. Registrar apenas routers reais via `srv.RegisterRouters(...)`, adaptados aos modulos que de fato expõem HTTP no repositório.
3. Tratar `cmd/worker/worker.go` como ponto de partida obrigatorio para runners, schedulers, consumers e processamento assíncrono.
4. Alinhar o desenho alvo dos modulos `billing` e `identity` ao "Padrao Obrigatorio de Modulo" de `AGENTS.md` e `CLAUDE.md`, sem perpetuar `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como padrão novo.
5. Garantir que qualquer uso de `internal/platform/worker.NewManager(...)` siga a assinatura real visível no codebase atual: `Config`, `[]worker.Job`, `[]worker.Consumer` e `logger`.
6. Preservar HTTP outbound em `internal/<modulo>/infrastructure/http/client` usando `internal/platform/httpclient`.
7. Garantir graceful shutdown robusto, com contexto derivado, timeout explicito e desligamento ordenado de servidor HTTP, workers, observabilidade e banco.

Diretrizes de desenho obrigatorias:
1. Preserve a arquitetura do repositorio e a DI manual por construtores; nao use framework de DI.
2. Preserve a fronteira canônica `internal/<modulo>/infrastructure/http/server` para HTTP inbound e `internal/<modulo>/infrastructure/http/client` para HTTP outbound.
3. Se houver necessidade de organizar `handlers` e `routes`, faca isso dentro da fronteira HTTP do modulo, sem mover responsabilidade para fora de `infrastructure/http`.
4. `module.go` deve ser o ponto de composicao do modulo quando esse arquivo ou responsabilidade estiver visivel no workspace atual: criar repositories, use cases, middleware, handlers HTTP e router do modulo.
5. O router do modulo deve expor `Register(router chi.Router)` e ser registravel pelo servidor central via `srv.RegisterRouters(...)`.
6. Toda rota protegida deve aplicar middleware de autorizacao no router do modulo, nunca no handler.
7. O bootstrap de `cmd/server/server.go` e, quando aplicavel, de `cmd/worker/worker.go`, deve ser o ponto de partida obrigatorio da composicao. O servidor HTTP deve ser inicializado com options coerentes com a API real do pacote `chi_server`, incluindo health checks, metrics, CORS, porta, service name e service version somente quando esses dados existirem no codebase real.
8. Se algum detalhe do snippet divergir da API real de `devkit-go` ou dos nomes reais de config, adapte ao estado verdadeiro do codebase. Nao invente API, campos de config ou wrappers desnecessarios.
9. Se algum modulo nao tiver rotas HTTP reais no estado atual, ele nao deve receber router vazio, estrutura artificial ou pasta `http/server` criada apenas por simetria.
10. Para `internal/identity` e `internal/billing`, siga o "Padrao Obrigatorio de Modulo" de `AGENTS.md`: construtor nomeado pelo modulo, struct concreta, campos nomeados para routers/providers/adapters/jobs/consumers reais e entrega de jobs/consumers ao WorkerManager como `worker.Job` e `worker.Consumer`.
11. Nao use `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como padrao alvo novo de composicao de modulo.
12. Se algum modulo ja constroi handler e router no `module.go`, prefira composicao direta e previsivel. Evite registrars vazios, wiring lazy e ponteiros atomicos quando nao forem estritamente necessarios.
13. E obrigatorio usar os exemplos da skill `go-implementation` e os exemplos contidos neste prompt como base concreta de implementacao, adaptando-os ao contexto real do repositorio sem copia cega.
14. E mandatorio e inegociavel manter `0 comentarios` em qualquer codigo Go novo ou alterado.

Estrutura obrigatoria para modulos com HTTP:
1. HTTP inbound:
   - `internal/<modulo>/infrastructure/http/server/...`
   - handlers com naming claro, por exemplo `kiwify_webhook_handler.go` ou `handlers/<entidade>_handler.go`
   - router/registrar com metodo `Register(router chi.Router)`
2. HTTP outbound:
   - `internal/<modulo>/infrastructure/http/client/...`
   - uso obrigatorio de `internal/platform/httpclient`
3. Wiring do modulo:
   - `internal/<modulo>/module.go` deve expor o router real do modulo e demais ports/adapters necessarios, quando esse modulo fizer parte do estado atual visivel
   - o desenho desejado e o do snippet de `InvoiceModule`, adaptado ao contexto real dos modulos efetivamente presentes no workspace atual

Arquivos e areas minimas que devem ser inspecionadas antes de editar:
- `go.mod`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- os imports reais de `cmd/server/server.go` e `cmd/worker/worker.go`
- os paths reais de modulo, HTTP inbound, HTTP outbound, worker e events que estiverem acessiveis no workspace atual
- usos atuais de `github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server`
- `configs/...` e os pacotes reais de observabilidade/database usados no bootstrap

Requisitos funcionais:
1. `cmd/server/server.go` deve continuar como ponto de bootstrap HTTP explicito usando a API real do pacote `github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server`.
2. `cmd/worker/worker.go` deve continuar como ponto de bootstrap explicito de runners, schedulers e consumers.
3. O servidor central deve registrar os routers reais dos modulos via `srv.RegisterRouters(...)` somente quando esses modulos fizerem parte do estado atual visivel.
4. Cada modulo que exponha rotas HTTP deve ter wiring explicito em `module.go`, no espirito do "Padrao Obrigatorio de Modulo" de `AGENTS.md`, mas adaptado ao contexto real do modulo.
5. Cada router de modulo deve encapsular middlewares, handlers e paths do proprio modulo.
6. O bootstrap de jobs e consumers deve usar a assinatura real de `internal/platform/worker.NewManager(...)`: config, jobs, consumers e logger.
7. O shutdown deve derivar contexto com timeout a partir de `ctx`, encerrar servidor HTTP, workers, observabilidade e banco de forma ordenada e sem falso positivo de sucesso.
8. A implementacao deve se manter fiel ao estado atual visivel do workspace, sem introduzir comportamento baseado em historico presumido ou estrutura antiga.

Requisitos nao funcionais obrigatorios:
1. Production-ready de verdade: nada de bootstrap parcial, alias cosmetico, placeholder ou migracao pela metade.
2. Sem falso positivo: se `cmd/server/server.go` nao continuar sendo o bootstrap HTTP explicito real, a tarefa deve ser considerada incompleta.
3. Sem concorrencia de servidores, runners ou bootstraps duplicados para a mesma responsabilidade.
4. Logging, tracing, health checks e shutdown devem permanecer coerentes com observabilidade, database manager e WorkerManager reais do repositorio.
5. Sem vazamento de PII, segredos, payloads sensiveis, CPF, dados de cartao, email ou WhatsApp em claro.
6. A solucao final deve reduzir complexidade incidental quando possivel, em vez de adicionar wrappers apenas para aparentar padronizacao.
7. E mandatorio e inegociavel manter `0 comentarios` em qualquer codigo Go novo ou alterado.

Proibicoes explicitas:
- Nao copiar literalmente o snippet quando ele conflitar com o repositorio.
- Nao inventar modulos inexistentes nem substituir o estado atual do workspace por estruturas presumidas de historico ou placeholders locais.
- Nao criar wrappers redundantes sobre `chi_server` apenas para aparentar compliance.
- Nao deixar qualquer bootstrap HTTP legado ativo em paralelo se isso significar dois bootstraps HTTP concorrentes.
- Nao mover HTTP outbound para fora de `internal/<modulo>/infrastructure/http/client`.
- Nao registrar rota diretamente no `cmd/server/server.go` quando ela pertence ao modulo.
- Nao criar router vazio para qualquer modulo sem HTTP real.
- Nao manter wiring lazy no router se o handler puder ser construido e injetado de forma direta no `module.go`.
- Nao quebrar as fronteiras `handler -> usecase -> repositories e/ou client http`.
- Nao logar PII em claro.
- Nao adicionar comentarios em hipotese nenhuma.

Criterios de aceitacao:
1. `cmd/server/server.go` continua usando explicitamente `chi_server` com options aderentes ao codebase real.
2. `cmd/worker/worker.go` continua sendo o ponto de bootstrap dos runners reais.
3. Os routers dos modulos reais sao registrados pelo servidor central via `srv.RegisterRouters(...)` somente quando esses modulos fizerem parte do estado atual visivel.
4. O wiring de cada modulo HTTP segue composicao explicita em `module.go`, sem copia cega do snippet de outro dominio.
5. A estrutura HTTP final respeita `internal/<modulo>/infrastructure/http/server` e `internal/<modulo>/infrastructure/http/client`.
6. Nao permanece bootstrap duplicado, concorrente ou redundante entre server e worker.
7. Graceful shutdown de servidor HTTP, workers, observabilidade e database manager continua correto, com contexto derivado e timeout explicito.
8. O uso de `internal/platform/httpclient` permanece obrigatorio para HTTP outbound quando isso fizer parte do estado atual visivel.
9. PII, segredos e payloads sensiveis continuam protegidos no desenho final.
10. Nenhum arquivo Go novo ou alterado recebe comentarios, sem excecao.
11. A resposta final explica quais arquivos foram alterados e como os entrypoints permaneceram obrigatorios, sempre tomando o estado atual do workspace como fonte da verdade.

Saida esperada:
1. Analise curta das ambiguidades entre o snippet desejado e o estado real do repositorio antes de codar.
2. Implementacao completa, sem workaround cosmetico.
3. Ajustes proporcionais em testes e wiring para cobrir bootstrap, registro de routers e shutdown.
4. Resumo final objetivo em PT-BR com foco em `cmd/server/server.go`, routers modulares reais, estrutura HTTP e graceful shutdown.

Se houver conflito entre o snippet fornecido, `AGENTS.md`, `agent-governance`, `go-implementation` e o estado real do repositorio, prevalecem `AGENTS.md`, `go-implementation` e a restricao mais segura.
```

# Exemplo concreto do que sera implementado

O objetivo nao e copiar literalmente os snippets abaixo, e sim deixar claro o alvo arquitetural esperado no estado atual do repositorio.

## Exemplo alvo para `cmd/server/server.go`

```go
identityModule, err := identity.NewIdentityModule(identity.ModuleDeps{
    DB: mgr,
})
if err != nil {
    return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
}

billingModule, err := billing.NewBillingModule(billing.ModuleDeps{
    Config:         cfg,
    DB:             mgr,
    Logger:         logger,
    Provider:       provider,
    UserRepository: identityModule.UserRepository,
})
if err != nil {
    return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
}

runnerManager := platformworker.NewManager(
    platformworker.Config{ShutdownTimeout: 30 * time.Second},
    billingModule.Jobs,
    billingModule.Consumers,
    logger,
)
if err := runnerManager.Start(ctx); err != nil {
    return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
}

server, err := chiserver.New(
    provider.Observability(),
    chiserver.WithPort(strconv.Itoa(cfg.HTTPConfig.Port)),
    chiserver.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
    chiserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
    chiserver.WithEnvironment(cfg.AppConfig.Environment),
    chiserver.WithCORS(cfg.HTTPConfig.CORSAllowedOrigins),
    chiserver.WithMetrics(),
    chiserver.WithTracing(),
    chiserver.WithOTelMetrics(),
)
if err != nil {
    return errors.Join(
        err,
        runnerManager.Stop(context.Background()),
        provider.Shutdown(context.Background()),
        mgr.Shutdown(context.Background()),
    )
}

server.RegisterRouters(append(identityModule.Routers, billingModule.Routers...)...)

if err := server.Start(ctx); err != nil {
    return errors.Join(
        err,
        runnerManager.Stop(context.Background()),
        provider.Shutdown(context.Background()),
        mgr.Shutdown(context.Background()),
    )
}

return errors.Join(
    runnerManager.Stop(context.Background()),
    provider.Shutdown(context.Background()),
    mgr.Shutdown(context.Background()),
)
```

O ponto inegociavel do exemplo acima e:
- `cmd/server/server.go` cria o servidor HTTP com `chiserver.New(...)`
- `server.RegisterRouters(...)` registra os routers reais
- o lifecycle final nasce do proprio entrypoint, incluindo `platformworker.NewManager(...)` com `Config`, `Jobs`, `Consumers` e `server.Start(ctx)`

## Exemplo alvo de wiring de modulo HTTP, alinhado ao "Padrao Obrigatorio de Modulo"

```go
type BillingModule struct {
    Routers   []chiserver.Router
    Jobs      []worker.Job
    Consumers []worker.Consumer
}

func NewBillingModule(deps ModuleDeps) (BillingModule, error) {
    webhookRepo := billingrepos.NewPgxWebhookEventRepository(deps.DB)
    subscriptionRepo := billingrepos.NewPgxSubscriptionRepository(deps.DB)

    adapter, err := newWiring(deps).buildKiwifyAdapter(context.Background(), subscriptionRepo)
    if err != nil {
        return BillingModule{}, err
    }

    ingestUseCase, err := newWiring(deps).buildWebhookUseCase(
        adapter,
        webhookRepo,
    )
    if err != nil {
        return BillingModule{}, err
    }

    kiwifyHandler := billinghttp.NewKiwifyWebhookHandler(
        ingestUseCase,
        deps.Logger,
        deps.Config.KiwifyConfig.WebhookTokenHeader,
    )

    kiwifyRouter := billinghttp.NewKiwifyRouteRegistrar(kiwifyHandler)

    return BillingModule{
        Routers:   []chiserver.Router{kiwifyRouter},
        Jobs:      nil,
        Consumers: nil,
    }, nil
}
```

O alvo aqui e tornar o `module.go` explicitamente responsavel pelo wiring do handler, router, jobs e consumers reais do modulo, sem perpetuar `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como padrao novo.

## Exemplo alvo para a estrutura de um modulo com HTTP, se esse modulo existir no workspace atual

```text
internal/billing/
  module.go
  infrastructure/
    http/
      client/
        kiwify/
      server/
        kiwify_webhook_handler.go
        route_registrar.go
```

Se novos modulos passarem a expor HTTP no futuro, o mesmo padrao vale dentro de `infrastructure/http/server`, sem criar estruturas artificiais em modulos que ainda nao tenham rotas.

# Melhorias aplicadas

- Amarrei o prompt ao estado real do repositorio: `cmd/server/server.go` e `cmd/worker/worker.go` sao os pontos de partida obrigatorios do bootstrap visivel hoje.
- Explicitei o principal risco de falso positivo: introduzir novo bootstrap HTTP no entrypoint sem respeitar o bootstrap explicito que ja existe no estado atual.
- Tornei o prompt mais robusto contra invencao de contexto: ele agora proibe routers vazios, modulos artificiais, wrappers cosmeticos e qualquer camada intermediaria desnecessaria entre o entrypoint e o servidor HTTP.
- Transformei “robusto”, “eficiente”, “escalavel”, “production-ready”, “production-proof” e “sem falso positivo” em exigencias verificaveis de composicao, shutdown, registro de routers reais e ausencia de bootstrap duplicado.
- Adicionei exemplos concretos do alvo para `cmd/server/server.go`, para `cmd/worker/worker.go` e para wiring de modulo HTTP quando esse wiring estiver visivel no workspace atual.
- Mantive a exigencia de `0 comentarios` em codigo Go novo ou alterado e reforcei a protecao contra vazamento de PII e segredos no desenho final.

# Exemplo de codigo real para analisar a proposta

Os exemplos abaixo sao uma proposta concreta de implementacao baseada no estado atual do repositorio. Eles existem para analise humana do desenho final esperado e nao devem ser copiados cegamente sem validar a API real de `chi_server`.

## Exemplo proposto de bootstrap em `cmd/server/server.go`

```go
func Run(ctx context.Context) error {
    cfg, err := configs.LoadConfig(".")
    if err != nil {
        return err
    }

    logger := slog.Default()

    provider, _, err := observability.NewProvider(cfg)
    if err != nil {
        return err
    }

    mgr, err := database.NewManager(ctx, cfg, provider.Observability())
    if err != nil {
        shutdownErr := provider.Shutdown(context.Background())
        if shutdownErr != nil {
            return errors.Join(err, shutdownErr)
        }
        return err
    }

    identityModule, err := identity.NewIdentityModule(identity.ModuleDeps{
        DB: mgr,
    })
    if err != nil {
        return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
    }

    billingModule, err := billing.NewBillingModule(billing.ModuleDeps{
        Config:         cfg,
        DB:             mgr,
        Logger:         logger,
        Provider:       provider,
        UserRepository: identityModule.UserRepository,
    })
    if err != nil {
        return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
    }

    runnerManager := platformworker.NewManager(
        platformworker.Config{ShutdownTimeout: 30 * time.Second},
        billingModule.Jobs,
        billingModule.Consumers,
        logger,
    )
    if err := runnerManager.Start(ctx); err != nil {
        return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
    }

    server, err := chiserver.New(
        provider.Observability(),
        chiserver.WithPort(strconv.Itoa(cfg.HTTPConfig.Port)),
        chiserver.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
        chiserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
        chiserver.WithEnvironment(cfg.AppConfig.Environment),
        chiserver.WithCORS(cfg.HTTPConfig.CORSAllowedOrigins),
        chiserver.WithMetrics(),
        chiserver.WithTracing(),
        chiserver.WithOTelMetrics(),
    )
    if err != nil {
        return errors.Join(
            err,
            runnerManager.Stop(context.Background()),
            provider.Shutdown(context.Background()),
            mgr.Shutdown(context.Background()),
        )
    }

    server.RegisterRouters(append(identityModule.Routers, billingModule.Routers...)...)

    if err := server.Start(ctx); err != nil {
        return errors.Join(
            err,
            runnerManager.Stop(context.Background()),
            provider.Shutdown(context.Background()),
            mgr.Shutdown(context.Background()),
        )
    }

    return errors.Join(
        runnerManager.Stop(context.Background()),
        provider.Shutdown(context.Background()),
        mgr.Shutdown(context.Background()),
    )
}
```

## Exemplo proposto de wiring de modulo HTTP, alinhado ao "Padrao Obrigatorio de Modulo"

```go
type BillingModule struct {
    Routers   []chiserver.Router
    Jobs      []worker.Job
    Consumers []worker.Consumer
}

func NewBillingModule(deps ModuleDeps) (BillingModule, error) {
    webhookRepo := billingrepos.NewPgxWebhookEventRepository(deps.DB)
    subscriptionRepo := billingrepos.NewPgxSubscriptionRepository(deps.DB)

    adapter, err := newWiring(deps).buildKiwifyAdapter(context.Background(), subscriptionRepo)
    if err != nil {
        return BillingModule{}, err
    }

    ingestUseCase, err := newWiring(deps).buildWebhookUseCase(
        adapter,
        webhookRepo,
    )
    if err != nil {
        return BillingModule{}, err
    }

    kiwifyHandler := billinghttp.NewKiwifyWebhookHandler(
        ingestUseCase,
        deps.Logger,
        deps.Config.KiwifyConfig.WebhookTokenHeader,
    )

    kiwifyRouter := billinghttp.NewKiwifyRouteRegistrar(kiwifyHandler)

    return BillingModule{
        Routers:   []chiserver.Router{kiwifyRouter},
        Jobs:      nil,
        Consumers: nil,
    }, nil
}
```

## Exemplo proposto de bootstrap em `cmd/worker/worker.go`

```go
func Run(ctx context.Context) error {
    cfg, err := configs.LoadConfig(".")
    if err != nil {
        return err
    }

    logger := slog.Default()

    provider, _, err := observability.NewProvider(cfg)
    if err != nil {
        return err
    }

    mgr, err := database.NewManager(ctx, cfg, provider.Observability())
    if err != nil {
        shutdownErr := provider.Shutdown(context.Background())
        if shutdownErr != nil {
            return errors.Join(err, shutdownErr)
        }
        return err
    }

    identityModule, err := identity.NewIdentityModule(identity.ModuleDeps{
        DB: mgr,
    })
    if err != nil {
        return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
    }

    billingModule, err := billing.NewBillingModule(billing.ModuleDeps{
        Config:         cfg,
        DB:             mgr,
        Logger:         logger,
        Provider:       provider,
        UserRepository: identityModule.UserRepository,
    })
    if err != nil {
        return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
    }

    runnerManager := platformworker.NewManager(
        platformworker.Config{ShutdownTimeout: 30 * time.Second},
        billingModule.Jobs,
        billingModule.Consumers,
        logger,
    )
    if err := runnerManager.Start(ctx); err != nil {
        return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
    }

    slog.InfoContext(ctx, "worker running background modules")

    <-ctx.Done()

    return errors.Join(
        runnerManager.Stop(context.Background()),
        provider.Shutdown(context.Background()),
        mgr.Shutdown(context.Background()),
    )
}
```

## Exemplo proposto do fluxo `handler -> usecase -> repository -> client`

```go
type KiwifyWebhookHandler struct {
    useCase ingestWebhookExecutor
    logger  *slog.Logger
    header  string
}

func (h *KiwifyWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(io.LimitReader(r.Body, webhookBodyLimitBytes))
    if err != nil {
        h.logger.ErrorContext(r.Context(), "billing webhook: leitura body", "error", err)
        writeWebhookJSON(w, http.StatusInternalServerError)
        return
    }

    result, err := h.useCase.Execute(r.Context(), input.IngestWebhookInput{
        RawBody:             body,
        Headers:             extractHeaders(r),
        SignatureHeaderName: h.header,
        ReceivedAt:          time.Now().UTC(),
    })
    if err != nil {
        h.handleError(r.Context(), w, r, err)
        return
    }

    if result.Duplicate {
        w.WriteHeader(http.StatusNoContent)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]any{
        "received":  true,
        "duplicate": false,
    })
}

type IngestKiwifyWebhookUseCase struct {
    webhookRepo WebhookRepository
    adapter     BillingProviderAdapter
    publisher   EventPublisher
    idGenerator IDGenerator
}

func (uc *IngestKiwifyWebhookUseCase) Execute(
    ctx context.Context,
    in input.IngestWebhookInput,
) (output.IngestWebhookResult, error) {
    if err := uc.adapter.ParseWebhook(ctx, in.RawBody, in.Headers, in.SignatureHeaderName); err != nil {
        return output.IngestWebhookResult{}, err
    }

    eventID, duplicate, err := uc.webhookRepo.SaveReceived(ctx, in)
    if err != nil {
        return output.IngestWebhookResult{}, err
    }

    if duplicate {
        return output.IngestWebhookResult{Duplicate: true}, nil
    }

    if err := uc.publisher.Publish(ctx, eventID); err != nil {
        return output.IngestWebhookResult{}, err
    }

    return output.IngestWebhookResult{
        EventID:   eventID,
        Duplicate: false,
    }, nil
}

type PgxWebhookEventRepository struct {
    db *database.Manager
}

func (r *PgxWebhookEventRepository) SaveReceived(
    ctx context.Context,
    in input.IngestWebhookInput,
) (string, bool, error) {
    tx := r.db.DBTX(ctx)
    _ = tx
    _ = in
    return "generated-event-id", false, nil
}

type Client struct {
    httpClient platformhttpclient.Client
}

func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*http.Response, error) {
    return c.httpClient.Get(ctx, "/v1/subscriptions/"+subscriptionID, nil)
}
```

## Leitura esperada da proposta

- `cmd/server/server.go` vira o ponto obrigatorio de bootstrap HTTP explicito com `chiserver.New(...)`
- `cmd/server/server.go` tambem sobe os runners de background visiveis hoje via `platformworker.NewManager(...)`
- `cmd/worker/worker.go` vira o ponto obrigatorio de bootstrap dedicado de processamento em background
- se houver modulo HTTP real visivel no workspace atual, ele deve ser registrado pelo entrypoint sem router placeholder
- o fluxo HTTP fica estritamente `route registrar -> handler -> usecase -> repository/client`
- o bootstrap dos entrypoints deve respeitar o estado atual visivel, incluindo `platformworker.NewManager(...)` e `server.Start(ctx)`
- `internal/platform/httpclient` continua sendo obrigatorio para HTTP outbound
