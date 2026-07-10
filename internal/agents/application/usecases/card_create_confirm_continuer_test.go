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

type mockCardCreateEngine struct {
	mock.Mock
}

func (m *mockCardCreateEngine) Start(ctx context.Context, def workflow.Definition[workflows.CardCreateState], key string, initial workflows.CardCreateState) (workflow.RunResult[workflows.CardCreateState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[workflows.CardCreateState]), args.Error(1)
}

func (m *mockCardCreateEngine) Resume(ctx context.Context, def workflow.Definition[workflows.CardCreateState], key string, resume []byte) (workflow.RunResult[workflows.CardCreateState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[workflows.CardCreateState]), args.Error(1)
}

func (m *mockCardCreateEngine) LoadLatestState(ctx context.Context, def workflow.Definition[workflows.CardCreateState], key string) (workflows.CardCreateState, workflow.Snapshot, bool, error) {
	args := m.Called(ctx, def, key)
	return args.Get(0).(workflows.CardCreateState), args.Get(1).(workflow.Snapshot), args.Bool(2), args.Error(3)
}

type fakeCardCreateThreadGateway struct {
	thread memory.Thread
	err    error
}

func (f *fakeCardCreateThreadGateway) GetOrCreate(_ context.Context, _, _ string) (memory.Thread, error) {
	return f.thread, f.err
}

type fakeCardCreateRunStore struct {
	insertErr error
	updateErr error
	inserted  []agent.Run
	updated   []agent.Run
}

func (f *fakeCardCreateRunStore) Insert(_ context.Context, run agent.Run) error {
	f.inserted = append(f.inserted, run)
	return f.insertErr
}

func (f *fakeCardCreateRunStore) Update(_ context.Context, run agent.Run) error {
	f.updated = append(f.updated, run)
	return f.updateErr
}

func (f *fakeCardCreateRunStore) Load(_ context.Context, _ uuid.UUID) (agent.Run, error) {
	return agent.Run{}, nil
}

type CardCreateConfirmContinuerSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	emptyDef   workflow.Definition[workflows.CardCreateState]
	engineMock *mockCardCreateEngine
	threads    *fakeCardCreateThreadGateway
	runs       *fakeCardCreateRunStore
}

func TestCardCreateConfirmContinuerSuite(t *testing.T) {
	suite.Run(t, new(CardCreateConfirmContinuerSuite))
}

func (s *CardCreateConfirmContinuerSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.emptyDef = workflow.Definition[workflows.CardCreateState]{}
	s.engineMock = &mockCardCreateEngine{}
	s.engineMock.Test(s.T())
	s.T().Cleanup(func() { s.engineMock.AssertExpectations(s.T()) })
	s.threads = &fakeCardCreateThreadGateway{thread: memory.Thread{ID: uuid.New()}}
	s.runs = &fakeCardCreateRunStore{}
}

func (s *CardCreateConfirmContinuerSuite) TestContinue() {
	type args struct {
		resourceID string
		peer       string
		message    string
		messageID  string
	}
	type dependencies struct {
		engine *mockCardCreateEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(handled bool, reply string, err error)
	}{
		{
			name: "deve retornar handled=false quando nao ha run suspenso",
			args: args{resourceID: "user-1", peer: "+5511999999999", message: "sim", messageID: "wamid-001"},
			dependencies: dependencies{
				engine: func() *mockCardCreateEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-1:card-create", mock.Anything).
						Return(workflow.RunResult[workflows.CardCreateState]{}, nil).Once()
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
			args: args{resourceID: "user-2", peer: "+5511999999999", message: "não entendi", messageID: "wamid-002"},
			dependencies: dependencies{
				engine: func() *mockCardCreateEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-2:card-create", mock.Anything).
						Return(workflow.RunResult[workflows.CardCreateState]{
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
			args: args{resourceID: "user-3", peer: "+5511999999999", message: "sim", messageID: "wamid-003"},
			dependencies: dependencies{
				engine: func() *mockCardCreateEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-3:card-create", mock.Anything).
						Return(workflow.RunResult[workflows.CardCreateState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.CardCreateState{
								Status:       workflows.CardCreateStatusCompleted,
								ResponseText: "✅ Cartão *Nu* cadastrado com sucesso.",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.True(handled)
				s.Equal("✅ Cartão *Nu* cadastrado com sucesso.", reply)
			},
		},
		{
			name: "deve retornar handled=false quando expirado",
			args: args{resourceID: "user-4", peer: "+5511999999999", message: "sim", messageID: "wamid-004"},
			dependencies: dependencies{
				engine: func() *mockCardCreateEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-4:card-create", mock.Anything).
						Return(workflow.RunResult[workflows.CardCreateState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.CardCreateState{
								Status:  workflows.CardCreateStatusExpired,
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
			name: "deve retornar erro quando engine falha",
			args: args{resourceID: "user-5", peer: "+5511999999999", message: "sim", messageID: "wamid-005"},
			dependencies: dependencies{
				engine: func() *mockCardCreateEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-5:card-create", mock.Anything).
						Return(workflow.RunResult[workflows.CardCreateState]{}, errors.New("engine error")).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.Error(err)
				s.False(handled)
				s.Contains(err.Error(), "card_create_confirm_continuer")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewCardCreateConfirmContinuer(scenario.dependencies.engine, s.emptyDef, s.threads, s.runs, s.obs)
			handled, reply, err := uc.Continue(s.ctx, scenario.args.resourceID, scenario.args.peer, scenario.args.message, scenario.args.messageID)
			scenario.expect(handled, reply, err)
		})
	}
}

func (s *CardCreateConfirmContinuerSuite) TestContinue_AbreEFechaRunAuditavel() {
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-6:card-create", mock.Anything).
		Return(workflow.RunResult[workflows.CardCreateState]{
			Status: workflow.RunStatusSucceeded,
			State: workflows.CardCreateState{
				Status:       workflows.CardCreateStatusCompleted,
				ResponseText: "✅ Cartão cadastrado com sucesso.",
			},
		}, nil).Once()

	uc := NewCardCreateConfirmContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	_, _, err := uc.Continue(s.ctx, "user-6", "+5511999999999", "sim", "wamid-006")

	s.NoError(err)
	s.Require().Len(s.runs.inserted, 1)
	s.Equal(agent.RunStatusRunning, s.runs.inserted[0].Status)
	s.Equal(workflows.CardCreateConfirmWorkflowID, s.runs.inserted[0].Workflow)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusSucceeded, s.runs.updated[0].Status)
	s.Empty(s.runs.updated[0].Error)
}

func (s *CardCreateConfirmContinuerSuite) TestContinue_EscritaFalhaGravaErroRealNoRun() {
	writeErr := errors.New("db unavailable")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-7:card-create", mock.Anything).
		Return(workflow.RunResult[workflows.CardCreateState]{}, writeErr).Once()

	uc := NewCardCreateConfirmContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	_, _, err := uc.Continue(s.ctx, "user-7", "+5511999999999", "sim", "wamid-007")

	s.Error(err)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusFailed, s.runs.updated[0].Status)
	s.Contains(s.runs.updated[0].Error, "db unavailable")
}

func (s *CardCreateConfirmContinuerSuite) TestContinue_FalhaAoFecharRunObservaSemQuebrarNegocio() {
	s.runs.updateErr = errors.New("update falhou no fechamento")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-9:card-create", mock.Anything).
		Return(workflow.RunResult[workflows.CardCreateState]{
			Status: workflow.RunStatusSucceeded,
			State: workflows.CardCreateState{
				Status:       workflows.CardCreateStatusCompleted,
				ResponseText: "✅ Cartão cadastrado com sucesso.",
			},
		}, nil).Once()

	uc := NewCardCreateConfirmContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	handled, reply, err := uc.Continue(s.ctx, "user-9", "+5511999999999", "sim", "wamid-009")

	s.NoError(err)
	s.True(handled)
	s.Equal("✅ Cartão cadastrado com sucesso.", reply)

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("agents_run_update_errors_total")
	s.Require().NotNil(counter)
	values := counter.GetValues()
	s.Require().Len(values, 1)
	assertRunUpdateErrorLabels(s.T(), values[0].Fields, workflows.CardCreateConfirmWorkflowID, "close", "succeeded")

	entries := s.obs.Logger().(*fake.FakeLogger).GetEntries()
	s.True(hasRunUpdateErrorLog(entries), "deve emitir log estruturado de falha de fechamento")
}

func (s *CardCreateConfirmContinuerSuite) TestContinue_FalhaAoAbrirThreadNaoAbreRun() {
	s.threads.err = errors.New("thread store down")

	uc := NewCardCreateConfirmContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	handled, reply, err := uc.Continue(s.ctx, "user-8", "+5511999999999", "sim", "wamid-008")

	s.Error(err)
	s.False(handled)
	s.Empty(reply)
	s.Empty(s.runs.inserted)
}
