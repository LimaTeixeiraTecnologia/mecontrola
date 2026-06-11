# Producer

<!-- TL;DR
Producer e adapter outbound: serializa e publica payload decidido pela aplicacao, sem criar semantica de negocio no proprio adapter.
Keywords: producer, outbox, publish, payload
Load complete when: tarefa altera producer ou dispatch de evento.
-->

## Objetivo
Padronizar producers/outbox com contrato claro e sem excesso de abstracao.

## Regras
- Producer publica evento ou mensagem ja decidido pela camada application.
- Payload deve ser estavel e coerente com o contrato do evento.
- Transacao explicita ou integracao com outbox fica no adapter/persistencia, nao em handler.
- Idempotencia e metadata devem seguir o contrato existente do sistema de eventos.

## Exemplo
```go
type SubscriptionActivatedProducer struct {
    publisher outbox.Publisher
}

func NewSubscriptionActivatedProducer(publisher outbox.Publisher) *SubscriptionActivatedProducer {
    return &SubscriptionActivatedProducer{publisher: publisher}
}

func (p *SubscriptionActivatedProducer) Publish(ctx context.Context, event SubscriptionActivatedEvent) error {
    payload, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("serializar evento: %w", err)
    }

    if err := p.publisher.Publish(ctx, "billing.subscription.activated", payload); err != nil {
        return fmt.Errorf("publicar evento: %w", err)
    }
    return nil
}
```

Exemplo ruim:
- producer decidindo se o evento deve ou nao existir;
- payload montado a partir de consulta extra que reinterpreta regra de negocio;
- regra de dominio escondida em serializacao.

## Validacao Minima
- `go test -count=1` no pacote do producer.
- `go build` no bounded context afetado.
- Promover para `boundary` com `persistence.md` quando o diff tocar transacao/outbox concretos.

## Proibido
- Decidir trigger de negocio no producer.
- Chamar use case inversamente a partir do producer para descobrir semantica de payload.
