package services

import (
	"context"
	"errors"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

const (
	workflowTransactions   = "transactions"
	workflowBudget         = "budget"
	workflowCards          = "cards"
	workflowConversational = "conversational"
)

type stubRuntimeRouter struct {
	result RouteResult
	calls  int
}

func (s *stubRuntimeRouter) route(_ context.Context, _ Principal, _, _, _, _ string) RouteResult {
	s.calls++
	return s.result
}

type stubThreadGateway struct {
	thread  entities.Thread
	err     error
	calls   int
	lastUID uuid.UUID
	lastCh  string
}

func (s *stubThreadGateway) GetOrCreate(_ context.Context, userID uuid.UUID, channel string) (entities.Thread, error) {
	s.calls++
	s.lastUID = userID
	s.lastCh = channel
	if s.err != nil {
		return entities.Thread{}, s.err
	}
	return s.thread, nil
}

type stubRunGateway struct {
	inserted  []entities.Run
	finished  []entities.Run
	insertErr error
	finishErr error
}

func (s *stubRunGateway) Insert(_ context.Context, run entities.Run) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.inserted = append(s.inserted, run)
	return nil
}

func (s *stubRunGateway) Finish(_ context.Context, run entities.Run) error {
	if s.finishErr != nil {
		return s.finishErr
	}
	s.finished = append(s.finished, run)
	return nil
}

type AgentRuntimeSuite struct {
	suite.Suite

	ctx       context.Context
	obs       *fake.Provider
	principal Principal
	thread    entities.Thread
}

func TestAgentRuntimeSuite(t *testing.T) {
	suite.Run(t, new(AgentRuntimeSuite))
}

func (s *AgentRuntimeSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.principal = Principal{UserID: uuid.New()}
	thread, err := entities.NewThread(s.principal.UserID, ChannelWhatsApp)
	s.Require().NoError(err)
	s.thread = thread
}

func (s *AgentRuntimeSuite) TestExecute() {
	type args struct {
		result RouteResult
	}
	type dependencies struct {
		router  *stubRuntimeRouter
		threads *stubThreadGateway
		runs    *stubRunGateway
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out RouteResult, deps dependencies)
	}{
		{
			name: "deve retornar resultado identico e persistir run succeeded em routed",
			args: args{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindCreateCard}},
			dependencies: dependencies{
				router:  &stubRuntimeRouter{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindCreateCard}},
				threads: &stubThreadGateway{thread: s.thread},
				runs:    &stubRunGateway{},
			},
			expect: func(out RouteResult, deps dependencies) {
				s.Equal("ok", out.Reply)
				s.Equal(tools.OutcomeRouted, out.Outcome)
				s.Equal(intent.KindCreateCard, out.Kind)
				s.Equal(1, deps.router.calls)
				s.Require().Len(deps.runs.inserted, 1)
				s.Require().Len(deps.runs.finished, 1)
				finished := deps.runs.finished[0]
				s.Equal(entities.RunStatusSucceeded, finished.Status())
				s.Equal(tools.OutcomeRouted.String(), finished.Outcome())
				s.Equal(runtimeAgentID, finished.AgentID())
				s.Equal(workflowCards, finished.Workflow())
				s.GreaterOrEqual(finished.DurationMs(), int64(0))
			},
		},
		{
			name: "deve derivar workflow e tool do catalogo em kind com drift corrigido",
			args: args{result: RouteResult{Reply: "resumo", Outcome: tools.OutcomeRouted, Kind: intent.KindQueryIncomeSummary}},
			dependencies: dependencies{
				router:  &stubRuntimeRouter{result: RouteResult{Reply: "resumo", Outcome: tools.OutcomeRouted, Kind: intent.KindQueryIncomeSummary}},
				threads: &stubThreadGateway{thread: s.thread},
				runs:    &stubRunGateway{},
			},
			expect: func(out RouteResult, deps dependencies) {
				s.Equal("resumo", out.Reply)
				s.Require().Len(deps.runs.finished, 1)
				finished := deps.runs.finished[0]
				s.Equal(workflowTransactions, finished.Workflow())
				s.Equal(intent.KindQueryIncomeSummary.String(), finished.ToolName())
			},
		},
		{
			name: "deve persistir run failed em usecase_error",
			args: args{result: RouteResult{Reply: "erro", Outcome: tools.OutcomeUsecaseError, Kind: intent.KindRecordIncome}},
			dependencies: dependencies{
				router:  &stubRuntimeRouter{result: RouteResult{Reply: "erro", Outcome: tools.OutcomeUsecaseError, Kind: intent.KindRecordIncome}},
				threads: &stubThreadGateway{thread: s.thread},
				runs:    &stubRunGateway{},
			},
			expect: func(out RouteResult, deps dependencies) {
				s.Equal(tools.OutcomeUsecaseError, out.Outcome)
				s.Require().Len(deps.runs.finished, 1)
				finished := deps.runs.finished[0]
				s.Equal(entities.RunStatusFailed, finished.Status())
				s.Equal(tools.OutcomeUsecaseError.String(), finished.ErrText())
				s.Equal(workflowTransactions, finished.Workflow())
			},
		},
		{
			name: "deve persistir run succeeded em replay",
			args: args{result: RouteResult{Reply: "replay", Outcome: tools.OutcomeReplay, Kind: intent.KindRecordExpense}},
			dependencies: dependencies{
				router:  &stubRuntimeRouter{result: RouteResult{Reply: "replay", Outcome: tools.OutcomeReplay, Kind: intent.KindRecordExpense}},
				threads: &stubThreadGateway{thread: s.thread},
				runs:    &stubRunGateway{},
			},
			expect: func(out RouteResult, deps dependencies) {
				s.Equal(tools.OutcomeReplay, out.Outcome)
				s.Require().Len(deps.runs.finished, 1)
				s.Equal(entities.RunStatusSucceeded, deps.runs.finished[0].Status())
			},
		},
		{
			name: "deve degradar sem propagar erro quando insert do run falha",
			args: args{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindListCards}},
			dependencies: dependencies{
				router:  &stubRuntimeRouter{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindListCards}},
				threads: &stubThreadGateway{thread: s.thread},
				runs:    &stubRunGateway{insertErr: errors.New("boom")},
			},
			expect: func(out RouteResult, deps dependencies) {
				s.Equal("ok", out.Reply)
				s.Equal(tools.OutcomeRouted, out.Outcome)
				s.Empty(deps.runs.inserted)
				s.Empty(deps.runs.finished)
			},
		},
		{
			name: "deve degradar sem propagar erro quando thread falha",
			args: args{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindListCards}},
			dependencies: dependencies{
				router:  &stubRuntimeRouter{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindListCards}},
				threads: &stubThreadGateway{err: errors.New("db down")},
				runs:    &stubRunGateway{},
			},
			expect: func(out RouteResult, deps dependencies) {
				s.Equal("ok", out.Reply)
				s.Empty(deps.runs.inserted)
			},
		},
	}

	catalog := s.buildCatalog()

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			obs := fake.NewProvider()
			rt := NewAgentRuntime(obs, catalog, scenario.dependencies.router, scenario.dependencies.threads, scenario.dependencies.runs)
			out := rt.Execute(s.ctx, s.principal, ChannelWhatsApp, "", "registrar", "msg-1")
			scenario.expect(out, scenario.dependencies)
			s.assertMetricLabels(obs.Metrics().(*fake.FakeMetrics), scenario.args.result)
		})
	}
}

func (s *AgentRuntimeSuite) TestExecuteReusesThreadAcrossMessages() {
	router := &stubRuntimeRouter{result: RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Kind: intent.KindListCards}}
	threads := &stubThreadGateway{thread: s.thread}
	runs := &stubRunGateway{}

	rt := NewAgentRuntime(s.obs, s.buildCatalog(), router, threads, runs)
	rt.Execute(s.ctx, s.principal, ChannelWhatsApp, "", "msg um", "m-1")
	rt.Execute(s.ctx, s.principal, ChannelWhatsApp, "", "msg dois", "m-2")

	s.Equal(2, threads.calls)
	s.Equal(s.principal.UserID, threads.lastUID)
	s.Equal(ChannelWhatsApp, threads.lastCh)
	s.Require().Len(runs.inserted, 2)
	s.Equal(s.thread.ID(), runs.inserted[0].ThreadID())
	s.Equal(s.thread.ID(), runs.inserted[1].ThreadID())
}

func TestCatalogClassificationMatchesLegacyLabelsExceptKnownDrift(t *testing.T) {
	t.Parallel()

	catalog, err := capability.BuildCatalog()
	require.NoError(t, err)

	expectedDrift := map[intent.Kind]struct {
		workflow string
		tool     string
	}{
		intent.KindQueryIncomeSummary:     {workflow: workflowTransactions, tool: intent.KindQueryIncomeSummary.String()},
		intent.KindBudgetRecurrence:       {workflow: workflowBudget, tool: intent.KindBudgetRecurrence.String()},
		intent.KindDeleteTransactionByRef: {workflow: workflowTransactions, tool: intent.KindDeleteTransactionByRef.String()},
		intent.KindEditTransactionByRef:   {workflow: workflowTransactions, tool: intent.KindEditTransactionByRef.String()},
	}
	require.Len(t, expectedDrift, 4)

	for _, kind := range catalogEquivalenceKinds() {
		workflow, tool := catalog.Classify(kind)
		legacyWorkflow := legacyWorkflowFor(kind)
		legacyTool := legacyToolFor(kind)
		if expected, ok := expectedDrift[kind]; ok {
			require.Equal(t, expected.workflow, workflow, "workflow corrigido para kind %s", kind.String())
			require.Equal(t, expected.tool, tool, "tool corrigida para kind %s", kind.String())
			require.NotEqual(t, legacyWorkflow, workflow, "kind %s deveria ser excecao de drift", kind.String())
			continue
		}
		require.Equal(t, legacyWorkflow, workflow, "workflow legado divergente para kind %s", kind.String())
		require.Equal(t, legacyTool, tool, "tool legada divergente para kind %s", kind.String())
	}
}

func (s *AgentRuntimeSuite) buildCatalog() *capability.Catalog {
	catalog, err := capability.BuildCatalog()
	s.Require().NoError(err)
	return catalog
}

func (s *AgentRuntimeSuite) assertMetricLabels(metrics *fake.FakeMetrics, result RouteResult) {
	s.Require().NotNil(metrics)

	expectedWorkflow, expectedTool := s.buildCatalog().Classify(result.Kind)

	runsTotal := metrics.GetCounter("agent_runs_total")
	s.Require().NotNil(runsTotal)
	s.Require().Len(runsTotal.GetValues(), 1)
	runFields := fieldsByKey(runsTotal.GetValues()[0].Fields)
	s.Equal(runtimeAgentID, runFields["agent_id"])
	s.Equal(ChannelWhatsApp, runFields["channel"])
	s.Equal(expectedWorkflow, runFields["workflow"])

	toolInvocations := metrics.GetCounter("agent_tool_invocations_total")
	if expectedTool == "" {
		if toolInvocations != nil {
			s.Empty(toolInvocations.GetValues())
		}
		return
	}
	s.Require().NotNil(toolInvocations)
	s.Require().Len(toolInvocations.GetValues(), 1)
	toolFields := fieldsByKey(toolInvocations.GetValues()[0].Fields)
	s.Equal(expectedTool, toolFields["tool"])
	s.Equal(result.Outcome.String(), toolFields["outcome"])
}

func catalogEquivalenceKinds() []intent.Kind {
	kinds := append([]intent.Kind(nil), routableKinds()...)
	for kind := range intentToOperationKind {
		found := false
		for _, existing := range kinds {
			if existing == kind {
				found = true
				break
			}
		}
		if !found {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

func legacyWorkflowFor(kind intent.Kind) string {
	switch kind {
	case intent.KindRecordExpense,
		intent.KindRecordIncome,
		intent.KindRecordCardPurchase,
		intent.KindListTransactions,
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindCreateRecurring,
		intent.KindListRecurring:
		return workflowTransactions
	case intent.KindMonthlySummary,
		intent.KindHowAmIDoing,
		intent.KindConfigureBudget,
		intent.KindEditCategoryPercentage,
		intent.KindQueryCategory,
		intent.KindQueryGoal,
		intent.KindQueryCard:
		return workflowBudget
	case intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindUpdateCard,
		intent.KindDeleteCard:
		return workflowCards
	default:
		return workflowConversational
	}
}

func legacyToolFor(kind intent.Kind) string {
	if legacyWorkflowFor(kind) == workflowConversational {
		return ""
	}
	return kind.String()
}

func fieldsByKey(fields []observability.Field) map[string]any {
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		out[field.Key] = field.AnyValue()
	}
	return out
}
