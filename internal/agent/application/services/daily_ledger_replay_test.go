package services

import (
	"context"
	"errors"
	"testing"
	"time"

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
)

type stubExpenseLogger struct {
	calls int
}

func (s *stubExpenseLogger) Execute(_ context.Context, _ ExpenseRecorderInput) (ExpenseRecorderResult, error) {
	s.calls++
	return ExpenseRecorderResult{Persisted: true, AmountCents: 5800, CategoryPath: "Prazeres"}, nil
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
	deps := IntentRouterDeps{
		ExpenseRecorder:     s.expenses,
		PolicyMinConfidence: 0.8,
		Decision:            DecisionAuditDeps{Factory: factory, UoW: unit},
	}
	return newDailyLedgerAgent(s.obs, counter(), counter(), counter(), counter(), time.UTC, deps)
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
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.dup").
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
				s.Equal(OutcomeReplay, result.Outcome)
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
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.low").
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
				s.Equal(OutcomePolicyBlocked, result.Outcome)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "conflito de insert (corrida) faz replay e nao executa",
			args: args{messageID: "wamid.race", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.race").
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
				s.Equal(OutcomeReplay, result.Outcome)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "mensagem nova com alta confianca executa e audita",
			args: args{messageID: "wamid.new", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.new").
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
				s.Equal(OutcomeRouted, result.Outcome)
				s.Equal(1, expenseCalls)
			},
		},
		{
			name: "audit insert falha por erro transitorio bloqueia execucao",
			args: args{messageID: "wamid.fail", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.fail").
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
				s.Equal(OutcomeUsecaseError, result.Outcome)
				s.Equal(0, expenseCalls)
			},
		},
		{
			name: "settle falha apos execucao bem-sucedida nao afeta resposta",
			args: args{messageID: "wamid.settle", confidence: 0.95},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().
						FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), ChannelWhatsApp, "wamid.settle").
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
				s.Equal(OutcomeRouted, result.Outcome)
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
