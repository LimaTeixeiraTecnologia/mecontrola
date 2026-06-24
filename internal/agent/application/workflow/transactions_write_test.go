package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type testStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newTestStore() *testStore {
	return &testStore{runs: make(map[string]platform.Snapshot)}
}

func (t *testStore) key(workflow, correlationKey string) string {
	return workflow + ":" + correlationKey
}

func (t *testStore) Insert(_ context.Context, snap platform.Snapshot) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runs[t.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (t *testStore) Load(_ context.Context, workflow, key string) (platform.Snapshot, bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	snap, ok := t.runs[t.key(workflow, key)]
	return snap, ok, nil
}

func (t *testStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runs[t.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (t *testStore) AppendStep(_ context.Context, _ platform.StepRecord) error {
	return nil
}

func (t *testStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

type TransactionsWriteSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestTransactionsWriteSuite(t *testing.T) {
	suite.Run(t, new(TransactionsWriteSuite))
}

func (s *TransactionsWriteSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func baseExpenseState() steps.ExpenseState {
	return steps.ExpenseState{
		UserID:          uuid.New(),
		Channel:         "whatsapp",
		MessageID:       "msg-1",
		Kind:            intent.KindRecordExpense,
		TransactionKind: pendingexpense.TransactionKindExpense,
		AmountCents:     5000,
		Merchant:        "Loja",
		PaymentMethod:   "debit",
		Direction:       "outcome",
		CategoryHint:    "Alimentação",
	}
}

func allowAll(_ context.Context, _ steps.ExpenseState) bool { return true }

func noReplay(_ context.Context, _ steps.ExpenseState) (string, bool) { return "", false }

func allowPolicy(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" }

func successAudit(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
	return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
}

func autoCategory(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
	st.CategoryID = "cat-1"
	st.CategoryPath = "Alimentação"
	return st, nil
}

func successPersist(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
	return steps.PersistResult{AmountCents: 5000, CategoryPath: "Alimentação"}, nil
}

func (s *TransactionsWriteSuite) newEngine() platform.Engine[steps.ExpenseState] {
	store := newTestStore()
	return platform.NewEngine[steps.ExpenseState](store, s.obs)
}

func (s *TransactionsWriteSuite) TestDefinition_FullFlow_Success() {
	type dependencies struct {
		authorize steps.AuthorizeFunc
		replay    steps.ReplayFunc
		policy    steps.PolicyFunc
		audit     steps.AuditBeginFunc
		resolver  steps.CategoryResolverFunc
		persist   steps.PersistFunc
	}
	scenarios := []struct {
		name         string
		state        steps.ExpenseState
		dependencies dependencies
		expect       func(result platform.RunResult[steps.ExpenseState], err error)
	}{
		{
			name:  "deve completar fluxo completo com sucesso",
			state: baseExpenseState(),
			dependencies: dependencies{
				authorize: func() steps.AuthorizeFunc { return allowAll }(),
				replay:    func() steps.ReplayFunc { return noReplay }(),
				policy:    func() steps.PolicyFunc { return allowPolicy }(),
				audit:     func() steps.AuditBeginFunc { return successAudit }(),
				resolver:  func() steps.CategoryResolverFunc { return autoCategory }(),
				persist:   func() steps.PersistFunc { return successPersist }(),
			},
			expect: func(result platform.RunResult[steps.ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.RunStatusSucceeded, result.Status)
				s.Equal(tools.OutcomeRouted, result.State.Outcome)
				s.NotEmpty(result.State.Reply)
			},
		},
		{
			name:  "deve curto-circuitar em authz negado",
			state: baseExpenseState(),
			dependencies: dependencies{
				authorize: func() steps.AuthorizeFunc {
					return func(_ context.Context, _ steps.ExpenseState) bool { return false }
				}(),
				replay:   func() steps.ReplayFunc { return noReplay }(),
				policy:   func() steps.PolicyFunc { return allowPolicy }(),
				audit:    func() steps.AuditBeginFunc { return successAudit }(),
				resolver: func() steps.CategoryResolverFunc { return autoCategory }(),
				persist:  func() steps.PersistFunc { return successPersist }(),
			},
			expect: func(result platform.RunResult[steps.ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.RunStatusSucceeded, result.Status)
				s.Equal(tools.OutcomeAuthzDenied, result.State.Outcome)
				s.True(result.State.ShortCircuit)
			},
		},
		{
			name:  "deve suspender quando categoria ambigua",
			state: baseExpenseState(),
			dependencies: dependencies{
				authorize: func() steps.AuthorizeFunc { return allowAll }(),
				replay:    func() steps.ReplayFunc { return noReplay }(),
				policy:    func() steps.PolicyFunc { return allowPolicy }(),
				audit:     func() steps.AuditBeginFunc { return successAudit }(),
				resolver: func() steps.CategoryResolverFunc {
					return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
						return st, &tools.CategoryAmbiguousError{Hint: "Alimentação", Candidates: []string{"A", "B"}}
					}
				}(),
				persist: func() steps.PersistFunc { return successPersist }(),
			},
			expect: func(result platform.RunResult[steps.ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.RunStatusSuspended, result.Status)
				s.NotNil(result.Suspend)
				s.Equal(pendingexpense.AwaitingCategoryChoice, result.State.AwaitingKind)
			},
		},
		{
			name:  "deve curto-circuitar quando replay detectado",
			state: baseExpenseState(),
			dependencies: dependencies{
				authorize: func() steps.AuthorizeFunc { return allowAll }(),
				replay: func() steps.ReplayFunc {
					return func(_ context.Context, _ steps.ExpenseState) (string, bool) {
						return "resposta anterior", true
					}
				}(),
				policy:   func() steps.PolicyFunc { return allowPolicy }(),
				audit:    func() steps.AuditBeginFunc { return successAudit }(),
				resolver: func() steps.CategoryResolverFunc { return autoCategory }(),
				persist:  func() steps.PersistFunc { return successPersist }(),
			},
			expect: func(result platform.RunResult[steps.ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.RunStatusSucceeded, result.Status)
				s.Equal(tools.OutcomeReplay, result.State.Outcome)
			},
		},
		{
			name:  "deve falhar quando persist retorna erro",
			state: baseExpenseState(),
			dependencies: dependencies{
				authorize: func() steps.AuthorizeFunc { return allowAll }(),
				replay:    func() steps.ReplayFunc { return noReplay }(),
				policy:    func() steps.PolicyFunc { return allowPolicy }(),
				audit:     func() steps.AuditBeginFunc { return successAudit }(),
				resolver:  func() steps.CategoryResolverFunc { return autoCategory }(),
				persist: func() steps.PersistFunc {
					return func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
						return steps.PersistResult{}, errors.New("db error")
					}
				}(),
			},
			expect: func(result platform.RunResult[steps.ExpenseState], err error) {
				s.Error(err)
				s.Equal(platform.RunStatusFailed, result.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			eng := s.newEngine()
			def := NewTransactionsWriteDefinition(TransactionsWriteDeps{
				Authorize:      scenario.dependencies.authorize,
				Replay:         scenario.dependencies.replay,
				Policy:         scenario.dependencies.policy,
				AuditBegin:     scenario.dependencies.audit,
				OnSettle:       nil,
				Resolver:       scenario.dependencies.resolver,
				Persist:        scenario.dependencies.persist,
				DenyReply:      "negado",
				ReplayReply:    "replay",
				AuditFailReply: "falha",
			})
			result, err := eng.Start(s.ctx, def, "user:channel", scenario.state)
			scenario.expect(result, err)
		})
	}
}

func (s *TransactionsWriteSuite) TestDefinition_DefinitionID() {
	def := NewTransactionsWriteDefinition(TransactionsWriteDeps{
		Authorize:  allowAll,
		Replay:     noReplay,
		Policy:     allowPolicy,
		AuditBegin: successAudit,
		Resolver:   autoCategory,
		Persist:    successPersist,
	})
	s.Equal(TransactionsWriteWorkflowID, def.ID)
	s.True(def.Durable)
	s.Greater(def.MaxAttempts, 0)
}

func (s *TransactionsWriteSuite) TestExpenseStateFromToolInput() {
	kind := intent.KindRecordExpense
	in := tools.ToolInput{
		UserID:    uuid.New(),
		Channel:   "telegram",
		MessageID: "msg-42",
		Intent:    mustBuildExpenseIntent(5000, "Mercado"),
	}
	state := ExpenseStateFromToolInput(in)
	s.Equal(in.UserID, state.UserID)
	s.Equal("telegram", state.Channel)
	s.Equal(kind, state.Kind)
	s.Equal(int64(5000), state.AmountCents)
	s.Equal("Mercado", state.Merchant)
	s.Equal("outcome", state.Direction)
	s.Equal(pendingexpense.TransactionKindExpense, state.TransactionKind)
}

func (s *TransactionsWriteSuite) TestExpenseStateToToolResult() {
	state := steps.ExpenseState{
		Kind:    intent.KindRecordExpense,
		Outcome: tools.OutcomeRouted,
		Reply:   "ok",
	}
	result := ExpenseStateToToolResult(state)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal("ok", result.Reply)
	s.Equal(intent.KindRecordExpense, result.Kind)
}

func mustBuildExpenseIntent(amountCents int64, merchant string) intent.Intent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents: amountCents,
		Merchant:    merchant,
	})
	if err != nil {
		panic(err)
	}
	return in
}
