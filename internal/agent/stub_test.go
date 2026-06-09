package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

type mockWAGateway struct {
	mock.Mock
}

func (m *mockWAGateway) SendTextMessage(ctx context.Context, toE164, text string) error {
	args := m.Called(ctx, toE164, text)
	return args.Error(0)
}

type StubAgentSuite struct {
	suite.Suite
	ctx context.Context
}

func TestStubAgentSuite(t *testing.T) {
	suite.Run(t, new(StubAgentSuite))
}

func (s *StubAgentSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *StubAgentSuite) TestHandleMessage() {
	const stubTemplate = "MeControla recebeu sua mensagem — estamos preparando sua experiência."

	templates := map[string]string{
		"agent_stub_received": stubTemplate,
	}

	userID := uuid.New()
	msg := payload.Message{
		From:  "+5511987654321",
		WAMID: "wamid.test.001",
		Text:  "oi",
	}

	scenarios := []struct {
		name      string
		ctx       func() context.Context
		setupMock func(gw *mockWAGateway)
		assertErr func(err error)
	}{
		{
			name: "Principal presente: SendTextMessage chamado com template correto",
			ctx: func() context.Context {
				p := auth.Principal{UserID: userID, Source: auth.SourceWhatsApp}
				return auth.WithPrincipal(s.ctx, p)
			},
			setupMock: func(gw *mockWAGateway) {
				gw.On("SendTextMessage", mock.Anything, msg.From, stubTemplate).Return(nil)
			},
			assertErr: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "gateway falha: erro propagado",
			ctx: func() context.Context {
				p := auth.Principal{UserID: userID, Source: auth.SourceWhatsApp}
				return auth.WithPrincipal(s.ctx, p)
			},
			setupMock: func(gw *mockWAGateway) {
				gw.On("SendTextMessage", mock.Anything, msg.From, stubTemplate).
					Return(errors.New("send error"))
			},
			assertErr: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "send template")
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			gw := &mockWAGateway{}
			sc.setupMock(gw)
			defer gw.AssertExpectations(s.T())

			sut := agent.NewStubAgent(gw, templates, noop.NewProvider())
			err := sut.HandleMessage(sc.ctx(), msg)
			sc.assertErr(err)
		})
	}
}

func (s *StubAgentSuite) TestHandleMessage_MissingTemplate() {
	gw := &mockWAGateway{}
	sut := agent.NewStubAgent(gw, map[string]string{}, noop.NewProvider())
	err := sut.HandleMessage(s.ctx, payload.Message{From: "+5511987654321"})
	s.Error(err)
	s.Contains(err.Error(), "agent_stub_received")
}
