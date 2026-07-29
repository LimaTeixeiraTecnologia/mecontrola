package agents

import (
	"context"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents/guards"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const (
	guardDecisionPass    = "pass"
	guardDecisionHandled = "handled"
	maxGuardEvidenceSize = 500
)

type guardChainMetrics struct {
	decisions observability.Counter
}

type guardChainAgent struct {
	agent.Agent
	pre     []guards.PreGuard
	post    []guards.PostGuard
	metrics guardChainMetrics
	o11y    observability.Observability
}

func WithGuardChain(a agent.Agent, o11y observability.Observability, pre []guards.PreGuard, post []guards.PostGuard) agent.Agent {
	g := &guardChainAgent{
		Agent: a,
		pre:   pre,
		post:  post,
		o11y:  o11y,
	}
	if o11y != nil {
		g.metrics.decisions = o11y.Metrics().Counter(
			"agent_guard_decisions_total",
			"Total decisions taken by conversational guards",
			"1",
		)
	}
	return g
}

func (g *guardChainAgent) Execute(ctx context.Context, in agent.Request) (agent.Result, error) {
	for _, guard := range g.pre {
		decision := guard.Inspect(ctx, in)
		if decision.InvokeErr != nil {
			g.logGuardInvokeError(ctx, in.AgentID, guard.Name(), decision.InvokeErr)
		}
		if decision.Handled {
			g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionHandled)
			return decision.Result, nil
		}
		g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionPass)
	}

	result, err := g.Agent.Execute(ctx, in)
	if err != nil {
		return result, err
	}

	for _, guard := range g.post {
		decision := guard.Inspect(ctx, in, result)
		if decision.Handled {
			g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionHandled)
			if decision.Result.ToolOutcome == agent.ToolOutcomeUsecaseError && result.ToolOutcome != agent.ToolOutcomeUsecaseError {
				decision.Result.ToolCalls = append(decision.Result.ToolCalls, agent.ToolCallRecord{
					Tool:    "guard:" + guard.Name(),
					Outcome: agent.ToolCallOutcomeError,
					Content: truncateGuardEvidence(result.Content, maxGuardEvidenceSize),
				})
			}
			result = decision.Result
			continue
		}
		g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionPass)
	}

	return result, nil
}

func truncateGuardEvidence(content string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}

func (g *guardChainAgent) logGuardInvokeError(ctx context.Context, agentID, guardName string, err error) {
	if g.o11y == nil {
		return
	}
	g.o11y.Logger().Warn(ctx, "agents.guards: invoke do shortcut falhou; delegando ao llm",
		observability.String("agent_id", agentID),
		observability.String("guard", guardName),
		observability.Error(err),
	)
}

func (g *guardChainAgent) recordDecision(ctx context.Context, agentID, guardName, decision string) {
	if g.metrics.decisions == nil {
		return
	}
	g.metrics.decisions.Add(ctx, 1,
		observability.String("agent_id", agentID),
		observability.String("guard", guardName),
		observability.String("decision", decision),
	)
}
