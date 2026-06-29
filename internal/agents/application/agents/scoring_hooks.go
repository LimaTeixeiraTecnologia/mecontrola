package agents

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type scoringStateKey struct{}

type scoringObservation struct {
	input     string
	toolCalls []scorer.ToolCallRecord
}

type ScoringHooks struct {
	runner scorer.ScorerRunner
}

func NewScoringHooks(runner scorer.ScorerRunner) *ScoringHooks {
	return &ScoringHooks{runner: runner}
}

func (h *ScoringHooks) BeforeExecute(ctx context.Context, _ string, in agent.Request) context.Context {
	obs := &scoringObservation{input: lastUserMessage(in.Messages)}
	return context.WithValue(ctx, scoringStateKey{}, obs)
}

func (h *ScoringHooks) AfterExecute(ctx context.Context, _ string, result agent.Result, err error) {
	if err != nil || h.runner == nil {
		return
	}
	obs, ok := ctx.Value(scoringStateKey{}).(*scoringObservation)
	if !ok {
		return
	}
	runID, ok := agent.RunIDFromContext(ctx)
	if !ok {
		return
	}
	h.runner.Observe(ctx, runID, scorer.RunSample{
		Input:     obs.input,
		Output:    result.Content,
		ToolCalls: obs.toolCalls,
	})
}

func (h *ScoringHooks) BeforeTool(ctx context.Context, _, _ string) context.Context {
	return ctx
}

func (h *ScoringHooks) AfterTool(ctx context.Context, _, toolID string, _ []byte, err error) {
	if err != nil {
		return
	}
	obs, ok := ctx.Value(scoringStateKey{}).(*scoringObservation)
	if !ok {
		return
	}
	obs.toolCalls = append(obs.toolCalls, scorer.ToolCallRecord{ID: toolID, Name: toolID})
}

func lastUserMessage(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}
