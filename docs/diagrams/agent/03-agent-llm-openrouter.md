# Fluxo: Agent → LLM → OpenRouter

Diagrama detalhado do processamento de uma mensagem pelo módulo de agent, da invocação do LLM via OpenRouter até a resposta de volta ao usuário.

## Referências de código

| Componente | Arquivo |
|---|---|
| Agent module / wiring | `internal/agent/module.go` |
| WhatsApp agent route | `internal/agent/module.go:425-445` |
| IntentRouter | `internal/agent/application/services/intent_router.go` |
| ParseInbound use case | `internal/agent/application/usecases/parse_inbound.go` |
| ComposeConversationalReply | `internal/agent/application/usecases/compose_conversational_reply.go` |
| FallbackChain | `internal/agent/application/services/fallback_chain.go` |
| OpenRouter client | `internal/agent/infrastructure/providers/openrouter/client.go` |
| LLM interfaces | `internal/agent/application/interfaces/llm_provider.go` |
| Meta client | `internal/onboarding/infrastructure/http/client/meta/client.go` |
| WhatsApp gateway | `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go` |
| Intent event publisher | `internal/agent/infrastructure/events/intent_event_publisher.go` |

---

## Sequência Completa: agentRoute → OpenRouter → Reply

```mermaid
sequenceDiagram
    autonumber

    participant DISP as Dispatcher
    participant AR as AgentRoute
    participant IR as IntentRouter
    participant IP as ParseInbound
    participant FC as FallbackChain
    participant CB as CircuitBreaker
    participant OR as OpenRouter Provider
    participant ORA as openrouter.ai
    participant HD as Intent Handler
    participant EVT as EventPublisher
    participant GW as WhatsAppGateway
    participant META as Meta Graph API

    DISP->>+AR: agentRoute(ctx, msg)
    Note over AR: extrai Principal do ctx<br/>auth.FromContext(ctx)

    AR->>+IR: RouteWhatsApp(ctx, Principal, InboundMessage)

    IR->>+IP: Parse(ctx, userID, text)
    Note over IP: renderiza system prompt + user message<br/>carrega JSON schema da intent

    IP->>+FC: Interpret(ctx, LLMRequest)
    Note over FC: itera providers em ordem:<br/>1. PrimaryModel<br/>2. FallbackModels[]

    FC->>+CB: State(primaryModel)?
    alt Circuit OPEN (muitas falhas recentes)
        CB-->>FC: open
        Note over FC: pula provider<br/>agent_llm_fallback_skipped_total++
        FC->>CB: State(fallbackModel)?
    end
    CB-->>-FC: closed

    FC->>+OR: Interpret(ctx, LLMRequest)
    Note over OR: Slug: google/gemini-2.5-flash-lite<br/>MaxTokens: 256<br/>Temperature: 0<br/>Timeout: 8s

    OR->>+ORA: POST /api/v1/chat/completions
    Note over OR,ORA: Authorization: Bearer OPENROUTER_API_KEY<br/>HTTP-Referer: AGENT_LLM_HTTP_REFERER<br/>X-Title: AGENT_LLM_X_TITLE<br/>Content-Type: application/json
    Note over ORA: Body: model, messages[system+user],<br/>max_tokens, temperature,<br/>response_format{type:json_schema, schema:intentSchema}

    alt HTTP 200
        ORA-->>-OR: chatResponse{choices, usage}
        OR->>CB: RecordSuccess()
        Note over OR: agent_llm_provider_call_total{status=ok}++<br/>agent_llm_provider_latency_seconds observe
        OR-->>FC: LLMResponse{RawJSON, PromptTokens, CompletionTokens}
    else Erro HTTP ou timeout
        ORA-->>OR: error
        OR->>CB: RecordFailure()
        Note over OR: agent_llm_provider_errors_total{reason}++<br/>apos 5 falhas em 30s: circuit abre por 60s
        OR-->>FC: error
        Note over FC: tenta proximo fallback model<br/>mistralai/mistral-small-3.2-24b-instruct
    end

    FC-->>-IP: LLMResponse

    alt finish_reason = "length" (truncado)
        IP->>IP: fallback para DirectReply
    end

    IP->>IP: json.Unmarshal(RawJSON) -> rawIntentDTO
    alt JSON invalido ou texto livre
        IP->>IP: kind = DirectReply<br/>usa choices[0].message.content
    end
    IP-->>-IR: ParsedIntent{Intent, Raw, DirectReply}

    alt DirectReply (resposta em prosa)
        IR->>+HD: ComposeConversationalReply.Execute(ctx, text)
        Note over HD: segunda chamada LLM com FreeText=true<br/>MaxTokens: AGENT_LLM_PROSE_MAX_TOKENS=200<br/>sem JSON schema, resposta em texto livre
        HD-->>-IR: reply string
    else Intent estruturada
        IR->>+HD: handler para intent.Kind
        Note over HD: ver 05-intent-dispatch-micro.md<br/>17 kinds e handlers
        HD-->>-IR: reply string
    end

    IR->>+EVT: PublishExecuted(ctx, IntentEvent)
    Note over EVT: outbox.Event type=agent.intent.executed.v1<br/>payload: event_id, channel, outcome,<br/>module, action, provider_used,<br/>latency_ms, prompt_tokens,<br/>completion_tokens, occurred_at
    EVT-->>-IR: nil

    IR->>+GW: SendTextMessage(ctx, whatsappTo, reply)
    GW->>+META: POST /{META_PHONE_NUMBER_ID}/messages
    Note over GW,META: Authorization: Bearer META_ACCESS_TOKEN<br/>Body: {messaging_product:whatsapp,<br/>to:5511999999999, type:text,<br/>text:{body:reply}}
    META-->>-GW: {messages:[{id:wamid.xxx}]}
    GW-->>-IR: nil

    IR-->>-AR: RouteResult{Reply, Outcome, Kind}
    AR-->>-DISP: OutcomeAgent
```

---

## Circuit Breaker — Estados

```mermaid
stateDiagram-v2
    [*] --> Closed : inicio

    Closed --> Open : CircuitFailures(5) falhas em CircuitWindow(30s)
    Open --> HalfOpen : apos CircuitCooldown(60s)
    HalfOpen --> Closed : proxima chamada OK
    HalfOpen --> Open : proxima chamada falha
    Closed --> Closed : chamada OK RecordSuccess

    note right of Open
        Provider ignorado
        agent_llm_fallback_skipped_total++
        label: model, state=open
    end note

    note right of HalfOpen
        1 chamada de teste
        permitida
    end note
```

---

## Structs OpenRouter — Request e Response

```mermaid
classDiagram
    class chatRequest {
        +string Model
        +chatMessage[] Messages
        +int MaxTokens
        +float64 Temperature
        +responseFmt ResponseFmt
        +toolDefinition[] Tools
        +any ToolChoice
        +bool ParallelToolCalls
    }

    class chatMessage {
        +string Role
        +string Content
        +toolCall[] ToolCalls
    }

    class responseFmt {
        +string Type
        +responseFmtJSONSchema JSONSchema
    }

    class toolDefinition {
        +string Type
        +toolFunctionDef Function
    }

    class toolFunctionDef {
        +string Name
        +string Description
        +map Parameters
    }

    class chatResponse {
        +chatChoice[] Choices
        +chatUsage Usage
        +chatError Error
    }

    class chatChoice {
        +chatMessage Message
        +string FinishReason
    }

    class chatUsage {
        +int PromptTokens
        +int CompletionTokens
    }

    class toolCall {
        +string ID
        +string Type
        +toolCallFunction Function
    }

    chatRequest --> chatMessage
    chatRequest --> responseFmt
    chatRequest --> toolDefinition
    chatResponse --> chatChoice
    chatResponse --> chatUsage
    chatChoice --> chatMessage
    chatMessage --> toolCall
    toolDefinition --> toolFunctionDef
```

---

## Interface LLMProvider

```mermaid
classDiagram
    class LLMProvider {
        <<interface>>
        +Slug() ModelSlug
        +Interpret(ctx, LLMRequest) LLMResponse
    }

    class LLMRequest {
        +string SystemPrompt
        +string UserMessage
        +JSONSchemaSpec JSONSchema
        +bool FreeText
        +int MaxTokens
        +ToolSpec[] Tools
        +string ToolChoice
    }

    class LLMResponse {
        +ModelSlug Provider
        +bytes RawJSON
        +int PromptTokens
        +int CompletionTokens
        +ToolCall[] ToolCalls
    }

    class ToolCall {
        +string ID
        +string FunctionName
        +map ArgumentsJSON
    }

    class Provider {
        -ProviderConfig cfg
        -httpclient.Client client
        -observability o11y
        -Counter callTotal
        -Counter callError
        -Counter toolCalls
        -Histogram latency
        +Slug() ModelSlug
        +Interpret(ctx, LLMRequest) LLMResponse
    }

    LLMProvider <|.. Provider
    LLMResponse --> ToolCall
```

---

## Metricas Emitidas

| Metrica | Tipo | Labels | Descricao |
|---------|------|--------|-----------|
| `agent_llm_provider_call_total` | Counter | `model`, `status` | Total de chamadas por modelo |
| `agent_llm_provider_errors_total` | Counter | `model`, `reason` | Erros por tipo |
| `agent_llm_provider_tool_calls_total` | Counter | `model`, `function` | Tool calls emitidos |
| `agent_llm_provider_latency_seconds` | Histogram | `model` | Latencia (0.1s a 10s) |
| `agent_llm_fallback_attempts_total` | Counter | `model`, `outcome` | Tentativas na chain |
| `agent_llm_fallback_exhausted_total` | Counter | — | Chain esgotada |
| `agent_llm_fallback_skipped_total` | Counter | `model`, `state` | Providers pulados |
| `agent_intent_parsed_total` | Counter | `kind`, `outcome` | Intents por tipo |
| `agent_intent_parse_decode_failed_total` | Counter | `reason` | Falhas de decode |

---

## Configuracao Relevante

```bash
OPENROUTER_BASE_URL=https://openrouter.ai
OPENROUTER_API_KEY=sk-or-v1-xxxxx
AGENT_LLM_HTTP_REFERER=https://mecontrola.app
AGENT_LLM_X_TITLE=MeControla

AGENT_LLM_PRIMARY_MODEL=google/gemini-2.5-flash-lite
AGENT_LLM_FALLBACK_MODELS=mistralai/mistral-small-3.2-24b-instruct

AGENT_LLM_MAX_TOKENS=256
AGENT_LLM_PROSE_MAX_TOKENS=200
AGENT_LLM_TEMPERATURE=0
AGENT_LLM_REQUEST_TIMEOUT=8s
AGENT_LLM_PROMPT_PAD_TOKENS=1100

AGENT_LLM_CIRCUIT_FAILURES=5
AGENT_LLM_CIRCUIT_WINDOW=30s
AGENT_LLM_CIRCUIT_COOLDOWN=60s

AGENT_ONBOARDING_LLM_ENABLED=true
AGENT_ONBOARDING_LLM_MODEL=anthropic/claude-haiku-4.5
AGENT_ONBOARDING_LLM_MAX_TOKENS=512
```
