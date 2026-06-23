package consumers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubTelegramRouter struct {
	called    bool
	principal appservices.Principal
	msg       appservices.InboundMessage
}

func (s *stubTelegramRouter) RouteTelegram(_ context.Context, p appservices.Principal, m appservices.InboundMessage) appservices.RouteResult {
	s.called = true
	s.principal = p
	s.msg = m
	return appservices.RouteResult{}
}

type TelegramInboundConsumerSuite struct {
	suite.Suite
	ctx    context.Context
	router *stubTelegramRouter
	sut    *TelegramInboundConsumer
}

func TestTelegramInboundConsumerSuite(t *testing.T) {
	suite.Run(t, new(TelegramInboundConsumerSuite))
}

func (s *TelegramInboundConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.router = &stubTelegramRouter{}
	s.sut = NewTelegramInboundConsumer(s.router, fake.NewProvider())
}

func (s *TelegramInboundConsumerSuite) TestHandle_ValidPayload_CallsRouter() {
	userID := uuid.New()
	raw, _ := json.Marshal(telegramInboundPayload{
		UserID:    userID.String(),
		Peer:      "987654321",
		Text:      "listar gastos do mês",
		MessageID: "42",
	})
	event := &stubEvent{payload: outbox.Envelope{Payload: raw}}

	err := s.sut.Handle(s.ctx, event)

	s.NoError(err)
	s.True(s.router.called)
	s.Equal(userID, s.router.principal.UserID)
	s.Equal(int64(987654321), s.router.msg.TelegramTo)
	s.Equal("listar gastos do mês", s.router.msg.Text)
	s.Equal("42", s.router.msg.MessageID)
}

func (s *TelegramInboundConsumerSuite) TestHandle_InvalidPayloadType_ReturnsError() {
	event := &stubEvent{payload: 42}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *TelegramInboundConsumerSuite) TestHandle_MalformedJSON_ReturnsError() {
	event := &stubEvent{payload: outbox.Envelope{Payload: []byte("{bad json")}}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *TelegramInboundConsumerSuite) TestHandle_InvalidPeerChatID_ReturnsError() {
	raw, _ := json.Marshal(telegramInboundPayload{
		UserID:    uuid.New().String(),
		Peer:      "not-a-number",
		Text:      "teste",
		MessageID: "1",
	})
	event := &stubEvent{payload: outbox.Envelope{Payload: raw}}

	err := s.sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}
