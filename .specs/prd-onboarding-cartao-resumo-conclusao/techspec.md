<!-- spec-hash-prd: e224d1c169b0515b28bde355eb50b3bec8b2ae0f74605a747a428422c62d0f3d -->

# Especificação Técnica — Onboarding: Cartão por Extenso, Exemplo de Cadastro e Resumo/Conclusão Final

> PRD: `.specs/prd-onboarding-cartao-resumo-conclusao/prd.md` (spec-version 1).
> Base: `internal/agents/application/workflows/onboarding_workflow.go` (workflow durável, consumidor Mastra sobre `internal/platform/{agent,workflow,memory}`).
> ADRs: [adr-001](./adr-001-conclusao-le-fontes-de-verdade.md), [adr-002](./adr-002-validacao-realllm-extracao-e-unit-copy.md), [adr-003](./adr-003-nao-aplicar-design-pattern.md).

## Resumo Executivo

A mudança é inteiramente de copy e de montagem de mensagem dentro do consumidor `internal/agents`, sem tocar o kernel de workflow, os schemas de extração LLM, as regras de orçamento/recorrência ou a idempotência de escrita. Três funções de prompt da etapa de cartões passam a usar "cartão 💳" por extenso, a destacar apenas a palavra "outro" em negrito no convite ao próximo cartão e a exibir um exemplo exato de cadastro (com e sem apelido, "dia 1"/"dia primeiro"). O passo de conclusão (`BuildConclusionStep`), hoje restrito a `WorkingMemory`, passa a receber também `CardManager` e `BudgetPlanner` — ambos já disponíveis no mesmo ponto de montagem do workflow (`module.go:231`) — para montar um bloco "Resumo de Onboarding" (objetivo, meta, orçamento, distribuição, cartões cadastrados ou "nenhum cartão 💳", recorrência) que antecede a frase de conclusão existente. A mensagem final continua fluindo pelo caminho atual (`FinalMessage` → `sendReply` → `NormalizeOutboundText`), que já converte `**` no negrito do WhatsApp e prefixa `📊` ao título "Resumo de Onboarding".

Nenhum novo estado de domínio, tabela, migração ou design pattern é introduzido (ADR-003). A distribuição e a lista de cartões são lidas das fontes de verdade (`BudgetPlanner.SuggestAllocation`, `CardManager.ListCards`) para refletir exatamente o que foi persistido (ADR-001), reutilizando os helpers `renderAllocationLines`, `categoryLabels`, `canonicalSlugs` e `money.FromCents(...).BRL()`. A prova de aceite combina testes unitários exact-copy (determinístico) com o harness real-LLM já existente do onboarding (`onboarding_workflow_integration_test.go`, `RUN_REAL_LLM=1`) para as partes que dependem do modelo (ADR-002).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados (todos em `internal/agents/application/workflows/onboarding_workflow.go`, salvo indicação):

- `cardsPrompt(existing int) string` (`:605-610`) — **modificado**: usa "cartão 💳"; ramo `existing > 0` produz a frase exata "Deseja cadastrar **outro** cartão 💳 agora?" com exemplo; ramo `existing == 0` inclui exemplo com e sem apelido.
- `cardsReprompt` e variantes `cardsRepromptMissingName|MissingDueDay|MissingBoth` (`:547-566`) — **modificado**: cada uma usa "cartão 💳" e inclui exemplo pertinente ao dado faltante.
- `conclusionFinalMessage(goal string, valueCents int64) string` (`:635-645`) — **inalterado** (continua retornando a cauda "Tudo pronto! 🚀 …"); preserva o teste exact-copy existente.
- `conclusionSummaryMessage(state OnboardingState, items []interfaces.AllocationCents, cards []interfaces.Card) string` — **novo**: função pura que monta o bloco "Resumo de Onboarding" e concatena a cauda de `conclusionFinalMessage`.
- `BuildConclusionStep(workingMem memory.WorkingMemory, budgets interfaces.BudgetPlanner, cards interfaces.CardManager)` (`:1036`) — **assinatura ampliada**: passa a resolver `SuggestAllocation` + `ListCards` e a compor `state.FinalMessage` via `conclusionSummaryMessage`.
- `BuildOnboardingWorkflow(...)` (`:1089-1115`) — **modificado**: encaminha `budgets` e `cards` (já recebidos) para `BuildConclusionStep` no registro do `stepConclusionID` (`:1110`).

Componentes reutilizados sem alteração: `renderAllocationLines` (`:412-420`), `allocationBPList` (`:396-402`), `categoryLabels`/`canonicalSlugs` (`:43-57`), `money.FromCents(...).BRL()`, `normalizeCardExtract` (`:370-380`, herança apelido←banco), `NormalizeOutboundText` (`internal/platform/whatsapp/formatting/normalize.go:5-10`).

Não modificados e fora de escopo: `module.go` (já passa `cardManager` e `budgetPlanner` a `BuildOnboardingWorkflow`, `:231`), `resolve_onboarding_or_agent.go` (`OnboardingResult.Message` = `FinalMessage`, `:151`), `whatsapp_inbound_consumer.go` (`sendReply` + normalização, `:301,338`), `internal/platform/workflow` (kernel), schemas de extração LLM.

### Fluxo de dados

```
step-cards (loop suspend/resume) -> cria cartão(ões) via CardManager.CreateCard (inalterado)
  -> usuário recusa próximo cartão -> CardsDone=true -> completeStep
step-conclusion (último passo, BuildConclusionStep):
  1. WorkingMemory.Upsert/UpsertMetadata (inalterado)
  2. budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(state.Allocations)) -> []AllocationCents
  3. cards.ListCards(ctx, userUUID) -> []interfaces.Card
  4. state.FinalMessage = conclusionSummaryMessage(state, items, cards)
  5. completeStep(state)
resolve_onboarding_or_agent.resume -> OnboardingResult{Done:true, Message: FinalMessage}
whatsapp_inbound_consumer.sendReply -> NormalizeOutboundText (** -> *, "Resumo de Onboarding" -> "📊 Resumo de Onboarding") -> gateway.SendTextMessage
```

## Design de Implementação

### Interfaces Chave

Nenhuma interface nova. Assinaturas já existentes que passam a ser consumidas pela conclusão (evidência de subagentes):

```go
// internal/agents/application/interfaces/card_manager.go:11
ListCards(ctx context.Context, userID uuid.UUID) ([]Card, error)

// internal/agents/application/interfaces/budget_planner.go:17
SuggestAllocation(ctx context.Context, totalCents int64, allocations []AllocationBP) ([]AllocationCents, error)
```

Assinatura ampliada do builder (única mudança estrutural de assinatura):

```go
// onboarding_workflow.go:1036 (antes: apenas workingMem)
func BuildConclusionStep(
    workingMem memory.WorkingMemory,
    budgets interfaces.BudgetPlanner,
    cards interfaces.CardManager,
) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error)
```

### Modelos de Dados

Sem novos tipos de domínio, sem novo estado fechado, sem migração. Tipos lidos (já existentes):

```go
// internal/agents/application/interfaces/types.go:136-146
type Card struct {
    ID, UserID       string
    Nickname, Bank   string
    ClosingDay       int
    DueDay           int
    BestPurchaseDay  int
    CreatedAt, UpdatedAt time.Time
}

// internal/agents/application/interfaces/types.go:256-260
type AllocationCents struct {
    RootSlug     string
    BasisPoints  int
    PlannedCents int64
}
```

### Copy — especificação exata (fonte da verdade dos testes exact-copy)

Convenções: negrito no prompt via `**...**` (o normalizador converte para `*...*` do WhatsApp); "cartão" sempre com acento e acompanhado de 💳 (RF-01/RF-02/RF-03); nenhum outro emoji para cartão.

- `cardsPrompt(existing > 0)` (RF-04, RF-05):
  Deve conter a substring literal `Deseja cadastrar **outro** cartão 💳 agora?` (apenas "outro" em negrito; "Deseja" mantém a inicial maiúscula), seguida de exemplo com e sem apelido em ambos os formatos de dia, por exemplo: `"Roxinho, Nubank e vencimento dia 1"` e `"Nubank e vencimento dia primeiro"`, e a saída `responda "não"`.
- `cardsPrompt(existing == 0)` (RF-05, RF-08):
  Deve usar "cartão 💳", trazer exemplo com apelido e exemplo sem apelido (comunicando que sem apelido o apelido do cartão 💳 fica igual ao banco) e a saída para recusar.
- `cardsRepromptMissingBoth` (RF-06, RF-07, RF-08): "cartão 💳" + exemplo com e sem apelido + ambos os formatos de dia.
- `cardsRepromptMissingName` (RF-06, RF-08): "cartão 💳" + exemplo de banco/apelido (com nota de herança apelido←banco).
- `cardsRepromptMissingDueDay` (RF-06, RF-07): "cartão 💳" + exemplo de dia em ambos os formatos ("dia 1"/"dia primeiro").

- `conclusionSummaryMessage` (RF-10..RF-16) — estrutura exata do bloco:
  - Linha de título literal `Resumo de Onboarding` (recebe `📊` automático via normalizador — RF-13).
  - `🎯 Objetivo: <goal>` e, quando `GoalValueCents > 0`, ` (meta de <BRL>)` (RF-12).
  - `💵 Orçamento mensal: <BRL>` (RF-12, RF-15).
  - `Distribuição:` seguido de `renderAllocationLines(items)` (5 categorias com rótulo+emoji, valor BRL e `%` — RF-12, RF-15).
  - `Cartões 💳:` seguido de uma linha por cartão. Formato por cartão: quando `Nickname == Bank`, `- <Bank> — vencimento dia <DueDay>`; caso contrário `- <Nickname> (<Bank>) — vencimento dia <DueDay>`. Quando não houver cartões: linha única `Nenhum cartão 💳 cadastrado.` (RF-12, RF-16).
  - `🔁 Recorrência: ligada (repete pelos próximos 12 meses)` quando `state.Recurrence`, senão `🔁 Recorrência: desligada` (RF-12).
  - Linha em branco e a cauda inalterada de `conclusionFinalMessage(state.Goal, state.GoalValueCents)`.

### Tratamento de erros

`SuggestAllocation` e `ListCards` na conclusão seguem o mesmo padrão já vigente para IO do passo de conclusão (o `WorkingMemory.Upsert` atual retorna `failStep` com erro embrulhado, `:1039-1041`): em falha, retornar `failStep(state, fmt.Errorf("agents.onboarding.conclusion: <op>: %w", err))`. O workflow é durável com `MaxAttempts: 3` (`:1113`); a retomada reexecuta o passo. Não há degradação silenciosa (não emitir resumo parcial/errado), preservando RF-14 (reflexo exato) e evitando falso positivo de conclusão com dados incompletos.

## Pontos de Integração

Nenhuma nova integração externa. As chamadas `SuggestAllocation` e `ListCards` são bindings internos já usados em outros passos do onboarding (`SuggestAllocation` em `budget-review`, `:882,904,944`; `ListCards` em `step-cards`, `:775,821`).

## Abordagem de Testes

### Testes Unitários (determinístico — exact-copy)

Arquivo: `internal/agents/application/workflows/onboarding_workflow_test.go` (whitebox `package workflows`, testify/suite, mocks mockery via `SetupTest`).

Novos/alterados:
- Copy de cartão: asserts exatos de que `cardsPrompt(1)` contém `Deseja cadastrar **outro** cartão 💳 agora?` e os exemplos; `cardsPrompt(0)` contém "cartão 💳" + exemplo com/sem apelido; cada `cardsRepromptFor(...)` contém "cartão 💳" e o exemplo pertinente. **Atualizar** dois testes load-bearing da copy antiga: `onboarding_workflow_test.go:1959` (`s.Contains(cardsPrompt(1), "OUTRO 💳")`) e `onboarding_workflow_integration_test.go:657` (`require.Contains(..., "OUTRO 💳")` em `TestCardFlow_Integration`, mock-based, `//go:build integration`); os asserts de 💳 em `:1952,1966,1967,1815,1832` continuam válidos (o emoji permanece).
- Migração obrigatória de assinatura: as 5 chamadas existentes de `BuildConclusionStep(s.wmMock)` em `onboarding_workflow_test.go:1980,2006,2027,2050,2070` DEVEM migrar para os 3 args (`workingMem, budgets, cards`) com expectativas de `SuggestAllocation`/`ListCards` adicionadas — sem isso o binário de teste whitebox (sem build tag) não compila. Atenção especial a `TestBuildConclusionStep_DoesNotReopenDistributionSummaryOrActivation` (`:2050`), que hoje só mocka WM.
- Resumo/conclusão: novo teste do passo `BuildConclusionStep` com `BudgetPlanner`/`CardManager` mockados (SuggestAllocation retornando as 5 `AllocationCents`; ListCards retornando 0, 1 e ≥2 cartões) assertando que `state.FinalMessage` contém "Resumo de Onboarding", objetivo, orçamento BRL, as 5 categorias, cada cartão no formato especificado (ou "Nenhum cartão 💳 cadastrado.") e a linha de recorrência. Cobrir os três desfechos (sem cartão, um cartão, múltiplos cartões — RF-10, RF-11, RF-16).
- Não-regressão: manter `TestConclusionFinalMessage_WithoutValueMatchesPreviousBehavior` (`:2083-2092`) verde (a função `conclusionFinalMessage` permanece inalterada); manter os asserts de `FinalMessage`-contém-objetivo (`:1987,2011`).
- Herança apelido←banco: manter/estender teste de `normalizeCardExtract` garantindo `nickname == bank` quando só o banco é informado (RF-09).

Mocks (mockery) a usar: `interfacemocks.CardManager` (`internal/agents/application/interfaces/mocks/card_manager.go`), `interfacemocks.BudgetPlanner` (`.../budget_planner.go`), `memorymocks.WorkingMemory` (`internal/platform/memory/mocks/working_memory.go`), `agentmocks.Agent` (`internal/platform/agent/mocks/agent.go`).

### Testes de Integração / Real-LLM (RF-18)

Critérios (fronteira LLM real): a extração de cartão depende do modelo; mocks não bastam (memória de projeto: real-LLM obrigatório). Harness existente: `internal/agents/application/workflows/onboarding_workflow_integration_test.go` (`//go:build integration`, `package workflows_test`, gate `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`, provider real via `llm.NewOpenRouterProvider`, padrão `TestGoalValueCombinedExtractionGate`).

Modelo do gate (variável de ambiente oficial, verificado): o gate roda com `AGENT_HARNESS_MODEL`, cujo default é `openai/gpt-4o-mini`. Esse é exatamente o modelo do agente/onboarding em produção — `AGENT_LLM_PRIMARY_MODEL=openai/gpt-4o-mini` (`.env.example:227`; `configs/config.go:183,1347`, obrigatório em produção `:1047`; confirmado no container de produção em `root@187.77.45.48`). Não existe modelo de onboarding separado. Portanto o gate é prova fiel: valida a extração no mesmo modelo que atende os usuários, sem trade-off de fidelidade.

Novo cenário `TestCardExtractionRealLLMGate` como **método de suite** em `OnboardingWorkflowRealLLMSuite` (o provider real vem do `SetupTest`; método existente de referência: `TestCardExtractionGate`, `:553`), dirigindo `BuildCardsStep` com o agente real (`agents.BuildMeControlaAgent(provider, ...)`) e um `CardManager` mockado que captura os argumentos de `CreateCard`, cobrindo:
- `"Nubank e vencimento dia primeiro"` → cria cartão com `DueDay == 1` e `Nickname == "Nubank"` (RF-07 ordinal + RF-09 herança).
- `"Roxinho, Nubank e vencimento dia 1"` → `Nickname == "Roxinho"`, `Bank == "Nubank"`, `DueDay == 1` (RF-05/RF-07).
- `"Nubank e vencimento dia 1"` → `DueDay == 1`, `Nickname == "Nubank"`.

Gate de aceite (caminho de subteste da suite — `-run` com só o nome do método não casa e passaria vazio/falso-positivo): `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration -run 'TestOnboardingWorkflowRealLLMSuite/TestCardExtractionRealLLMGate' ./internal/agents/application/workflows -v` verde. Além disso, a validação final (task 4.0) roda o pacote de integração completo `go test -tags integration ./internal/agents/application/workflows/... -v` para compilar e exercitar `TestCardFlow_Integration` (que contém o assert `:657` a ser atualizado). Harness golden do agente (`internal/agents/application/golden`, `TestGoldenSetGate`, limiar `0.90`) permanece verde (a copy determinística não é gerada pelo agente; nenhum caso golden novo é obrigatório, mas o gate não pode regredir).

### Testes E2E

Não aplicável além do harness real-LLM do onboarding acima, que exercita o fluxo durável ponta a ponta com o modelo real.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Copy da etapa de cartões (`cardsPrompt`, reprompts) + atualização dos testes exact-copy — isolado, baixo risco (RF-01..RF-08).
2. `conclusionSummaryMessage` (função pura) + testes unitários dos três desfechos (RF-10..RF-16).
3. Ampliar `BuildConclusionStep` e o wiring em `BuildOnboardingWorkflow` (`:1110`) para injetar `budgets`+`cards`; ajustar testes do passo (RF-14, RF-17).
4. Cenário real-LLM `TestCardExtractionRealLLMGate` (RF-07/RF-09) e execução do gate golden do agente (RF-18).
5. Validação R0-R7 + gates de governança + gofmt/vet/lint/test -race.

### Dependências Técnicas

Nenhuma dependência de infraestrutura nova. `cardManager` e `budgetPlanner` já construídos e em escopo no ponto de montagem (`module.go:150-162,231`). Execução do gate real-LLM exige `OPENROUTER_API_KEY`.

## Monitoramento e Observabilidade

Sem novas métricas. As existentes cobrem o comportamento: `workflow_steps_total{workflow="onboarding-workflow",step="step-conclusion",status}` e `onboarding_workflow_total{status}` continuam refletindo sucesso/falha do passo de conclusão (a falha de `SuggestAllocation`/`ListCards` incrementa `status="failed"` e é observável, coerente com R-AGENT-WF-001.5). Cardinalidade preservada (sem `user_id`/`category_id` em labels). Validação em produção pós-deploy: inspecionar `platform_messages`/entrega do onboarding de um usuário real (como na coleta via `otel-lgtm`/Postgres em `root@187.77.45.48`) confirmando a nova copy e o bloco "📊 Resumo de Onboarding".

## Considerações Técnicas

### Decisões Chave

- ADR-001: o passo de conclusão lê distribuição e cartões das fontes de verdade (`SuggestAllocation`, `ListCards`) em vez de recomputar localmente ou carregar no estado — garante reflexo exato (RF-14) e reuso de `renderAllocationLines`.
- ADR-002: validação de aceite divide-se em exact-copy determinístico (unit) para a copy e real-LLM (`onboarding_workflow_integration_test.go`) para a extração dependente do modelo — satisfaz RF-18 sem falso positivo, dado que a copy não é gerada por LLM.
- ADR-003: não aplicar design pattern (mudança localizada de copy + função pura de formatação); o seletor determinístico não identificou sinal estrutural canônico.

### Riscos Conhecidos

- Testes de copy load-bearing quebram ao mudar strings (`:1959`, e asserts de `FinalMessage`): mitigado tornando a atualização desses testes parte explícita do escopo (não é regressão oculta).
- Divergência de arredondamento na distribuição do resumo vs. a revisão pré-ativação: mitigado reutilizando `SuggestAllocation` (mesma fonte usada na revisão), garantindo renderização idêntica (ADR-001).
- Falha de IO na conclusão bloqueando o encerramento: mitigado pelo `MaxAttempts: 3` do workflow durável e pela observabilidade de `status="failed"`; comportamento idêntico ao já existente para `WorkingMemory.Upsert`.
- Modelo real-LLM interpretar "dia primeiro" incorretamente: coberto pelo gate `TestCardExtractionRealLLMGate`; falha do gate bloqueia a entrega.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001.1): zero comentários no código Go alterado.
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): comportamento no consumidor `internal/agents`, sem `switch case intent.Kind`; LLM apenas nas call-sites já sancionadas (extração no passo de cartões, inalterada); Run auditável preservado.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): `internal/platform/workflow` não é tocado.
- `.claude/rules/go-testing.md` (R-TESTING-001): suíte whitebox testify/suite com mocks mockery e `fake.NewProvider()`.
- DMMF (domain-modeling-production): nenhum estado de domínio novo; o resumo é projeção de estado/fontes de verdade existentes; funções de montagem são puras.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/onboarding_workflow.go` — modificado (copy de cartões, `conclusionSummaryMessage`, `BuildConclusionStep`, wiring `BuildOnboardingWorkflow`).
- `internal/agents/application/workflows/onboarding_workflow_test.go` — modificado (exact-copy + resumo/conclusão; atualizar `:1959`).
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — modificado (novo `TestCardExtractionRealLLMGate`).
- `internal/agents/application/interfaces/{card_manager.go,budget_planner.go,types.go}` — lidos (sem alteração).
- `internal/platform/whatsapp/formatting/normalize.go` — dependência (sem alteração; já suporta `**`→`*` e `📊 Resumo de Onboarding`).
- `internal/agents/module.go` — dependência de wiring (sem alteração; já passa `cardManager` e `budgetPlanner`).
- `internal/agents/application/golden/` — gate golden do agente (não regredir).
</content>
