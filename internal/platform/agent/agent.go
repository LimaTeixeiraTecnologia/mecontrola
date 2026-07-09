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

type toolExecStatus int

const (
	toolExecOK toolExecStatus = iota + 1
	toolExecError
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

func WithDefaultMaxTokens(n int) AgentOption {
	return func(a *agentImpl) {
		a.defaultMaxTokens = n
	}
}

type agentMetrics struct {
	streamTotal     observability.Counter
	toolInvocations observability.Counter
}

type agentImpl struct {
	id               string
	instructions     string
	provider         llm.Provider
	tools            []tool.ToolHandle
	hooks            Hooks
	o11y             observability.Observability
	metrics          agentMetrics
	defaultMaxTokens int
	maxToolRounds    int
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
		MaxTokens:   a.resolveMaxTokens(in.MaxTokens),
		Temperature: in.Temperature,
		Tools:       toolSpecs,
	}
	if in.Schema != nil {
		llmReq.Schema = in.Schema
	}

	ctx = a.hooks.BeforeExecute(ctx, a.id, in)

	resp, exhausted, toolStatus, toolCalls, err := a.completeWithTools(ctx, &llmReq, toolMap)
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

	toolOutcome := ToolOutcomeRouted
	if toolStatus == toolExecError {
		toolOutcome = ToolOutcomeUsecaseError
	}

	result := Result{
		Content:           resp.Content,
		RawJSON:           resp.RawJSON,
		Mode:              ExecutionModeSync,
		ToolOutcome:       toolOutcome,
		ToolCalls:         toolCalls,
		TruncatedByLength: resp.TruncatedByLength,
	}

	if resp.TruncatedByLength {
		span.SetAttributes(observability.Bool("truncated_by_length", true))
		a.o11y.Logger().Warn(ctx, "agent.execute.truncated_by_length",
			observability.String("agent_id", a.id),
		)
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

func (a *agentImpl) completeWithTools(ctx context.Context, llmReq *llm.Request, toolMap map[string]tool.ToolHandle) (llm.Response, bool, toolExecStatus, []ToolCallRecord, error) {
	var resp llm.Response
	var lastToolStatus toolExecStatus
	var allRecords []ToolCallRecord
	for round := 0; round < a.maxToolRounds; round++ {
		var err error
		resp, err = a.provider.Complete(ctx, *llmReq)
		if err != nil {
			return llm.Response{}, false, lastToolStatus, allRecords, fmt.Errorf("agent.execute: complete: %w", err)
		}
		if len(resp.ToolCalls) == 0 {
			return resp, false, lastToolStatus, allRecords, nil
		}
		llmReq.Messages = append(llmReq.Messages, llm.Message{
			Role:      roleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})
		roundStatus := toolExecOK
		verbatimText := ""
		for _, tc := range resp.ToolCalls {
			msg, record, status, verbatim, ok := a.invokeToolCall(ctx, toolMap, tc)
			if ok {
				llmReq.Messages = append(llmReq.Messages, msg)
				allRecords = append(allRecords, record)
			}
			if status == toolExecError {
				roundStatus = toolExecError
			}
			if verbatimText == "" && verbatim != "" {
				verbatimText = verbatim
			}
		}
		lastToolStatus = roundStatus
		if verbatimText != "" {
			return llm.Response{Content: verbatimText}, false, lastToolStatus, allRecords, nil
		}
	}
	return resp, len(resp.ToolCalls) > 0 && resp.Content == "", lastToolStatus, allRecords, nil
}

func (a *agentImpl) invokeToolCall(ctx context.Context, toolMap map[string]tool.ToolHandle, tc llm.ToolCall) (llm.Message, ToolCallRecord, toolExecStatus, string, bool) {
	h, ok := toolMap[tc.FunctionName]
	if !ok {
		return llm.Message{}, ToolCallRecord{}, toolExecError, "", false
	}
	argsBytes, marshalErr := json.Marshal(tc.ArgumentsJSON)
	if marshalErr != nil {
		return llm.Message{}, ToolCallRecord{}, toolExecError, "", false
	}
	tCtx := a.hooks.BeforeTool(ctx, a.id, h.ID())
	if id, idOk := ctx.Value(identityKey{}).(*toolIdentity); idOk && id != nil {
		seq := id.itemSeq
		id.itemSeq++
		tCtx = context.WithValue(tCtx, invocationItemSeqKey{}, seq)
	}
	result, verbatimText, invokeErr := h.Invoke(tCtx, argsBytes)
	a.hooks.AfterTool(ctx, a.id, h.ID(), result, invokeErr)
	a.metrics.toolInvocations.Add(ctx, 1,
		observability.String("agent_id", a.id),
		observability.String("tool", h.ID()),
	)
	if invokeErr != nil {
		content := fmt.Errorf("tool %s: %w", h.ID(), invokeErr).Error()
		record := ToolCallRecord{Tool: h.ID(), Outcome: ToolCallOutcomeError, Content: content}
		return llm.Message{Role: roleTool, ToolCallID: tc.ID, Content: content}, record, toolExecError, "", true
	}
	record := ToolCallRecord{Tool: h.ID(), Outcome: ToolCallOutcomeSuccess, Content: string(result)}
	return llm.Message{Role: roleTool, ToolCallID: tc.ID, Content: string(result)}, record, toolExecOK, verbatimText, true
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
		MaxTokens:   a.resolveMaxTokens(in.MaxTokens),
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

func (a *agentImpl) resolveMaxTokens(requested int) int {
	if requested > 0 {
		return requested
	}
	return a.defaultMaxTokens
}
