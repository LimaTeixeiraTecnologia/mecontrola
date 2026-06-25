package services

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type stubObservationRepo struct {
	insertSignal chan struct{}
	listResult   []entities.Observation
	listErr      error
	deleteErr    error
}

func newStubRepo() *stubObservationRepo {
	return &stubObservationRepo{insertSignal: make(chan struct{}, 1)}
}

func (s *stubObservationRepo) Insert(_ context.Context, _ entities.Observation) error {
	select {
	case s.insertSignal <- struct{}{}:
	default:
	}
	return nil
}

func (s *stubObservationRepo) ListRecent(_ context.Context, _ uuid.UUID, _ string, _ int) ([]entities.Observation, error) {
	return s.listResult, s.listErr
}

func (s *stubObservationRepo) DeleteExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (s *stubObservationRepo) DeleteOldestBeyondLimit(_ context.Context, _ uuid.UUID, _ string, _ int) error {
	return s.deleteErr
}

type stubObserverInterpreter struct {
	resp interfaces.LLMResponse
	err  error
}

func (s *stubObserverInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	return s.resp, s.err
}

type ObservationMemorySuite struct {
	suite.Suite
	ctx  context.Context
	o11y observability.Observability
}

func TestObservationMemorySuite(t *testing.T) {
	suite.Run(t, new(ObservationMemorySuite))
}

func (s *ObservationMemorySuite) SetupTest() {
	s.ctx = context.Background()
	s.o11y = fake.NewProvider()
}

func (s *ObservationMemorySuite) TestLoadContextEmpty() {
	type args struct {
		userID  uuid.UUID
		channel string
	}
	type dependencies struct {
		repo *stubObservationRepo
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result string)
	}{
		{
			name: "repo returns empty list → empty string",
			args: args{userID: uuid.New(), channel: "whatsapp"},
			dependencies: dependencies{
				repo: newStubRepo(),
			},
			expect: func(result string) {
				s.Empty(result)
			},
		},
		{
			name: "repo returns error → empty string",
			args: args{userID: uuid.New(), channel: "whatsapp"},
			dependencies: dependencies{
				repo: &stubObservationRepo{insertSignal: make(chan struct{}, 1), listErr: context.DeadlineExceeded},
			},
			expect: func(result string) {
				s.Empty(result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			svc := NewObservationMemory(
				&stubObserverInterpreter{},
				scenario.dependencies.repo,
				s.o11y,
				5,
				10,
			)
			result := svc.LoadContext(s.ctx, scenario.args.userID, scenario.args.channel)
			scenario.expect(result)
		})
	}
}

func (s *ObservationMemorySuite) TestLoadContextWithObservations() {
	userID := uuid.New()
	now := time.Now().UTC()

	type args struct {
		userID  uuid.UUID
		channel string
	}
	type dependencies struct {
		repo *stubObservationRepo
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result string)
	}{
		{
			name: "two observations joined with separator",
			args: args{userID: userID, channel: "whatsapp"},
			dependencies: dependencies{
				repo: &stubObservationRepo{
					insertSignal: make(chan struct{}, 1),
					listResult: []entities.Observation{
						{ID: uuid.New(), UserID: userID, Channel: "whatsapp", Content: "obs2", CreatedAt: now},
						{ID: uuid.New(), UserID: userID, Channel: "whatsapp", Content: "obs1", CreatedAt: now.Add(-time.Minute)},
					},
				},
			},
			expect: func(result string) {
				s.NotEmpty(result)
				s.Contains(result, "obs1")
				s.Contains(result, "obs2")
				s.Contains(result, "---")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			svc := NewObservationMemory(
				&stubObserverInterpreter{},
				scenario.dependencies.repo,
				s.o11y,
				5,
				10,
			)
			result := svc.LoadContext(s.ctx, scenario.args.userID, scenario.args.channel)
			scenario.expect(result)
		})
	}
}

func (s *ObservationMemorySuite) TestMaybeTriggerBelowThreshold() {
	type args struct {
		userID   uuid.UUID
		channel  string
		turns    []entities.ConversationMessage
		maxTurns int
	}
	type dependencies struct {
		repo *stubObservationRepo
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(repo *stubObservationRepo)
	}{
		{
			name: "turns below maxTurns → no goroutine launched",
			args: args{
				userID:   uuid.New(),
				channel:  "whatsapp",
				turns:    []entities.ConversationMessage{{Role: "user", Content: "hello"}},
				maxTurns: 5,
			},
			dependencies: dependencies{
				repo: newStubRepo(),
			},
			expect: func(repo *stubObservationRepo) {
				select {
				case <-repo.insertSignal:
					s.Fail("insert should not have been called")
				case <-time.After(50 * time.Millisecond):
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			svc := NewObservationMemory(
				&stubObserverInterpreter{},
				scenario.dependencies.repo,
				s.o11y,
				scenario.args.maxTurns,
				10,
			)
			svc.MaybeTrigger(s.ctx, scenario.args.userID, scenario.args.channel, scenario.args.turns)
			scenario.expect(scenario.dependencies.repo)
		})
	}
}

func (s *ObservationMemorySuite) TestMaybeTriggerAboveThreshold() {
	type args struct {
		userID   uuid.UUID
		channel  string
		turns    []entities.ConversationMessage
		maxTurns int
	}
	type dependencies struct {
		repo        *stubObservationRepo
		interpreter *stubObserverInterpreter
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(repo *stubObservationRepo)
	}{
		{
			name: "turns at maxTurns → goroutine launched and insert called",
			args: args{
				userID:  uuid.New(),
				channel: "whatsapp",
				turns: []entities.ConversationMessage{
					{Role: "user", Content: "msg1"},
					{Role: "assistant", Content: "resp1"},
					{Role: "user", Content: "msg2"},
					{Role: "assistant", Content: "resp2"},
					{Role: "user", Content: "msg3"},
				},
				maxTurns: 5,
			},
			dependencies: dependencies{
				repo: newStubRepo(),
				interpreter: &stubObserverInterpreter{
					resp: interfaces.LLMResponse{RawJSON: []byte("summary observation text")},
				},
			},
			expect: func(repo *stubObservationRepo) {
				select {
				case <-repo.insertSignal:
				case <-time.After(2 * time.Second):
					s.Fail("insert was not called within 2 seconds")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			svc := NewObservationMemory(
				scenario.dependencies.interpreter,
				scenario.dependencies.repo,
				s.o11y,
				scenario.args.maxTurns,
				10,
			)
			svc.MaybeTrigger(s.ctx, scenario.args.userID, scenario.args.channel, scenario.args.turns)
			scenario.expect(scenario.dependencies.repo)
		})
	}
}
