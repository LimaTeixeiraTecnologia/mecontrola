# LLM Provider, structured output e streaming

`internal/platform/llm` é a única porta de modelo. OpenRouter é o **único** provider oficial — não há
FallbackChain nem CircuitBreaker. LLM aparece apenas em call-sites sancionadas (agent loop, plan step que
chama `Stream`, scorer LLM-judged); nunca no kernel `internal/platform/workflow`.

## Provider (`provider.go`)

```go
type Provider interface {
    Slug() string
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (TokenStream, error)
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}
type TokenStream interface { Deltas() <-chan string; Close() error; Err() error }
```

`NewOpenRouterProvider(httpClient, llm.Config{Model, EmbedModel, APIKey, BaseURL, MaxTokens, Temperature}, o11y)`
(`openrouter.go`). Métricas: `call_total`, `call_error`, `tokens_total`, `latency_seconds`; classificação de
erro (`unauthorized`, `no_credit`, `rate_limited`, `timeout`, `upstream_5xx`, `client_4xx`).

## Request / Response (`types.go`)

```go
type Request struct {
    Messages []Message; Schema *Schema; Tools []ToolSpec; ToolChoice string
    MaxTokens int; Temperature float64; FreeText bool
}
type Response struct {
    Content string; RawJSON []byte; PromptTokens, CompletionTokens int
    TruncatedByLength bool; ToolCalls []ToolCall
}
```

`Message{Role, Content, ToolCalls, ToolCallID, Name}`; `ToolSpec{Name, Description, Parameters}`;
`ToolCall{ID, FunctionName, ArgumentsJSON}`.

## Tool-calling no Agent (`agent/agent.go`)

`Agent.Execute` monta `llm.Request` com `Tools` (de cada `ToolHandle`) e roda `completeWithTools` (máx.
`maxToolRounds = 5`): a cada round chama `Complete`; se `resp.ToolCalls` não vazio, invoca cada tool
(`ToolHandle.Invoke`) e anexa o resultado como `Message{Role: "tool"}`; encerra quando não há mais tool calls.
Esgotar os rounds sem resposta → `ErrMaxToolRounds`.

## Structured output (`agent/decoder.go`, `llm/types.go`)

```go
type StructuredContract[T any] interface { Schema() Schema; Decode(raw []byte) (T, error) }   // llm
type StructuredDecoder interface { Schema() Schema; Validate(raw []byte) error }                // agent
func NewDecoder[T any](c llm.StructuredContract[T]) StructuredDecoder
```

- **Síncrono**: passe `Request.Schema` e `Request.Decoder`. `Agent.Execute` valida `resp.RawJSON` no fim;
  contrato não satisfeito → `ErrContractNotMet` (falha explícita e auditável — ADR-003).
- **Streaming**: `Agent.Stream` valida na **conclusão** do stream (em `ResultStream.Result`), nunca durante.
  `llm.Request.FreeText = (Schema == nil)`.

`Schema{Name, Strict bool, Schema map[string]any}` — use `Strict: true` e `additionalProperties: false` para
contratos rígidos (ex.: o `judgeContract` do scorer, os schemas de input/output da tool).

## Embeddings

`Provider.Embed(ctx, texts) ([][]float32, error)` — usado pelo `EmbeddingIndexHandler` para popular
`platform_embeddings`. Modelo de embedding via `llm.Config.EmbedModel`. Ver `memory-recall.md`.

## Proibições
- LLM no kernel `internal/platform/workflow` (R-WF-KERNEL-001.5).
- Inventar fallback chain / circuit breaker — não existem; provider é único.
- Validar contrato no meio do stream — só na conclusão.
