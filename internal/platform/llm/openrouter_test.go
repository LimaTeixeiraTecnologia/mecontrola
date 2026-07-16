package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type LLMProviderSuite struct {
	suite.Suite
	ctx context.Context
}

func TestLLMProviderSuite(t *testing.T) {
	suite.Run(t, new(LLMProviderSuite))
}

func (s *LLMProviderSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *LLMProviderSuite) buildProvider(server *httptest.Server) Provider {
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("llm_test"),
		httpclient.WithTimeout(5*time.Second),
	)
	s.Require().NoError(err)
	return NewOpenRouterProvider(client, Config{
		Model:       "google/gemini-flash",
		EmbedModel:  "openai/text-embedding-3-small",
		BaseURL:     server.URL,
		APIKey:      "test-key",
		HTTPReferer: "https://example.com",
		XTitle:      "TestApp",
		MaxTokens:   256,
		Temperature: 0,
	}, fake.NewProvider())
}

func (s *LLMProviderSuite) TestComplete_HappyPath() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("POST", r.Method)
		s.Equal("/api/v1/chat/completions", r.URL.Path)
		s.Equal("Bearer test-key", r.Header.Get("Authorization"))
		s.Equal("application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		s.Contains(string(body), `"google/gemini-flash"`)
		s.NotContains(string(body), `"stream"`)

		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"{\"result\":\"ok\"}"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":100,"completion_tokens":50}
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	resp, err := sut.Complete(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})

	s.NoError(err)
	s.Equal("{\"result\":\"ok\"}", resp.Content)
	s.Equal(100, resp.PromptTokens)
	s.Equal(50, resp.CompletionTokens)
	s.False(resp.TruncatedByLength)
}

func (s *LLMProviderSuite) TestComplete_RetriesTransientPost() {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"{\"result\":\"ok\"}"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":1}
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	resp, err := sut.Complete(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})

	s.NoError(err)
	s.Equal("{\"result\":\"ok\"}", resp.Content)
	s.Equal(int32(2), attempts.Load())
}

func (s *LLMProviderSuite) TestComplete_WithSchema() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.Contains(string(body), `"json_schema"`)
		s.Contains(string(body), `"my_schema"`)
		s.Contains(string(body), `"strict":true`)

		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"{\"x\":1}"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":5}
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	resp, err := sut.Complete(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "q"}},
		Schema: &Schema{
			Name:   "my_schema",
			Strict: true,
			Schema: map[string]any{"type": "object"},
		},
	})

	s.NoError(err)
	s.Equal("{\"x\":1}", resp.Content)
}

func (s *LLMProviderSuite) TestComplete_FreeText_NoResponseFormat() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.NotContains(string(body), `"response_format"`)

		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"hello there"},"finish_reason":"stop"}],
			"usage":{}
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	resp, err := sut.Complete(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "hi"}},
		FreeText: true,
	})

	s.NoError(err)
	s.Equal("hello there", resp.Content)
}

func (s *LLMProviderSuite) TestComplete_TruncatedByLength() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"partial"},"finish_reason":"length"}],
			"usage":{"prompt_tokens":50,"completion_tokens":256}
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	resp, err := sut.Complete(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "q"}},
		FreeText: true,
	})

	s.NoError(err)
	s.True(resp.TruncatedByLength)
}

func (s *LLMProviderSuite) TestComplete_UpstreamError_Status5xx() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	_, err := sut.Complete(s.ctx, Request{Messages: []Message{{Role: "user", Content: "q"}}})

	s.Error(err)
	s.True(errors.Is(err, ErrProviderUpstream))
}

func (s *LLMProviderSuite) TestComplete_EmptyChoices() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[],"usage":{}}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	_, err := sut.Complete(s.ctx, Request{Messages: []Message{{Role: "user", Content: "q"}}})

	s.Error(err)
	s.True(errors.Is(err, ErrEmptyChoices))
}

func (s *LLMProviderSuite) TestComplete_UpstreamErrorField() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded","code":429}}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	_, err := sut.Complete(s.ctx, Request{Messages: []Message{{Role: "user", Content: "q"}}})

	s.Error(err)
	s.True(errors.Is(err, ErrProviderUpstream))
}

func (s *LLMProviderSuite) TestComplete_ToolCalls() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.Contains(string(body), `"tools"`)
		s.NotContains(string(body), `"response_format"`)

		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"","tool_calls":[
				{"id":"tc1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SP\"}"}}
			]},"finish_reason":"tool_calls"}],
			"usage":{"prompt_tokens":50,"completion_tokens":20}
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	resp, err := sut.Complete(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "weather?"}},
		Tools: []ToolSpec{{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters:  map[string]any{"type": "object"},
		}},
	})

	s.NoError(err)
	s.Len(resp.ToolCalls, 1)
	s.Equal("get_weather", resp.ToolCalls[0].FunctionName)
	s.Equal("tc1", resp.ToolCalls[0].ID)
	args := resp.ToolCalls[0].ArgumentsJSON
	s.Equal("SP", args["city"])
}

func (s *LLMProviderSuite) TestStream_HappyPath() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.Contains(string(body), `"stream":true`)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		s.True(ok)

		chunks := []string{"Hello", " world", "!"}
		for _, c := range chunks {
			chunk := sseChunk{Choices: []sseChoice{{Delta: sseDelta{Content: c}}}}
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		doneChunk := sseChunk{
			Choices: []sseChoice{{Delta: sseDelta{}, FinishReason: strPtr("stop")}},
			Usage:   &chatUsage{PromptTokens: 10, CompletionTokens: 5},
		}
		doneData, _ := json.Marshal(doneChunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", doneData)
		flusher.Flush()
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	stream, err := sut.Stream(s.ctx, Request{
		Messages: []Message{{Role: "user", Content: "hello"}},
		FreeText: true,
	})
	s.Require().NoError(err)
	defer stream.Close() //nolint:errcheck

	var collected []string
	for delta := range stream.Deltas() {
		collected = append(collected, delta)
	}
	s.NoError(stream.Err())
	s.Equal([]string{"Hello", " world", "!"}, collected)
}

func (s *LLMProviderSuite) TestStream_ContextCancellation() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunk := sseChunk{Choices: []sseChoice{{Delta: sseDelta{Content: "first"}}}}
		data, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(s.ctx)
	sut := s.buildProvider(server)
	stream, err := sut.Stream(ctx, Request{
		Messages: []Message{{Role: "user", Content: "hello"}},
		FreeText: true,
	})
	s.Require().NoError(err)

	<-stream.Deltas()
	cancel()

	for range stream.Deltas() {
	}
	_ = stream.Close()
}

func (s *LLMProviderSuite) TestEmbed_HappyPath() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("/api/v1/embeddings", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		s.Contains(string(body), `"openai/text-embedding-3-small"`)

		_, _ = w.Write([]byte(`{
			"data":[
				{"embedding":[0.1,0.2,0.3],"index":0},
				{"embedding":[0.4,0.5,0.6],"index":1}
			]
		}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	result, err := sut.Embed(s.ctx, []string{"hello", "world"})

	s.NoError(err)
	s.Len(result, 2)
	s.InDelta(0.1, float64(result[0][0]), 0.001)
	s.InDelta(0.4, float64(result[1][0]), 0.001)
}

func (s *LLMProviderSuite) TestEmbed_UpstreamError() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	_, err := sut.Embed(s.ctx, []string{"text"})

	s.Error(err)
	s.True(errors.Is(err, ErrProviderUpstream))
}

func (s *LLMProviderSuite) TestSlug() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	sut := s.buildProvider(server)
	s.Equal("google/gemini-flash", sut.Slug())
}

func (s *LLMProviderSuite) TestStructuredContract_Decode_Conformant() {
	type myOutput struct {
		Result string `json:"result"`
	}
	contract := &testContract[myOutput]{
		schema: Schema{
			Name:   "my_output",
			Strict: true,
			Schema: map[string]any{"type": "object"},
		},
		decodeFn: func(raw []byte) (myOutput, error) {
			var out myOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return myOutput{}, fmt.Errorf("%w: %w", ErrContractViolation, err)
			}
			if out.Result == "" {
				return myOutput{}, fmt.Errorf("%w: result is empty", ErrContractViolation)
			}
			return out, nil
		},
	}

	out, err := contract.Decode([]byte(`{"result":"ok"}`))
	s.NoError(err)
	s.Equal("ok", out.Result)
}

func (s *LLMProviderSuite) TestStructuredContract_Decode_NotConformant() {
	type myOutput struct {
		Result string `json:"result"`
	}
	contract := &testContract[myOutput]{
		schema: Schema{Name: "my_output", Strict: true, Schema: map[string]any{}},
		decodeFn: func(raw []byte) (myOutput, error) {
			var out myOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return myOutput{}, fmt.Errorf("%w: %w", ErrContractViolation, err)
			}
			if out.Result == "" {
				return myOutput{}, fmt.Errorf("%w: result is empty", ErrContractViolation)
			}
			return out, nil
		},
	}

	_, err := contract.Decode([]byte(`{"result":""}`))
	s.Error(err)
	s.True(errors.Is(err, ErrContractViolation))
}

func (s *LLMProviderSuite) TestClassifyStatus() {
	cases := []struct {
		code   int
		expect string
	}{
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusPaymentRequired, "no_credit"},
		{http.StatusTooManyRequests, "rate_limited"},
		{http.StatusRequestTimeout, "timeout"},
		{http.StatusInternalServerError, "upstream_5xx"},
		{http.StatusBadGateway, "upstream_5xx"},
		{http.StatusBadRequest, "client_4xx"},
		{http.StatusNotFound, "client_4xx"},
	}
	for _, tc := range cases {
		s.Equal(tc.expect, classifyStatus(tc.code), "status %d", tc.code)
	}
}

func (s *LLMProviderSuite) TestGate_SchemaIsInjected() {
	req := Request{
		Messages: []Message{{Role: "user", Content: "q"}},
		Schema:   &Schema{Name: "test_schema", Strict: true, Schema: map[string]any{"type": "object"}},
	}
	built := (&openrouterProvider{cfg: Config{Model: "m", MaxTokens: 10}}).buildChatRequest(req, false)
	s.Require().NotNil(built.ResponseFmt)
	s.Equal("json_schema", built.ResponseFmt.Type)
	s.Require().NotNil(built.ResponseFmt.JSONSchema)
	s.Equal("test_schema", built.ResponseFmt.JSONSchema.Name)
	s.NotEqual("mecontrola_intent", built.ResponseFmt.JSONSchema.Name)
}

func (s *LLMProviderSuite) TestGate_ToolResultMessageSerialized() {
	req := Request{
		Messages: []Message{
			{Role: "user", Content: "weather?"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{{
				ID:            "tc1",
				FunctionName:  "get_weather",
				ArgumentsJSON: map[string]any{"city": "NY"},
			}}},
			{Role: "tool", ToolCallID: "tc1", Content: `{"temp":"20C"}`},
		},
	}
	built := (&openrouterProvider{cfg: Config{Model: "m", MaxTokens: 10}}).buildChatRequest(req, false)
	s.Require().Len(built.Messages, 3)

	assistant := built.Messages[1]
	s.Require().Len(assistant.ToolCalls, 1)
	s.Equal("tc1", assistant.ToolCalls[0].ID)
	s.Equal("function", assistant.ToolCalls[0].Type)
	s.Equal("get_weather", assistant.ToolCalls[0].Function.Name)
	s.Contains(assistant.ToolCalls[0].Function.Arguments, "NY")

	toolMsg := built.Messages[2]
	s.Equal("tool", toolMsg.Role)
	s.Equal("tc1", toolMsg.ToolCallID)
	s.Equal(`{"temp":"20C"}`, toolMsg.Content)

	encoded, err := json.Marshal(built)
	s.Require().NoError(err)
	s.Contains(string(encoded), `"tool_call_id":"tc1"`)
	s.Contains(string(encoded), `"role":"tool"`)
}

type testContract[T any] struct {
	schema   Schema
	decodeFn func([]byte) (T, error)
}

func (c *testContract[T]) Schema() Schema               { return c.schema }
func (c *testContract[T]) Decode(raw []byte) (T, error) { return c.decodeFn(raw) }

func strPtr(s string) *string { v := s; return &v }
