# Agent Tool-Calling Loop — Nivel Micro

> **Nota de descontinuacao.** O mecanismo antigo de roteamento por enum (roteador de intent com 17 kinds e um handler por kind, agente financeiro de ledger diario e rascunho de despesa pendente) foi **removido** junto com o antigo modulo de agente financeiro (no singular). Nao ha mais dispatch por enum. O equivalente atual e o **loop de tool-calling** do `Agent` da plataforma (`internal/platform/agent`), resolvido por `AgentRegistry` e validado no consumidor de referencia `internal/agents` (port do exemplo Weather do Mastra). Este documento descreve esse mecanismo novo. O nome do arquivo e mantido por compatibilidade de links.

Documentacao detalhada do loop de tool-calling executado por `Agent.Execute` dentro do `AgentRuntime`, cobrindo a resolucao por `AgentRegistry`, a invocacao do LLM, o ciclo de tool calls e a persistencia de mensagens.

## Referencias de codigo

| Componente | Arquivo |
|---|---|
| AgentRuntime | `internal/platform/agent/runtime.go` |
| Agent (loop tool-calling) | `internal/platform/agent/agent.go` |
| AgentRegistry | `internal/platform/agent/registry.go` |
| RunStore | `internal/platform/agent/infrastructure/postgres/` |
| ToolHandle | `internal/platform/tool/tool.go` |
| Tool Registry | `internal/platform/tool/registry.go` |
| LLM Provider | `internal/platform/llm/provider.go` |
| Memory ports | `internal/platform/memory/ports.go` |
| HandleInbound use case | `internal/agents/application/usecases/handle_inbound.go` |
| WhatsApp inbound consumer | `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` |
| Weather agent (build) | `internal/agents/application/agents/agent.go` |
| Weather tool (build) | `internal/agents/application/tools/tool.go` |

---

## Resolucao e Loop de Tool-Calling

```mermaid
flowchart TD
    START([InboundRequest]) --> RT["AgentRuntime.Execute\nResourceID, ThreadID, AgentID, Message, MessageID"]

    RT --> TH["ThreadGateway.GetOrCreate(resourceID, threadID)\n-> platform_threads"]
    TH --> RUN["RunStore.Insert(Run status=running)\n-> platform_runs"]
    RUN --> RES["AgentRegistry.Resolve(agentID)"]

    RES -->|encontrado| AG["Agent.Execute(ctx, req)"]
    RES -->|nao encontrado| ERR["erro: agent nao registrado\ncloseRun status=failed"]

    AG --> LOOP{"loop tool-calling\nmax 5 rounds"}
    LOOP -->|round| LLM["llm.Provider.Complete(ctx, Request)"]
    LLM --> DEC{"Response tem\ntool_calls?"}

    DEC -->|sim| TOOL["tool.ToolHandle.Invoke(ctx, argsJSON)\nresultado como role=tool"]
    TOOL --> LOOP
    DEC -->|nao| OUT["Response.Content final"]

    LOOP -->|excedeu 5 rounds| EXH["erro: tool rounds exhausted"]

    OUT --> APPEND["MessageStore.Append(user + assistant)\n-> platform_messages"]
    APPEND --> CLOSE["closeRun(status=succeeded)\nplatform_runs.ended_at, duration_ms"]
    CLOSE --> REPLY["Outcome{RunID, Content, Status, Mode}"]
```

---

## Sequencia Completa: WhatsApp Inbound → AgentRuntime → Reply

```mermaid
sequenceDiagram
    autonumber

    participant CONS as WhatsAppInboundConsumer
    participant UC as HandleInbound
    participant RT as AgentRuntime
    participant TG as ThreadGateway
    participant RS as RunStore
    participant REG as AgentRegistry
    participant AG as Agent
    participant LLM as llm.Provider (OpenRouter)
    participant TH as ToolHandle
    participant MS as MessageStore
    participant GW as WhatsAppGateway

    CONS->>+UC: Execute(InboundInput{ResourceID, ThreadID, AgentID, Message, MessageID})
    Note over CONS: AgentID = "weather-agent"<br/>ResourceID = UserID, ThreadID = Peer

    UC->>+RT: Execute(ctx, InboundRequest)

    RT->>+TG: GetOrCreate(resourceID, threadID)
    TG-->>-RT: Thread (platform_threads)

    RT->>+RS: Insert(Run status=running)
    RS-->>-RT: ok (platform_runs)

    RT->>+REG: Resolve(agentID)
    REG-->>-RT: Agent

    RT->>+AG: Execute(ctx, req)

    loop max 5 rounds
        AG->>+LLM: Complete(ctx, Request{Messages, Tools})
        LLM-->>-AG: Response{Content, ToolCalls}

        alt Response tem tool_calls
            AG->>+TH: Invoke(ctx, argsJSON)
            Note over TH: ex: get-weather (open-meteo)<br/>geocoding + forecast
            TH-->>-AG: resultJSON (role=tool)
            Note over AG: append result em Messages<br/>continua loop
        else Sem tool_calls
            Note over AG: resposta final pronta
        end
    end

    AG-->>-RT: result{Content}

    RT->>+MS: Append(user message)
    RT->>MS: Append(assistant message)
    MS-->>-RT: ok (platform_messages)
    Note over MS: PublishingMessageStore publica<br/>platform.memory.embedding.index.v1<br/>para indexacao assincrona

    RT->>RT: closeRun(status=succeeded)
    RT-->>-UC: Outcome{RunID, Content, Status, Mode}
    UC-->>-CONS: Outcome

    alt outcome.Content nao vazio
        CONS->>+GW: SendTextMessage(peer, outcome.Content)
        GW-->>-CONS: nil
    end
```

---

## Tipos Fechados (DMMF state-as-type)

```mermaid
classDiagram
    class InboundRequest {
        +string ResourceID
        +string ThreadID
        +string AgentID
        +string Message
        +string MessageID
    }

    class Outcome {
        +uuid.UUID RunID
        +string Content
        +RunStatus Status
        +ExecutionMode Mode
    }

    class RunStatus {
        <<enum>>
        RunStatusRunning
        RunStatusSucceeded
        RunStatusFailed
    }

    class ToolOutcome {
        <<enum>>
        ToolOutcomeRouted
        ToolOutcomeClarify
        ToolOutcomeUsecaseError
        ToolOutcomeMissingResolver
        ToolOutcomeReplay
    }

    class ExecutionMode {
        <<enum>>
        ExecutionModeSync
        ExecutionModeStream
    }

    InboundRequest --> Outcome : AgentRuntime.Execute
    Outcome --> RunStatus
    Outcome --> ExecutionMode
```

`RunStatus`, `ToolOutcome` e `ExecutionMode` sao tipos fechados com constantes enumeradas (DMMF state-as-type). Nunca representados como string livre na fronteira de codigo.

---

## ToolHandle — Contrato

```mermaid
classDiagram
    class ToolHandle {
        <<interface>>
        +ID() string
        +Description() string
        +Parameters() map~string,any~
        +Invoke(ctx, argsJSON []byte) ([]byte, error)
    }

    class Registry {
        <<interface>>
        +Register(h ToolHandle)
        +Resolve(id string) (ToolHandle, error)
    }

    Registry --> ToolHandle : resolve
```

Cada `ToolHandle` e um adapter fino de responsabilidade unica: valida o JSON de entrada contra o schema, executa a funcao de dominio e serializa o resultado. Zero regra de negocio, SQL ou branching de dominio (R-ADAPTER-001 / R-AGENT-WF-001.2). No consumidor de referencia, a tool e `get-weather` (`BuildWeatherTool`), que consulta a API open-meteo via `weather.Client`.

---

## Agent de Referencia: Weather (internal/agents)

| Bloco | Construtor | Identificador |
|------|-----------|---------------|
| Agent | `BuildWeatherAgent` | `weather-agent` |
| Tool | `BuildWeatherTool` | `get-weather` (open-meteo) |
| Workflow | `BuildWeatherWorkflow` | `weather-workflow` (steps `fetch-weather`, `plan-activities`) |
| Scorers | `BuildWeatherScorers` | `tool-call-accuracy`, `completeness`, `translation` (LLM-judged) |

O wiring vive em `internal/agents/module.go`: monta `llm.NewOpenRouterProvider`, registra o agent no `AgentRegistry`, instancia memoria (thread/message/working/embedding), `RunStore`, `ScorerRunner` e o `AgentRuntime`. O canal WhatsApp entra via `WhatsAppInboundConsumer` ouvindo o evento `agents.whatsapp.inbound.v1`.

---

## Metricas Emitidas

| Metrica | Tipo | Labels | Descricao |
|---------|------|--------|-----------|
| `agent_runs_total` | Counter | `agent_id`, `status` | Runs por agente e status |
| `agent_run_duration_seconds` | Histogram | `agent_id` | Duracao do run |
| `agent_tool_invocations_total` | Counter | `agent_id`, `tool` | Tool calls invocados |
| `agent_stream_total` | Counter | `agent_id` | Execucoes em modo stream |
| `agents_whatsapp_inbound_total` | Counter | `channel`, `outcome` | Inbound processados pelo consumer |
| `agents_whatsapp_route_total` | Counter | `outcome` | Mensagens roteadas no server |

**Cardinalidade controlada:** nenhum label de alta cardinalidade (`user_id`, `resource_id`, `correlation_key`). Labels sao enums fechados.
</content>
