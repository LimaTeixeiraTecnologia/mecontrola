# Tarefa 1.0: Orçamento — validação simétrica e personalização preservada

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Eliminar o falso sucesso da personalização de orçamento: a distribuição confirmada pela cliente que fecha 100% deve persistir exatamente, e a distribuição padrão nunca pode sobrescrever uma personalização válida. A correção impõe o invariante `Σ basisPoints == 10000` de forma simétrica em profundidade (domínio autoritativo + workflow como guarda de UX), reforça a extração LLM com uma guarda determinística obrigatória e reusa o estado de espera tipado existente. Detalhes em `techspec.md` e `adr-003-validacao-simetrica-distribuicao-orcamento.md` — não duplicar aqui.

<requirements>
- RF-01, RF-02, RF-03, RF-04, RF-29 conforme `prd.md`.
- Decisões firmes do ADR-003 (vetores a/b/c + reuso do estado de espera).
- Sem novo design pattern GoF — refactor local (operador, guarda de ramo, função pura).
- DMMF: `Decide*` puro, invariante como state-as-type, smart constructor no domínio.
- Zero comentários em Go de produção (R-ADAPTER-001.1); `errors.Join` preservado.
- Nenhum `SuspendReason`/`PendingStatus` novo — reuso de `AwaitingBudgetDistribution` e `methodologyReprompt`.
</requirements>

## Subtarefas

- [ ] 1.1 (Vetor c — domínio) Alterar `NewCreateBudgetCommand` (`internal/budgets/domain/commands/create_budget.go:69`) de `sumBP > 10000` para `sumBP != 10000`, preservando `errors.Join` e a sentinela `ErrCommandInvalidAllocation`. Alinha o smart constructor a `Budget.Activate`/`RebalanceAllocations` (que já usam `!= 10000`); nenhum draft parcial fica gravável.
- [ ] 1.2 (Vetor b — não sobrescrever personalização) No ramo `allocationInputConfirm` de `DecideAllocationsBP` (`internal/agents/application/workflows/onboarding_workflow.go` ~L212), aplicar `defaultDistributionBP` apenas quando `sum(valuesBySlug) == 0`; se `action == "confirm"` com valores não-nulos, retornar erro de reprompt (RF-02), impedindo o padrão de descartar a distribuição enviada.
- [ ] 1.3 (Vetor a — endurecer extração) Reforçar `allocationInputSystemPrompt` (~L446-450) com a regra de desambiguação reais vs percent ancorada no caso real, E adicionar a função pura determinística OBRIGATÓRIA `DecideAllocationKind(raw, incomeCents) allocationInputKind` que reclassifica por invariante numérica quando o `action` do LLM vier ambíguo/incompatível (soma≈renda ⇒ `reais`; soma≈100 ⇒ `percent`), sem coagir uma categoria na outra.
- [ ] 1.4 (Step) Garantir que `BuildDistributionStep` trate e propague o erro de `NewCreateBudgetCommand`, mapeando para reprompt/`StepStatusFailed` observável — sem erro engolido.
- [ ] 1.5 (Estado de espera — reuso) Confirmar que o run suspende via `workflow.SuspendAwaitingInput` reusando `AwaitingBudgetDistribution` (budget-creation) e `methodologyReprompt` (onboarding); nenhum `SuspendReason`/`PendingStatus` novo.
- [ ] 1.6 (Testes) Ver seção "Testes da Tarefa".

## Detalhes de Implementação

Ver `techspec.md` e `adr-003-validacao-simetrica-distribuicao-orcamento.md` desta pasta — **referenciar em vez de duplicar**. ADR-003 fixa os quatro pontos de mudança (Plano de Implementação, itens 1-6), o invariante único `Σ basisPoints == 10000` imposto no domínio como autoridade e no workflow como guarda de UX antecipada, e a obrigatoriedade da guarda determinística `DecideAllocationKind` (a classificação LLM é não-determinística e foi o vetor real da falha). RISCO registrado no ADR-003: `DecideAllocationsBP` e helpers são COMPARTILHADOS entre onboarding e budget-creation — testar AMBOS os fluxos.

## Critérios de Sucesso

- Caso real preservado EXATO: Custo Fixo 2500 / Conhecimento 0 / Prazeres 500 / Metas 0 / Liberdade 2000 sobre renda 500000 cents ⇒ basis points 5000/0/1000/0/4000 (fecha 100%) persiste exatamente; nunca `4000/1000/1000/1000/3000`.
- Distribuição que não fecha 100% ⇒ orçamento pendente (sem ativação parcial) + reprompt de correção com estado de espera tipado.
- `NewCreateBudgetCommand` rejeita `sumBP == 9000` e `sumBP == 11000`; aceita `sumBP == 10000`.
- `action == "confirm"` com valores não-nulos ⇒ erro de reprompt; distribuição padrão nunca sobrescreve personalização válida.
- Gates Go verdes no escopo alterado (build, vet, `test -race`, lint quando disponível) e gates de governança limpos (R-ADAPTER-001, DMMF state-as-type).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — `Decide*` puro, invariante `Σbp==10000` como state-as-type e smart constructor do command.
- `mastra` — workflows de onboarding/budget-creation e extração structured output do substrato.
- `design-patterns-mandatory` — confirmar gate: sem novo GoF pattern (refactor local: operador, guarda de ramo, função pura).

## Testes da Tarefa

- [ ] Testes unitários — puros, sem mock: `DecideAllocationsBP` (ramo `confirm` com valores não-nulos ⇒ reprompt; ramo `confirm` com soma zero ⇒ default), `DecideDistribution`/`DecideBudgetDistribution` (rejeitam `< 10000` e `> 10000`), `DecideAllocationKind` (reclassifica soma≈renda ⇒ reais, soma≈100 ⇒ percent), `NewCreateBudgetCommand` (rejeita 9000 e 11000, aceita 10000). Cobrir o caso real nos DOIS fluxos (onboarding e budget-creation), pois `DecideAllocationsBP` é compartilhado.
- [ ] Testes de integração — Postgres: assert `budgets_allocations == distribuição enviada` (5000/0/1000/0/4000) e NUNCA `4000/1000/1000/1000/3000` quando a cliente personalizou.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/domain/commands/create_budget.go` — smart constructor `NewCreateBudgetCommand` (vetor c, L69).
- `internal/budgets/domain/entities/budget.go` — `Budget.Activate`/`RebalanceAllocations` (`== 10000`), referência de simetria.
- `internal/agents/application/workflows/onboarding_workflow.go` — `DecideAllocationsBP`, ramo `allocationInputConfirm`, `allocationInputSystemPrompt`, `DecideAllocationKind` (vetores a/b).
- `internal/agents/application/workflows/budget_creation_workflow.go` — fluxo budget-creation (consumidor compartilhado de `DecideAllocationsBP`, `BuildDistributionStep`).
- `internal/agents/application/workflows/budget_creation_decisions.go` — `DecideBudgetDistribution` (`== 10000`).
