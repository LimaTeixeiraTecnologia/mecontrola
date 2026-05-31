# Módulo `notifications`

Responsabilidade: notificação, preferências de entrega, templates de mensagem, agendamento de alertas e lembretes.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/notifications/
├── domain/       # Regras de negócio puras (sem IO)
├── application/  # Casos de uso + ports (interfaces)
└── adapters/     # Implementações concretas (Postgres, HTTP, eventbus)
```

---

## Scaffold Pattern

Os PRDs subsequentes que adicionarem código a este módulo DEVEM seguir os
padrões abaixo. Use os snippets como ponto de partida.

### Aggregate

```go
// internal/notifications/domain/notification.go

// NotificationID é o identificador único imutável de uma notificação.
type NotificationID struct{ value string }

func NewNotificationID(v string) (NotificationID, error) {
    if v == "" {
        return NotificationID{}, errors.New("notifications: notification id cannot be empty")
    }
    return NotificationID{value: v}, nil
}

func (n NotificationID) String() string { return n.value }

// Notification é o aggregate root do módulo notifications.
type Notification struct {
    id       NotificationID
    template NotificationTemplate
    status   DeliveryStatus
    events   []DomainEvent
}

func NewNotification(id NotificationID, template NotificationTemplate) (*Notification, error) {
    // validações de invariante aqui
    return &Notification{id: id, template: template, status: StatusPending}, nil
}

func (n *Notification) ID() NotificationID    { return n.id }
func (n *Notification) Events() []DomainEvent  { return n.events }
func (n *Notification) ClearEvents()           { n.events = nil }
```

### Value Object (VO)

```go
// internal/notifications/domain/notification_template.go

// NotificationTemplate é um Value Object imutável representando o template de mensagem.
type NotificationTemplate struct {
    name string
    body string
}

func NewNotificationTemplate(name, body string) (NotificationTemplate, error) {
    if name == "" {
        return NotificationTemplate{}, errors.New("notifications: template name cannot be empty")
    }
    if body == "" {
        return NotificationTemplate{}, errors.New("notifications: template body cannot be empty")
    }
    return NotificationTemplate{name: name, body: body}, nil
}

func (t NotificationTemplate) Name() string { return t.name }
func (t NotificationTemplate) Body() string { return t.body }
```

### Port `Repository`

```go
// internal/notifications/application/repository.go

// NotificationRepository define o contrato de persistência para o agregado Notification.
// A implementação concreta vive em adapters/.
type NotificationRepository interface {
    FindByID(ctx context.Context, id domain.NotificationID) (*domain.Notification, error)
    Save(ctx context.Context, n *domain.Notification) error
}
```

### Port `EventPublisher`

```go
// internal/notifications/application/event_publisher.go

// EventPublisher define o contrato de publicação de domain events.
// A implementação concreta vive em adapters/ e delega ao eventbus de infrastructure.
type EventPublisher interface {
    Publish(ctx context.Context, events []domain.DomainEvent) error
}
```

### Use Case com `UnitOfWork[T]`

```go
// internal/notifications/application/send_notification.go

// SendNotificationInput é o DTO de entrada do caso de uso.
type SendNotificationInput struct {
    NotificationID string
    TemplateName   string
    TemplateBody   string
    RecipientID    string
}

// SendNotificationUseCase coordena o envio de uma notificação.
type SendNotificationUseCase struct {
    repo      NotificationRepository
    publisher EventPublisher
    uow       database.UnitOfWork[*domain.Notification]
}

func NewSendNotificationUseCase(
    repo NotificationRepository,
    publisher EventPublisher,
    uow database.UnitOfWork[*domain.Notification],
) *SendNotificationUseCase {
    return &SendNotificationUseCase{repo: repo, publisher: publisher, uow: uow}
}

func (uc *SendNotificationUseCase) Execute(ctx context.Context, in SendNotificationInput) (*domain.Notification, error) {
    return uc.uow.Do(ctx, func(tx database.Tx) (*domain.Notification, error) {
        id, err := domain.NewNotificationID(in.NotificationID)
        if err != nil {
            return nil, err
        }
        tmpl, err := domain.NewNotificationTemplate(in.TemplateName, in.TemplateBody)
        if err != nil {
            return nil, err
        }
        notification, err := domain.NewNotification(id, tmpl)
        if err != nil {
            return nil, err
        }
        if err := uc.repo.Save(ctx, notification); err != nil {
            return nil, err
        }
        if err := uc.publisher.Publish(ctx, notification.Events()); err != nil {
            return nil, err
        }
        notification.ClearEvents()
        return notification, nil
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
ai-spec check-spec-drift .specs/prd-notifications-<feature>/tasks.md

# Semver e changelog após merge
ai-spec semver-next
ai-spec changelog
```

## Fronteiras de Import (enforçadas por `depguard`)

| Pacote | Pode importar | Proibido |
|--------|--------------|---------|
| `domain` | stdlib, VOs próprios | `application`, `adapters`, `infrastructure/*`, `configs/*`, `viper` |
| `application` | `domain` | `adapters`, bibliotecas de IO concretas |
| `adapters` | `domain`, `application`, `infrastructure/*` | cross-module direto (ex: `finance/adapters`) |

Cross-module: comunicação SOMENTE via interface declarada em `application` do consumidor
ou via Domain Event publicado no `internal/infrastructure/events`.
