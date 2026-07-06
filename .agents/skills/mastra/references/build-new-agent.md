# Construir ou Estender Consumidor Agentivo

Use `internal/agents` como molde real. Ele é o consumidor financeiro `mecontrola`, não um exemplo histórico externo. Crie comportamento novo montando primitivos do substrato e preservando DI manual explícita.

## Estrutura Real

```text
internal/agents/
  application/
    agents/       BuildMeControlaAgent, scoring hooks
    tools/        BuildXTool com tool.NewTool[I,O]
    workflows/    onboarding e confirmação destrutiva
    scorers/      BuildMeControlaScorers
    usecases/     HandleInbound, onboarding resolver, idempotent write
    interfaces/   portas consumidas pelas tools/use cases
    dtos/input/   input DTOs com Validate()
  infrastructure/
    binding/      adapters para módulos financeiros
    messaging/database/consumers/
    jobs/handlers/
    persistence/
  module.go       composition root
```

Para outro consumidor, replique o padrão de camadas e DI, mas não copie dependências financeiras que não existam no novo contexto.

## Tool Financeira

Modelo real: `BuildRegisterExpenseTool`.

```go
func BuildRegisterExpenseTool(registrar entryRegistrar) tool.ToolHandle {
    in := llm.Schema{Name: "register_expense_input", Strict: true, Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "amountCents": map[string]any{"type": "integer"},
            "description": map[string]any{"type": "string"},
            "paymentMethod": map[string]any{"type": "string", "enum": []string{"pix", "debit_card", "credit_card"}},
        },
        "required": []string{"amountCents", "description", "paymentMethod"},
        "additionalProperties": false,
    }}
    out := llm.Schema{Name: "register_expense_output", Strict: true, Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "resourceId": map[string]any{"type": "string"},
            "kind": map[string]any{"type": "string"},
            "isReplay": map[string]any{"type": "boolean"},
            "outcome": map[string]any{"type": "string"},
        },
        "required": []string{"resourceId", "kind", "isReplay", "outcome"},
        "additionalProperties": false,
    }}
    return tool.NewTool[RegisterExpenseInput, RegisterExpenseOutput]("register_expense", "Registra uma despesa no ledger financeiro do usuário.", in, out, buildRegisterExpenseExec(registrar))
}
```

Regras obrigatórias:

- Use schema estrito e descrição curta, focada no uso correto pelo agente.
- Delegue para use case ou interface da aplicação.
- Para escrita, leia `agent.InboundIdentityFromContext(ctx)` e preserve idempotência.
- Traduza erro com contexto e `%w`.
- Registre tools de escrita no `agent.WithWriteToolSet`.

## Agent

Modelo real: `BuildMeControlaAgent`.

```go
func BuildMeControlaAgent(provider llm.Provider, tools []tool.ToolHandle, hooks agent.Hooks, o11y observability.Observability) agent.Agent {
    opts := []agent.AgentOption{
        agent.WithMaxToolRounds(12),
        agent.WithDefaultMaxTokens(1536),
    }
    if len(tools) > 0 {
        opts = append(opts, agent.WithTools(tools...))
    }
    if hooks != nil {
        opts = append(opts, agent.WithHooks(hooks))
    }
    return agent.NewAgent(MecontrolaAgentID, mecontrolaAgentInstructions, provider, o11y, opts...)
}
```

Atualize instructions com extremo cuidado: elas são contrato de comportamento para WhatsApp, seleção determinística de ferramentas, idioma, confirmação e anti-simulação.

## Workflows do Consumidor

Use o kernel para fluxos definidos e retomáveis:

- Onboarding: `BuildOnboardingWorkflow(onboardingAgent, cardManager, budgetPlanner, workingMem, threadGateway, messageStore)`.
- Confirmação destrutiva: `BuildDestructiveConfirmWorkflow(txLedger, cardManager, categoriesReader, recurrenceManager)`.

Não implemente confirmação destrutiva no prompt, handler ou consumer. A tool inicia/sinaliza workflow; o consumer retoma com `Resume` e merge-patch de `resumeText`.

## Wiring Real em `module.go`

Sequência mandatória para `internal/agents`:

```text
validar deps
-> httpclient.NewClient(target=openrouter)
-> llm.NewOpenRouterProvider
-> scorerpostgres.NewResultStore
-> agentscorers.BuildMeControlaScorers
-> scorer.NewScorerRunner
-> agentapplication.NewScoringHooks
-> persistence.NewWriteLedgerRepository
-> usecases.NewIdempotentWrite
-> binding.New*Adapter para categories/card/budgets/transactions/recurrences
-> usecases.NewRegisterEntry
-> workflowpostgres.NewPostgresStore
-> workflow.NewEngine[OnboardingState]
-> workflow.NewEngine[ConfirmState]
-> memorypostgres repositories
-> BuildMeControlaAgent sem tools para onboarding
-> BuildDestructiveConfirmWorkflow
-> buildFinancialTools
-> BuildMeControlaAgent com tools financeiras
-> agent.NewAgentRegistry().Register(meControlaAgent)
-> memory.NewPublishingMessageStore + EmbeddingIndexHandler quando houver outbox
-> BuildOnboardingWorkflow
-> agentpostgres.NewRunStore
-> agent.NewAgentRuntime(..., agent.WithWriteToolSet(...))
-> usecases.NewHandleInbound
-> onboarding/destructive resolvers
-> WhatsApp consumer e route quando houver outbox + gateway
```

Antes de adicionar dependência ao `Deps`, confirme que o módulo/provedor/use case existe. Não crie placeholders para satisfazer wiring.

## Scorers

Use `BuildMeControlaScorers(provider)` como padrão:

- `NewFinancialToolCallAccuracyScorer`: exige ao menos uma tool financeira quando aplicável.
- `NewFinancialCompletenessScorer`: mede presença de linguagem financeira esperada.
- `NewCategorizationScorer`: LLM-judged com JSON estruturado.

Conecte via `NewScoringHooks(scorerRunner)` e chame `Module.Shutdown(ctx)` para drenar workers.

## Persistência

Reuse tabelas `platform_*`: threads, messages, resources, runs, embeddings, workflow snapshots/steps e scorer results. Não crie schema novo para agente sem requisito explícito e aprovação arquitetural.
