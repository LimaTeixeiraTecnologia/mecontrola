package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	endpointChatCompletions = "/api/v1/chat/completions"
	defaultMaxTokens        = 256
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
	latency := o11y.Metrics().HistogramWithBuckets(
		"agent_llm_provider_latency_seconds",
		"Latencia de respostas dos providers LLM",
		"s",
		[]float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
	)
	return &Provider{cfg: cfg, client: client, o11y: o11y, callTotal: callTotal, callError: callError, latency: latency}
}

func (p *Provider) Slug() valueobjects.ModelSlug { return p.cfg.Slug }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	ResponseFmt responseFmt   `json:"response_format"`
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
	Message chatMessage `json:"message"`
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

	body := chatRequest{
		Model: p.cfg.Slug.String(),
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserMessage},
		},
		MaxTokens:   resolveMaxTokens(p.cfg.MaxTokens),
		Temperature: p.cfg.Temperature,
		ResponseFmt: resolveResponseFormat(req.JSONSchema),
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return interfaces.LLMResponse{}, fmt.Errorf("agent.llm.openrouter: marshal request: %w", err)
	}

	start := time.Now()
	resp, err := p.client.Post(ctx, endpointChatCompletions, bytes.NewReader(encoded),
		httpclient.WithHeader("Content-Type", "application/json"),
		httpclient.WithHeader("Authorization", "Bearer "+p.cfg.APIKey),
		httpclient.WithHeader("HTTP-Referer", p.cfg.HTTPReferer),
		httpclient.WithHeader("X-Title", p.cfg.XTitle),
	)
	elapsed := time.Since(start).Seconds()
	p.latency.Record(ctx, elapsed, observability.String("model", p.cfg.Slug.String()))

	if err != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "transport"))
		span.RecordError(err)
		return interfaces.LLMResponse{}, fmt.Errorf("agent.llm.openrouter: post: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			p.o11y.Logger().Warn(ctx, "agent.llm.openrouter.close_body_failed",
				observability.Error(closeErr),
			)
		}
	}()

	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "read_body"))
		return interfaces.LLMResponse{}, fmt.Errorf("agent.llm.openrouter: read body: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Slug.String()),
			observability.String("reason", classifyStatus(resp.StatusCode)),
		)
		return interfaces.LLMResponse{}, fmt.Errorf("%w: status=%d body=%s", ErrProviderUpstream, resp.StatusCode, truncatePreview(rawBody))
	}

	var parsed chatResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "decode"))
		return interfaces.LLMResponse{}, fmt.Errorf("agent.llm.openrouter: decode response: %w", err)
	}
	if parsed.Error != nil {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "upstream_error"))
		return interfaces.LLMResponse{}, fmt.Errorf("%w: %s", ErrProviderUpstream, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		p.callError.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("reason", "empty_choices"))
		return interfaces.LLMResponse{}, ErrEmptyChoices
	}

	p.callTotal.Add(ctx, 1, observability.String("model", p.cfg.Slug.String()), observability.String("status", "ok"))

	return interfaces.LLMResponse{
		Provider:         p.cfg.Slug,
		RawJSON:          []byte(parsed.Choices[0].Message.Content),
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
	}, nil
}

func resolveResponseFormat(spec *interfaces.JSONSchemaSpec) responseFmt {
	if spec != nil && spec.Name != "" && spec.Schema != nil {
		return responseFmt{
			Type: "json_schema",
			JSONSchema: &responseFmtJSONSchema{
				Name:   spec.Name,
				Strict: spec.Strict,
				Schema: spec.Schema,
			},
		}
	}
	return responseFmt{
		Type: "json_schema",
		JSONSchema: &responseFmtJSONSchema{
			Name:   "mecontrola_intent",
			Strict: true,
			Schema: intentJSONSchema,
		},
	}
}

func resolveMaxTokens(configured int) int {
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
