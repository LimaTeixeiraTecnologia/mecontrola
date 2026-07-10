package agents

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents/guards"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const (
	guardDecisionPass    = "pass"
	guardDecisionHandled = "handled"
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
			result = decision.Result
			continue
		}
		g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionPass)
	}

	return result, nil
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
