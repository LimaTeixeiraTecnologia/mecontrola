package agents

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

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
	runner  scorer.ScorerRunner
	o11y    observability.Observability
	skipped observability.Counter
}

func NewScoringHooks(runner scorer.ScorerRunner, o11y observability.Observability) *ScoringHooks {
	h := &ScoringHooks{runner: runner, o11y: o11y}
	if o11y != nil {
		h.skipped = o11y.Metrics().Counter("agent_run_scorer_skipped_total", "Total agent runs without scorer observation", "1")
	}
	return h
}

func (h *ScoringHooks) BeforeExecute(ctx context.Context, _ string, in agent.Request) context.Context {
	obs := &scoringObservation{input: lastUserMessage(in.Messages)}
	return context.WithValue(ctx, scoringStateKey{}, obs)
}

func (h *ScoringHooks) AfterExecute(ctx context.Context, agentID string, result agent.Result, err error) {
	if err != nil {
		return
	}
	if h.runner == nil {
		h.recordScorerSkipped(ctx, agentID, "runner_nil")
		return
	}
	obs, ok := ctx.Value(scoringStateKey{}).(*scoringObservation)
	if !ok {
		h.recordScorerSkipped(ctx, agentID, "observation_missing")
		return
	}
	runID, ok := agent.RunIDFromContext(ctx)
	if !ok {
		h.recordScorerSkipped(ctx, agentID, "run_id_missing")
		return
	}
	h.runner.Observe(ctx, runID, scorer.RunSample{
		Input:     obs.input,
		Output:    result.Content,
		ToolCalls: obs.toolCalls,
	})
}

func (h *ScoringHooks) recordScorerSkipped(ctx context.Context, agentID, reason string) {
	if h.o11y == nil {
		return
	}
	if h.skipped != nil {
		h.skipped.Add(ctx, 1, observability.String("agent_id", agentID), observability.String("reason", reason))
	}
	h.o11y.Logger().Warn(ctx, "agents.scoring_hooks.after_execute: scorer nao observado",
		observability.String("agent_id", agentID),
		observability.String("reason", reason),
	)
}

func (h *ScoringHooks) BeforeTool(ctx context.Context, _, _ string) context.Context {
	return ctx
}

func (h *ScoringHooks) AfterTool(ctx context.Context, _, toolID string, argsJSON, _ []byte, err error) {
	if err != nil {
		return
	}
	obs, ok := ctx.Value(scoringStateKey{}).(*scoringObservation)
	if !ok {
		return
	}
	var args map[string]any
	if len(argsJSON) > 0 {
		_ = json.Unmarshal(argsJSON, &args)
	}
	obs.toolCalls = append(obs.toolCalls, scorer.ToolCallRecord{ID: toolID, Name: toolID, Args: args})
}

func lastUserMessage(msgs []llm.Message) string {
	for _, msg := range slices.Backward(msgs) {
		if msg.Role == "user" {
			return msg.Content
		}
	}
	return ""
}
