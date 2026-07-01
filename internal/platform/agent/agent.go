package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const (
	defaultMaxToolRounds = 5
	roleAssistant        = "assistant"
	roleTool             = "tool"
)

type AgentOption func(*agentImpl)

func WithTools(tools ...tool.ToolHandle) AgentOption {
	return func(a *agentImpl) {
		a.tools = append(a.tools, tools...)
	}
}

func WithHooks(h Hooks) AgentOption {
	return func(a *agentImpl) {
		a.hooks = h
	}
}

func WithMaxToolRounds(n int) AgentOption {
	return func(a *agentImpl) {
		a.maxToolRounds = n
	}
}

type agentMetrics struct {
	streamTotal     observability.Counter
	toolInvocations observability.Counter
}

type agentImpl struct {
	id            string
	instructions  string
	provider      llm.Provider
	tools         []tool.ToolHandle
	hooks         Hooks
	o11y          observability.Observability
	metrics       agentMetrics
	maxToolRounds int
}

func NewAgent(id, instructions string, provider llm.Provider, o11y observability.Observability, opts ...AgentOption) Agent {
	a := &agentImpl{
		id:            id,
		instructions:  instructions,
		provider:      provider,
		hooks:         NoopHooks{},
		o11y:          o11y,
		maxToolRounds: defaultMaxToolRounds,
		metrics: agentMetrics{
			streamTotal:     o11y.Metrics().Counter("agent_stream_total", "Total agent stream runs", "1"),
			toolInvocations: o11y.Metrics().Counter("agent_tool_invocations_total", "Total agent tool invocations", "1"),
		},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *agentImpl) ID() string {
	return a.id
}

func (a *agentImpl) Instructions() string {
	return a.instructions
}

func (a *agentImpl) Execute(ctx context.Context, in Request) (Result, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agent.execute",
		observability.WithAttributes(
			observability.String("agent_id", a.id),
		),
	)
	defer span.End()

	toolSpecs, toolMap := a.prepareTools()
	llmReq := llm.Request{
		Messages:    in.Messages,
		MaxTokens:   in.MaxTokens,
		Temperature: in.Temperature,
		Tools:       toolSpecs,
	}
	if in.Schema != nil {
		llmReq.Schema = in.Schema
	}

	ctx = a.hooks.BeforeExecute(ctx, a.id, in)

	resp, exhausted, err := a.completeWithTools(ctx, &llmReq, toolMap)
	if err != nil {
		span.RecordError(err)
		a.hooks.AfterExecute(ctx, a.id, Result{}, err)
		return Result{}, err
	}
	if exhausted {
		wrapped := fmt.Errorf("agent.execute: %w", ErrMaxToolRounds)
		span.RecordError(wrapped)
		a.hooks.AfterExecute(ctx, a.id, Result{}, wrapped)
		return Result{}, wrapped
	}

	if in.Decoder != nil {
		if validateErr := in.Decoder.Validate(resp.RawJSON); validateErr != nil {
			wrapped := fmt.Errorf("%w: %w", ErrContractNotMet, validateErr)
			span.RecordError(wrapped)
			a.hooks.AfterExecute(ctx, a.id, Result{}, wrapped)
			return Result{}, wrapped
		}
	}

	result := Result{
		Content: resp.Content,
		RawJSON: resp.RawJSON,
		Mode:    ExecutionModeSync,
	}

	a.hooks.AfterExecute(ctx, a.id, result, nil)

	return result, nil
}

func (a *agentImpl) prepareTools() ([]llm.ToolSpec, map[string]tool.ToolHandle) {
	specs := make([]llm.ToolSpec, 0, len(a.tools))
	toolMap := make(map[string]tool.ToolHandle, len(a.tools))
	for _, t := range a.tools {
		specs = append(specs, llm.ToolSpec{
			Name:        t.ID(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
		toolMap[t.ID()] = t
	}
	return specs, toolMap
}

func (a *agentImpl) completeWithTools(ctx context.Context, llmReq *llm.Request, toolMap map[string]tool.ToolHandle) (llm.Response, bool, error) {
	var resp llm.Response
	for round := 0; round < a.maxToolRounds; round++ {
		var err error
		resp, err = a.provider.Complete(ctx, *llmReq)
		if err != nil {
			return llm.Response{}, false, fmt.Errorf("agent.execute: complete: %w", err)
		}
		if len(resp.ToolCalls) == 0 {
			return resp, false, nil
		}
		llmReq.Messages = append(llmReq.Messages, llm.Message{
			Role:      roleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			if msg, ok := a.invokeToolCall(ctx, toolMap, tc); ok {
				llmReq.Messages = append(llmReq.Messages, msg)
			}
		}
	}
	return resp, len(resp.ToolCalls) > 0 && resp.Content == "", nil
}

func (a *agentImpl) invokeToolCall(ctx context.Context, toolMap map[string]tool.ToolHandle, tc llm.ToolCall) (llm.Message, bool) {
	h, ok := toolMap[tc.FunctionName]
	if !ok {
		return llm.Message{}, false
	}
	argsBytes, marshalErr := json.Marshal(tc.ArgumentsJSON)
	if marshalErr != nil {
		return llm.Message{}, false
	}
	tCtx := a.hooks.BeforeTool(ctx, a.id, h.ID())
	result, invokeErr := h.Invoke(tCtx, argsBytes)
	a.hooks.AfterTool(ctx, a.id, h.ID(), result, invokeErr)
	a.metrics.toolInvocations.Add(ctx, 1,
		observability.String("agent_id", a.id),
		observability.String("tool", h.ID()),
	)
	content := ""
	if invokeErr == nil {
		content = string(result)
	}
	return llm.Message{Role: roleTool, ToolCallID: tc.ID, Content: content}, true
}

func (a *agentImpl) Stream(ctx context.Context, in Request) (ResultStream, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agent.stream",
		observability.WithAttributes(
			observability.String("agent_id", a.id),
		),
	)
	defer span.End()

	llmReq := llm.Request{
		Messages:    in.Messages,
		MaxTokens:   in.MaxTokens,
		Temperature: in.Temperature,
		FreeText:    in.Schema == nil,
	}
	if in.Schema != nil {
		llmReq.Schema = in.Schema
	}

	ctx = a.hooks.BeforeExecute(ctx, a.id, in)

	ts, err := a.provider.Stream(ctx, llmReq)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agent.stream: provider.stream: %w", err)
	}

	a.metrics.streamTotal.Add(ctx, 1,
		observability.String("agent_id", a.id),
	)

	return newResultStream(ctx, ts, in.Decoder, a.hooks, a.id), nil
}
