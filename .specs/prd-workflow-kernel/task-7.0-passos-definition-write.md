# Tarefa 7.0: Passos do agent + Definition transactions_write

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os passos finos do agent sobre o kernel e montar o `Definition[ExpenseState]` do write de
transactions, expressando `record-expense` como workflow multi-step real: WriteGuard como passos 1:1,
`resolve_category` com branch + suspend/resume, `persist` e `format`. O `pendingexpense.Draft` vira o
estado serializado do run.

<requirements>
- RF-20: `pendingexpense.Draft` migra para o suspend/resume do kernel, preservando contrato.
- RF-21: WriteGuard expresso como passos 1:1 (Authorize→Replay→Policy→Audit, ordem e short-circuit).
- RF-23: `record-expense` multi-step real (branch de categoria + suspend/resume + guard-steps).
- RF-26: passos reutilizáveis entre workflows (sem duplicar guarda/auditoria/formatação).
- RF-30: LLM proibido nos passos de execução (só no parse).
- RF-31: passos testáveis isoladamente.
- Ver ADR-003 (suspend/resume) e ADR-002 (estado).
</requirements>

## Subtarefas

- [ ] 7.1 `application/workflow/steps/`: `authorize`, `replay`, `policy`, `audit_begin` (settle
  in-process), reusando `authorizeWrite`/`replayDecision`/`policy.Evaluate`/`beginDecisionAudit` —
  short-circuit 1:1 com o `WriteGuard` atual.
- [ ] 7.2 `steps/resolve_category.go`: `Branch` (auto | ambiguous→suspend `category_choice` |
  confirm→suspend `category_confirm`), com handler de resume interpretando a resposta **sem LLM**
  (lógica portada de `resolvePendingCategoryChoice/Confirm`).
- [ ] 7.3 `steps/persist.go` e `steps/format.go`: persist via binding→usecase `log_transaction_from_agent`
  (uow do módulo transactions); format sem regra de domínio.
- [ ] 7.4 `ExpenseState` espelhando `pendingexpense.Draft` (campos exportados) + round-trip `Encode/Decode`.
- [ ] 7.5 `application/workflow/transactions_write.go`: `Definition[ExpenseState]{Durable:true, Root:
  Sequence(authorize, replay, policy, audit_begin, resolve_category, persist, format)}`.
- [ ] 7.6 Testes unitários por passo (testify/suite, whitebox, `fake.NewProvider()`, IIFE) e do Definition.

## Detalhes de Implementação

Ver techspec.md → "Decomposição do fluxo de prova (record-expense) — preservação 1:1", "Resume" e
"Estado do consumidor (agent)". Carregar `mastra`. Tools/steps finos: zero regra de domínio, zero SQL,
sem branching de domínio, sem LLM. `ToolOutcome`/estados fechados.

## Critérios de Sucesso

- `record-expense` roda como Sequence multi-step; guarda preservada 1:1 (ordem + short-circuit).
- Suspensão grava o `Draft` como estado do run; resume reentra sem LLM e conclui `persist`/`format`.
- Cada passo testável isolado; gates `R-AGENT-WF-001`/`R-ADAPTER-001`/`R-WF-KERNEL-001` verdes.
- Nenhum novo `case intent.Kind` em `daily_ledger_agent.go`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — implementação de Workflow/Tool/suspend-resume em `internal/agent` sob R-AGENT-WF-001.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/steps/*.go` (novos)
- `internal/agent/application/workflow/transactions_write.go` (novo)
- `internal/agent/domain/pendingexpense/draft.go` (uso como estado)
- bindings/usecases: `infrastructure/binding/{transaction_log,expense_confirmation,category_error}.go`,
  `application/usecases/log_transaction_from_agent.go` (consumo)
