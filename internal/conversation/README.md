# Módulo `conversation`

Responsabilidade: mensagem, thread conversacional, intent, contexto de sessão e ciclo de vida da conversa via WhatsApp.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/conversation/
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
// internal/conversation/domain/message.go

// MessageID é o identificador único imutável de uma mensagem.
type MessageID struct{ value string }

func NewMessageID(v string) (MessageID, error) {
    if v == "" {
        return MessageID{}, errors.New("conversation: message id cannot be empty")
    }
    return MessageID{value: v}, nil
}

func (m MessageID) String() string { return m.value }

// Message é o aggregate root do módulo conversation.
type Message struct {
    id      MessageID
    content MessageContent
    events  []DomainEvent
}

func NewMessage(id MessageID, content MessageContent) (*Message, error) {
    // validações de invariante aqui
    return &Message{id: id, content: content}, nil
}

func (m *Message) ID() MessageID         { return m.id }
func (m *Message) Events() []DomainEvent  { return m.events }
func (m *Message) ClearEvents()           { m.events = nil }
```

### Value Object (VO)

```go
// internal/conversation/domain/message_content.go

// MessageContent é um Value Object imutável com invariante de tamanho máximo.
type MessageContent struct{ value string }

const maxMessageLength = 4096

func NewMessageContent(v string) (MessageContent, error) {
    if v == "" {
        return MessageContent{}, errors.New("conversation: message content cannot be empty")
    }
    if len(v) > maxMessageLength {
        return MessageContent{}, fmt.Errorf("conversation: message content exceeds %d characters", maxMessageLength)
    }
    return MessageContent{value: v}, nil
}

func (mc MessageContent) String() string { return mc.value }
```

### Port `Repository`

```go
// internal/conversation/application/repository.go

// MessageRepository define o contrato de persistência para o agregado Message.
// A implementação concreta vive em adapters/.
type MessageRepository interface {
    FindByID(ctx context.Context, id domain.MessageID) (*domain.Message, error)
    Save(ctx context.Context, msg *domain.Message) error
    FindThreadByUser(ctx context.Context, userID string) ([]*domain.Message, error)
}
```

### Port `EventPublisher`

```go
// internal/conversation/application/event_publisher.go

// EventPublisher define o contrato de publicação de domain events.
// A implementação concreta vive em adapters/ e delega ao eventbus de infrastructure.
type EventPublisher interface {
    Publish(ctx context.Context, events []domain.DomainEvent) error
}
```

### Use Case com `UnitOfWork[T]`

```go
// internal/conversation/application/receive_message.go

// ReceiveMessageInput é o DTO de entrada do caso de uso.
type ReceiveMessageInput struct {
    MessageID string
    Content   string
    UserID    string
}

// ReceiveMessageUseCase coordena o recebimento de uma nova mensagem.
type ReceiveMessageUseCase struct {
    repo      MessageRepository
    publisher EventPublisher
    uow       database.UnitOfWork[*domain.Message]
}

func NewReceiveMessageUseCase(
    repo MessageRepository,
    publisher EventPublisher,
    uow database.UnitOfWork[*domain.Message],
) *ReceiveMessageUseCase {
    return &ReceiveMessageUseCase{repo: repo, publisher: publisher, uow: uow}
}

func (uc *ReceiveMessageUseCase) Execute(ctx context.Context, in ReceiveMessageInput) (*domain.Message, error) {
    return uc.uow.Do(ctx, func(tx database.Tx) (*domain.Message, error) {
        id, err := domain.NewMessageID(in.MessageID)
        if err != nil {
            return nil, err
        }
        content, err := domain.NewMessageContent(in.Content)
        if err != nil {
            return nil, err
        }
        msg, err := domain.NewMessage(id, content)
        if err != nil {
            return nil, err
        }
        if err := uc.repo.Save(ctx, msg); err != nil {
            return nil, err
        }
        if err := uc.publisher.Publish(ctx, msg.Events()); err != nil {
            return nil, err
        }
        msg.ClearEvents()
        return msg, nil
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
ai-spec check-spec-drift .specs/prd-conversation-<feature>/tasks.md

# Semver e changelog após merge
ai-spec semver-next
ai-spec changelog
```

## Fronteiras de Import (enforçadas por `depguard`)

| Pacote | Pode importar | Proibido |
|--------|--------------|---------|
| `domain` | stdlib, VOs próprios | `application`, `adapters`, `infrastructure/*`, `configs/*`, `viper` |
| `application` | `domain` | `adapters`, bibliotecas de IO concretas |
| `adapters` | `domain`, `application`, `infrastructure/*` | cross-module direto (ex: `identity/adapters`) |

Cross-module: comunicação SOMENTE via interface declarada em `application` do consumidor
ou via Domain Event publicado no `internal/infrastructure/events`.
