package consumers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type OnboardingBoundConsumerSuite struct {
	suite.Suite
	ctx    context.Context
	router *stubWhatsAppRouter
	sut    *OnboardingBoundConsumer
}

func TestOnboardingBoundConsumerSuite(t *testing.T) {
	suite.Run(t, new(OnboardingBoundConsumerSuite))
}

func (s *OnboardingBoundConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.router = &stubWhatsAppRouter{}
	s.sut = NewOnboardingBoundConsumer(s.router, fake.NewProvider())
}

func (s *OnboardingBoundConsumerSuite) buildEvent(userID, peer string) platformevents.Event {
	raw, _ := json.Marshal(onboardingBoundPayload{
		UserID:   userID,
		PeerE164: peer,
	})
	return &stubEvent{payload: outbox.Envelope{Payload: raw}}
}

func (s *OnboardingBoundConsumerSuite) TestHandle_ValidPayload_CallsRouter() {
	userID := uuid.New()
	event := s.buildEvent(userID.String(), "+5511999990000")

	err := s.sut.Handle(s.ctx, event)

	s.NoError(err)
	s.True(s.router.called)
	s.Equal(userID, s.router.principal.UserID)
	s.Equal(appservices.OnboardingWelcomeSignal, s.router.msg.Text)
	s.Equal("+5511999990000", s.router.msg.WhatsAppTo)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_MissingPeer_DoesNotCallRouter() {
	userID := uuid.New()
	event := s.buildEvent(userID.String(), "")

	err := s.sut.Handle(s.ctx, event)

	s.NoError(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_MalformedJSON_ReturnsError() {
	event := &stubEvent{payload: outbox.Envelope{Payload: []byte("not json")}}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_InvalidUserID_ReturnsError() {
	event := s.buildEvent("not-a-uuid", "+5511999990000")

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_InvalidPayloadType_ReturnsError() {
	event := &stubEvent{payload: "not_an_envelope"}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}
