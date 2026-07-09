# Tarefa 7.0: Endurecimento de workflows e pendências (integração)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir, por testes de integração (testcontainers Postgres), que os workflows duráveis produzem apenas
um efeito financeiro válido e respondem determinísticamente sob retomada pós-deploy, expiração,
cancelamento, mensagem repetida, WAMID duplicado, concorrência e replay idempotente.

<requirements>
- RF-13: confirmação posterior resolvida pelo workflow pendente, sem nova chamada LLM de escrita
  duplicada.
- RF-14: repetição idempotente informa confirmação sem criar novo registro (efeito financeiro único).
- RF-15: estado de espera persistido no `Snapshot` antes de pedir clarificação/confirmação; retomada por
  merge-patch antes do parse.
- RF-45: pending entry, onboarding, confirmação destrutiva, cadastro de cartão e criação de orçamento
  resistem a retomada pós-deploy, expiração, cancelamento, repetição, WAMID duplicado, concorrência,
  replay; texto determinístico para sucesso/cancelamento/expiração/repetição.
- RF-46: após efetivar/cancelar/expirar, run completa (`Succeeded`/`Failed`), nunca `Suspended`
  (reapers cobrem housekeeping).
</requirements>

## Subtarefas

- [ ] 7.1 Integration (`//go:build integration`, testcontainers Postgres) de retomada pós-deploy +
  merge-patch antes do parse para pending entry.
- [ ] 7.2 Casos de expiração, cancelamento, mensagem repetida e WAMID duplicado (efeito único) por
  workflow (pending, destructive-confirm, card-create, budget-creation, onboarding).
- [ ] 7.3 Concorrência/replay idempotente (`IdempotentWriter` + WriteLedger) → um efeito válido; texto
  determinístico.
- [ ] 7.4 Confirmar que reapers levam runs concluídos a `Succeeded`/`Failed` (nunca `Suspended` órfão).

## Detalhes de Implementação

Ver `techspec.md` → "Testes de Integração" e os estados fechados de espera (`AwaitingSlot`,
`PendingStatus`, `ConfirmState`, `CardCreateState`, `BudgetCreationState`). Idempotência via
`executeWithIdempotency`/`IdempotentWriter` (`pending_entry_workflow.go`), chave
`(userID, wamid, itemSeq, operation, resourceKind)`. TTLs/reapers existentes: confirm 10min,
card-create 15min, pending-entry 35min, budget-creation via `BuildBudgetCreationReaper`. Não alterar
contrato dos workflows; apenas endurecer cobertura (usa o runtime robusto de 2.0).

## Critérios de Sucesso

- Cada workflow: um efeito financeiro válido sob repetição/replay/concorrência; texto determinístico.
- Retomada pós-deploy aplica merge-patch antes do parse; estado de espera persistido antes da pergunta.
- Nenhum run permanece `Suspended` após efetivar/cancelar/expirar.
- Integration verde com testcontainers; `go test -race` verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — endurece workflows duráveis (suspend/resume, pending step, idempotência) do stack mecontrola.
- `postgresql-production-standards` — testes de integração com Postgres real (testcontainers) para persistência/idempotência.

## Testes da Tarefa

- [ ] Testes unitários: decisões puras de confirmação/idempotência já cobertas; complementar gaps.
- [ ] Testes de integração: retomada pós-deploy, expiração, cancelamento, WAMID duplicado, concorrência,
  replay por workflow (testcontainers Postgres).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/*_workflow.go`
- `internal/agents/application/workflows/*_state.go`
- `internal/agents/infrastructure/**` (idempotent writer / stores) e testes de integração
