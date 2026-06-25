package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DestructiveConfirmSuite struct {
	suite.Suite
	ctx context.Context
	obs platform.Engine[confirmation.ConfirmState]
}

func TestDestructiveConfirmSuite(t *testing.T) {
	suite.Run(t, new(DestructiveConfirmSuite))
}

func (s *DestructiveConfirmSuite) SetupTest() {
	s.ctx = context.Background()
	store := newTestStore()
	s.obs = platform.NewEngine[confirmation.ConfirmState](store, fake.NewProvider())
}

func baseConfirmDefinition() DestructiveConfirmDeps {
	return DestructiveConfirmDeps{
		Authorize: func(_ context.Context, _ confirmation.ConfirmState) bool { return true },
		Replay:    func(_ context.Context, _ confirmation.ConfirmState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
			return steps.ConfirmAuditBeginResult{}
		},
		OnSettle:       nil,
		Targets:        map[confirmation.OperationKind]steps.TargetResolver{},
		Executors:      map[confirmation.OperationKind]steps.DestructiveExecutor{},
		TTL:            10 * time.Minute,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	}
}

func baseConfirmInitialState() confirmation.ConfirmState {
	return confirmation.ConfirmState{
		OperationKind: confirmation.OperationDeleteLast,
		UserID:        "user-1",
		Channel:       "whatsapp",
		MessageID:     "msg-1",
	}
}

func (s *DestructiveConfirmSuite) TestDefinition_ID_And_Durable() {
	def := NewDestructiveConfirmDefinition(baseConfirmDefinition())
	s.Equal(DestructiveConfirmWorkflowID, def.ID)
	s.True(def.Durable)
	s.NotEqual(TransactionsWriteWorkflowID, def.ID)
}

func (s *DestructiveConfirmSuite) TestDefinition_AuthzDenied_ShortCircuit() {
	type dependencies struct {
		authorize steps.ConfirmAuthorizeFunc
	}
	scenarios := []struct {
		name         string
		state        confirmation.ConfirmState
		dependencies dependencies
		expect       func(result platform.RunResult[confirmation.ConfirmState], err error)
	}{
		{
			name:  "deve curto-circuitar em authz negado",
			state: baseConfirmInitialState(),
			dependencies: dependencies{
				authorize: func() steps.ConfirmAuthorizeFunc {
					return func(_ context.Context, _ confirmation.ConfirmState) bool { return false }
				}(),
			},
			expect: func(result platform.RunResult[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.RunStatusSucceeded, result.Status)
				s.True(result.State.ShortCircuit)
				s.Equal(int(tools.OutcomeAuthzDenied), result.State.Outcome)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := baseConfirmDefinition()
			deps.Authorize = scenario.dependencies.authorize
			def := NewDestructiveConfirmDefinition(deps)
			result, err := s.obs.Start(s.ctx, def, "user-1:whatsapp", scenario.state)
			scenario.expect(result, err)
		})
	}
}

func (s *DestructiveConfirmSuite) TestDefinition_Replay_ShortCircuit() {
	type dependencies struct {
		replay steps.ConfirmReplayFunc
	}
	scenarios := []struct {
		name         string
		state        confirmation.ConfirmState
		dependencies dependencies
		expect       func(result platform.RunResult[confirmation.ConfirmState], err error)
	}{
		{
			name:  "deve curto-circuitar quando replay detectado",
			state: baseConfirmInitialState(),
			dependencies: dependencies{
				replay: func() steps.ConfirmReplayFunc {
					return func(_ context.Context, _ confirmation.ConfirmState) (string, bool) {
						return "resposta anterior", true
					}
				}(),
			},
			expect: func(result platform.RunResult[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.RunStatusSucceeded, result.Status)
				s.Equal(int(tools.OutcomeReplay), result.State.Outcome)
				s.True(result.State.ShortCircuit)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := baseConfirmDefinition()
			deps.Replay = scenario.dependencies.replay
			def := NewDestructiveConfirmDefinition(deps)
			result, err := s.obs.Start(s.ctx, def, "user-1:whatsapp", scenario.state)
			scenario.expect(result, err)
		})
	}
}

func (s *DestructiveConfirmSuite) TestDefinition_PrepareTarget_MissingResolver_ShortCircuits() {
	deps := baseConfirmDefinition()
	def := NewDestructiveConfirmDefinition(deps)

	result, err := s.obs.Start(s.ctx, def, "user-1:whatsapp", baseConfirmInitialState())

	s.NoError(err)
	s.Equal(platform.RunStatusSucceeded, result.Status)
	s.True(result.State.ShortCircuit)
	s.Equal(int(tools.OutcomeMissingResolver), result.State.Outcome)
}

func (s *DestructiveConfirmSuite) TestDefinition_OperationKindMapping() {
	for i, kind := range []confirmation.OperationKind{
		confirmation.OperationDeleteLast,
		confirmation.OperationEditLast,
		confirmation.OperationDeleteCard,
		confirmation.OperationBudgetCommit,
	} {
		s.Run("deve mapear kind "+kind.String(), func() {
			deps := baseConfirmDefinition()
			deps.Targets = map[confirmation.OperationKind]steps.TargetResolver{
				kind: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
					st.PromptText = "confirme?"
					return st, nil
				},
			}
			def := NewDestructiveConfirmDefinition(deps)
			state := baseConfirmInitialState()
			state.OperationKind = kind
			result, err := s.obs.Start(s.ctx, def, fmt.Sprintf("user-%d:whatsapp", i), state)
			s.NoError(err)
			s.Equal(platform.RunStatusSuspended, result.Status)
			s.NotNil(result.Suspend)
		})
	}
}

func (s *DestructiveConfirmSuite) TestDefinition_FullFlow_SuspendsOnFirstPass() {
	deps := baseConfirmDefinition()
	deps.Targets = map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.PromptText = "confirme a exclusão?"
			return st, nil
		},
	}
	deps.Executors = map[confirmation.OperationKind]steps.DestructiveExecutor{
		confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.Outcome = int(tools.OutcomeRouted)
			return st, nil
		},
	}
	def := NewDestructiveConfirmDefinition(deps)
	result, err := s.obs.Start(s.ctx, def, "user-1:whatsapp", baseConfirmInitialState())

	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, result.Status)
	s.NotNil(result.Suspend)
	s.Equal("confirme a exclusão?", result.Suspend.Prompt)
}

type fakeSearcher struct {
	result tools.TransactionSearchResult
	err    error
}

func (f *fakeSearcher) Execute(_ context.Context, _ tools.TransactionSearchInput) (tools.TransactionSearchResult, error) {
	return f.result, f.err
}

func byRefDeps(searcher tools.TransactionSearcher, executed *bool) DestructiveConfirmDeps {
	deps := baseConfirmDefinition()
	deps.Searcher = searcher
	deps.Targets = map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationDeleteByRef: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			st.PromptText = fmt.Sprintf("apagar %s?", st.TargetDescription)
			return st, nil
		},
	}
	deps.Executors = map[confirmation.OperationKind]steps.DestructiveExecutor{
		confirmation.OperationDeleteByRef: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
			*executed = true
			st.Outcome = int(tools.OutcomeRouted)
			return st, nil
		},
	}
	return deps
}

func byRefInitialState() confirmation.ConfirmState {
	return confirmation.ConfirmState{
		OperationKind: confirmation.OperationDeleteByRef,
		UserID:        "user-1",
		Channel:       "whatsapp",
		MessageID:     "msg-1",
		SearchQuery:   "mercado",
	}
}

func (s *DestructiveConfirmSuite) TestByRef_NoneFound_ShortCircuit() {
	executed := false
	searcher := &fakeSearcher{result: tools.TransactionSearchResult{}}
	def := NewDestructiveConfirmDefinition(byRefDeps(searcher, &executed))

	result, err := s.obs.Start(s.ctx, def, "user-none:whatsapp", byRefInitialState())

	s.NoError(err)
	s.Equal(platform.RunStatusSucceeded, result.Status)
	s.True(result.State.ShortCircuit)
	s.False(executed)
	s.Contains(result.State.Reply, "mercado")
}

func (s *DestructiveConfirmSuite) TestByRef_SingleFound_SuspendsConfirm() {
	executed := false
	searcher := &fakeSearcher{result: tools.TransactionSearchResult{Candidates: []tools.TransactionView{
		{ID: "tx-1", Version: 1, Description: "Mercado", AmountCents: 12000},
	}}}
	def := NewDestructiveConfirmDefinition(byRefDeps(searcher, &executed))

	result, err := s.obs.Start(s.ctx, def, "user-single:whatsapp", byRefInitialState())

	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, result.Status)
	s.Require().NotNil(result.Suspend)
	s.Equal("apagar Mercado?", result.Suspend.Prompt)
	s.False(executed)
}

func (s *DestructiveConfirmSuite) TestByRef_MultipleFound_SelectThenConfirmThenExecute() {
	executed := false
	searcher := &fakeSearcher{result: tools.TransactionSearchResult{Candidates: []tools.TransactionView{
		{ID: "tx-1", Version: 1, Description: "Mercado", AmountCents: 12000},
		{ID: "tx-2", Version: 3, Description: "Mercado Extra", AmountCents: 8500},
		{ID: "tx-3", Version: 1, Description: "Mercadinho", AmountCents: 4300},
	}}}
	def := NewDestructiveConfirmDefinition(byRefDeps(searcher, &executed))
	key := "user-multi:whatsapp"

	first, err := s.obs.Start(s.ctx, def, key, byRefInitialState())
	s.Require().NoError(err)
	s.Equal(platform.RunStatusSuspended, first.Status)
	s.Require().NotNil(first.Suspend)
	s.Contains(first.Suspend.Prompt, "2) R$ 85,00 — Mercado Extra")

	second, err := s.obs.Resume(s.ctx, def, key, []byte(`{"resume_text":"2"}`))
	s.Require().NoError(err)
	s.Equal(platform.RunStatusSuspended, second.Status)
	s.Require().NotNil(second.Suspend)
	s.Equal("apagar Mercado Extra?", second.Suspend.Prompt)
	s.Equal("tx-2", second.State.TargetTransactionID)
	s.Equal(int64(3), second.State.TargetTransactionVersion)
	s.False(executed)

	third, err := s.obs.Resume(s.ctx, def, key, []byte(`{"resume_text":"sim"}`))
	s.Require().NoError(err)
	s.Equal(platform.RunStatusSucceeded, third.Status)
	s.True(executed)
	s.Equal("tx-2", third.State.TargetTransactionID)
}

func (s *DestructiveConfirmSuite) TestByRef_MultipleFound_InvalidIndexRepromptThenSelect() {
	executed := false
	searcher := &fakeSearcher{result: tools.TransactionSearchResult{Candidates: []tools.TransactionView{
		{ID: "tx-1", Version: 1, Description: "Mercado", AmountCents: 12000},
		{ID: "tx-2", Version: 1, Description: "Mercado Extra", AmountCents: 8500},
	}}}
	def := NewDestructiveConfirmDefinition(byRefDeps(searcher, &executed))
	key := "user-reprompt:whatsapp"

	_, err := s.obs.Start(s.ctx, def, key, byRefInitialState())
	s.Require().NoError(err)

	reprompt, err := s.obs.Resume(s.ctx, def, key, []byte(`{"resume_text":"99"}`))
	s.Require().NoError(err)
	s.Equal(platform.RunStatusSuspended, reprompt.Status)
	s.Equal(1, reprompt.State.SelectRepromptCount)

	good, err := s.obs.Resume(s.ctx, def, key, []byte(`{"resume_text":"1"}`))
	s.Require().NoError(err)
	s.Equal(platform.RunStatusSuspended, good.Status)
	s.Equal("tx-1", good.State.TargetTransactionID)
}

func (s *DestructiveConfirmSuite) TestFormatDestructiveReply_AllKinds() {
	cases := []struct {
		kind confirmation.OperationKind
	}{
		{confirmation.OperationDeleteLast},
		{confirmation.OperationEditLast},
		{confirmation.OperationDeleteCard},
		{confirmation.OperationBudgetCommit},
		{confirmation.OperationDeleteByRef},
		{confirmation.OperationEditByRef},
	}
	for _, tc := range cases {
		s.Run(tc.kind.String(), func() {
			reply := formatDestructiveReply(confirmation.ConfirmState{OperationKind: tc.kind})
			s.NotEmpty(reply)
		})
	}
}
