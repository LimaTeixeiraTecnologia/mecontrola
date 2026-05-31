# Módulo `identity`

Responsabilidade: usuário, sessão, JWT/refresh, RBAC e audit de acesso.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/identity/
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
// internal/identity/domain/user.go

// UserID é o identificador único imutável de um usuário.
type UserID struct{ value string }

func NewUserID(v string) (UserID, error) {
    if v == "" {
        return UserID{}, errors.New("identity: user id cannot be empty")
    }
    return UserID{value: v}, nil
}

func (u UserID) String() string { return u.value }

// User é o aggregate root do módulo identity.
// Toda mutação passa por métodos de domínio — sem setters mecânicos.
type User struct {
    id     UserID
    email  Email
    events []DomainEvent
}

func NewUser(id UserID, email Email) (*User, error) {
    // validações de invariante aqui
    return &User{id: id, email: email}, nil
}

func (u *User) ID() UserID          { return u.id }
func (u *User) Events() []DomainEvent { return u.events }
func (u *User) ClearEvents()          { u.events = nil }
```

### Value Object (VO)

```go
// internal/identity/domain/email.go

// Email é um Value Object imutável com invariante de formato.
type Email struct{ value string }

func NewEmail(v string) (Email, error) {
    if !strings.Contains(v, "@") {
        return Email{}, errors.New("identity: invalid email format")
    }
    return Email{value: strings.ToLower(v)}, nil
}

func (e Email) String() string { return e.value }
```

### Port `Repository`

```go
// internal/identity/application/repository.go

// UserRepository define o contrato de persistência para o agregado User.
// A implementação concreta vive em adapters/.
type UserRepository interface {
    FindByID(ctx context.Context, id domain.UserID) (*domain.User, error)
    Save(ctx context.Context, user *domain.User) error
}
```

### Port `EventPublisher`

```go
// internal/identity/application/event_publisher.go

// EventPublisher define o contrato de publicação de domain events.
// A implementação concreta vive em adapters/ e delega ao eventbus de infrastructure.
type EventPublisher interface {
    Publish(ctx context.Context, events []domain.DomainEvent) error
}
```

### Use Case com `UnitOfWork[T]`

```go
// internal/identity/application/create_user.go

// CreateUserInput é o DTO de entrada do caso de uso.
type CreateUserInput struct {
    Email string
}

// CreateUserUseCase coordena a criação de um novo usuário.
type CreateUserUseCase struct {
    repo      UserRepository
    publisher EventPublisher
    uow       database.UnitOfWork[*domain.User]
}

func NewCreateUserUseCase(
    repo UserRepository,
    publisher EventPublisher,
    uow database.UnitOfWork[*domain.User],
) *CreateUserUseCase {
    return &CreateUserUseCase{repo: repo, publisher: publisher, uow: uow}
}

func (uc *CreateUserUseCase) Execute(ctx context.Context, in CreateUserInput) (*domain.User, error) {
    return uc.uow.Do(ctx, func(tx database.Tx) (*domain.User, error) {
        email, err := domain.NewEmail(in.Email)
        if err != nil {
            return nil, err
        }
        id, err := domain.NewUserID(uuid.NewString())
        if err != nil {
            return nil, err
        }
        user, err := domain.NewUser(id, email)
        if err != nil {
            return nil, err
        }
        if err := uc.repo.Save(ctx, user); err != nil {
            return nil, err
        }
        if err := uc.publisher.Publish(ctx, user.Events()); err != nil {
            return nil, err
        }
        user.ClearEvents()
        return user, nil
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
ai-spec check-spec-drift .specs/prd-identity-<feature>/tasks.md

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
