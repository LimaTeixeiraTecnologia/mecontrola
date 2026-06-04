# ADR-002 — Reuso de `chiserver.Router` do `devkit-go` para registrar rotas de módulos

## Metadados

- **Título:** Padrão de registro de rotas de módulos no chi server
- **Data:** 2026-06-03 (revisada após descoberta no codebase)
- **Status:** Aceita
- **Decisores:** Equipe de plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-01, RF-06, RF-46), `techspec.md` §Interfaces Chave, `/Users/jailtonjunior/Git/devkit-go/pkg/http_server/chi_server/router.go`, `internal/platform/http/server.go`

## Contexto

Versão inicial deste ADR propôs **criar** uma interface `RouteRegistrar` em `internal/platform/http`. Investigação posterior ao código-fonte do `devkit-go` (utilizado como `chiserver` pelo projeto) revelou que o padrão **já existe oficialmente**:

```go
// devkit-go/pkg/http_server/chi_server/router.go
package chiserver

import "github.com/go-chi/chi/v5"

type Router interface {
    Register(router chi.Router)
}

// devkit-go/pkg/http_server/chi_server/server.go
func (s *Server) RegisterRouters(routers ...Router) *Server
func (s *Server) RegisterHandler(method, path string, h Handler, mws ...Middleware) *Server
func WithRouteTimeout(path string, timeout time.Duration) Option
```

A interface `chiserver.Router` é exatamente o contrato necessário. Criar uma interface paralela em `internal/platform/http` adicionaria boilerplate e divergência sem ganho.

Adicionalmente, `WithRouteTimeout(path, duration)` resolve um requisito não explicitado anteriormente: o webhook ingress deve responder em < 2s p99 (RF-06), enquanto o default do servidor é 25s. Per-route timeout é o mecanismo apropriado.

## Decisão

**Reusar `chiserver.Router`** sem criar interface concorrente.

```go
// internal/platform/http/server.go (alterado)
type Deps struct {
    DB         *database.Manager
    Provider   *observability.Provider
    Registrars []chiserver.Router          // novo, default nil
}

func (b *serverBuilder) buildOptions() []chiserver.Option {
    opts := []chiserver.Option{
        chiserver.WithPort(...),
        // ... existentes ...
        chiserver.WithRouteTimeout("/webhooks/kiwify", 2*time.Second),  // novo
    }
    return opts
}

func NewServer(cfg *configs.Config, deps Deps) (*chiserver.Server, error) {
    srv, err := chiserver.New(o11y, builder.buildOptions()...)
    if err != nil {
        return nil, fmt.Errorf("http: inicializando servidor: %w", err)
    }
    srv.RegisterRouters(deps.Registrars...)  // novo
    return srv, nil
}
```

Cada módulo de negócio implementa `chiserver.Router` em `internal/<modulo>/infrastructure/http/server/route_registrar.go`. Bootstrap injeta `[]chiserver.Router{billing.NewKiwifyRouter(...), ...}` em `Deps.Registrars`.

Per-route timeout do webhook fica hardcoded a `2*time.Second` (RF-06 ack < 2s p99). Se futuro endpoint exigir outro timeout, adicionar nova entrada em `buildOptions()`.

## Alternativas Consideradas

### Criar interface `RouteRegistrar` em `internal/platform/http` (proposta original)

- Vantagem: interface fica no domínio do projeto, não depende de detalhes do devkit-go.
- Desvantagem: duplicação; `chiserver.Router` é exatamente o mesmo contrato; manter dois nomes para o mesmo conceito é ruído.
- Rejeitada por desnecessária — o padrão upstream é o padrão.

### Subsystem dedicado por módulo (módulo expõe próprio router + lifecycle)

- Vantagem: maior isolamento.
- Desvantagem: mais boilerplate; coordenação de shutdown order; complica health checks.
- Rejeitada por aumentar complexidade sem ganho proporcional.

### Mount direto em `cmd/server` (`r.Mount("/webhooks/kiwify", billing.Handler())`)

- Vantagem: simples.
- Desvantagem: `cmd/server` passa a conhecer cada módulo; não escala.
- Rejeitada por acoplamento de bootstrap.

## Consequências

### Benefícios Esperados

- Zero código novo em `internal/platform/http` além de propagar `Registrars` para `srv.RegisterRouters`.
- Per-route timeout via `WithRouteTimeout` resolve RF-06 sem código custom.
- Padrão idêntico para billing, onboarding e futuros módulos.
- `cmd/server` orquestra registrars sem conhecer cada handler.

### Trade-offs e Custos

- Dependência da API `chiserver.Router`/`RegisterRouters` permanecer estável. Como devkit-go é mantido pelo time, controle direto.
- `Deps.Registrars` é novo campo — zero-value (nil slice) preserva comportamento atual.

### Riscos e Mitigações

- **Risco:** dois módulos registram a mesma rota → chi panica. **Mitigação:** convenção de path prefix por módulo (`/webhooks/*` para billing, `/api/identity/*` para identity, etc.); documentar em `internal/platform/http/AGENTS.md`.
- **Risco:** ordem de registro afeta middleware. **Mitigação:** `chiserver` aplica middlewares globais em `New`; per-route via `RegisterHandler(method, path, h, mws...)`. Registrars não interferem.

## Plano de Implementação

1. Adicionar `Registrars []chiserver.Router` em `internal/platform/http/Deps`.
2. Em `NewServer`, chamar `srv.RegisterRouters(deps.Registrars...)` antes de devolver.
3. Adicionar `chiserver.WithRouteTimeout("/webhooks/kiwify", 2*time.Second)` em `buildOptions`.
4. `runtime/http_subsystem.go` aceita `Registrars` via `infrahttp.Deps`.
5. `runtime/billing_subsystem.go` (novo) constrói `billing.NewKiwifyRouter(...)` e a injeta.
6. Teste em `internal/platform/http/server_test.go`: cenário com router fake (`type fakeRouter struct{}` implementando `Register(r chi.Router) { r.Get("/test", h) }`) valida que a rota responde após `NewServer`.

## Monitoramento e Validação

- Boot log: `slog.InfoContext(ctx, "http: registrars carregados", slog.Int("count", len(deps.Registrars)))`.
- Métricas devkit-go já expõem `http_request_duration_seconds` por rota.
- Smoke E2E em CI confirma `/webhooks/kiwify` responde `401` em POST sem header.

## Impacto em Documentação e Operação

- `internal/platform/http/AGENTS.md` (criar) documenta convenção de prefixos por módulo.
- README do billing documenta como exportar `Router`.

## Revisão Futura

- Se número de módulos > 5, considerar gerar `Registrars` via reflexão ou config YAML.
- Se devkit-go evoluir a interface, atualizar este ADR.
