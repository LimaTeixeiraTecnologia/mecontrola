# Repository

<!-- TL;DR
Repository concreto mapeia persistencia para o dominio, sem vazar detalhes de SQL ou driver para application/domain, com validacao no escopo da fronteira alterada.
Keywords: repository, persistence, sql, mapping
Load complete when: tarefa altera repositorio concreto ou contrato de persistencia.
-->

## Objetivo
Definir o contrato de trabalho para mudancas em persistencia com foco em confiabilidade.

## Regras
- Repository e adapter concreto de persistencia; nao recebe regra de negocio que pertence ao use case.
- Aceitar `context.Context` como primeiro parametro em toda operacao de I/O.
- Receber dependencias explicitas (`database.DBTX`, `*sql.DB`, `pgx`, etc.) sem factory generica desnecessaria.
- Traduzir linhas/records para entidades e value objects sem vazar wire format.
- Transacao e detalhe de SQL ficam aqui, nao em handler/use case.
- Quando houver so leitura ou escrita simples sem ganho real, evitar interface extra no produtor.

## Exemplo
```go
type SubscriptionRepository struct {
    db database.DBTX
}

func NewSubscriptionRepository(db database.DBTX) *SubscriptionRepository {
    return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) FindByID(ctx context.Context, id string) (*entities.Subscription, error) {
    row := r.db.QueryRowContext(ctx, `SELECT id, status FROM billing.subscriptions WHERE id = $1`, id)

    var subscriptionID string
    var status string
    if err := row.Scan(&subscriptionID, &status); err != nil {
        return nil, fmt.Errorf("buscar assinatura: %w", err)
    }

    return entities.RehydrateSubscription(subscriptionID, status)
}
```

Exemplo ruim:
- handler montando SQL;
- repositorio retornando `pgx.Row` ou `map[string]any` para application;
- interface artificial so para encapsular `DBTX(ctx)`.

## Validacao Minima
- `go test -count=1` no pacote do repository.
- `go test -tags=integration -count=1` apenas se a mudanca depender de fronteira real que mocks nao cobrem.
- `go build` e `go vet` no bounded context alterado.

## Proibido
- Retornar detalhes de driver para application/domain.
- Guardar `context.Context` em struct.
- Criar interface local so para expor `DBTX(ctx)` quando o handle concreto basta.
