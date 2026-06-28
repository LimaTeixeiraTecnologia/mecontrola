package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubOnboardingCompletedUseCase struct {
	calls []usecases.ConsolidateOnboardingWorkingMemoryInput
	err   error
}

func (s *stubOnboardingCompletedUseCase) Execute(_ context.Context, in usecases.ConsolidateOnboardingWorkingMemoryInput) error {
	s.calls = append(s.calls, in)
	return s.err
}

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

func (s *OnboardingCompletedConsumerSuite) buildEvent(userID, eventID string, occurredAt time.Time) platformevents.Event {
	raw, _ := json.Marshal(onboardingCompletedPayload{UserID: userID})
	return &stubEvent{payload: outbox.Envelope{
		ID:         eventID,
		EventType:  "onboarding.completed",
		OccurredAt: occurredAt,
		Payload:    raw,
	}}
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_ValidPayload_DelegatesToUseCase() {
	uc := &stubOnboardingCompletedUseCase{}
	sut := NewOnboardingCompletedConsumer(uc, fake.NewProvider())
	userID := uuid.New().String()
	eventID := uuid.New().String()
	occurredAt := time.Now().UTC().Truncate(time.Second)

	event := s.buildEvent(userID, eventID, occurredAt)
	err := sut.Handle(s.ctx, event)

	s.NoError(err)
	s.Len(uc.calls, 1)
	s.Equal(userID, uc.calls[0].UserID.String())
	s.Equal(eventID, uc.calls[0].EventID.String())
	s.Equal("onboarding.completed", uc.calls[0].EventType)
	s.Equal(occurredAt, uc.calls[0].OccurredAt)
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_InvalidPayloadType_ReturnsError() {
	sut := NewOnboardingCompletedConsumer(&stubOnboardingCompletedUseCase{}, fake.NewProvider())
	event := &stubEvent{payload: "not_an_envelope"}
	err := sut.Handle(s.ctx, event)
	s.Error(err)
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_MalformedJSON_ReturnsError() {
	sut := NewOnboardingCompletedConsumer(&stubOnboardingCompletedUseCase{}, fake.NewProvider())
	event := &stubEvent{payload: outbox.Envelope{Payload: []byte("{bad json")}}
	err := sut.Handle(s.ctx, event)
	s.Error(err)
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_InvalidUserID_ReturnsError() {
	sut := NewOnboardingCompletedConsumer(&stubOnboardingCompletedUseCase{}, fake.NewProvider())
	event := s.buildEvent("not-a-uuid", uuid.New().String(), time.Now().UTC())
	err := sut.Handle(s.ctx, event)
	s.Error(err)
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_UseCaseError_ReturnsError() {
	uc := &stubOnboardingCompletedUseCase{err: errors.New("use case failed")}
	sut := NewOnboardingCompletedConsumer(uc, fake.NewProvider())
	event := s.buildEvent(uuid.New().String(), uuid.New().String(), time.Now().UTC())
	err := sut.Handle(s.ctx, event)
	s.Error(err)
}

func (s *OnboardingCompletedConsumerSuite) TestHandle_ZeroOccurredAt_FallsBackToNow() {
	uc := &stubOnboardingCompletedUseCase{}
	sut := NewOnboardingCompletedConsumer(uc, fake.NewProvider())
	userID := uuid.New().String()
	eventID := uuid.New().String()

	raw, _ := json.Marshal(onboardingCompletedPayload{UserID: userID})
	event := &stubEvent{payload: outbox.Envelope{
		ID:        eventID,
		EventType: "onboarding.completed",
		Payload:   raw,
	}}

	err := sut.Handle(s.ctx, event)
	s.NoError(err)
	s.Len(uc.calls, 1)
	s.False(uc.calls[0].OccurredAt.IsZero())
}
