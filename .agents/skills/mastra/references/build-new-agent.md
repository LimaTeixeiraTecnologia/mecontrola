# Construir um novo agente (molde `internal/agents`)

Receita para criar um agente novo igual ao consumidor de referência `internal/agents` (port weather),
montando os primitivos de `internal/platform/*`. Carregar `go-implementation` antes de qualquer `.go`.

## Estrutura de pastas (espelhe `internal/agents`)

```
internal/<modulo>/
  domain/                          tipos puros, sem IO (ex.: forecast.go, weather_condition.go + *_test.go)
  application/
    agents/    agent.go            BuildXAgent (+ scoring_hooks.go se houver evals)
    tools/     tool.go             BuildXTool (NewTool[I,O])
    workflows/ workflow.go         BuildXWorkflow (Definition[S] durável) — opcional
    scorers/   scorers.go          BuildXScorers — opcional
    usecases/  handle_inbound.go   fronteira fina sobre AgentRuntime
    dtos/input/                    InboundInput com Validate()
  infrastructure/
    <client>/                      clients externos (ex.: weather/client.go)
    messaging/database/consumers/  consumer do evento de entrada
  module.go                        DI manual (NewModule)
```

## 1. Tool (`application/tools/tool.go`)

`NewTool[I, O]` gera o `ToolHandle`. Schemas `llm.Schema` strict; `exec` é adapter fino que delega ao client.

```go
func BuildWeatherTool(client weather.Client) tool.ToolHandle {
    in := llm.Schema{Name: "weather_tool_input", Strict: true, Schema: map[string]any{ ... }}
    out := llm.Schema{Name: "weather_tool_output", Strict: true, Schema: map[string]any{ ... }}
    exec := func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
        lat, lon, name, err := client.Geocode(ctx, in.Location)
        if err != nil {
            return WeatherOutput{}, fmt.Errorf("agents.tool.get_weather: geocode: %w", err)
        }
        forecast, err := client.Forecast(ctx, lat, lon)
        if err != nil {
            return WeatherOutput{}, fmt.Errorf("agents.tool.get_weather: forecast: %w", err)
        }
        return WeatherOutput{ ... Conditions: forecast.Condition.String(), Location: name}, nil
    }
    return tool.NewTool[WeatherInput, WeatherOutput]("get-weather", "Get current weather for a location", in, out, exec)
}
```

Regra: zero regra de negócio, SQL ou branching de domínio na tool (R-ADAPTER-001.2). Cálculo puro fica em `domain/`.

## 2. Agent (`application/agents/agent.go`)

```go
func BuildWeatherAgent(provider llm.Provider, weatherTool tool.ToolHandle, hooks agent.Hooks, o11y observability.Observability) agent.Agent {
    opts := []agent.AgentOption{agent.WithTools(weatherTool)}
    if hooks != nil {
        opts = append(opts, agent.WithHooks(hooks))
    }
    return agent.NewAgent(weatherAgentID, weatherAgentInstructions, provider, o11y, opts...)
}
```

As `instructions` são o system prompt base; o runtime concatena WorkingMemory automaticamente.

## 3. Usecase de inbound (`application/usecases/handle_inbound.go`)

Fronteira fina: abre span, valida o input DTO (R-DTO-VALIDATE-001) e delega ao `AgentRuntime`.

```go
func (uc *HandleInbound) Execute(ctx context.Context, in input.InboundInput) (agent.Outcome, error) {
    ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.handle_inbound")
    defer span.End()
    if err := in.Validate(); err != nil {
        return agent.Outcome{}, err
    }
    outcome, err := uc.runtime.Execute(ctx, agent.InboundRequest{
        ResourceID: in.ResourceID, ThreadID: in.ThreadID, AgentID: in.AgentID,
        Message: in.Message, MessageID: in.MessageID,
    })
    if err != nil {
        span.RecordError(err)
        return agent.Outcome{}, fmt.Errorf("agents.usecase.handle_inbound: %w", err)
    }
    return outcome, nil
}
```

## 4. Wiring (`module.go` — `NewModule`)

Sequência real do consumidor (`internal/agents/module.go`):

```go
provider := llm.NewOpenRouterProvider(httpClient, llm.Config{Model, EmbedModel, APIKey, BaseURL, MaxTokens, Temperature}, o11y)

scorerResultStore := scorerpostgres.NewResultStore(db)
scorerEntries := agentscorers.BuildWeatherScorers(provider)
scorerRunner := scorer.NewScorerRunner(scorerEntries, scorerResultStore, o11y)
scoringHooks := agentapplication.NewScoringHooks(scorerRunner)

weatherTool := agenttools.BuildWeatherTool(deps.WeatherClient)
weatherAgent := agentapplication.BuildWeatherAgent(provider, weatherTool, scoringHooks, o11y)

registry := agent.NewAgentRegistry()
registry.Register(weatherAgent)

threadGateway := memorypostgres.NewThreadRepository(db, o11y)
rawMessageStore := memorypostgres.NewMessageRepository(db, o11y)
workingMem := memorypostgres.NewWorkingMemoryRepository(db, o11y)
semanticRecall := memorypostgres.NewEmbeddingRepository(db, o11y)

messageStore := rawMessageStore
if deps.OutboxPublisher != nil {
    indexPublisher := indexer.NewOutboxMessageIndexPublisher(deps.OutboxPublisher)
    messageStore = memory.NewPublishingMessageStore(rawMessageStore, indexPublisher, deps.LLM.EmbedModel, o11y)
    indexHandler := indexer.NewEmbeddingIndexHandler(provider, semanticRecall, o11y)
    // registrar indexHandler para EventType memory.EventTypeEmbeddingIndex
}

runStore := agentpostgres.NewRunStore(db)
runtime := agent.NewAgentRuntime(registry, threadGateway, messageStore, workingMem, runStore, o11y)
handleInbound := usecases.NewHandleInbound(runtime, o11y)
```

O consumer de entrada (`infrastructure/messaging/database/consumers/`) recebe o evento, monta o
`InboundInput` e chama `handleInbound.Execute`. O `Shutdown` do módulo chama `scorerRunner.Shutdown`.

## 5. Workflow durável (opcional) — `application/workflows/workflow.go`

Use o kernel quando precisar de multi-step com estado/suspend/resume. Ver `workflow-engine.md`.

```go
func BuildWeatherWorkflow(a agent.Agent, client weather.Client, forecastBase string) workflow.Definition[WeatherState] {
    fetchStep := workflow.NewStepFunc[WeatherState](StepFetchWeatherID, BuildFetchWeatherStep(client, forecastBase))
    planStep := workflow.NewStepFunc[WeatherState](StepPlanActivities, BuildPlanActivitiesStep(a))
    return workflow.Definition[WeatherState]{
        ID: WeatherWorkflowID, Root: workflow.Sequence("root", fetchStep, planStep), Durable: true, MaxAttempts: 3,
    }
}
```

Um step pode chamar `a.Stream(...)` (ex.: plan-activities consome o deltas e lê `stream.Result`).

## 6. Persistência

Reuse a migration `000003` (`platform_threads`, `platform_messages`, `platform_resources`,
`platform_runs`, `platform_embeddings`, `platform_scorer_results`). Não crie schema novo de agente;
o substrato já modela tudo de forma genérica.

## Checklist
- Tool fina (zero regra/SQL/branching); cálculo em `domain/`.
- Input DTO com `Validate()` chamado após `defer span.End()`.
- Zero comentários em `.go` (R-ADAPTER-001.1).
- Não tocar o kernel `internal/platform/workflow`.
- Validar com `rules-checklist.md`.

Ver também `scorer-evals.md`, `memory-recall.md`, `llm-structured-output.md`.
