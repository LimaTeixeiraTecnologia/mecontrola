# Módulo `telemetry`

Responsabilidade: eventos de telemetria de domínio, métricas de uso do produto, rastreamento de jornada do usuário e análise de comportamento conversacional.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/telemetry/
├── domain/       # Regras de negócio puras (sem IO)
├── application/  # Casos de uso + ports (interfaces)
└── infrastructure/ # Implementações concretas (Postgres, HTTP, eventbus)
```

---

## Scaffold Pattern

Os PRDs subsequentes que adicionarem código a este módulo DEVEM seguir os
padrões abaixo. Use os snippets como ponto de partida.

### Aggregate

```go
// internal/telemetry/domain/product_event.go

// ProductEventID é o identificador único imutável de um evento de telemetria de produto.
type ProductEventID struct{ value string }

func NewProductEventID(v string) (ProductEventID, error) {
    if v == "" {
        return ProductEventID{}, errors.New("telemetry: event id cannot be empty")
    }
    return ProductEventID{value: v}, nil
}

func (p ProductEventID) String() string { return p.value }

// ProductEvent é o aggregate root do módulo telemetry.
type ProductEvent struct {
    id         ProductEventID
    eventName  EventName
    properties EventProperties
    events     []DomainEvent
}

func NewProductEvent(id ProductEventID, name EventName, props EventProperties) (*ProductEvent, error) {
    // validações de invariante aqui
    return &ProductEvent{id: id, eventName: name, properties: props}, nil
}

func (p *ProductEvent) ID() ProductEventID    { return p.id }
func (p *ProductEvent) Events() []DomainEvent  { return p.events }
func (p *ProductEvent) ClearEvents()           { p.events = nil }
```

### Value Object (VO)

```go
// internal/telemetry/domain/event_name.go

// EventName é um Value Object imutável representando o nome canônico do evento.
// Segue o padrão kebab-case `<modulo>.<acao>` (ex: `conversation.message-received`).
type EventName struct{ value string }

func NewEventName(v string) (EventName, error) {
    if v == "" {
        return EventName{}, errors.New("telemetry: event name cannot be empty")
    }
    // validação de formato kebab-case pode ser adicionada aqui
    return EventName{value: v}, nil
}

func (e EventName) String() string { return e.value }
```

### Port `Repository`

```go
// internal/telemetry/application/repository.go

// ProductEventRepository define o contrato de persistência para o agregado ProductEvent.
// A implementação concreta vive em infrastructure/.
type ProductEventRepository interface {
    FindByID(ctx context.Context, id domain.ProductEventID) (*domain.ProductEvent, error)
    Save(ctx context.Context, event *domain.ProductEvent) error
    FindByUser(ctx context.Context, userID string) ([]*domain.ProductEvent, error)
}
```

### Port `EventPublisher`

```go
// internal/telemetry/application/event_publisher.go

// EventPublisher define o contrato de publicação de domain events.
// A implementação concreta vive em infrastructure/ e delega ao eventbus compartilhado.
type EventPublisher interface {
    Publish(ctx context.Context, events []domain.DomainEvent) error
}
```

### Use Case com `UnitOfWork[T]`

```go
// internal/telemetry/application/track_event.go

// TrackEventInput é o DTO de entrada do caso de uso.
type TrackEventInput struct {
    EventID    string
    EventName  string
    UserID     string
    Properties map[string]string
}

// TrackEventUseCase coordena o rastreamento de um evento de produto.
type TrackEventUseCase struct {
    repo      ProductEventRepository
    publisher EventPublisher
    uow       database.UnitOfWork[*domain.ProductEvent]
}

func NewTrackEventUseCase(
    repo ProductEventRepository,
    publisher EventPublisher,
    uow database.UnitOfWork[*domain.ProductEvent],
) *TrackEventUseCase {
    return &TrackEventUseCase{repo: repo, publisher: publisher, uow: uow}
}

func (uc *TrackEventUseCase) Execute(ctx context.Context, in TrackEventInput) (*domain.ProductEvent, error) {
    return uc.uow.Do(ctx, func(tx database.Tx) (*domain.ProductEvent, error) {
        id, err := domain.NewProductEventID(in.EventID)
        if err != nil {
            return nil, err
        }
        name, err := domain.NewEventName(in.EventName)
        if err != nil {
            return nil, err
        }
        props, err := domain.NewEventProperties(in.Properties)
        if err != nil {
            return nil, err
        }
        event, err := domain.NewProductEvent(id, name, props)
        if err != nil {
            return nil, err
        }
        if err := uc.repo.Save(ctx, event); err != nil {
            return nil, err
        }
        if err := uc.publisher.Publish(ctx, event.Events()); err != nil {
            return nil, err
        }
        event.ClearEvents()
        return event, nil
    })
}
```

---

## Comandos `ai-spec` recomendados

Ao criar novos agregados neste módulo via PRD subsequente:

```bash
# Iniciar novo ciclo de feature
ai-spec create-prd

# Derivar especificação técnica do PRD aprovado
ai-spec create-technical-specification

# Decompor em tarefas incrementais
ai-spec create-tasks

# Executar tarefa isolada
ai-spec execute-task

# Verificar drift entre PRD e techspec
ai-spec check-spec-drift .specs/prd-telemetry-<feature>/tasks.md

# Semver e changelog após merge
ai-spec semver-next
ai-spec changelog
```

## Fronteiras de Import (enforçadas por `depguard`)

| Pacote | Pode importar | Proibido |
|--------|--------------|---------|
| `domain` | stdlib, VOs próprios | `application`, `infrastructure`, `internal/platform/*`, `configs/*`, `viper` |
| `application` | `domain` | `infrastructure`, bibliotecas de IO concretas |
| `infrastructure` | `domain`, `application`, `internal/platform/*` | cross-module direto (ex: `identity/infrastructure`) |

Cross-module: comunicação SOMENTE via interface declarada em `application` do consumidor
ou via Domain Event publicado no `internal/platform/events`.
