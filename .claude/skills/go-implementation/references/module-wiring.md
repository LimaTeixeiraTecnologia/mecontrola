# Module Wiring

<!-- TL;DR
Module wiring faz DI manual explicita, liga repository -> use case -> handler/router/job/consumer e exige validacao mais ampla por risco de bootstrap.
Keywords: module, wiring, bootstrap, lifecycle
Load complete when: tarefa altera module.go, cmd/, main.go ou lifecycle de bootstrap.
-->

## Objetivo
Padronizar wiring com compatibilidade forte ao estilo atual do repositório.

## Regras
- Manter `go-implementation` como entrypoint nao muda o contrato dos modulos reais.
- Wiring segue `repository/client -> use case -> handler/router/job/consumer/producer`.
- `module.go` expoe apenas artefatos reais necessarios ao bootstrap.
- Registrar router apenas quando houver rota real.
- Worker/job/consumer devem ser conectados via adapters da plataforma.
- Mudancas em wiring aumentam o risco de regressao sistêmica; preferir diff pequeno e verificavel.

## Exemplo
```go
type BillingModule struct {
    Router   *Router
    Consumer worker.Consumer
}

func NewBillingModule(db database.DBTX, publisher outbox.Publisher) BillingModule {
    repository := repositories.NewSubscriptionRepository(db)
    activateSubscription := usecases.NewActivateSubscriptionUseCase(repository)
    handler := handlers.NewKiwifyWebhookHandler(activateSubscription)
    router := NewRouter(handler)

    projector := consumers.NewSubscriptionActivatedConsumer(activateSubscription)
    runner := consumer.NewAdapter(projector)

    return BillingModule{
        Router:   router,
        Consumer: runner,
    }
}
```

Exemplo ruim:
- `NewModule(opts...)`;
- `Routers()` retornando slice generico sem necessidade;
- modulo expondo dependencias que nao participam do bootstrap.

## Validacao Minima
- `go build` no entrypoint afetado.
- `go vet` no modulo ou repositório quando o bootstrap tocar multiplos bounded contexts.
- `go test -count=1` no escopo mais amplo impactado; promover para `global` quando o diff tocar contratos compartilhados.

## Proibido
- `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como novo padrao.
- Inventar dependencia inexistente no workspace.
