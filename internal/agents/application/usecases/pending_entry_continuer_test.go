package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

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

type mockPendingEntryEngine struct {
	mock.Mock
}

func (m *mockPendingEntryEngine) Start(ctx context.Context, def workflow.Definition[workflows.PendingEntryState], key string, initial workflows.PendingEntryState) (workflow.RunResult[workflows.PendingEntryState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[workflows.PendingEntryState]), args.Error(1)
}

func (m *mockPendingEntryEngine) Resume(ctx context.Context, def workflow.Definition[workflows.PendingEntryState], key string, resume []byte) (workflow.RunResult[workflows.PendingEntryState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[workflows.PendingEntryState]), args.Error(1)
}

func (m *mockPendingEntryEngine) LoadLatestState(ctx context.Context, def workflow.Definition[workflows.PendingEntryState], key string) (workflows.PendingEntryState, workflow.Snapshot, bool, error) {
	args := m.Called(ctx, def, key)
	return args.Get(0).(workflows.PendingEntryState), args.Get(1).(workflow.Snapshot), args.Bool(2), args.Error(3)
}

type fakePendingEntryThreadGateway struct {
	thread memory.Thread
	err    error
}

func (f *fakePendingEntryThreadGateway) GetOrCreate(_ context.Context, _, _ string) (memory.Thread, error) {
	return f.thread, f.err
}

type fakePendingEntryRunStore struct {
	insertErr error
	updateErr error
	inserted  []agent.Run
	updated   []agent.Run
}

func (f *fakePendingEntryRunStore) Insert(_ context.Context, run agent.Run) error {
	f.inserted = append(f.inserted, run)
	return f.insertErr
}

func (f *fakePendingEntryRunStore) Update(_ context.Context, run agent.Run) error {
	f.updated = append(f.updated, run)
	return f.updateErr
}

func (f *fakePendingEntryRunStore) Load(_ context.Context, _ uuid.UUID) (agent.Run, error) {
	return agent.Run{}, nil
}

type PendingEntryContinuerSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	emptyDef   workflow.Definition[workflows.PendingEntryState]
	engineMock *mockPendingEntryEngine
	threads    *fakePendingEntryThreadGateway
	runs       *fakePendingEntryRunStore
}

func TestPendingEntryContinuerSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryContinuerSuite))
}

func (s *PendingEntryContinuerSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.emptyDef = workflow.Definition[workflows.PendingEntryState]{}
	s.engineMock = &mockPendingEntryEngine{}
	s.engineMock.Test(s.T())
	s.T().Cleanup(func() { s.engineMock.AssertExpectations(s.T()) })
	s.threads = &fakePendingEntryThreadGateway{thread: memory.Thread{ID: uuid.New()}}
	s.runs = &fakePendingEntryRunStore{}
}

func (s *PendingEntryContinuerSuite) TestContinue() {
	type args struct {
		userID    string
		peer      string
		message   string
		messageID string
	}
	type dependencies struct {
		engine *mockPendingEntryEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result workflows.PendingEntryResult, err error)
	}{
		{
			name: "deve retornar Handled=false quando nao ha run suspenso",
			args: args{userID: "user-1", peer: "+5511999999999", message: "sim", messageID: "wamid-001"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-1:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{}, nil).Once()
					s.engineMock.On("LoadLatestState", mock.Anything, mock.Anything, "user-1:+5511999999999:pending-entry").
						Return(workflows.PendingEntryState{}, workflow.Snapshot{}, false, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.False(result.Handled)
			},
		},
		{
			name: "deve retornar erro quando engine falha",
			args: args{userID: "user-2", peer: "+5511999999999", message: "sim", messageID: "wamid-002"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-2:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{}, errors.New("engine error")).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.Error(err)
				s.Contains(err.Error(), "pending_entry_continuer")
			},
		},
		{
			name: "deve retornar Handled=true com modo replied quando run ainda suspenso",
			args: args{userID: "user-3", peer: "+5511999999999", message: "pix", messageID: "wamid-003"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-3:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSuspended,
							State: workflows.PendingEntryState{
								Awaiting: workflows.AwaitingSlotConfirmation,
							},
							Suspend: &workflow.Suspension{
								Reason: workflow.SuspendAwaitingInput,
								Prompt: "Confirma o lançamento?",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.Equal(workflows.PendingEntryModeReplied, result.Mode)
				s.Equal("Confirma o lançamento?", result.Message)
			},
		},
		{
			name: "deve retornar Handled=true com modo completed quando escrita bem sucedida",
			args: args{userID: "user-4", peer: "+5511999999999", message: "sim", messageID: "wamid-004"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-4:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.PendingEntryState{
								Status:       workflows.PendingStatusCompleted,
								ResponseText: "Despesa de R$ 50,00 registrada ✅",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.Equal(workflows.PendingEntryModeCompleted, result.Mode)
				s.Equal("Despesa de R$ 50,00 registrada ✅", result.Message)
			},
		},
		{
			name: "deve retornar Handled=false com modo replaced quando mensagem e nova operacao",
			args: args{userID: "user-5", peer: "+5511999999999", message: "gastei 100 no mercado", messageID: "wamid-005"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-5:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.PendingEntryState{
								Status: workflows.PendingStatusReplaced,
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.False(result.Handled)
				s.Equal(workflows.PendingEntryModeReplaced, result.Mode)
			},
		},
		{
			name: "deve retornar Handled=true com modo cancelled quando run cancelado",
			args: args{userID: "user-6", peer: "+5511999999999", message: "não", messageID: "wamid-006"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-6:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.PendingEntryState{
								Status:       workflows.PendingStatusCancelled,
								ResponseText: "Tudo certo, o registro foi cancelado.",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.Equal(workflows.PendingEntryModeCancelled, result.Mode)
				s.Equal("Tudo certo, o registro foi cancelado.", result.Message)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewPendingEntryContinuer(scenario.dependencies.engine, s.emptyDef, s.threads, s.runs, s.obs)
			result, err := uc.Continue(s.ctx, scenario.args.userID, scenario.args.peer, scenario.args.message, scenario.args.messageID)
			scenario.expect(result, err)
		})
	}
}

func (s *PendingEntryContinuerSuite) TestContinue_AbreEFechaRunAuditavel() {
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-7:+5511999999999:pending-entry", mock.Anything).
		Return(workflow.RunResult[workflows.PendingEntryState]{
			Status: workflow.RunStatusSucceeded,
			State: workflows.PendingEntryState{
				Status:       workflows.PendingStatusCompleted,
				ResponseText: "Despesa registrada",
			},
		}, nil).Once()

	uc := NewPendingEntryContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	_, err := uc.Continue(s.ctx, "user-7", "+5511999999999", "sim", "wamid-007")

	s.NoError(err)
	s.Require().Len(s.runs.inserted, 1)
	s.Equal(agent.RunStatusRunning, s.runs.inserted[0].Status)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusSucceeded, s.runs.updated[0].Status)
	s.Empty(s.runs.updated[0].Error)
}

func (s *PendingEntryContinuerSuite) TestContinue_FalhaAoFecharRunObservaSemQuebrarNegocio() {
	s.runs.updateErr = errors.New("update falhou no fechamento")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-close:+5511999999999:pending-entry", mock.Anything).
		Return(workflow.RunResult[workflows.PendingEntryState]{
			Status: workflow.RunStatusSucceeded,
			State: workflows.PendingEntryState{
				Status:       workflows.PendingStatusCompleted,
				ResponseText: "Despesa registrada",
			},
		}, nil).Once()

	uc := NewPendingEntryContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	result, err := uc.Continue(s.ctx, "user-close", "+5511999999999", "sim", "wamid-close")

	s.NoError(err)
	s.True(result.Handled)
	s.Equal("Despesa registrada", result.Message)

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("agents_run_update_errors_total")
	s.Require().NotNil(counter)
	values := counter.GetValues()
	s.Require().Len(values, 1)
	assertRunUpdateErrorLabels(s.T(), values[0].Fields, workflows.PendingEntryWorkflowID, "close", "succeeded")

	entries := s.obs.Logger().(*fake.FakeLogger).GetEntries()
	s.True(hasRunUpdateErrorLog(entries), "deve emitir log estruturado de falha de fechamento")
}

func (s *PendingEntryContinuerSuite) TestContinue_EscritaFalhaGravaErroRealNoRun() {
	writeErr := errors.New("db unavailable")
	s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-8:+5511999999999:pending-entry", mock.Anything).
		Return(workflow.RunResult[workflows.PendingEntryState]{}, writeErr).Once()

	uc := NewPendingEntryContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	_, err := uc.Continue(s.ctx, "user-8", "+5511999999999", "sim", "wamid-008")

	s.Error(err)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusFailed, s.runs.updated[0].Status)
	s.Contains(s.runs.updated[0].Error, "db unavailable")
}

func (s *PendingEntryContinuerSuite) TestContinue_RF23_RevivaRunFalhoESeReexecutaAEscritaSemReclassificar() {
	key := "user-9:+5511999999999:pending-entry"
	candidates := []workflows.PendingCategoryCandidate{
		{RootCategoryID: uuid.New(), SubcategoryID: uuid.New(), Path: "Alimentação > Mercado"},
	}
	failedState := workflows.PendingEntryState{
		Status:          workflows.PendingStatusActive,
		Awaiting:        workflows.AwaitingSlotConfirmation,
		Candidates:      candidates,
		CategoryVersion: 7,
		AmountCents:     5000,
		SuspendedAt:     time.Now().UTC(),
	}

	s.engineMock.On("Resume", mock.Anything, mock.Anything, key, mock.Anything).
		Return(workflow.RunResult[workflows.PendingEntryState]{}, nil).Once()
	s.engineMock.On("LoadLatestState", mock.Anything, mock.Anything, key).
		Return(failedState, workflow.Snapshot{Status: workflow.RunStatusFailed}, true, nil).Once()
	s.engineMock.On("Start", mock.Anything, mock.Anything, key, mock.MatchedBy(func(seeded workflows.PendingEntryState) bool {
		return seeded.ResumeText == "sim" &&
			seeded.IncomingMessageID == "wamid-009" &&
			len(seeded.Candidates) == 1 &&
			seeded.CategoryVersion == 7
	})).Return(workflow.RunResult[workflows.PendingEntryState]{
		Status: workflow.RunStatusSucceeded,
		State: workflows.PendingEntryState{
			Status:       workflows.PendingStatusCompleted,
			ResponseText: "Despesa de R$ 50,00 registrada ✅",
		},
	}, nil).Once()

	uc := NewPendingEntryContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	result, err := uc.Continue(s.ctx, "user-9", "+5511999999999", "sim", "wamid-009")

	s.NoError(err)
	s.True(result.Handled)
	s.Equal(workflows.PendingEntryModeCompleted, result.Mode)
	s.Equal("Despesa de R$ 50,00 registrada ✅", result.Message)
}

func (s *PendingEntryContinuerSuite) TestContinue_RF23_NaoRevivaQuandoRunFalhoNaoTemCategoriaResolvida() {
	key := "user-10:+5511999999999:pending-entry"
	failedState := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotCategory,
		Candidates:  nil,
		SuspendedAt: time.Now().UTC(),
	}

	s.engineMock.On("Resume", mock.Anything, mock.Anything, key, mock.Anything).
		Return(workflow.RunResult[workflows.PendingEntryState]{}, nil).Once()
	s.engineMock.On("LoadLatestState", mock.Anything, mock.Anything, key).
		Return(failedState, workflow.Snapshot{Status: workflow.RunStatusFailed}, true, nil).Once()

	uc := NewPendingEntryContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	result, err := uc.Continue(s.ctx, "user-10", "+5511999999999", "sim", "wamid-010")

	s.NoError(err)
	s.False(result.Handled)
}

func (s *PendingEntryContinuerSuite) TestContinue_TTLExpirado_RunFalhoTransitaExplicitamenteParaExpired() {
	key := "user-11:+5511999999999:pending-entry"
	candidates := []workflows.PendingCategoryCandidate{
		{RootCategoryID: uuid.New(), SubcategoryID: uuid.New(), Path: "Alimentação > Mercado"},
	}
	failedState := workflows.PendingEntryState{
		Status:          workflows.PendingStatusActive,
		Awaiting:        workflows.AwaitingSlotConfirmation,
		Candidates:      candidates,
		CategoryVersion: 7,
		SuspendedAt:     time.Now().UTC().Add(-time.Hour),
	}

	s.engineMock.On("Resume", mock.Anything, mock.Anything, key, mock.Anything).
		Return(workflow.RunResult[workflows.PendingEntryState]{}, nil).Once()
	s.engineMock.On("LoadLatestState", mock.Anything, mock.Anything, key).
		Return(failedState, workflow.Snapshot{Status: workflow.RunStatusFailed}, true, nil).Once()
	s.engineMock.On("Start", mock.Anything, mock.Anything, key, mock.MatchedBy(func(seeded workflows.PendingEntryState) bool {
		return seeded.ResumeText == "sim" &&
			seeded.IncomingMessageID == "wamid-011" &&
			seeded.FailedWriteResumeCount == 0
	})).Return(workflow.RunResult[workflows.PendingEntryState]{
		Status: workflow.RunStatusSucceeded,
		State: workflows.PendingEntryState{
			Status:       workflows.PendingStatusExpired,
			ResponseText: "O registro expirou. Para registrar, envie a informação completa novamente.",
		},
	}, nil).Once()

	uc := NewPendingEntryContinuer(s.engineMock, s.emptyDef, s.threads, s.runs, s.obs)
	result, err := uc.Continue(s.ctx, "user-11", "+5511999999999", "sim", "wamid-011")

	s.NoError(err)
	s.True(result.Handled)
	s.Equal(workflows.PendingEntryModeExpired, result.Mode)
	s.Contains(result.Message, "expirou")
}
