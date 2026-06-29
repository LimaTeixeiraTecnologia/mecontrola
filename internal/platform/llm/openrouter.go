package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	endpointChatCompletions = "/api/v1/chat/completions"
	endpointEmbeddings      = "/api/v1/embeddings"
	defaultMaxTokens        = 256
	finishReasonLength      = "length"
	sseDoneSignal           = "[DONE]"
	sseDataPrefix           = "data: "
)

type Config struct {
	Model          string
	EmbedModel     string
	BaseURL        string
	APIKey         string
	HTTPReferer    string
	XTitle         string
	MaxTokens      int
	Temperature    float64
	RequestTimeout time.Duration
}

type openrouterProvider struct {
	cfg        Config
	client     *httpclient.Client
	streamHTTP *http.Client
	o11y       observability.Observability
	callTotal  observability.Counter
	callError  observability.Counter
	tokens     observability.Counter
	latency    observability.Histogram
}

func NewOpenRouterProvider(client *httpclient.Client, cfg Config, o11y observability.Observability) Provider {
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
	tokens := o11y.Metrics().Counter(
		"agent_llm_tokens_total",
		"Total de tokens consumidos por modelo e tipo",
		"1",
	)
	latency := o11y.Metrics().HistogramWithBuckets(
		"agent_llm_provider_latency_seconds",
		"Latencia de respostas dos providers LLM",
		"s",
		[]float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
	)
	streamHTTP := &http.Client{}
	return &openrouterProvider{
		cfg:        cfg,
		client:     client,
		streamHTTP: streamHTTP,
		o11y:       o11y,
		callTotal:  callTotal,
		callError:  callError,
		tokens:     tokens,
		latency:    latency,
	}
}

func (p *openrouterProvider) Slug() string { return p.cfg.Model }

func (p *openrouterProvider) Complete(ctx context.Context, req Request) (Response, error) {
	ctx, span := p.o11y.Tracer().Start(ctx, "llm.complete")
	defer span.End()

	body := p.buildChatRequest(req, false)
	rawBody, err := p.send(ctx, endpointChatCompletions, body, p.cfg.Model)
	if err != nil {
		span.RecordError(err)
		return Response{}, err
	}

	parsed, err := p.decodeChatResponse(ctx, rawBody)
	if err != nil {
		span.RecordError(err)
		return Response{}, err
	}

	status := "ok"
	if parsed.Choices[0].FinishReason == finishReasonLength {
		status = "truncated"
	}
	p.callTotal.Add(ctx, 1,
		observability.String("model", p.cfg.Model),
		observability.String("status", status),
	)
	p.recordTokens(ctx, parsed.Usage)

	msg := parsed.Choices[0].Message
	resp := Response{
		Content:           msg.Content,
		RawJSON:           []byte(msg.Content),
		PromptTokens:      parsed.Usage.PromptTokens,
		CompletionTokens:  parsed.Usage.CompletionTokens,
		TruncatedByLength: parsed.Choices[0].FinishReason == finishReasonLength,
	}
	if len(msg.ToolCalls) > 0 {
		calls, parseErr := parseToolCalls(msg.ToolCalls)
		if parseErr != nil {
			p.callError.Add(ctx, 1,
				observability.String("model", p.cfg.Model),
				observability.String("reason", "tool_args"),
			)
			span.RecordError(parseErr)
			return Response{}, parseErr
		}
		resp.ToolCalls = calls
	}
	return resp, nil
}

func (p *openrouterProvider) Stream(ctx context.Context, req Request) (TokenStream, error) {
	ctx, span := p.o11y.Tracer().Start(ctx, "llm.stream")

	body := p.buildChatRequest(req, true)
	encoded, err := json.Marshal(body)
	if err != nil {
		span.End()
		return nil, fmt.Errorf("llm.openrouter: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.streamBaseURL(), bytes.NewReader(encoded))
	if err != nil {
		span.End()
		return nil, fmt.Errorf("llm.openrouter: build stream request: %w", err)
	}
	p.setHeaders(httpReq)

	start := time.Now()
	resp, err := p.streamHTTP.Do(httpReq)
	p.latency.Record(ctx, time.Since(start).Seconds(), observability.String("model", p.cfg.Model))
	if err != nil {
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Model),
			observability.String("reason", "transport"),
		)
		span.RecordError(err)
		span.End()
		return nil, fmt.Errorf("llm.openrouter: stream post: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawPreview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		_ = resp.Body.Close()
		reason := classifyStatus(resp.StatusCode)
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Model),
			observability.String("reason", reason),
		)
		span.End()
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProviderUpstream, resp.StatusCode, truncatePreview(rawPreview))
	}

	stream := newSSEStream(resp, span)
	go stream.consume(ctx, p)
	return stream, nil
}

func (p *openrouterProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, span := p.o11y.Tracer().Start(ctx, "llm.embed")
	defer span.End()

	embedModel := p.cfg.EmbedModel
	if embedModel == "" {
		embedModel = "openai/text-embedding-3-small"
	}
	payload := map[string]any{
		"model": embedModel,
		"input": texts,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("llm.openrouter: marshal embed request: %w", err)
	}

	rawBody, err := p.send(ctx, endpointEmbeddings, encoded, embedModel)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var parsed embedResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		p.callError.Add(ctx, 1,
			observability.String("model", embedModel),
			observability.String("reason", "decode"),
		)
		return nil, fmt.Errorf("llm.openrouter: decode embed response: %w", err)
	}

	result := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		result[i] = d.Embedding
	}
	p.callTotal.Add(ctx, 1,
		observability.String("model", embedModel),
		observability.String("status", "ok"),
	)
	return result, nil
}

func (p *openrouterProvider) send(ctx context.Context, path string, body any, model string) ([]byte, error) {
	var encoded []byte
	switch v := body.(type) {
	case []byte:
		encoded = v
	default:
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("llm.openrouter: marshal request: %w", err)
		}
		encoded = b
	}

	start := time.Now()
	resp, err := p.client.Post(ctx, path, bytes.NewReader(encoded),
		httpclient.WithHeader("Content-Type", "application/json"),
		httpclient.WithHeader("Authorization", "Bearer "+p.cfg.APIKey),
		httpclient.WithHeader("HTTP-Referer", p.cfg.HTTPReferer),
		httpclient.WithHeader("X-Title", p.cfg.XTitle),
	)
	p.latency.Record(ctx, time.Since(start).Seconds(), observability.String("model", model))
	if err != nil {
		p.callError.Add(ctx, 1,
			observability.String("model", model),
			observability.String("reason", "transport"),
		)
		return nil, fmt.Errorf("llm.openrouter: post: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			p.o11y.Logger().Warn(ctx, "llm.openrouter.close_body_failed", observability.Error(closeErr))
		}
	}()

	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		p.callError.Add(ctx, 1,
			observability.String("model", model),
			observability.String("reason", "read_body"),
		)
		return nil, fmt.Errorf("llm.openrouter: read body: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason := classifyStatus(resp.StatusCode)
		p.callError.Add(ctx, 1,
			observability.String("model", model),
			observability.String("reason", reason),
		)
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProviderUpstream, resp.StatusCode, truncatePreview(rawBody))
	}
	return rawBody, nil
}

func (p *openrouterProvider) decodeChatResponse(ctx context.Context, rawBody []byte) (chatResponse, error) {
	var parsed chatResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Model),
			observability.String("reason", "decode"),
		)
		return chatResponse{}, fmt.Errorf("llm.openrouter: decode response: %w", err)
	}
	if parsed.Error != nil {
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Model),
			observability.String("reason", "upstream_error"),
		)
		return chatResponse{}, fmt.Errorf("%w: %s", ErrProviderUpstream, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Model),
			observability.String("reason", "empty_choices"),
		)
		return chatResponse{}, ErrEmptyChoices
	}
	return parsed, nil
}

func (p *openrouterProvider) recordTokens(ctx context.Context, usage chatUsage) {
	if usage.PromptTokens > 0 {
		p.tokens.Add(ctx, int64(usage.PromptTokens),
			observability.String("model", p.cfg.Model),
			observability.String("type", "prompt"),
		)
	}
	if usage.CompletionTokens > 0 {
		p.tokens.Add(ctx, int64(usage.CompletionTokens),
			observability.String("model", p.cfg.Model),
			observability.String("type", "completion"),
		)
	}
}

func (p *openrouterProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("HTTP-Referer", p.cfg.HTTPReferer)
	req.Header.Set("X-Title", p.cfg.XTitle)
}

func (p *openrouterProvider) buildChatRequest(req Request, stream bool) chatRequest {
	msgs := make([]chatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		cm := chatMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, Name: m.Name}
		if len(m.ToolCalls) > 0 {
			cm.ToolCalls = toWireToolCalls(m.ToolCalls)
		}
		msgs = append(msgs, cm)
	}

	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = resolveMaxTokens(0, p.cfg.MaxTokens)
	}
	temp := req.Temperature
	if temp == 0 {
		temp = p.cfg.Temperature
	}

	body := chatRequest{
		Model:       p.cfg.Model,
		Messages:    msgs,
		MaxTokens:   maxTok,
		Temperature: temp,
		Stream:      stream,
	}

	if len(req.Tools) > 0 {
		noParallel := false
		body.Tools = buildToolDefinitions(req.Tools)
		body.ToolChoice = resolveToolChoice(req.ToolChoice)
		body.ParallelToolCalls = &noParallel
		return body
	}

	if !req.FreeText && req.Schema != nil {
		body.ResponseFmt = &responseFmt{
			Type: "json_schema",
			JSONSchema: &responseFmtJSONSchema{
				Name:   req.Schema.Name,
				Strict: req.Schema.Strict,
				Schema: req.Schema.Schema,
			},
		}
	}
	return body
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
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

type chatRequest struct {
	Model             string           `json:"model"`
	Messages          []chatMessage    `json:"messages"`
	MaxTokens         int              `json:"max_tokens"`
	Temperature       float64          `json:"temperature"`
	Stream            bool             `json:"stream,omitempty"`
	ResponseFmt       *responseFmt     `json:"response_format,omitempty"`
	Tools             []toolDefinition `json:"tools,omitempty"`
	ToolChoice        any              `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`
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

type sseDelta struct {
	Content string `json:"content"`
}

type sseChoice struct {
	Delta        sseDelta `json:"delta"`
	FinishReason *string  `json:"finish_reason"`
}

type sseChunk struct {
	Choices []sseChoice `json:"choices"`
	Usage   *chatUsage  `json:"usage,omitempty"`
}

type embedDatum struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type embedResponse struct {
	Data []embedDatum `json:"data"`
}

type sseStream struct {
	deltas chan string
	resp   *http.Response
	once   sync.Once
	errVal atomic.Pointer[error]
	span   observability.Span
}

func newSSEStream(resp *http.Response, span observability.Span) *sseStream {
	return &sseStream{
		deltas: make(chan string, 64),
		resp:   resp,
		span:   span,
	}
}

func (s *sseStream) Deltas() <-chan string { return s.deltas }

func (s *sseStream) Err() error {
	if p := s.errVal.Load(); p != nil {
		return *p
	}
	return nil
}

func (s *sseStream) Close() error {
	var closeErr error
	s.once.Do(func() {
		closeErr = s.resp.Body.Close()
		s.span.End()
	})
	return closeErr
}

func (s *sseStream) consume(ctx context.Context, p *openrouterProvider) {
	defer close(s.deltas)
	defer s.Close() //nolint:errcheck

	scanner := bufio.NewScanner(s.resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			s.setErr(ctx.Err())
			return
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, sseDataPrefix) {
			continue
		}
		raw := strings.TrimPrefix(line, sseDataPrefix)
		if raw == sseDoneSignal {
			p.callTotal.Add(ctx, 1,
				observability.String("model", p.cfg.Model),
				observability.String("status", "ok"),
			)
			return
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			s.setErr(fmt.Errorf("llm.openrouter: parse sse chunk: %w", err))
			p.callError.Add(ctx, 1,
				observability.String("model", p.cfg.Model),
				observability.String("reason", "sse_parse"),
			)
			return
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == finishReasonLength {
			p.callTotal.Add(ctx, 1,
				observability.String("model", p.cfg.Model),
				observability.String("status", "truncated"),
			)
		}
		if delta != "" {
			select {
			case s.deltas <- delta:
			case <-ctx.Done():
				s.setErr(ctx.Err())
				return
			}
		}
		if chunk.Usage != nil {
			p.recordTokens(ctx, *chunk.Usage)
		}
	}
	if err := scanner.Err(); err != nil {
		s.setErr(fmt.Errorf("llm.openrouter: sse scanner: %w", err))
		p.callError.Add(ctx, 1,
			observability.String("model", p.cfg.Model),
			observability.String("reason", "sse_read"),
		)
	}
}

func (s *sseStream) setErr(err error) {
	s.errVal.Store(&err)
	if s.span != nil {
		s.span.RecordError(err)
	}
}

func (p *openrouterProvider) streamBaseURL() string {
	base := p.cfg.BaseURL
	if base != "" {
		return strings.TrimRight(base, "/") + endpointChatCompletions
	}
	return endpointChatCompletions
}

func buildToolDefinitions(specs []ToolSpec) []toolDefinition {
	defs := make([]toolDefinition, 0, len(specs))
	for _, spec := range specs {
		defs = append(defs, toolDefinition{
			Type:     "function",
			Function: toolFunctionDef(spec),
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

func toWireToolCalls(calls []ToolCall) []toolCall {
	wire := make([]toolCall, 0, len(calls))
	for _, c := range calls {
		args := "{}"
		if len(c.ArgumentsJSON) > 0 {
			if b, err := json.Marshal(c.ArgumentsJSON); err == nil {
				args = string(b)
			}
		}
		wire = append(wire, toolCall{
			ID:       c.ID,
			Type:     "function",
			Function: toolCallFunction{Name: c.FunctionName, Arguments: args},
		})
	}
	return wire
}

func parseToolCalls(raw []toolCall) ([]ToolCall, error) {
	calls := make([]ToolCall, 0, len(raw))
	for _, tc := range raw {
		args := map[string]any{}
		if trimmed := strings.TrimSpace(tc.Function.Arguments); trimmed != "" {
			if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
				return nil, fmt.Errorf("llm.openrouter: decode tool arguments (%s): %w", tc.Function.Name, err)
			}
		}
		calls = append(calls, ToolCall{
			ID:            tc.ID,
			FunctionName:  tc.Function.Name,
			ArgumentsJSON: args,
		})
	}
	return calls, nil
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
