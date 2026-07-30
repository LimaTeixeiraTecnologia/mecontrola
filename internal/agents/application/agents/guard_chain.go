package agents

import (
	"context"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents/guards"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

const (
	guardDecisionPass    = "pass"
	guardDecisionHandled = "handled"
	maxGuardEvidenceSize = 500

	guardRetryOutcomeRecovered = "recovered"
	guardRetryOutcomeExhausted = "exhausted"

	guardRetryNudgeRole    = "system"
	guardRetryNudgeContent = "LEMBRETE CRITICO: sua ultima resposta continha um bloco de confirmacao ou pergunta de escrita fabricado por voce mesmo, sem chamar a ferramenta correspondente — isso e proibido pelas suas instrucoes. Chame AGORA a ferramenta de escrita apropriada (register_expense, register_income, create_recurrence, edit_entry ou equivalente) com os dados ja presentes nesta conversa. Nao escreva o texto de confirmacao ou pergunta voce mesmo; a ferramenta retorna esse texto pronto."
)

type guardChainMetrics struct {
	decisions observability.Counter
	retries   observability.Counter
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
		g.metrics.retries = o11y.Metrics().Counter(
			"agent_guard_retry_total",
			"Total re-executions of the LLM triggered by a retryable post-guard",
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
			result, _ := g.runPostGuards(ctx, in, decision.Result)
			return result, nil
		}
		g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionPass)
	}

	result, err := g.Agent.Execute(ctx, in)
	if err != nil {
		return result, err
	}

	result, retryGuard := g.runPostGuards(ctx, in, result)
	if retryGuard == "" {
		return result, nil
	}

	retryIn := in
	retryIn.Messages = append(append([]llm.Message{}, in.Messages...), llm.Message{
		Role:    guardRetryNudgeRole,
		Content: guardRetryNudgeContent,
	})

	retried, retryErr := g.Agent.Execute(ctx, retryIn)
	if retryErr != nil {
		g.logGuardRetryError(ctx, in.AgentID, retryGuard, retryErr)
		g.recordGuardRetry(ctx, in.AgentID, retryGuard, guardRetryOutcomeExhausted)
		return result, nil
	}

	finalResult, secondRetryGuard := g.runPostGuards(ctx, in, retried)
	if secondRetryGuard == "" {
		g.recordGuardRetry(ctx, in.AgentID, retryGuard, guardRetryOutcomeRecovered)
		return finalResult, nil
	}
	g.recordGuardRetry(ctx, in.AgentID, secondRetryGuard, guardRetryOutcomeExhausted)
	return finalResult, nil
}

func (g *guardChainAgent) runPostGuards(ctx context.Context, in agent.Request, result agent.Result) (agent.Result, string) {
	retryGuard := ""
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
			if decision.Retryable {
				retryGuard = guard.Name()
			}
			continue
		}
		g.recordDecision(ctx, in.AgentID, guard.Name(), guardDecisionPass)
	}
	return result, retryGuard
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

func (g *guardChainAgent) logGuardRetryError(ctx context.Context, agentID, guardName string, err error) {
	if g.o11y == nil {
		return
	}
	g.o11y.Logger().Warn(ctx, "agents.guard_chain: retry apos guard retryable falhou; mantendo fallback",
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

func (g *guardChainAgent) recordGuardRetry(ctx context.Context, agentID, guardName, outcome string) {
	if g.metrics.retries == nil {
		return
	}
	g.metrics.retries.Add(ctx, 1,
		observability.String("agent_id", agentID),
		observability.String("guard", guardName),
		observability.String("outcome", outcome),
	)
}
