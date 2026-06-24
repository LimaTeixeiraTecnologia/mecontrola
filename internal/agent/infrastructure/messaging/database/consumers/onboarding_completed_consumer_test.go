package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agententities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubContextReader struct {
	result onbusecases.GetOnboardingContextResult
	err    error
}

func (r *stubContextReader) Execute(_ context.Context, _ onbusecases.GetOnboardingContextInput) (onbusecases.GetOnboardingContextResult, error) {
	return r.result, r.err
}

type stubWMRepo struct {
	getResult agententities.WorkingMemory
	getFound  bool
	getErr    error
	upsertErr error
	upserted  *agententities.WorkingMemory
}

func (r *stubWMRepo) Get(_ context.Context, _ uuid.UUID) (agententities.WorkingMemory, bool, error) {
	return r.getResult, r.getFound, r.getErr
}

func (r *stubWMRepo) Upsert(_ context.Context, wm agententities.WorkingMemory) error {
	r.upserted = &wm
	return r.upsertErr
}

var _ agentinterfaces.WorkingMemoryRepository = (*stubWMRepo)(nil)

type OnboardingCompletedConsumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestOnboardingCompletedConsumerSuite(t *testing.T) {
	suite.Run(t, new(OnboardingCompletedConsumerSuite))
}

func (s *OnboardingCompletedConsumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *OnboardingCompletedConsumerSuite) buildEvent(userID string) platformevents.Event {
	raw, _ := json.Marshal(onboardingCompletedPayload{UserID: userID})
	return &stubEvent{payload: outbox.Envelope{Payload: raw}}
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_ValidPayload_UpsertsCalled() {
	type dependencies struct {
		reader *stubContextReader
		wmRepo *stubWMRepo
	}
	type args struct {
		userID string
	}

	scenarios := []struct {
		name   string
		args   args
		deps   dependencies
		expect func(wmRepo *stubWMRepo, err error)
	}{
		{
			name: "deve chamar upsert quando onboarding encontrado e WM ausente",
			args: args{userID: uuid.New().String()},
			deps: func() dependencies {
				return dependencies{
					reader: &stubContextReader{
						result: onbusecases.GetOnboardingContextResult{
							Found:       true,
							Objective:   "quitar dívidas",
							IncomeCents: 500000,
						},
					},
					wmRepo: &stubWMRepo{getFound: false},
				}
			}(),
			expect: func(wmRepo *stubWMRepo, err error) {
				s.NoError(err)
				s.NotNil(wmRepo.upserted)
				s.Contains(wmRepo.upserted.Content, "quitar dívidas")
			},
		},
		{
			name: "deve ser no-op quando WM ja possui conteudo",
			args: args{userID: uuid.New().String()},
			deps: func() dependencies {
				existing := agententities.NewWorkingMemory(uuid.New())
				existing.Content = "conteudo existente"
				return dependencies{
					reader: &stubContextReader{
						result: onbusecases.GetOnboardingContextResult{
							Found:       true,
							Objective:   "economizar",
							IncomeCents: 300000,
						},
					},
					wmRepo: &stubWMRepo{getFound: true, getResult: existing},
				}
			}(),
			expect: func(wmRepo *stubWMRepo, err error) {
				s.NoError(err)
				s.Nil(wmRepo.upserted)
			},
		},
		{
			name: "deve retornar erro quando contextReader falha",
			args: args{userID: uuid.New().String()},
			deps: func() dependencies {
				return dependencies{
					reader: &stubContextReader{err: errors.New("db error")},
					wmRepo: &stubWMRepo{},
				}
			}(),
			expect: func(wmRepo *stubWMRepo, err error) {
				s.Error(err)
				s.Nil(wmRepo.upserted)
			},
		},
		{
			name: "deve retornar erro e incrementar decodeFails quando user_id invalido",
			args: args{userID: "not-a-uuid"},
			deps: func() dependencies {
				return dependencies{
					reader: &stubContextReader{},
					wmRepo: &stubWMRepo{},
				}
			}(),
			expect: func(wmRepo *stubWMRepo, err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro quando upsert falha",
			args: args{userID: uuid.New().String()},
			deps: func() dependencies {
				return dependencies{
					reader: &stubContextReader{
						result: onbusecases.GetOnboardingContextResult{
							Found:       true,
							Objective:   "investir",
							IncomeCents: 800000,
						},
					},
					wmRepo: &stubWMRepo{getFound: false, upsertErr: errors.New("upsert failed")},
				}
			}(),
			expect: func(wmRepo *stubWMRepo, err error) {
				s.Error(err)
			},
		},
		{
			name: "deve ser no-op quando onboarding nao encontrado",
			args: args{userID: uuid.New().String()},
			deps: func() dependencies {
				return dependencies{
					reader: &stubContextReader{result: onbusecases.GetOnboardingContextResult{Found: false}},
					wmRepo: &stubWMRepo{},
				}
			}(),
			expect: func(wmRepo *stubWMRepo, err error) {
				s.NoError(err)
				s.Nil(wmRepo.upserted)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewOnboardingCompletedConsumer(scenario.deps.reader, scenario.deps.wmRepo, fake.NewProvider())
			event := s.buildEvent(scenario.args.userID)
			err := sut.Handle(s.ctx, event)
			scenario.expect(scenario.deps.wmRepo, err)
		})
	}
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_InvalidPayloadType_ReturnsError() {
	sut := NewOnboardingCompletedConsumer(&stubContextReader{}, &stubWMRepo{}, fake.NewProvider())
	event := &stubEvent{payload: "not_an_envelope"}
	err := sut.Handle(s.ctx, event)
	s.Error(err)
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_MalformedJSON_ReturnsError() {
	sut := NewOnboardingCompletedConsumer(&stubContextReader{}, &stubWMRepo{}, fake.NewProvider())
	event := &stubEvent{payload: outbox.Envelope{Payload: []byte("{bad json")}}
	err := sut.Handle(s.ctx, event)
	s.Error(err)
}
