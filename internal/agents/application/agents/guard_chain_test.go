package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents/guards"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
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

type sequencedGuardChainUnderlyingAgent struct {
	results        []agent.Result
	errs           []error
	calls          int
	capturedInputs []agent.Request
}

func (a *sequencedGuardChainUnderlyingAgent) ID() string           { return "stub-agent" }
func (a *sequencedGuardChainUnderlyingAgent) Instructions() string { return "" }
func (a *sequencedGuardChainUnderlyingAgent) Stream(ctx context.Context, in agent.Request) (agent.ResultStream, error) {
	return nil, nil
}

func (a *sequencedGuardChainUnderlyingAgent) Execute(ctx context.Context, in agent.Request) (agent.Result, error) {
	idx := a.calls
	if idx >= len(a.results) {
		idx = len(a.results) - 1
	}
	a.capturedInputs = append(a.capturedInputs, in)
	a.calls++
	var err error
	if idx < len(a.errs) {
		err = a.errs[idx]
	}
	return a.results[idx], err
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

func (s *GuardChainAgentSuite) TestExecute_PreGuardInvokeError_LogsAndDelegates() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta do llm"}}
	pre := &stubPreGuard{name: "pre-err", decision: guards.GuardDecision{InvokeErr: errors.New("schema validation")}}

	built := WithGuardChain(underlying, fake.NewProvider(), []guards.PreGuard{pre}, nil)
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.True(underlying.executeCalled, "erro de invoke no shortcut deve delegar ao LLM")
	s.Equal("resposta do llm", output.Content)
}

func (s *GuardChainAgentSuite) TestExecute_PreGuardInvokeError_NilObservability_NoPanic() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta do llm"}}
	pre := &stubPreGuard{name: "pre-err", decision: guards.GuardDecision{InvokeErr: errors.New("schema validation")}}

	built := WithGuardChain(underlying, nil, []guards.PreGuard{pre}, nil)
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.True(underlying.executeCalled)
	s.Equal("resposta do llm", output.Content)
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

func (s *GuardChainAgentSuite) TestExecute_PostGuardForcedUsecaseError_AppendsEvidenceRecord() {
	fabricated := "Qual é a categoria para esse gasto? 📂\n1. *Conhecimento*\n\nResponda o número ou o nome. 🙂"
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: fabricated}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{guards.NewCategoryWithoutToolGuard()})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(agent.ToolOutcomeUsecaseError, output.ToolOutcome)
	s.Require().Len(output.ToolCalls, 1)
	s.Equal("guard:category_without_tool", output.ToolCalls[0].Tool)
	s.Equal(agent.ToolCallOutcomeError, output.ToolCalls[0].Outcome)
	s.Equal(fabricated, output.ToolCalls[0].Content)
}

func (s *GuardChainAgentSuite) TestExecute_PostGuardHandledWithoutUsecaseError_NoEvidenceRecord() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta original"}}
	post := &stubPostGuard{name: "post-1", decision: guards.GuardDecision{Handled: true, Result: agent.Result{Content: "resposta corrigida"}}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{post})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal("resposta corrigida", output.Content)
	s.Empty(output.ToolCalls)
}

func (s *GuardChainAgentSuite) TestExecute_ResultAlreadyUsecaseError_NoDuplicateEvidence() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{
		Content:     "texto com Posso registrar?",
		ToolOutcome: agent.ToolOutcomeUsecaseError,
		ToolCalls: []agent.ToolCallRecord{{
			Tool:    "register_expense",
			Outcome: agent.ToolCallOutcomeError,
			Content: "tool register_expense: falha real",
		}},
	}}

	built := WithGuardChain(underlying, fake.NewProvider(), nil, []guards.PostGuard{guards.NewConfirmationWithoutToolGuard()})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Require().Len(output.ToolCalls, 1)
	s.Equal("register_expense", output.ToolCalls[0].Tool)
}

func (s *GuardChainAgentSuite) TestExecute_RetryableGuardHandledFirstAttempt_RecoversOnSecondAttempt() {
	fabricated := "Posso registrar?"
	recovered := agent.Result{
		Content: "Como você pagou?",
		ToolCalls: []agent.ToolCallRecord{{
			Tool:    "register_expense",
			Outcome: agent.ToolCallOutcomeSuccess,
			Content: `{"outcome":"clarify","message":"Como você pagou?"}`,
		}},
	}
	underlying := &sequencedGuardChainUnderlyingAgent{results: []agent.Result{
		{Content: fabricated},
		recovered,
	}}
	o11y := fake.NewProvider()

	original := agent.Request{AgentID: "agent-1", Messages: []llm.Message{{Role: "user", Content: "Fiz a compra do mês e foi 150 no mercado"}}}
	built := WithGuardChain(underlying, o11y, nil, []guards.PostGuard{guards.NewConfirmationWithoutToolGuard()})
	output, err := built.Execute(s.ctx, original)

	s.NoError(err)
	s.Equal(2, underlying.calls)
	s.Equal("Como você pagou?", output.Content)
	s.NotEqual(agent.ToolOutcomeUsecaseError, output.ToolOutcome)

	s.Require().Len(underlying.capturedInputs, 2)
	s.Equal(original.Messages, underlying.capturedInputs[0].Messages, "primeira tentativa deve usar as mensagens originais sem alteracao")
	s.Require().Len(underlying.capturedInputs[1].Messages, len(original.Messages)+1)
	s.Equal(original.Messages, underlying.capturedInputs[1].Messages[:len(original.Messages)], "retry deve preservar o historico original intacto")
	nudge := underlying.capturedInputs[1].Messages[len(original.Messages)]
	s.Equal("system", nudge.Role)
	s.Contains(nudge.Content, "Chame AGORA a ferramenta de escrita")

	counter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_retry_total")
	s.Require().NotNil(counter)
	var found bool
	for _, v := range counter.GetValues() {
		if s.hasLabel(v.Fields, "guard", "confirmation_without_tool") && s.hasLabel(v.Fields, "outcome", "recovered") {
			found = true
			break
		}
	}
	s.True(found, "deveria registrar outcome=recovered para confirmation_without_tool")
}

func (s *GuardChainAgentSuite) TestExecute_RetryableGuardHandledBothAttempts_FallsBackAfterExhausted() {
	fabricated := agent.Result{Content: "Posso registrar?"}
	underlying := &sequencedGuardChainUnderlyingAgent{results: []agent.Result{fabricated, fabricated}}
	o11y := fake.NewProvider()

	built := WithGuardChain(underlying, o11y, nil, []guards.PostGuard{guards.NewConfirmationWithoutToolGuard()})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(2, underlying.calls)
	s.Equal("Não consegui registrar. Tente novamente em breve.", output.Content)
	s.Equal(agent.ToolOutcomeUsecaseError, output.ToolOutcome)
	s.Require().Len(output.ToolCalls, 1)
	s.Equal("guard:confirmation_without_tool", output.ToolCalls[0].Tool)

	counter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_retry_total")
	s.Require().NotNil(counter)
	var found bool
	for _, v := range counter.GetValues() {
		if s.hasLabel(v.Fields, "guard", "confirmation_without_tool") && s.hasLabel(v.Fields, "outcome", "exhausted") {
			found = true
			break
		}
	}
	s.True(found, "deveria registrar outcome=exhausted para confirmation_without_tool")

	decisionsCounter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_decisions_total")
	s.Require().NotNil(decisionsCounter)
	handledCount := 0
	for _, v := range decisionsCounter.GetValues() {
		if s.hasLabel(v.Fields, "guard", "confirmation_without_tool") && s.hasLabel(v.Fields, "decision", "handled") {
			handledCount++
		}
	}
	s.Equal(2, handledCount, "guard deve ser inspecionado e marcado handled nas duas tentativas")
}

func (s *GuardChainAgentSuite) TestExecute_NonRetryableGuardHandled_DoesNotRetry() {
	underlying := &stubGuardChainUnderlyingAgent{result: agent.Result{Content: "resposta original"}}
	post := &stubPostGuard{name: "post-1", decision: guards.GuardDecision{Handled: true, Retryable: false, Result: agent.Result{Content: "corrigido"}}}
	o11y := fake.NewProvider()

	built := WithGuardChain(underlying, o11y, nil, []guards.PostGuard{post})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal("corrigido", output.Content)

	counter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_retry_total")
	if counter != nil {
		s.Empty(counter.GetValues())
	}
}

func (s *GuardChainAgentSuite) TestExecute_RetryableGuardHandled_RetryLLMErrors_KeepsFirstFallback() {
	fabricated := agent.Result{Content: "Posso registrar?"}
	underlying := &sequencedGuardChainUnderlyingAgent{
		results: []agent.Result{fabricated, {}},
		errs:    []error{nil, assertAnError{}},
	}
	o11y := fake.NewProvider()

	built := WithGuardChain(underlying, o11y, nil, []guards.PostGuard{guards.NewConfirmationWithoutToolGuard()})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(2, underlying.calls)
	s.Equal("Não consegui registrar. Tente novamente em breve.", output.Content)
	s.Equal(agent.ToolOutcomeUsecaseError, output.ToolOutcome)

	counter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_retry_total")
	s.Require().NotNil(counter)
	var found bool
	for _, v := range counter.GetValues() {
		if s.hasLabel(v.Fields, "guard", "confirmation_without_tool") && s.hasLabel(v.Fields, "outcome", "exhausted") {
			found = true
			break
		}
	}
	s.True(found, "deveria registrar outcome=exhausted quando o retry falha com erro")
}

func (s *GuardChainAgentSuite) TestExecute_ExpiredWithoutTool_NotRetryable_NoRetryCall() {
	underlying := &sequencedGuardChainUnderlyingAgent{results: []agent.Result{
		{Content: "O registro expirou. Para registrar, envie a informação completa novamente."},
	}}
	o11y := fake.NewProvider()

	built := WithGuardChain(underlying, o11y, nil, []guards.PostGuard{guards.NewExpiredWithoutToolGuard()})
	output, err := built.Execute(s.ctx, agent.Request{AgentID: "agent-1"})

	s.NoError(err)
	s.Equal(1, underlying.calls)
	s.Equal(agent.ToolOutcomeClarify, output.ToolOutcome)

	counter := o11y.Metrics().(*fake.FakeMetrics).GetCounter("agent_guard_retry_total")
	if counter != nil {
		s.Empty(counter.GetValues())
	}
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
