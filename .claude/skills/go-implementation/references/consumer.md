# Consumer

<!-- TL;DR
Consumer e adapter de entrada de mensagem/evento: desserializa, garante idempotencia na fronteira quando aplicavel e delega ao use case.
Keywords: consumer, messaging, idempotency, adapter
Load complete when: tarefa altera consumer de mensagem ou outbox.
-->

## Objetivo
Padronizar consumers com foco em robustez operacional sem exagerar nos gates.

## Regras
- Consumer desserializa, valida envelope e delega ao use case.
- Idempotencia deve ser tratada na fronteira apropriada quando o contrato do evento exigir.
- Retry, nack, ack ou dead-letter devem respeitar a tecnologia usada; nao inventar politica generica ausente.
- Observabilidade e lifecycle so entram quando o diff realmente tocar esses pontos.

## Exemplo
```go
type SubscriptionActivatedConsumer struct {
    useCase *usecases.ActivateSubscriptionUseCase
}

func NewSubscriptionActivatedConsumer(useCase *usecases.ActivateSubscriptionUseCase) *SubscriptionActivatedConsumer {
    return &SubscriptionActivatedConsumer{useCase: useCase}
}

func (c *SubscriptionActivatedConsumer) Handle(ctx context.Context, event events.Event) error {
    cmd := usecases.ActivateSubscriptionCommand{
        SubscriptionID: event.AggregateID,
        ApprovedAt:     event.OccurredAt,
    }
    return c.useCase.Execute(ctx, cmd)
}
```

Exemplo ruim:
- consumer consultando dois repositorios e decidindo branching de negocio;
- SQL direto no `Handle`;
- politica de retry inventada no proprio consumer sem contrato da plataforma.

## Validacao Minima
- `go test -count=1` no pacote do consumer e no use case acionado.
- `go build` no runner/worker afetado quando houver alteracao de registro ou bootstrap.

## Proibido
- Regra de negocio no consumer.
- SQL direto no consumer.
- Publicacao de side effect sem passar pela aplicacao quando o fluxo ja tiver use case.
