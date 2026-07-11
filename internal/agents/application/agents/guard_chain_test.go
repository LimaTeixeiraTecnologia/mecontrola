package agents

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents/guards"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type stubPreGuard struct {
	name     string
	decision guards.GuardDecision
	inspects int
}

func (g *stubPreGuard) Name() string { return g.name }
func (g *stubPreGuard) Inspect(_ context.Context, _ agent.Request) guards.GuardDecision {
	g.inspects++
	return g.decision
}

type stubPostGuard struct {
	name     string
	decision guards.GuardDecision
	inspects int
}

func (g *stubPostGuard) Name() string { return g.name }
func (g *stubPostGuard) Inspect(_ context.Context, _ agent.Request, _ agent.Result) guards.GuardDecision {
	g.inspects++
	return g.decision
}

type stubGuardChainUnderlyingAgent struct {
	executeCalled bool
	result        agent.Result
	err           error
}

func (a *stubGuardChainUnderlyingAgent) ID() string           { return "stub-agent" }
func (a *stubGuardChainUnderlyingAgent) Instructions() string { return "" }
func (a *stubGuardChainUnderlyingAgent) Stream(ctx context.Context, in agent.Request) (agent.ResultStream, error) {
	return nil, nil
}

func (a *stubGuardChainUnderlyingAgent) Execute(ctx context.Context, in agent.Request) (agent.Result, error) {
	a.executeCalled = true
	return a.result, a.err
}

type GuardChainAgentSuite struct {
	suite.Suite
	ctx context.Context
}

func TestGuardChainAgentSuite(t *testing.T) {
	suite.Run(t, new(GuardChainAgentSuite))
}

func (s *GuardChainAgentSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *GuardChainAgentSuite) TestExecute_PreGuardShortCircuits_DoesNotCallLLM() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta do llm"}}
	handledResult := agent.Result{Content: "tratado pelo guard", ToolOutcome: agent.ToolOutcomeClarify}
	pre := &stubPreGuard{name: "pre-1", decision: guards.GuardDecision{Handled: true, Result: handledResult}}

	built := WithGuardChain(underlying, fake.NewProvider(), []guards.PreGuard{pre}, nil)
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.False(underlying.executeCalled, "nao deve chamar o LLM subjacente quando um PreGuard trata")
	s.Equal(1, pre.inspects)
	s.Equal(handledResult.Content, output.Content)
	s.Equal(handledResult.ToolOutcome, output.ToolOutcome)
}

func (s *GuardChainAgentSuite) TestExecute_PreGuardOrder_FirstHandledWins() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta do llm"}}
	first := &stubPreGuard{name: "pre-first", decision: guards.GuardDecision{Handled: true, Result: agent.Result{Content: "primeiro"}}}
	second := &stubPreGuard{name: "pre-second", decision: guards.GuardDecision{Handled: true, Result: agent.Result{Content: "segundo"}}}

	built := WithGuardChain(underlying, fake.NewProvider(), []guards.PreGuard{first, second}, nil)
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal("primeiro", output.Content)
	s.Equal(1, first.inspects)
	s.Equal(0, second.inspects, "guard apos o primeiro que tratou nao deve ser inspecionado")
}

func (s *GuardChainAgentSuite) TestExecute_NoPreGuardHandles_DelegatesToUnderlying() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta do llm"}}
	pre := &stubPreGuard{name: "pre-1", decision: guards.GuardDecision{}}

	built := WithGuardChain(underlying, fake.NewProvider(), []guards.PreGuard{pre}, nil)
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.True(underlying.executeCalled)
	s.Equal(1, pre.inspects)
	s.Equal("resposta do llm", output.Content)
}

func (s *GuardChainAgentSuite) TestExecute_PostGuardOverridesResult() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta original"}}
	post := &stubPostGuard{name: "post-1", decision: guards.GuardDecision{Handled: true, Result: agent.Result{Content: "resposta corrigida"}}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{post})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(1, post.inspects)
	s.Equal("resposta corrigida", output.Content)
}

func (s *GuardChainAgentSuite) TestExecute_PostGuardPass_DoesNotOverride() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta válida"}}
	post := &stubPostGuard{name: "post-1", decision: guards.GuardDecision{}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{post})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(1, post.inspects)
	s.Equal("resposta válida", output.Content)
}

func (s *GuardChainAgentSuite) TestExecute_UnderlyingAgentError_PropagatesWithoutPostGuards() {
	underlying := &stubGuardChainUnderlyingAgent{err: assertAnError{}}
	post := &stubPostGuard{name: "post-1", decision: guards.GuardDecision{Handled: true, Result: agent.Result{Content: "nao deveria aparecer"}}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{post})
	_, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.Error(err)
	s.Equal(0, post.inspects, "post guards nao devem rodar quando o agente subjacente falha")
}

type assertAnError struct{}

func (assertAnError) Error() string { return "erro simulado" }

func (s *GuardChainAgentSuite) TestExecute_MultiplePostGuards_AllInspected() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta"}}
	post1 := &stubPostGuard{name: "post-1", decision: guards.GuardDecision{}}
	post2 := &stubPostGuard{name: "post-2", decision: guards.GuardDecision{Handled: true, Result: agent.Result{Content: "corrigido por post-2"}}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{post1, post2})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(1, post1.inspects)
	s.Equal(1, post2.inspects)
	s.Equal("corrigido por post-2", output.Content)
}

func (s *GuardChainAgentSuite) TestExecute_VerbatimRelayBeforeCardProvenance_PreservesPixConfirmation() {
	verbatim := "Confirma o lançamento de R$ 50,00 no supermercado via pix?"
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{
		Content: "resposta original do agente",
		ToolCalls: []agent.ToolCallRecord{
			{
				Tool:          "register_expense",
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       `{"outcome":"clarify","message":"` + verbatim + `"}`,
				ArgumentsJSON: map[string]any{"paymentMethod": "pix"},
			},
		},
	}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{
		guards.NewVerbatimRelayGuard(),
		guards.NewCardProvenanceGuard(),
	})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(verbatim, output.Content)
}

func (s *GuardChainAgentSuite) TestExecute_CardProvenance_DecisionsRecorded() {
	scenarios := []struct {
		name           string
		paymentMethod  string
		expectDecision string
	}{
		{
			name:           "credit_card sem resolucao e handled",
			paymentMethod:  "credit_card",
			expectDecision: "handled",
		},
		{
			name:           "pix nao e handled",
			paymentMethod:  "pix",
			expectDecision: "pass",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			o11y := fake.NewProvider()
			underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{
				Content: "resposta original",
				ToolCalls: []agent.ToolCallRecord{
					{
						Tool:          "register_expense",
						Outcome:       agent.ToolCallOutcomeSuccess,
						Content:       `{"outcome":"routed"}`,
						ArgumentsJSON: map[string]any{"paymentMethod": scenario.paymentMethod},
					},
				},
			}}

			built := WithGuardChain(underlying, o11y, nil, []guards.PostGuard{guards.NewCardProvenanceGuard()})
			_, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

			s.NoError(err)
			counter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_decisions_total")
			s.Require().NotNil(counter)
			var found bool
			for _, v := range counter.GetValues() {
				if s.hasLabel(v.Fields, "guard", "card_provenance") && s.hasLabel(v.Fields, "decision", scenario.expectDecision) {
					found = true
					break
				}
			}
			s.True(found, "deveria registrar decisao %s para card_provenance", scenario.expectDecision)
		})
	}
}

func (s *GuardChainAgentSuite) hasLabel(fields []observability.Field, key, value string) bool {
	for _, f := range fields {
		if f.Key == key && f.StringValue() == value {
			return true
		}
	}
	return false
}

func (s *GuardChainAgentSuite) TestStream_DelegatesToUnderlyingAgent() {
	underlying := &stubGuardChainUnderlyingAgent{}
	built := WithGuardChain(underlying, fake.NewProvider(), nil, nil)

	stream, err := built.Stream(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Nil(stream)
}

func (s *GuardChainAgentSuite) TestID_DelegatesToUnderlyingAgent() {
	underlying := &stubGuardChainUnderlyingAgent{}
	built := WithGuardChain(underlying, fake.NewProvider(), nil, nil)

	s.Equal("stub-agent", built.ID())
}
