# Tarefa 5.0: Workflow destructive_confirm + wiring no module.go

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Compor o workflow único `destructive_confirm` (`platform.Definition[ConfirmState]`) com a sequência
de passos e fazer o wiring em `module.go`: instanciar `Engine[ConfirmState]`, montar os mapas de
`TargetResolver`/`DestructiveExecutor` a partir dos bindings existentes e registrar a definition.

<requirements>
- RF-08: confirmação durável antes de efetivar (workflow + suspend/resume do kernel).
- RF-09: Run durável e retomável após restart/crash.
- RF-12: execução auditável como Run + decision-id (status fechado, duration, erro).
</requirements>

## Subtarefas

- [ ] 5.1 `destructive_confirm.go`: `NewDestructiveConfirmDefinition` = `Sequence(authorize, replay, policy, audit_begin, prepare, confirm, execute, format)`, `Durable=true`, `MaxAttempts` da config.
- [ ] 5.2 `module.go`: instanciar `platform.NewEngine[ConfirmState]` com o store do kernel; montar `Targets`/`Executors` a partir de `LastTransactionDeleterAdapter`, `LastTransactionEditorAdapter`, `CardDeleterAdapter`, `BudgetConfigCommitterAdapter`.
- [ ] 5.3 Expor a definition + engine ao `DailyLedgerAgent` via `KernelDeps`/novo campo, sem quebrar o wiring existente do `transactions_write`.
- [ ] 5.4 `formatDestructiveReply` para a resposta de sucesso por operação.

## Detalhes de Implementação

Ver `techspec.md` seções "Visão Geral dos Componentes" e "Interfaces Chave"
(`NewDestructiveConfirmDefinition`) + ADR-002 (workflow único, ID distinto `destructive_confirm`).
Reusar `SettleRegistry`/`OnSettle` do padrão `transactions_write`. Zero comentários.

## Critérios de Sucesso

- Definition com ID `destructive_confirm` (distinto de `transactions_write`), `Durable=true`.
- Engine `Engine[ConfirmState]` wired no `module.go` com os 4 executores mapeados aos bindings reais.
- Nenhum binding tem assinatura de mutação alterada (reuso 1:1).
- Run auditável: status fechado + duration + decision-id correlacionado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — composição da definition (ordem/IDs dos passos); mapeamento `OperationKind`→executor; wiring resolve sem nil.
- [ ] Testes de integração — não aplicável nesta tarefa (coberto em 7.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/destructive_confirm.go` (novo)
- `internal/agent/module.go` (modificado — engine + mapas)
- `internal/agent/application/services/intent_router.go` (modificado — KernelDeps/novo campo)
- `internal/agent/infrastructure/binding/{transaction_query,cards_write,budget_config}.go` (reuso)
