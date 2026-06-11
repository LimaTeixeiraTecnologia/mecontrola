# HTTP Handler

<!-- TL;DR
Handler HTTP/gRPC e adapter fino: decodifica input, chama use case, traduz erro e resposta. Validacao foca no pacote do adapter e no use case acionado.
Keywords: handler, http, grpc, dto, adapter
Load complete when: tarefa altera handler HTTP ou gRPC.
-->

## Objetivo
Manter adapters HTTP/gRPC finos e previsiveis.

## Regras
- Handler decodifica input, valida shape e chama o use case.
- Traducao de erro para status/metadata acontece no adapter.
- DTO de entrada/saida nao substitui linguagem de dominio dentro do use case.
- Timeouts, auth, tracing e metricas entram apenas quando a superficie alterada realmente exigir.

## Exemplo
```go
type ActivateSubscriptionHandler struct {
    useCase *usecases.ActivateSubscriptionUseCase
}

func NewActivateSubscriptionHandler(useCase *usecases.ActivateSubscriptionUseCase) *ActivateSubscriptionHandler {
    return &ActivateSubscriptionHandler{useCase: useCase}
}

func (h *ActivateSubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    subscriptionID := chi.URLParam(r, "subscriptionID")
    if subscriptionID == "" {
        http.Error(w, "subscriptionID obrigatorio", http.StatusBadRequest)
        return
    }

    cmd := usecases.ActivateSubscriptionCommand{
        SubscriptionID: subscriptionID,
        ApprovedAt:     time.Now().UTC(),
    }
    if err := h.useCase.Execute(r.Context(), cmd); err != nil {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

Exemplo ruim:
- handler chamando repositorio direto;
- status HTTP decidido dentro do use case;
- DTO HTTP vazando como contrato interno do dominio.

## Validacao Minima
- `go test -count=1` no pacote do handler e no use case afetado quando houver cobertura separada.
- `go build` no bounded context ou entrypoint HTTP afetado.

## Proibido
- SQL direto no handler.
- Chamada direta a repositorio ou client externo quando houver use case correspondente.
- Regra de negocio ou branching de dominio no adapter.
