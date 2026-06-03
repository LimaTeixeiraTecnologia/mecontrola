# Módulo `finance`

Responsabilidade: movimentações financeiras, categorias, metas, saldos e regras de orçamento pessoal.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/finance/
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
// internal/finance/domain/transaction.go

// TransactionID é o identificador único imutável de uma movimentação financeira.
type TransactionID struct{ value string }

func NewTransactionID(v string) (TransactionID, error) {
    if v == "" {
        return TransactionID{}, errors.New("finance: transaction id cannot be empty")
    }
    return TransactionID{value: v}, nil
}

func (t TransactionID) String() string { return t.value }

// Transaction é o aggregate root do módulo finance.
type Transaction struct {
    id       TransactionID
    amount   Money
    category Category
    events   []DomainEvent
}

func NewTransaction(id TransactionID, amount Money, category Category) (*Transaction, error) {
    // validações de invariante aqui
    return &Transaction{id: id, amount: amount, category: category}, nil
}

func (t *Transaction) ID() TransactionID     { return t.id }
func (t *Transaction) Events() []DomainEvent  { return t.events }
func (t *Transaction) ClearEvents()           { t.events = nil }
```

### Value Object (VO)

```go
// internal/finance/domain/money.go

// Money é um Value Object imutável que representa valor monetário em centavos.
// Armazenado em int64 para evitar imprecisão de ponto flutuante.
type Money struct {
    amountCents int64
    currency    string
}

func NewMoney(cents int64, currency string) (Money, error) {
    if currency == "" {
        return Money{}, errors.New("finance: currency cannot be empty")
    }
    // cents pode ser negativo (débito); validação de negócio fica no aggregate
    return Money{amountCents: cents, currency: currency}, nil
}

func (m Money) Cents() int64    { return m.amountCents }
func (m Money) Currency() string { return m.currency }
```

### Port `Repository`

```go
// internal/finance/application/repository.go

// TransactionRepository define o contrato de persistência para o agregado Transaction.
// A implementação concreta vive em infrastructure/.
type TransactionRepository interface {
    FindByID(ctx context.Context, id domain.TransactionID) (*domain.Transaction, error)
    Save(ctx context.Context, tx *domain.Transaction) error
    FindByUser(ctx context.Context, userID string) ([]*domain.Transaction, error)
}
```

### Port `EventPublisher`

```go
// internal/finance/application/event_publisher.go

// EventPublisher define o contrato de publicação de domain events.
// A implementação concreta vive em infrastructure/ e delega ao eventbus compartilhado.
type EventPublisher interface {
    Publish(ctx context.Context, events []domain.DomainEvent) error
}
```

### Use Case com `UnitOfWork[T]`

```go
// internal/finance/application/register_transaction.go

// RegisterTransactionInput é o DTO de entrada do caso de uso.
type RegisterTransactionInput struct {
    TransactionID string
    AmountCents   int64
    Currency      string
    CategoryName  string
    UserID        string
}

// RegisterTransactionUseCase coordena o registro de uma movimentação financeira.
type RegisterTransactionUseCase struct {
    repo      TransactionRepository
    publisher EventPublisher
    uow       database.UnitOfWork[*domain.Transaction]
}

func NewRegisterTransactionUseCase(
    repo TransactionRepository,
    publisher EventPublisher,
    uow database.UnitOfWork[*domain.Transaction],
) *RegisterTransactionUseCase {
    return &RegisterTransactionUseCase{repo: repo, publisher: publisher, uow: uow}
}

func (uc *RegisterTransactionUseCase) Execute(ctx context.Context, in RegisterTransactionInput) (*domain.Transaction, error) {
    return uc.uow.Do(ctx, func(tx database.Tx) (*domain.Transaction, error) {
        id, err := domain.NewTransactionID(in.TransactionID)
        if err != nil {
            return nil, err
        }
        amount, err := domain.NewMoney(in.AmountCents, in.Currency)
        if err != nil {
            return nil, err
        }
        category, err := domain.NewCategory(in.CategoryName)
        if err != nil {
            return nil, err
        }
        transaction, err := domain.NewTransaction(id, amount, category)
        if err != nil {
            return nil, err
        }
        if err := uc.repo.Save(ctx, transaction); err != nil {
            return nil, err
        }
        if err := uc.publisher.Publish(ctx, transaction.Events()); err != nil {
            return nil, err
        }
        transaction.ClearEvents()
        return transaction, nil
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
ai-spec check-spec-drift .specs/prd-finance-<feature>/tasks.md

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
