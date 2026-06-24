# Tarefa 5.0: Limpeza de eventos órfãos cross-module

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Remover producers/consumers de eventos sem par (inclusive cross-module: agent, transactions, budgets),
confirmados por **constante de event-type** (não por nome de arquivo), preservando os eventos que têm
par (guarda anti-falso-positivo).

<requirements>
- RF-41: eventos órfãos removidos neste PRD, inclusive cross-module; remoção precedida de guarda anti-falso-positivo por constante de event-type; eventos com par são mantidos.
</requirements>

## Subtarefas

- [ ] 5.1 Confirmar por constante + contagem de consumers reais (excluindo testes/e2e): `agent.intent.rejected.v1` (e `agent.intent.executed.v1`), `budgets.budget_activated.v1`, `transactions.recurring_template.{created,updated,deleted}.v1`.
- [ ] 5.2 Remover `agent.intent.rejected`/`executed` órfãos: producer + interface + wiring em `intent_event_publisher.go`/`module.go`.
- [ ] 5.3 Remover `budgets.budget_activated.v1`: producer + interface `BudgetActivatedPublisher` + chamadas em `activate_budget.go` e `edit_category_percentage.go` + wiring `budgets/module.go` (respeitar R-TXN-WORKFLOWS-001).
- [ ] 5.4 Remover `transactions.recurring_template.*`: producer + interface + mock + 3 usecases param de publicar + wiring `transactions/module.go`.
- [ ] 5.5 Guarda anti-falso-positivo: confirmar e **MANTER** `onboarding.splits_calculated`, `onboarding.card_registered`, `external.expense.v1`, `transactions.card_purchase.deleted.v1` (têm par).
- [ ] 5.6 `deadcode`/`staticcheck` para resíduo; `go build`/`go test` verdes.

## Detalhes de Implementação

Ver `adr-007-orphan-events-cleanup.md` (tabelas [REMOVER]/[MANTER]) e techspec §"Riscos: falso
positivo em evento órfão". Confirmar a string literal exata antes de cada remoção.

## Critérios de Sucesso

- Zero producer sem consumer e zero consumer sem producer (confirmado por constante).
- Eventos com par preservados (nenhum falso positivo removido).
- Build/test verdes; outbox sem tipos órfãos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — remove eventos do `internal/agent` (`agent.intent.*`) e coordena o contrato de eventos do agent (R-AGENT-WF-001.5).

## Testes da Tarefa

- [ ] Testes unitários (publishers/consumers afetados; ausência de regressão nos eventos mantidos).
- [ ] Testes de integração (outbox sem tipos removidos; consumers mantidos seguem processando).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/infrastructure/events/intent_event_publisher.go`
- `internal/budgets/infrastructure/messaging/database/producers/budget_activated_publisher.go`
- `internal/transactions/infrastructure/messaging/database/producers/recurring_template_event_publisher.go`
- `internal/{budgets,transactions}/module.go` (wiring)
