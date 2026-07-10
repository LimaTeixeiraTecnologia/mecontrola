package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type mockBudgetCreationEngine struct {
	mock.Mock
}

func (m *mockBudgetCreationEngine) Start(ctx context.Context, def workflow.Definition[workflows.BudgetCreationState], key string, initial workflows.BudgetCreationState) (workflow.RunResult[workflows.BudgetCreationState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[workflows.BudgetCreationState]), args.Error(1)
}

func (m *mockBudgetCreationEngine) Resume(ctx context.Context, def workflow.Definition[workflows.BudgetCreationState], key string, resume []byte) (workflow.RunResult[workflows.BudgetCreationState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[workflows.BudgetCreationState]), args.Error(1)
}

func (m *mockBudgetCreationEngine) LoadLatestState(ctx context.Context, def workflow.Definition[workflows.BudgetCreationState], key string) (workflows.BudgetCreationState, workflow.Snapshot, bool, error) {
	args := m.Called(ctx, def, key)
	return args.Get(0).(workflows.BudgetCreationState), args.Get(1).(workflow.Snapshot), args.Bool(2), args.Error(3)
}

type fakeBudgetCreationThreadGateway struct {
	thread memory.Thread
	err    error
}

func (f *fakeBudgetCreationThreadGateway) GetOrCreate(_ context.Context, _, _ string) (memory.Thread, error) {
	return f.thread, f.err
}

type fakeBudgetCreationRunStore struct {
	insertErr error
	updateErr error
	inserted  []agent.Run
	updated   []agent.Run
}

func (f *fakeBudgetCreationRunStore) Insert(_ context.Context, run agent.Run) error {
	f.inserted = append(f.inserted, run)
	return f.insertErr
}

func (f *fakeBudgetCreationRunStore) Update(_ context.Context, run agent.Run) error {
	f.updated = append(f.updated, run)
	return f.updateErr
}

func (f *fakeBudgetCreationRunStore) Load(_ context.Context, _ uuid.UUID) (agent.Run, error) {
	return agent.Run{}, nil
}

type BudgetCreationContinuerSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	emptyDef   workflow.Definition[workflows.BudgetCreationState]
	engineMock *mockBudgetCreationEngine
	threads    *fakeBudgetCreationThreadGateway
	runs       *fakeBudgetCreationRunStore
}

func TestBudgetCreationContinuerSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationContinuerSuite))
}

func (s *BudgetCreationContinuerSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.emptyDef = workflow.Definition[workflows.BudgetCreationState]{}
	s.engineMock = &mockBudgetCreationEngine{}
	s.engineMock.Test(s.T())
	s.T().Cleanup(func() { s.engineMock.AssertExpectations(s.T()) })
	s.threads = &fakeBudgetCreationThreadGateway{thread: memory.Thread{ID: uuid.New()}}
	s.runs = &fakeBudgetCreationRunStore{}
}

func (s *BudgetCreationContinuerSuite) TestContinue() {
	type args struct {
		resourceID string
		message    string
		messageID  string
	}
	type dependencies struct {
		engine *mockBudgetCreationEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(handled bool, reply string, err error)
	}{
		{
			name: "deve retornar handled=false quando nao ha run suspenso",
			args: args{resourceID: "user-1", message: "500", messageID: "wamid-001"},
			dependencies: dependencies{
				engine: func() *mockBudgetCreationEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-1:budget-creation", mock.Anything).
						Return(workflow.RunResult[workflows.BudgetCreationState]{}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.False(handled)
				s.Empty(reply)
			},
		},
		{
			name: "deve retornar reply de ResponseText quando suspenso",
			args: args{resourceID: "user-2", message: "não entendi", messageID: "wamid-002"},
			dependencies: dependencies{
				engine: func() *mockBudgetCreationEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-2:budget-creation", mock.Anything).
						Return(workflow.RunResult[workflows.BudgetCreationState]{
							Status: workflow.RunStatusSuspended,
							Suspend: &workflow.Suspension{
								Reason: workflow.SuspendAwaitingInput,
								Prompt: "Não entendi. Responda sim ou não.",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.True(handled)
				s.Equal("Não entendi. Responda sim ou não.", reply)
			},
		},
		{
			name: "deve retornar reply de ResponseText quando completado com sucesso",
			args: args{resourceID: "user-3", message: "sim", messageID: "wamid-003"},
			dependencies: dependencies{
				engine: func() *mockBudgetCreationEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-3:budget-creation", mock.Anything).
						Return(workflow.RunResult[workflows.BudgetCreationState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.BudgetCreationState{
								Status:       workflows.BudgetCreationCompleted,
								ResponseText: "🎉 Orçamento de junho de 2026 criado e ativado com sucesso!",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.True(handled)
				s.Equal("🎉 Orçamento de junho de 2026 criado e ativado com sucesso!", reply)
			},
		},
		{
			name: "deve retornar handled=false quando expirado",
			args: args{resourceID: "user-4", message: "sim", messageID: "wamid-004"},
			dependencies: dependencies{
				engine: func() *mockBudgetCreationEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-4:budget-creation", mock.Anything).
						Return(workflow.RunResult[workflows.BudgetCreationState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.BudgetCreationState{
								Status:  workflows.BudgetCreationExpired,
								Expired: true,
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.False(handled)
				s.Empty(reply)
			},
		},
		{
			name: "deve retornar erro quando engine falha sem step falho reportado",
			args: args{resourceID: "user-5", message: "sim", messageID: "wamid-005"},
			dependencies: dependencies{
				engine: func() *mockBudgetCreationEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-5:budget-creation", mock.Anything).
						Return(workflow.RunResult[workflows.BudgetCreationState]{}, errors.New("engine error")).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.Error(err)
				s.False(handled)
				s.Contains(err.Error(), "budget_creation_continuer")
			},
		},
		{
			name: "falha de persistencia no slot de confirmacao devolve mensagem especifica handled=true (RF-26)",
			args: args{resourceID: "user-9", message: "sim", messageID: "wamid-009"},
			dependencies: dependencies{
				engine: func() *mockBudgetCreationEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-9:budget-creation", mock.Anything).
						Return(workflow.RunResult[workflows.BudgetCreationState]{
							Status: workflow.RunStatusFailed,
							State: workflows.BudgetCreationState{
								ResponseText: "Não consegui criar o orçamento. Tente novamente em breve.",
							},
						}, errors.New("planner.CreateBudget: db unavailable")).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.True(handled)
				s.Equal("Não consegui criar o orçamento. Tente novamente em breve.", reply)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewBudgetCreationContinuer(scenario.dependencies.engine, s.emptyDef, s.threads, s.runs, s.obs)
			handled, reply, err := uc.Continue(s.ctx, scenario.args.resourceID, scenario.args.message, scenario.args.messageID)
			scenario.expect(handled, reply, err)
		})
	}
}

func (s *BudgetCreationContinuerSuite) TestContinue_AbreEFechaRunAuditavel() {
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-6:budget-creation", mock.Anything).
		Return(workflow.RunResult[workflows.BudgetCreationState]{
			Status: workflow.RunStatusSucceeded,
			State: workflows.BudgetCreationState{
				Status:       workflows.BudgetCreationCompleted,
				ResponseText: "🎉 Orçamento criado e ativado com sucesso!",
			},
		}, nil).Once()

	uc := NewBudgetCreationContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	_, _, err := uc.Continue(s.ctx, "user-6", "sim", "wamid-006")

	s.NoError(err)
	s.Require().Len(s.runs.inserted, 1)
	s.Equal(agent.RunStatusRunning, s.runs.inserted[0].Status)
	s.Equal(workflows.BudgetCreationWorkflowID, s.runs.inserted[0].Workflow)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusSucceeded, s.runs.updated[0].Status)
	s.Empty(s.runs.updated[0].Error)
}

func (s *BudgetCreationContinuerSuite) TestContinue_FalhaAoFecharRunObservaSemQuebrarNegocio() {
	s.runs.updateErr = errors.New("update falhou no fechamento")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-close:budget-creation", mock.Anything).
		Return(workflow.RunResult[workflows.BudgetCreationState]{
			Status: workflow.RunStatusSucceeded,
			State: workflows.BudgetCreationState{
				Status:       workflows.BudgetCreationCompleted,
				ResponseText: "🎉 Orçamento criado e ativado com sucesso!",
			},
		}, nil).Once()

	uc := NewBudgetCreationContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	handled, reply, err := uc.Continue(s.ctx, "user-close", "sim", "wamid-close")

	s.NoError(err)
	s.True(handled)
	s.Equal("🎉 Orçamento criado e ativado com sucesso!", reply)

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("agents_run_update_errors_total")
	s.Require().NotNil(counter)
	values := counter.GetValues()
	s.Require().Len(values, 1)
	assertRunUpdateErrorLabels(s.T(), values[0].Fields, workflows.BudgetCreationWorkflowID, "close", "succeeded")

	entries := s.obs.Logger().(*fake.FakeLogger).GetEntries()
	s.True(hasRunUpdateErrorLog(entries), "deve emitir log estruturado de falha de fechamento")
}

func (s *BudgetCreationContinuerSuite) TestContinue_EscritaFalhaGravaErroRealNoRun() {
	writeErr := errors.New("db unavailable")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-7:budget-creation", mock.Anything).
		Return(workflow.RunResult[workflows.BudgetCreationState]{}, writeErr).Once()

	uc := NewBudgetCreationContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	_, _, err := uc.Continue(s.ctx, "user-7", "sim", "wamid-007")

	s.Error(err)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusFailed, s.runs.updated[0].Status)
	s.Contains(s.runs.updated[0].Error, "db unavailable")
}

func (s *BudgetCreationContinuerSuite) TestContinue_FalhaDePersistenciaGravaErroRealERunFalho() {
	writeErr := errors.New("planner.CreateBudget: db unavailable")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-10:budget-creation", mock.Anything).
		Return(workflow.RunResult[workflows.BudgetCreationState]{
			Status: workflow.RunStatusFailed,
			State: workflows.BudgetCreationState{
				ResponseText: "Não consegui criar o orçamento. Tente novamente em breve.",
			},
		}, writeErr).Once()

	uc := NewBudgetCreationContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	handled, reply, err := uc.Continue(s.ctx, "user-10", "sim", "wamid-010")

	s.NoError(err)
	s.True(handled)
	s.Equal("Não consegui criar o orçamento. Tente novamente em breve.", reply)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusFailed, s.runs.updated[0].Status)
	s.Contains(s.runs.updated[0].Error, "db unavailable")
}

func (s *BudgetCreationContinuerSuite) TestContinue_FalhaAoAbrirThreadNaoAbreRun() {
	s.threads.err = errors.New("thread store down")

	uc := NewBudgetCreationContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	handled, reply, err := uc.Continue(s.ctx, "user-8", "sim", "wamid-008")

	s.Error(err)
	s.False(handled)
	s.Empty(reply)
	s.Empty(s.runs.inserted)
}
