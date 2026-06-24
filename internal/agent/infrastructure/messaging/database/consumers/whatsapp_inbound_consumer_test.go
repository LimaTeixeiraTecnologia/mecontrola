package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubWhatsAppRouter struct {
	called    bool
	principal appservices.Principal
	msg       appservices.InboundMessage
	result    *appservices.RouteResult
}

func (s *stubWhatsAppRouter) RouteWhatsApp(_ context.Context, p appservices.Principal, m appservices.InboundMessage) appservices.RouteResult {
	s.called = true
	s.principal = p
	s.msg = m
	if s.result != nil {
		return *s.result
	}
	return appservices.RouteResult{Reply: "ok", Outcome: tools.OutcomeRouted, Delivered: true}
}

type stubEvent struct {
	payload any
}

func (e *stubEvent) GetEventType() string { return "agent.whatsapp.inbound.v1" }
func (e *stubEvent) GetPayload() any      { return e.payload }

type WhatsAppInboundConsumerSuite struct {
	suite.Suite
	ctx    context.Context
	router *stubWhatsAppRouter
	sut    *WhatsAppInboundConsumer
}

func TestWhatsAppInboundConsumerSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppInboundConsumerSuite))
}

func (s *WhatsAppInboundConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.router = &stubWhatsAppRouter{}
	s.sut = NewWhatsAppInboundConsumer(s.router, fake.NewProvider())
}

func (s *WhatsAppInboundConsumerSuite) buildEvent(userID uuid.UUID, peer, text, messageID string) platformevents.Event {
	raw, _ := json.Marshal(whatsAppInboundPayload{
		UserID:    userID.String(),
		Peer:      peer,
		Text:      text,
		MessageID: messageID,
	})
	return &stubEvent{payload: outbox.Envelope{Payload: raw}}
}

func (s *WhatsAppInboundConsumerSuite) TestHandle_ValidPayload_CallsRouter() {
	userID := uuid.New()
	event := s.buildEvent(userID, "+5511999990000", "quero registrar um gasto", "wamid.abc123")

	err := s.sut.Handle(s.ctx, event)

	s.NoError(err)
	s.True(s.router.called)
	s.Equal(userID, s.router.principal.UserID)
	s.Equal("+5511999990000", s.router.msg.WhatsAppTo)
	s.Equal("quero registrar um gasto", s.router.msg.Text)
	s.Equal("wamid.abc123", s.router.msg.MessageID)
}

func (s *WhatsAppInboundConsumerSuite) TestHandle_InvalidPayloadType_ReturnsError() {
	event := &stubEvent{payload: "not_an_envelope"}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *WhatsAppInboundConsumerSuite) TestHandle_MalformedJSON_ReturnsError() {
	event := &stubEvent{payload: outbox.Envelope{Payload: []byte("not json")}}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *WhatsAppInboundConsumerSuite) TestHandle_InvalidUserID_ReturnsError() {
	raw, _ := json.Marshal(whatsAppInboundPayload{
		UserID:    "not-a-uuid",
		Peer:      "+5511999990000",
		Text:      "teste",
		MessageID: "wamid.xyz",
	})
	event := &stubEvent{payload: outbox.Envelope{Payload: raw}}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.True(errors.Is(err, err))
	s.False(s.router.called)
}
