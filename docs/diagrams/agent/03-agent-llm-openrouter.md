# Fluxo: Agent → LLM → OpenRouter

Diagrama detalhado do processamento de uma mensagem pelo substrato de agent da plataforma, da invocacao do LLM via OpenRouter (provider unico) ate a resposta de volta ao usuario.

> O antigo modulo financeiro de agente (no singular), com roteamento por enum de intent, cadeia de fallback de LLM e disjuntor de circuito, foi removido. O substrato atual vive em `internal/platform/{agent,llm,memory,tool,workflow,scorer}` e e exercitado pelo consumidor de referencia `internal/agents` (port do exemplo Weather do Mastra). Existe **um unico provider LLM** (`llm.NewOpenRouterProvider`); nao ha mais cadeia de fallback nem disjuntor de circuito.

## Referencias de codigo

| Componente | Arquivo |
|---|---|
| Agents module / wiring | `internal/agents/module.go` |
| WhatsApp agent route | `internal/agents/module.go` (`buildWhatsAppAgentRoute`) |
| HandleInbound use case | `internal/agents/application/usecases/handle_inbound.go` |
| WhatsApp inbound consumer | `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` |
| AgentRuntime | `internal/platform/agent/runtime.go` |
| Agent (loop tool-calling) | `internal/platform/agent/agent.go` |
| LLM Provider interface | `internal/platform/llm/provider.go` |
| OpenRouter Provider | `internal/platform/llm/openrouter.go` |
| ToolHandle | `internal/platform/tool/tool.go` |
| Weather client (open-meteo) | `internal/agents/infrastructure/weather/client.go` |
| WhatsApp gateway | `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go` |

---

## Sequencia Completa: Inbound → OpenRouter → Reply

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
    participant OR as OpenRouter Provider
    participant ORA as openrouter.ai
    participant TH as ToolHandle (get-weather)
    participant MS as MessageStore
    participant GW as WhatsAppGateway
    participant META as Meta Graph API

    CONS->>+UC: Execute(InboundInput)
    Note over CONS: AgentID = "weather-agent"<br/>ResourceID = UserID, ThreadID = Peer
    UC->>UC: in.Validate()
    UC->>+RT: Execute(ctx, InboundRequest)

    RT->>+TG: GetOrCreate(resourceID, threadID)
    TG-->>-RT: Thread (platform_threads)
    RT->>+RS: Insert(Run status=running)
    RS-->>-RT: ok (platform_runs)
    RT->>+REG: Resolve(agentID)
    REG-->>-RT: Agent
    RT->>+AG: Execute(ctx, req)

    loop max 5 rounds (maxToolRounds)
        AG->>+OR: Complete(ctx, Request{Messages, Tools})
        Note over OR: Model: AGENT_LLM_PRIMARY_MODEL<br/>(google/gemini-2.5-flash-lite)<br/>MaxTokens, Temperature da Config

        OR->>+ORA: POST /api/v1/chat/completions
        Note over OR,ORA: Authorization: Bearer OPENROUTER_API_KEY<br/>HTTP-Referer: HTTPReferer<br/>X-Title: XTitle<br/>Body: model, messages, max_tokens,<br/>temperature, tools, tool_choice

        alt HTTP 200
            ORA-->>-OR: chatResponse{choices, usage}
            Note over OR: agent_llm_provider_call_total{status=ok}++<br/>agent_llm_provider_latency_seconds observe<br/>agent_llm_tokens_total{type} add
            OR-->>AG: Response{Content, ToolCalls}
        else Erro HTTP ou timeout
            ORA-->>OR: error
            Note over OR: agent_llm_provider_errors_total{reason}++
            OR-->>AG: error
            Note over AG: erro propaga, run fecha status=failed
        end

        alt Response tem tool_calls
            AG->>+TH: Invoke(ctx, argsJSON)
            Note over TH: get-weather: geocoding +<br/>forecast via open-meteo
            TH-->>-AG: resultJSON (role=tool)
            Note over AG: append result em Messages,<br/>segue para proximo round
        else Sem tool_calls
            Note over AG: Content final pronto
        end
    end

    AG-->>-RT: result{Content}

    RT->>+MS: Append(user + assistant)
    MS-->>-RT: ok (platform_messages)
    Note over MS: PublishingMessageStore publica<br/>platform.memory.embedding.index.v1

    RT->>RT: closeRun(status=succeeded)
    RT-->>-UC: Outcome{RunID, Content, Status, Mode}
    UC-->>-CONS: Outcome

    alt outcome.Content nao vazio
        CONS->>+GW: SendTextMessage(peer, outcome.Content)
        GW->>+META: POST /{META_PHONE_NUMBER_ID}/messages
        META-->>-GW: {messages:[{id:wamid.xxx}]}
        GW-->>-CONS: nil
    end
```

---

## Indexacao Assincrona de Embeddings (Semantic Recall)

```mermaid
sequenceDiagram
    autonumber

    participant MS as PublishingMessageStore
    participant OUT as outbox table
    participant JOB as OutboxDispatcherJob
    participant IDX as EmbeddingIndexHandler
    participant OR as OpenRouter Provider
    participant SR as SemanticRecall

    MS->>MS: Append(message) -> platform_messages
    MS->>OUT: publica platform.memory.embedding.index.v1
    Note over MS,OUT: IndexMessagePayload{resource_id, thread_id,<br/>message_pk, content, model}

    JOB->>OUT: SELECT FOR UPDATE SKIP LOCKED
    JOB->>+IDX: Handle(ctx, event)
    IDX->>+OR: Embed(ctx, [content])
    OR-->>-IDX: [][]float32
    IDX->>+SR: Index(resourceID, threadID, messagePK, content, model, embedding)
    SR-->>-IDX: ok (platform_embeddings, vector 1536)
    IDX-->>-JOB: nil
    Note over IDX: platform_memory_embedding_index_succeeded_total++
```

---

## Interface llm.Provider

```mermaid
classDiagram
    class Provider {
        <<interface>>
        +Slug() string
        +Complete(ctx, Request) (Response, error)
        +Stream(ctx, Request) (TokenStream, error)
        +Embed(ctx, texts []string) ([][]float32, error)
    }

    class Request {
        +Message[] Messages
        +ToolSpec[] Tools
        +string ToolChoice
        +Schema ResponseSchema
        +int MaxTokens
        +float64 Temperature
    }

    class Response {
        +string Content
        +ToolCall[] ToolCalls
        +int PromptTokens
        +int CompletionTokens
        +string FinishReason
    }

    class ToolCall {
        +string ID
        +string FunctionName
        +bytes ArgumentsJSON
    }

    class openrouterProvider {
        -Config cfg
        -httpclient.Client client
        -observability o11y
        +Slug() string
        +Complete(ctx, Request) (Response, error)
        +Stream(ctx, Request) (TokenStream, error)
        +Embed(ctx, texts) ([][]float32, error)
    }

    Provider <|.. openrouterProvider
    Request --> ToolCall
    Response --> ToolCall
```

`Stream` alimenta o modo `ExecutionModeStream` do `AgentRuntime`; `Embed` alimenta a indexacao de embeddings para Semantic Recall (pgvector).

---

## Config do OpenRouter Provider

```mermaid
classDiagram
    class Config {
        +string Model
        +string EmbedModel
        +string BaseURL
        +string APIKey
        +string HTTPReferer
        +string XTitle
        +int MaxTokens
        +float64 Temperature
        +time.Duration RequestTimeout
    }
```

Endpoints internos: `POST /api/v1/chat/completions` (Complete/Stream) e `POST /api/v1/embeddings` (Embed).

---

## Metricas Emitidas

| Metrica | Tipo | Labels | Descricao |
|---------|------|--------|-----------|
| `agent_llm_provider_call_total` | Counter | `model`, `status` | Total de chamadas por modelo |
| `agent_llm_provider_errors_total` | Counter | `model`, `reason` | Erros por tipo |
| `agent_llm_provider_latency_seconds` | Histogram | `model` | Latencia das chamadas |
| `agent_llm_tokens_total` | Counter | `model`, `type` | Tokens prompt/completion |
| `agent_runs_total` | Counter | `agent_id`, `status` | Runs por agente |
| `agent_tool_invocations_total` | Counter | `agent_id`, `tool` | Tool calls invocados |
| `platform_memory_embedding_index_succeeded_total` | Counter | `model` | Embeddings indexados |
| `platform_memory_embedding_index_failed_total` | Counter | `reason` | Falhas de indexacao |

**Cardinalidade controlada:** labels sao enums fechados; sem `user_id`, `resource_id` ou `correlation_key`.

---

## Configuracao Relevante

```bash
OPENROUTER_BASE_URL=https://openrouter.ai
OPENROUTER_API_KEY=sk-or-v1-xxxxx
AGENT_LLM_HTTP_REFERER=https://mecontrola.app
AGENT_LLM_X_TITLE=MeControla

AGENT_LLM_PRIMARY_MODEL=google/gemini-2.5-flash-lite
AGENT_LLM_EMBED_MODEL=openai/text-embedding-3-small

AGENT_LLM_MAX_TOKENS=768
AGENT_LLM_TEMPERATURE=0
```
</content>
