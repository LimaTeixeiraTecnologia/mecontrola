# ADR-010 — State machine canônica como domain service stateless

## Metadados

- **Título:** Implementação da máquina de estados de Subscription
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de domínio
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-17, RF-17a, RF-20), `techspec.md` §StateMachine, `.agents/skills/agent-governance/references/ddd.md` "State Pattern"

## Contexto

PRD define 6 estados canônicos (`TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`) e transições explícitas (RF-17). DDD R-DDD-001 obriga "transições centralizadas no aggregate root ou em state object explícito" e "transições permitidas de forma explícita". Object Calisthenics OC #9 ordena métodos com intenção (não setters).

A regra de transição precisa ser:
1. Centralizada (não espalhada em handlers).
2. Exaustiva (cobre 100% das combinações 6×6 = 36).
3. Testável independente da persistência.
4. Reusável por aggregate root e por reconciliation.

## Decisão

Domain service stateless em `internal/billing/domain/services/state_machine.go`:

```go
type StateMachine struct{}

func NewStateMachine() StateMachine { return StateMachine{} }

func (StateMachine) AssertLegal(from, to valueobjects.SubscriptionStatus) error
```

Lista de transições legais em função privada `isLegalTransition(from, to)` com switch exaustivo. Transição não listada → `ErrIllegalTransition`. `StateMachine` é zero-state (não acumula histórico nem cache).

Agregado `Subscription` delega validação ao `StateMachine` em método privado `applyTransition`:

```go
func (s *Subscription) applyTransition(target SubscriptionStatus, reason TransitionReason, at time.Time, period PeriodChange) error {
    if err := NewStateMachine().AssertLegal(s.status, target); err != nil {
        return err
    }
    s.status = target
    ...
}
```

Métodos públicos de intenção (`Activate`, `Renew`, `MarkPastDue`, `Cancel`, `Expire`, `Refund`) chamam `applyTransition` com o target apropriado. Nunca há `SetStatus(s)` público — apenas verbos de domínio.

## Alternativas Consideradas

### State pattern OO com struct por estado

- Vantagem: clássico GoF; cada estado é tipo separado.
- Desvantagem: explosão de tipos (6 structs + 6 implementações de interface); overhead sem ganho em Go (sem polimorfismo dinâmico bonito).
- Rejeitada por não-idiomático Go.

### Tabela `subscription_transitions(from, to, allowed)` no banco

- Vantagem: configurável sem deploy.
- Desvantagem: regra de domínio acoplada a infrastructure; alteração de transição é deploy de SQL.
- Rejeitada por violar DDD (regra mora em domain).

### Map `from → []to` em variável global

- Vantagem: lookup O(1).
- Desvantagem: var global mutável é antipadrão; switch é tão rápido quanto map para 6 estados.
- Rejeitada por antipadrão.

## Consequências

### Benefícios Esperados

- Regra centralizada em 1 função (testável 100% sem mock).
- Agregado expõe verbos de domínio (`Activate`, `Refund`) — OC #9.
- Reconciliation reusa via `Subscription.ApplyEvent` (mesmo caminho de mutação).
- Documentação da máquina de estados está no código (test table 6×6).

### Trade-offs e Custos

- Switch com 6 cases — fácil de manter para 6 estados; revisitar se chegar a 10+.
- `NewStateMachine()` é alocação zero (`return StateMachine{}`), mas chamada em hot path — aceitável dado o pattern e a frequência (1× por aplicação de evento).

### Riscos e Mitigações

- **Risco:** novo estado adicionado e esquecemos de atualizar switch. **Mitigação:** test tabela exaustiva 7×7 cobrindo `Unknown` quebra build ao adicionar novo enum se test não for atualizado.
- **Risco:** transição válida em regra mas inválida em domínio (e.g., race de `ACTIVE → REFUNDED` sem passar por `CANCELED_PENDING`). **Mitigação:** state machine define o canônico; reconciliação detecta divergência via outbox.

## Plano de Implementação

1. `state_machine.go` com função `isLegalTransition` (switch exaustivo).
2. Tabela de transições legais documentada como constante em comentário (única exceção à regra "no comentários óbvios" — é a fonte de verdade visual da máquina).
3. Agregado chama `NewStateMachine().AssertLegal` em todos os métodos de mutação.
4. Unit test: tabela 6×6 (sucesso/falha) + `Unknown` em qualquer side → `false`.

## Monitoramento e Validação

- Métrica `billing_subscription_illegal_transition_total{from, to}` quando `ErrIllegalTransition` retorna (sinal de bug).

## Impacto em Documentação e Operação

- AGENTS.md billing documenta a máquina de estados como source of truth.
- Diagrama Mermaid no README do billing (documentação visual da máquina).

## Revisão Futura

- Se número de estados > 10, considerar gerador de código a partir de tabela YAML.
