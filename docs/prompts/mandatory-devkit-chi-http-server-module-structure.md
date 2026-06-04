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

# Conflitos e ambiguidades detectados

- O repositório atual já usa `github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server` como tipo de router nos módulos (`internal/billing/module.go` e `internal/identity/module.go`), mas o bootstrap de `cmd/server/server.go` ainda compõe o HTTP via `runtime.NewHTTPRunnerWithDeps(...)`, não via `httpserver.New(...)` explícito no entrypoint.
- O exemplo fornecido referencia módulos e rotas de outro contexto (`user`, `category`, `card`, `transaction`, `paymentMethod`, `budget`, `invoice`), mas o estado real deste repositório hoje expõe `billing` e `identity`. O prompt enriquecido precisa tratar o snippet apenas como referência arquitetural, sem inventar módulos inexistentes.
- Há tensão entre a estrutura desejada (`http/client`, `http/server`, `routes` na raiz da pasta HTTP, `handlers/...`) e a governança canônica do repositório, que exige preservar a fronteira `internal/<modulo>/infrastructure/http/server` e `internal/<modulo>/infrastructure/http/client`.
- O pedido exige `0 comentários`, então o prompt enriquecido precisa proibir comentários novos em código Go gerado, mesmo quando o codebase atual já contenha comentários legados.

# Prompt enriquecido

```text
Quero que voce implemente, de forma obrigatoria, mandatória e inegociavel, o bootstrap HTTP deste repositorio Go usando explicitamente `httpserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"` em `cmd/server/server.go`, e que padronize o wiring HTTP de cada modulo que exponha rotas para seguir um desenho consistente, production-ready e aderente a `AGENTS.md`.

E obrigatorio, mandatório e inegociavel usar a skill `go-implementation`, seus exemplos e tambem os exemplos deste prompt como referencia obrigatoria e normativa de implementacao, sempre adaptando ao estado real do repositorio quando houver conflito entre exemplo e codebase.

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
5. `go.mod` para respeitar a versao real do Go e as dependencias do repositorio.

Contexto real do repositorio que deve orientar a implementacao:
- `go.mod` declara Go `1.26.2` e `github.com/JailtonJunior94/devkit-go v0.4.0`.
- O repositorio e um monolito modular em Go e deve preservar fronteiras arquiteturais.
- `cmd/server/server.go` hoje inicializa config, observabilidade, database manager, modulos e depois delega o HTTP para `runtime.NewHTTPRunnerWithDeps(...)`.
- `internal/billing/module.go` e `internal/identity/module.go` ja retornam routers baseados em `github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server`.
- Hoje existe HTTP inbound em `internal/billing/infrastructure/http/server` e HTTP outbound em `internal/billing/infrastructure/http/client/kiwify`.
- O projeto ja possui `internal/platform/httpclient`; toda chamada HTTP outbound deve continuar passando por esse wrapper.
- `identity` e `billing` possuem politicas especificas de PII: nunca logar email, WhatsApp, CPF, dados de cartao ou payload sensivel em claro. Em `identity`, usar mascaramento `_masked`; em `billing`, respeitar a politica de redaction do `PIIRedactor`.

Ambiguidades e conflitos que voce deve resolver com seguranca antes de codar:
- O snippet desejado usa `httpserver.New(...)` diretamente em `cmd/server/server.go`, enquanto o estado atual usa `runtime.NewHTTPRunnerWithDeps(...)`. Se houver duplicidade de responsabilidade entre os dois caminhos, a implementacao final nao pode deixar bootstrap HTTP concorrente, parcial ou cosmetico.
- O exemplo de rotas e de `module.go` veio de outro contexto e cita modulos que nao existem neste repositorio. Use o exemplo como referencia de padrao de composicao, nunca como copia literal.
- O pedido fala em `http/client`, `http/server`, `routes` e `handlers/...`, mas `AGENTS.md` exige preservar a fronteira `internal/<modulo>/infrastructure/http/server` e `internal/<modulo>/infrastructure/http/client`. Resolva essa tensao sem violar a governanca: a estrutura final deve permanecer dentro de `infrastructure/http/...`, com organizacao interna coerente para routers e handlers.
- O pedido exige `0 comentarios`; portanto, nao adicione comentarios novos em arquivos Go e nao use comentarios para justificar a implementacao.

Objetivo principal:
1. Tornar obrigatorio o uso explicito de `httpserver.New(...)` em `cmd/server/server.go` para compor o servidor HTTP principal.
2. Registrar routers modulares via `srv.RegisterRouters(...)`, no espirito do snippet fornecido, mas adaptado aos modulos reais do repositorio.
3. Padronizar o wiring dos modulos que expõem rotas HTTP para que `module.go` construa handlers, middlewares, routers e adapters de forma explicita, previsivel e consistente com DI manual.
4. Garantir graceful shutdown robusto, derivando `shutdownCtx` a partir de `ctx`, encerrando observabilidade e database manager de forma ordenada e sem falso positivo de sucesso.
5. Manter HTTP outbound em `internal/<modulo>/infrastructure/http/client` usando `internal/platform/httpclient`.

Diretrizes de desenho obrigatorias:
1. Preserve a arquitetura do repositorio e a DI manual por construtores; nao use framework de DI.
2. Preserve a fronteira canônica `internal/<modulo>/infrastructure/http/server` para HTTP inbound e `internal/<modulo>/infrastructure/http/client` para HTTP outbound.
3. Se houver necessidade de organizar `handlers` e `routes`, faca isso dentro da fronteira HTTP do modulo, sem mover responsabilidade para fora de `infrastructure/http`.
4. `module.go` deve ser o ponto de composicao do modulo: criar repositories, use cases, error handlers, auth middleware, handlers HTTP e router do modulo.
5. O router do modulo deve expor `Register(router chi.Router)` e ser registravel pelo servidor central via `srv.RegisterRouters(...)`.
6. Toda rota protegida deve aplicar middleware de autorizacao no router do modulo, nao no handler.
7. O bootstrap de `cmd/server/server.go` deve inicializar o servidor com options coerentes com a API real do pacote `chi_server`, incluindo health checks, metrics, CORS, porta, service name e service version quando disponiveis no repositorio.
8. Se algum detalhe do snippet divergir da API real de `devkit-go` ou dos nomes reais de config, adapte ao estado verdadeiro do codebase. Nao invente API, campos de config ou wrappers desnecessarios.
9. Nao deixar coexistir dois caminhos de bootstrap HTTP equivalentes competindo entre si. Se `runtime.NewHTTPRunnerWithDeps(...)` passar a ficar redundante, refatore ou substitua de forma limpa, completa e segura.
10. E obrigatorio usar os exemplos da skill `go-implementation` e os exemplos contidos neste prompt como base concreta de implementacao, adaptando-os ao contexto real do repositorio sem copia cega.
11. E mandatório e inegociavel manter 0 comentarios em qualquer codigo Go novo ou alterado.

Estrutura obrigatoria para modulos com HTTP:
1. HTTP inbound:
   - `internal/<modulo>/infrastructure/http/server/...`
   - handlers em naming claro, como `handlers/<entidade>_handler.go` quando fizer sentido
   - router/registrar com metodo `Register(router chi.Router)`
2. HTTP outbound:
   - `internal/<modulo>/infrastructure/http/client/...`
   - usar obrigatoriamente `internal/platform/httpclient`
3. Wiring do modulo:
   - `internal/<modulo>/module.go` deve expor o router do modulo e demais adapters/ports necessarios
   - o modelo desejado e o do snippet de `InvoiceModule`: dependencies explicitas, router pronto para registro central e providers/adapters retornados pelo modulo quando necessario

Arquivos e areas minimas que devem ser inspecionadas antes de editar:
- `go.mod`
- `cmd/server/server.go`
- `internal/billing/module.go`
- `internal/identity/module.go`
- implementacoes atuais de HTTP inbound em `internal/billing/infrastructure/http/server`
- usos atuais de `github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server`
- o pacote que hoje centraliza o lifecycle HTTP (`runtime.NewHTTPRunnerWithDeps(...)` e dependencias relacionadas)
- `configs/...` e os pacotes reais de observabilidade/database usados no bootstrap

Requisitos funcionais:
1. `cmd/server/server.go` deve instanciar explicitamente `httpserver.New(...)` usando a API real do pacote `github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server`.
2. O servidor central deve registrar os routers dos modulos reais via `srv.RegisterRouters(...)`.
3. Cada modulo que exponha rotas HTTP deve ter wiring explicito em `module.go`, no espirito do exemplo de `InvoiceModule`, adaptado ao contexto real do modulo.
4. Cada router de modulo deve encapsular middlewares, handlers e paths do proprio modulo, no espirito do exemplo `InvoiceRouter`.
5. O shutdown deve derivar contexto com timeout a partir de `ctx`, encerrar observabilidade e banco de forma ordenada e manter rastreabilidade operacional.

Requisitos nao funcionais obrigatorios:
1. Production-ready de verdade: nada de bootstrap parcial, alias cosmetico ou migracao pela metade.
2. Sem falso positivo: se o servidor central nao estiver realmente usando `httpserver.New(...)`, a tarefa deve ser considerada incompleta.
3. Sem concorrencia de servidores/runtimes duplicados para a mesma responsabilidade.
4. Logging, tracing, health checks e shutdown devem permanecer coerentes com observabilidade e database manager reais do repositorio.
5. Sem vazamento de PII, segredos, payloads sensiveis, CPF, dados de cartao, email ou WhatsApp em claro.
6. E mandatório e inegociavel manter 0 comentarios em qualquer codigo Go novo ou alterado.

Proibicoes explicitas:
- Nao copiar literalmente o snippet quando ele conflitar com o repositorio.
- Nao inventar modulos inexistentes.
- Nao criar wrappers redundantes sobre `chi_server` apenas para aparentar compliance.
- Nao deixar `runtime.NewHTTPRunnerWithDeps(...)` ativo em paralelo se isso significar dois bootstraps HTTP concorrentes.
- Nao mover HTTP outbound para fora de `internal/<modulo>/infrastructure/http/client`.
- Nao registrar rota diretamente no `cmd/server/server.go` quando ela pertence ao modulo.
- Nao quebrar as fronteiras `handler -> usecase -> repositories e/ou client http`.
- Nao logar PII em claro.
- Nao adicionar comentarios em hipotese nenhuma.

Criterios de aceitacao:
1. `cmd/server/server.go` usa explicitamente `httpserver.New(...)` com options aderentes ao codebase real.
2. Os routers dos modulos reais sao registrados pelo servidor central via `srv.RegisterRouters(...)`.
3. O wiring de cada modulo HTTP segue composicao explicita em `module.go`, no espirito do exemplo fornecido, sem copia cega.
4. A estrutura HTTP final respeita `internal/<modulo>/infrastructure/http/server` e `internal/<modulo>/infrastructure/http/client`, com organizacao interna coerente para routers e handlers.
5. O bootstrap HTTP antigo nao permanece duplicado ou redundante.
6. Graceful shutdown de observabilidade e database manager continua correto, com contexto derivado e timeout explicito.
7. PII continua protegida segundo as politicas de `billing` e `identity`.
8. A implementacao usa obrigatoriamente a skill `go-implementation`, seus exemplos e os exemplos deste prompt como referencia adaptada ao repositório real.
9. Nenhum arquivo Go novo ou alterado recebe comentarios, sem excecao.
10. A resposta final explica quais arquivos foram alterados, como o bootstrap HTTP ficou obrigatorio e como a estrutura dos modulos foi padronizada.

Saida esperada:
1. Analise curta das ambiguidades entre o snippet desejado e o estado real do repositorio antes de codar.
2. Implementacao completa, sem workaround cosmetico.
3. Ajustes proporcionais em testes e wiring para cobrir bootstrap, registro de routers e shutdown.
4. Resumo final objetivo em PT-BR com foco em `cmd/server/server.go`, routers modulares, estrutura HTTP e graceful shutdown.

Se houver conflito entre o snippet fornecido, `AGENTS.md`, `agent-governance`, `go-implementation` e o estado real do repositorio, prevalecem `AGENTS.md`, `go-implementation` e a restricao mais segura.
```

# Melhorias aplicadas

- Amarrei o prompt ao estado real do repositório: hoje o bootstrap HTTP passa por `runtime.NewHTTPRunnerWithDeps(...)`, enquanto os módulos já conhecem `chi_server`.
- Explicitei o principal conflito estrutural do pedido: conciliar o exemplo desejado com a fronteira obrigatória de `AGENTS.md` (`infrastructure/http/server` e `infrastructure/http/client`).
- Transformei “mandatório”, “production-ready”, “production-proof” e “sem falso positivo” em critérios verificáveis de aceitação, incluindo remoção de bootstrap HTTP redundante.
- Preservai o objetivo de usar `httpserver.New(...)` e `srv.RegisterRouters(...)`, mas forcei adaptação ao codebase real para não inventar módulos, campos de config ou APIs inexistentes.
- Incorporei as políticas de PII de `billing` e `identity`, além da exigência explícita de `0 comentários` em código Go novo ou alterado.
