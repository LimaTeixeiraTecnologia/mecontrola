# Tarefa 2.0: Domain billing — agregado Subscription, value objects e tabela de transições

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o núcleo puro de domínio do bounded context `billing`: o agregado `Subscription` (state machine), value objects (`Status`, `Plan`, `FunnelToken`) e o domain service `transitions.go` que centraliza as transições permitidas entre estados, incluindo refund terminal e detecção de regressão. Pacote sem qualquer dependência de infra ou plataforma.

<requirements>
- Estrutura conforme `AGENTS.md` §"Layout Obrigatorio por Modulo" e §"Padrao Obrigatorio de Modulo".
- Sem imports de `application`, `infrastructure`, `platform`, HTTP, SQL, brokers, JSON ou OS.
- Sem `init()`, sem `panic`, sem `var _ Interface = (*Type)(nil)`, sem clock abstrato (R0/R5.12/R6.4/R6.7).
- `FunnelToken` recusa string vazia (invariante de domínio; RF-03 é traduzido em erro no use case).
- Enum `Status` espelha `identity.domain` (`ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`, `TRIALING` reservado) com cross-check em teste.
- Refund é terminal: nenhuma transição sai de `REFUNDED`.
</requirements>

## Subtarefas

- [ ] 2.1 Criar `internal/billing/domain/valueobjects/status.go` com `Status` enum (`iota+1`, zero value reservado para não inicializado) e helpers `IsTerminal`, `IsActiveForBilling`.
- [ ] 2.2 Criar `internal/billing/domain/valueobjects/plan.go` com `Plan{Code, DurationDays}` e construtor que valida `Code ∈ {MONTHLY,QUARTERLY,ANNUAL}` e `DurationDays > 0`.
- [ ] 2.3 Criar `internal/billing/domain/valueobjects/funnel_token.go` com construtor que recusa string vazia.
- [ ] 2.4 Criar `internal/billing/domain/entities/subscription.go` com campos e métodos: `Activate`, `Renew`, `MarkPastDue(graceDuration)`, `MarkCanceled`, `MarkRefunded`, `LastEventAt()`. `period_end`, `grace_end` calculados a partir do plano e `occurred_at` do command.
- [ ] 2.5 Criar `internal/billing/domain/services/transitions.go` com tabela 6×6 de transições permitidas e helper `IsRegression(currentStatus, incomingTrigger, occurredAt, lastEventAt) bool`.
- [ ] 2.6 Criar testes unitários cobrindo construtores, todas as transições da tabela §5.3 da techspec, terminais e regressões.
- [ ] 2.7 Adicionar teste cross-check de equivalência entre `billing.domain.valueobjects.Status` e `identity.domain` (compilação + paridade de constantes).

## Detalhes de Implementação

- Tabela de transições e regras de regressão em techspec §5.3.
- `grace_duration = 3 * 24h` é uniforme (Q-02 travada; RF-06).
- Métodos do agregado mutam estado via cópia/retorno controlado; sem ponteiro exportado para `last_event_at`.
- Cross-check de enum: teste de pacote que importa `identity.domain` e compara nomes/ordem das constantes. Se identity mudar, o teste falha — falha esperada para forçar revisão.

## Critérios de Sucesso

- `go build ./internal/billing/domain/...` verde.
- `go test -race -count=1 ./internal/billing/domain/...` cobre 6×6 transições.
- `go vet ./internal/billing/domain/...` sem warnings.
- Cross-check com identity falha **somente** se a enum em identity divergir (regressão de RF-04).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests por arquivo: `status_test.go`, `plan_test.go`, `funnel_token_test.go`, `subscription_test.go`, `transitions_test.go`.
- [ ] Cross-check com `identity/domain/entitlement.go` (paridade de estados).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/domain/entities/subscription.go` + `_test.go`
- `internal/billing/domain/valueobjects/{status,plan,funnel_token}.go` + `_test.go`
- `internal/billing/domain/services/transitions.go` + `_test.go`
- Referência: `internal/identity/domain/entitlement.go` (interface `Subscription`, função `IsEntitled`).
- Referência: techspec §4 (arquitetura), §5.3 (state machine), §6.7 (CHECKs).
