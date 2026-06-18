package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	endpointChatCompletions = "/api/v1/chat/completions"
	defaultMaxTokens        = 256
	finishReasonLength      = "length"
)

var ErrEmptyChoices = errors.New("agent.llm.openrouter: response has no choices")

var ErrProviderUpstream = errors.New("agent.llm.openrouter: upstream error")

type ProviderConfig struct {
	Slug           valueobjects.ModelSlug
	APIKey         string
	HTTPReferer    string
	XTitle         string
	MaxTokens      int
	Temperature    float64
	RequestTimeout time.Duration
}

type Provider struct {
	cfg       ProviderConfig
	client    *httpclient.Client
	o11y      observability.Observability
	callTotal observability.Counter
	callError observability.Counter
	toolCalls observability.Counter
	latency   observability.Histogram
}

func NewProvider(client *httpclient.Client, cfg ProviderConfig, o11y observability.Observability) *Provider {
	callTotal := o11y.Metrics().Counter(
		"agent_llm_provider_call_total",
		"Total de chamadas a providers LLM por modelo e status",
		"1",
	)
	callError := o11y.Metrics().Counter(
		"agent_llm_provider_errors_total",
		"Total de erros de providers LLM por modelo e reason",
		"1",
	)
	toolCalls := o11y.Metrics().Counter(
		"agent_llm_provider_tool_calls_total",
		"Total de tool calls emitidos por modelo e function",
		"1",
	)
	latency := o11y.Metrics().HistogramWithBuckets(
		"agent_llm_provider_latency_seconds",
		"Latencia de respostas dos providers LLM",
		"s",
		[]float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
	)
	return &Provider{cfg: cfg, client: client, o11y: o11y, callTotal: callTotal, callError: callError, toolCalls: toolCalls, latency: latency}
}

func (p *Provider) Slug() valueobjects.ModelSlug { return p.cfg.Slug }

type chatMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []toolCall `json:"tool_calls,omitempty"`
}

type chatRequest struct {
	Model             string           `json:"model"`
	Messages          []chatMessage    `json:"messages"`
	MaxTokens         int              `json:"max_tokens"`
	Temperature       float64          `json:"temperature"`
	ResponseFmt       *responseFmt     `json:"response_format,omitempty"`
	Tools             []toolDefinition `json:"tools,omitempty"`
	ToolChoice        any              `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`
}

type toolFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type toolDefinition struct {
	Type     string          `json:"type"`
	Function toolFunctionDef `json:"function"`
}

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type responseFmt struct {
	Type       string                 `json:"type"`
	JSONSchema *responseFmtJSONSchema `json:"json_schema,omitempty"`
}

type responseFmtJSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

var intentJSONSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"module": map[string]any{
			"type": "string",
			"enum": []string{"categories", "cards", "budgets", "transactions"},
		},
		"action": map[string]any{
			"type": "string",
			"enum": []string{"list", "get", "create", "update", "delete"},
		},
		"filters":       map[string]any{"type": "object"},
		"payload":       map[string]any{"type": "object"},
		"response_hint": map[string]any{"type": "string", "maxLength": 280},
		"error":         map[string]any{"type": "string"},
		"message":       map[string]any{"type": "string"},
	},
	"additionalProperties": false,
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chatError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
	Error   *chatError   `json:"error,omitempty"`
}

func (p *Provider) Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	ctx, span := p.o11y.Tracer().Start(ctx, "agent.llm.openrouter.interpret")
	defer span.End()

	rawBody, err := p.send(ctx, p.buildRequestBody(req))
	if err != nil {
		span.RecordError(err)
		return interfaces.LLMResponse{}, err
	}

	parsed, err := p.decode(ctx, rawBody)
	if err != nil {
		span.RecordError(err)
		return interfaces.LLMResponse{}, err
	}

	status := "ok"
	if parsed.Choices[0].FinishReason == finishReasonLength {
		status = "truncated"
		p.o11y.Logger().Warn(ctx, "agent.llm.openrouter.response_truncated",
			observability.String("model", p.cfg.Slug.String()),
			observability.Int("max_tokens", resolveMaxTokens(req.MaxTokens, p.cfg.MaxTokens)),
		)
	}
	p.callTotal.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("status", status))

	message := parsed.Choices[0].Message
	result := interfaces.LLMResponse{
		Provider:         p.cfg.Slug,
		RawJSON:          []byte(message.Content),
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
	}
	if len(message.ToolCalls) > 0 {
		calls, parseErr := parseToolCalls(message.ToolCalls)
		if parseErr != nil {
			p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "tool_args"))
			span.RecordError(parseErr)
			return interfaces.LLMResponse{}, parseErr
		}
		result.ToolCalls = calls
		for _, call := range calls {
			p.toolCalls.Add(ctx, 1,
				observability.String("model", p.cfg.Slug.String()),
				observability.String("function", call.FunctionName),
			)
		}
	}
	return result, nil
}

func (p *Provider) buildRequestBody(req interfaces.LLMRequest) chatRequest {
	body := chatRequest{
		Model: p.cfg.Slug.String(),
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserMessage},
		},
		MaxTokens:   resolveMaxTokens(req.MaxTokens, p.cfg.MaxTokens),
		Temperature: p.cfg.Temperature,
	}
	if len(req.Tools) > 0 {
		noParallel := false
		body.Tools = buildToolDefinitions(req.Tools)
		body.ToolChoice = resolveToolChoice(req.ToolChoice)
		body.ParallelToolCalls = &noParallel
		return body
	}
	body.ResponseFmt = resolveResponseFormat(req)
	return body
}

func (p *Provider) send(ctx context.Context, body chatRequest) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("agent.llm.openrouter: marshal request: %w", err)
	}

	start := time.Now()
	resp, err := p.client.Post(ctx, endpointChatCompletions, bytes.NewReader(encoded),
		httpclient.WithHeader("Content-Type", "application/json"),
		httpclient.WithHeader("Authorization", "Bearer "+p.cfg.APIKey),
		httpclient.WithHeader("HTTP-Referer", p.cfg.HTTPReferer),
		httpclient.WithHeader("X-Title", p.cfg.XTitle),
	)
	p.latency.Record(ctx, time.Since(start).Seconds(), observability.String("model", p.cfg.Slug.String()))
	if err != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "transport"))
		return nil, fmt.Errorf("agent.llm.openrouter: post: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			p.o11y.Logger().Warn(ctx, "agent.llm.openrouter.close_body_failed", observability.Error(closeErr))
		}
	}()

	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "read_body"))
		return nil, fmt.Errorf("agent.llm.openrouter: read body: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Slug.String()),
			observability.String("reason", classifyStatus(resp.StatusCode)),
		)
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProviderUpstream, resp.StatusCode, truncatePreview(rawBody))
	}
	return rawBody, nil
}

func (p *Provider) decode(ctx context.Context, rawBody []byte) (chatResponse, error) {
	var parsed chatResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "decode"))
		return chatResponse{}, fmt.Errorf("agent.llm.openrouter: decode response: %w", err)
	}
	if parsed.Error != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "upstream_error"))
		return chatResponse{}, fmt.Errorf("%w: %s", ErrProviderUpstream, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "empty_choices"))
		return chatResponse{}, ErrEmptyChoices
	}
	return parsed, nil
}

func buildToolDefinitions(specs []interfaces.ToolSpec) []toolDefinition {
	defs := make([]toolDefinition, 0, len(specs))
	for _, spec := range specs {
		defs = append(defs, toolDefinition{
			Type: "function",
			Function: toolFunctionDef{
				Name:        spec.Name,
				Description: spec.Description,
				Parameters:  spec.Parameters,
			},
		})
	}
	return defs
}

func resolveToolChoice(requested string) any {
	if strings.TrimSpace(requested) != "" {
		return requested
	}
	return "auto"
}

func parseToolCalls(raw []toolCall) ([]interfaces.ToolCall, error) {
	calls := make([]interfaces.ToolCall, 0, len(raw))
	for _, tc := range raw {
		args := map[string]any{}
		if trimmed := strings.TrimSpace(tc.Function.Arguments); trimmed != "" {
			if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
				return nil, fmt.Errorf("agent.llm.openrouter: decode tool arguments (%s): %w", tc.Function.Name, err)
			}
		}
		calls = append(calls, interfaces.ToolCall{
			ID:            tc.ID,
			FunctionName:  tc.Function.Name,
			ArgumentsJSON: args,
		})
	}
	return calls, nil
}

func resolveResponseFormat(req interfaces.LLMRequest) *responseFmt {
	if req.FreeText {
		return nil
	}
	spec := req.JSONSchema
	if spec != nil && spec.Name != "" && spec.Schema != nil {
		return &responseFmt{
			Type: "json_schema",
			JSONSchema: &responseFmtJSONSchema{
				Name:   spec.Name,
				Strict: spec.Strict,
				Schema: spec.Schema,
			},
		}
	}
	return &responseFmt{
		Type: "json_schema",
		JSONSchema: &responseFmtJSONSchema{
			Name:   "mecontrola_intent",
			Strict: true,
			Schema: intentJSONSchema,
		},
	}
}

func resolveMaxTokens(requested, configured int) int {
	if requested > 0 {
		return requested
	}
	if configured <= 0 {
		return defaultMaxTokens
	}
	return configured
}

func classifyStatus(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "unauthorized"
	case code == http.StatusPaymentRequired:
		return "no_credit"
	case code == http.StatusTooManyRequests:
		return "rate_limited"
	case code == http.StatusRequestTimeout:
		return "timeout"
	case code >= 500:
		return "upstream_5xx"
	default:
		return "client_4xx"
	}
}

func truncatePreview(b []byte) string {
	const maxLen = 256
	if len(b) > maxLen {
		return string(b[:maxLen]) + "..."
	}
	return string(b)
}
