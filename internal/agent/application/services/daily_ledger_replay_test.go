package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	uowmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow/mocks"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type stubExpenseLogger struct {
	calls int
}

func (s *stubExpenseLogger) Execute(_ context.Context, _ tools.ExpenseRecorderInput) (tools.ExpenseRecorderResult, error) {
	s.calls++
	return tools.ExpenseRecorderResult{Persisted: true, AmountCents: 5800, CategoryPath: "Prazeres"}, nil
}

type replayTestStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newReplayTestStore() *replayTestStore {
	return &replayTestStore{runs: make(map[string]platform.Snapshot)}
}

func (s *replayTestStore) key(wf, k string) string { return wf + ":" + k }

func (s *replayTestStore) Insert(_ context.Context, snap platform.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *replayTestStore) Load(_ context.Context, wf, k string) (platform.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.runs[s.key(wf, k)]
	return snap, ok, nil
}

func (s *replayTestStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *replayTestStore) AppendStep(_ context.Context, _ platform.StepRecord) error { return nil }
func (s *replayTestStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

type DailyLedgerWriteSuite struct {
	suite.Suite

	ctx      context.Context
	obs      observability.Observability
	expenses *stubExpenseLogger
	repo     *interfacesmocks.AgentDecisionRepository
	factory  *interfacesmocks.AgentDecisionRepositoryFactory
	uow      *uowmocks.UnitOfWork
}

func TestDailyLedgerWriteSuite(t *testing.T) {
	suite.Run(t, new(DailyLedgerWriteSuite))
}

func (s *DailyLedgerWriteSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.expenses = &stubExpenseLogger{}
	s.repo = interfacesmocks.NewAgentDecisionRepository(s.T())
	s.factory = interfacesmocks.NewAgentDecisionRepositoryFactory(s.T())
	s.uow = uowmocks.NewUnitOfWork(s.T())
}

func (s *DailyLedgerWriteSuite) buildAgent(factory agentinterfaces.AgentDecisionRepositoryFactory, unit *uowmocks.UnitOfWork) *DailyLedgerAgent {
	counter := func() observability.Counter { return s.obs.Metrics().Counter("agent_test_counter", "", "1") }
	store := newReplayTestStore()
	engine := platform.NewEngine[steps.ExpenseState](store, s.obs)
	settleReg := NewSettleRegistry()
	expenses := s.expenses
	deps := IntentRouterDeps{
		ExpenseRecorder:     expenses,
		PolicyMinConfidence: 0.8,
		Decision:            DecisionAuditDeps{Factory: factory, UoW: unit},
		Kernel: &KernelDeps{
			Engine:    engine,
			SettleReg: settleReg,
			CategoryResolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
				st.CategoryID = "prazeres"
				st.CategoryPath = "Prazeres"
				return st, nil
			},
			PersistFn: func(ctx context.Context, st steps.ExpenseState) (steps.PersistResult, error) {
				res, err := expenses.Execute(ctx, tools.ExpenseRecorderInput{
					UserID:        st.UserID.String(),
					AmountCents:   st.AmountCents,
					Merchant:      st.Merchant,
					PaymentMethod: st.PaymentMethod,
					Direction:     st.Direction,
				})
				if err != nil {
					return steps.PersistResult{}, err
				}
				return steps.PersistResult{AmountCents: res.AmountCents, CategoryPath: res.CategoryPath}, nil
			},
		},
	}
	agent, err := newDailyLedgerAgent(s.obs, counter(), counter(), counter(), counter(), time.UTC, deps)
	s.Require().NoError(err)
	return agent
}

func (s *DailyLedgerWriteSuite) writeIntent(confidence float64) ParsedIntent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: 5800, Merchant: "iFood", CategoryHint: "Prazeres"})
	s.Require().NoError(err)
	conf, err := valueobjects.NewConfidence(confidence)
	s.Require().NoError(err)
	return ParsedIntent{
		Intent:       in,
		Confidence:   conf,
		LLMModel:     "openai/gpt-4o-mini",
		PromptSHA256: "a3f1e9b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1",
	}
}

func (s *DailyLedgerWriteSuite) TestDispatchWrite() {
	type args struct {
		messageID  string
		confidence float64
	}

	type dependencies struct {
		repo    *interfacesmocks.AgentDecisionRepository
		factory *interfacesmocks.AgentDecisionRepositoryFactory
		uow     *uowmocks.UnitOfWork
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result RouteResult, expenseCalls int)
	}{
		{
			name: "mensagem repetida faz replay e nao reexecuta tool",
			args: args{messageID: "wamid.dup", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.dup", 0).
						Return(agentinterfaces.AgentDecisionSnapshot{Status: "executed", RedactedResponse: []byte(`{"redacted":"Lancei R$ 58,00 no iFood"}`)}, true, nil).
						Once()
					return s.repo
				}(),
				factory: func() *interfacesmocks.AgentDecisionRepositoryFactory {
					s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
					return s.factory
				}(),
				uow: func() *uowmocks.UnitOfWork {
					s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
						return fn(ctx, nil)
					})
					return s.uow
				}(),
			},
			expect: func(result RouteResult, expenseCalls int) {
				s.Equal(tools.OutcomeReplay, result.Outcome)
				s.Equal("Lancei R$ 58,00 no iFood", result.Reply)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "baixa confianca bloqueia e nao executa",
			args: args{messageID: "wamid.low", confidence: 0.5},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.low", 0).
						Return(agentinterfaces.AgentDecisionSnapshot{}, false, nil).
						Once()
					return s.repo
				}(),
				factory: func() *interfacesmocks.AgentDecisionRepositoryFactory {
					s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
					return s.factory
				}(),
				uow: func() *uowmocks.UnitOfWork {
					s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
						return fn(ctx, nil)
					})
					return s.uow
				}(),
			},
			expect: func(result RouteResult, expenseCalls int) {
				s.Equal(tools.OutcomePolicyBlocked, result.Outcome)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "conflito de insert (corrida) faz replay e nao executa",
			args: args{messageID: "wamid.race", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.race", 0).
						Return(agentinterfaces.AgentDecisionSnapshot{}, false, nil).Once()
					s.repo.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.AgentDecision")).
						Return(agentinterfaces.ErrAgentDecisionConflict).Once()
					return s.repo
				}(),
				factory: func() *interfacesmocks.AgentDecisionRepositoryFactory {
					s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
					return s.factory
				}(),
				uow: func() *uowmocks.UnitOfWork {
					s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
						return fn(ctx, nil)
					})
					return s.uow
				}(),
			},
			expect: func(result RouteResult, expenseCalls int) {
				s.Equal(tools.OutcomeReplay, result.Outcome)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "mensagem nova com alta confianca executa e audita",
			args: args{messageID: "wamid.new", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.new", 0).
						Return(agentinterfaces.AgentDecisionSnapshot{}, false, nil).
						Once()
					s.repo.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.AgentDecision")).Return(nil).Once()
					s.repo.EXPECT().UpdateSettlement(mock.Anything, mock.AnythingOfType("entities.AgentDecision")).Return(nil).Once()
					return s.repo
				}(),
				factory: func() *interfacesmocks.AgentDecisionRepositoryFactory {
					s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
					return s.factory
				}(),
				uow: func() *uowmocks.UnitOfWork {
					s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
						return fn(ctx, nil)
					})
					return s.uow
				}(),
			},
			expect: func(result RouteResult, expenseCalls int) {
				s.Equal(tools.OutcomeRouted, result.Outcome)
				s.Equal(1, expenseCalls)
			},
		},
		{
			name: "audit insert falha por erro transitorio bloqueia execucao",
			args: args{messageID: "wamid.fail", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.fail", 0).
						Return(agentinterfaces.AgentDecisionSnapshot{}, false, nil).Once()
					s.repo.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.AgentDecision")).
						Return(errors.New("db timeout")).Once()
					return s.repo
				}(),
				factory: func() *interfacesmocks.AgentDecisionRepositoryFactory {
					s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
					return s.factory
				}(),
				uow: func() *uowmocks.UnitOfWork {
					s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
						return fn(ctx, nil)
					})
					return s.uow
				}(),
			},
			expect: func(result RouteResult, expenseCalls int) {
				s.Equal(tools.OutcomeUsecaseError, result.Outcome)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "settle falha apos execucao bem-sucedida nao afeta resposta",
			args: args{messageID: "wamid.settle", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.settle", 0).
						Return(agentinterfaces.AgentDecisionSnapshot{}, false, nil).Once()
					s.repo.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.AgentDecision")).Return(nil).Once()
					s.repo.EXPECT().UpdateSettlement(mock.Anything, mock.AnythingOfType("entities.AgentDecision")).
						Return(errors.New("db connection lost")).Once()
					return s.repo
				}(),
				factory: func() *interfacesmocks.AgentDecisionRepositoryFactory {
					s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
					return s.factory
				}(),
				uow: func() *uowmocks.UnitOfWork {
					s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
						return fn(ctx, nil)
					})
					return s.uow
				}(),
			},
			expect: func(result RouteResult, expenseCalls int) {
				s.Equal(tools.OutcomeRouted, result.Outcome)
				s.Equal(1, expenseCalls)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			callsBefore := s.expenses.calls
			agent := s.buildAgent(scenario.dependencies.factory, scenario.dependencies.uow)
			principal := Principal{UserID: uuid.New()}
			result := agent.dispatchWrite(s.ctx, principal, ChannelWhatsApp, scenario.args.messageID, "gastei 58 no iFood", s.writeIntent(scenario.args.confidence))
			scenario.expect(result, s.expenses.calls-callsBefore)
		})
	}
}
