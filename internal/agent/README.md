# Módulo `agent`

Responsabilidade: agente conversacional, ferramentas (tools) registradas, prompt registry, working memory e budget de custo de inferência.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/agent/
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
// internal/agent/domain/agent_session.go

// AgentSessionID é o identificador único imutável de uma sessão de agente.
type AgentSessionID struct{ value string }

func NewAgentSessionID(v string) (AgentSessionID, error) {
    if v == "" {
        return AgentSessionID{}, errors.New("agent: session id cannot be empty")
    }
    return AgentSessionID{value: v}, nil
}

func (a AgentSessionID) String() string { return a.value }

// AgentSession é o aggregate root do módulo agent.
type AgentSession struct {
    id     AgentSessionID
    budget TokenBudget
    events []DomainEvent
}

func NewAgentSession(id AgentSessionID, budget TokenBudget) (*AgentSession, error) {
    // validações de invariante aqui
    return &AgentSession{id: id, budget: budget}, nil
}

func (a *AgentSession) ID() AgentSessionID    { return a.id }
func (a *AgentSession) Events() []DomainEvent  { return a.events }
func (a *AgentSession) ClearEvents()           { a.events = nil }
```

### Value Object (VO)

```go
// internal/agent/domain/token_budget.go

// TokenBudget é um Value Object imutável com invariante de limite máximo de tokens.
type TokenBudget struct{ maxTokens int }

const defaultMaxTokens = 4096

func NewTokenBudget(max int) (TokenBudget, error) {
    if max <= 0 {
        return TokenBudget{}, errors.New("agent: token budget must be positive")
    }
    if max > 128000 {
        return TokenBudget{}, errors.New("agent: token budget exceeds model context limit")
    }
    return TokenBudget{maxTokens: max}, nil
}

func (b TokenBudget) Max() int { return b.maxTokens }
```

### Port `Repository`

```go
// internal/agent/application/repository.go

// AgentSessionRepository define o contrato de persistência para o agregado AgentSession.
// A implementação concreta vive em infrastructure/.
type AgentSessionRepository interface {
    FindByID(ctx context.Context, id domain.AgentSessionID) (*domain.AgentSession, error)
    Save(ctx context.Context, session *domain.AgentSession) error
}
```

### Port `EventPublisher`

```go
// internal/agent/application/event_publisher.go

// EventPublisher define o contrato de publicação de domain events.
// A implementação concreta vive em infrastructure/ e delega ao eventbus compartilhado.
type EventPublisher interface {
    Publish(ctx context.Context, events []domain.DomainEvent) error
}
```

### Use Case com `UnitOfWork[T]`

```go
// internal/agent/application/run_agent.go

// RunAgentInput é o DTO de entrada do caso de uso.
type RunAgentInput struct {
    SessionID string
    Prompt    string
    MaxTokens int
}

// RunAgentUseCase coordena a execução do agente para uma sessão.
type RunAgentUseCase struct {
    repo      AgentSessionRepository
    publisher EventPublisher
    uow       database.UnitOfWork[*domain.AgentSession]
}

func NewRunAgentUseCase(
    repo AgentSessionRepository,
    publisher EventPublisher,
    uow database.UnitOfWork[*domain.AgentSession],
) *RunAgentUseCase {
    return &RunAgentUseCase{repo: repo, publisher: publisher, uow: uow}
}

func (uc *RunAgentUseCase) Execute(ctx context.Context, in RunAgentInput) (*domain.AgentSession, error) {
    return uc.uow.Do(ctx, func(tx database.Tx) (*domain.AgentSession, error) {
        id, err := domain.NewAgentSessionID(in.SessionID)
        if err != nil {
            return nil, err
        }
        budget, err := domain.NewTokenBudget(in.MaxTokens)
        if err != nil {
            return nil, err
        }
        session, err := domain.NewAgentSession(id, budget)
        if err != nil {
            return nil, err
        }
        if err := uc.repo.Save(ctx, session); err != nil {
            return nil, err
        }
        if err := uc.publisher.Publish(ctx, session.Events()); err != nil {
            return nil, err
        }
        session.ClearEvents()
        return session, nil
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
ai-spec check-spec-drift .specs/prd-agent-<feature>/tasks.md

# Semver e changelog após merge
ai-spec semver-next
ai-spec changelog
```

## Fronteiras de Import (enforçadas por `depguard`)

| Pacote | Pode importar | Proibido |
|--------|--------------|---------|
| `domain` | stdlib, VOs próprios | `application`, `infrastructure`, `internal/platform/*`, `configs/*`, `viper` |
| `application` | `domain` | `infrastructure`, bibliotecas de IO concretas |
| `infrastructure` | `domain`, `application`, `internal/platform/*` | cross-module direto (ex: `finance/infrastructure`) |

Cross-module: comunicação SOMENTE via interface declarada em `application` do consumidor
ou via Domain Event publicado no `internal/platform/events`.
