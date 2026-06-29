package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type mockHandleInbound struct {
	mock.Mock
}

func (m *mockHandleInbound) Execute(ctx context.Context, in input.InboundInput) (agent.Outcome, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(agent.Outcome), args.Error(1)
}

type mockWhatsAppSender struct {
	mock.Mock
}

func (m *mockWhatsAppSender) SendTextMessage(ctx context.Context, toE164, text string) error {
	args := m.Called(ctx, toE164, text)
	return args.Error(0)
}

type mockEvent struct {
	eventType string
	payload   any
}

func (e *mockEvent) GetEventType() string { return e.eventType }
func (e *mockEvent) GetPayload() any      { return e.payload }

type WhatsAppInboundConsumerSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestWhatsAppInboundConsumerSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppInboundConsumerSuite))
}

func (s *WhatsAppInboundConsumerSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
}

func buildEnvelope(p whatsAppInboundPayload) outbox.Envelope {
	raw, _ := json.Marshal(p)
	return outbox.Envelope{
		ID:      uuid.NewString(),
		Payload: raw,
	}
}

func (s *WhatsAppInboundConsumerSuite) TestHandle() {
	type args struct {
		event *mockEvent
	}
	type dependencies struct {
		inboundMock *mockHandleInbound
		senderMock  *mockWhatsAppSender
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve processar mensagem com sucesso e enviar resposta",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "clima em São Paulo",
						MessageID: "wamid-001",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, input.InboundInput{
						ResourceID: "user-uuid-123",
						ThreadID:   "+5511999999999",
						AgentID:    weatherAgentID,
						Message:    "clima em São Paulo",
						MessageID:  "wamid-001",
					}).Return(agent.Outcome{
						RunID:   uuid.New(),
						Content: "Em São Paulo está 28°C, ensolarado.",
						Status:  agent.RunStatusSucceeded,
					}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "Em São Paulo está 28°C, ensolarado.").
						Return(nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando payload nao e Envelope",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   "tipo_invalido",
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "unexpected payload type")
			},
		},
		{
			name: "deve retornar erro quando JSON do payload e invalido",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   outbox.Envelope{Payload: []byte("not-json")},
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "deserializar payload")
			},
		},
		{
			name: "deve retornar erro quando payload esta incompleto",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID: "user-uuid-123",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "payload incompleto")
			},
		},
		{
			name: "deve retornar erro quando HandleInbound falha",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "clima em São Paulo",
						MessageID: "wamid-002",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{}, errors.New("runtime falhou")).Once()
					return m
				}(),
				senderMock: &mockWhatsAppSender{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "handle inbound")
			},
		},
		{
			name: "deve retornar nil sem enviar quando reply vazio",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "clima em São Paulo",
						MessageID: "wamid-003",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: &mockWhatsAppSender{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando gateway falha",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "clima em São Paulo",
						MessageID: "wamid-004",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Resposta do agente.",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "Resposta do agente.").
						Return(errors.New("gateway timeout")).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "send reply")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			consumer := NewWhatsAppInboundConsumer(
				scenario.dependencies.inboundMock,
				scenario.dependencies.senderMock,
				s.obs,
			)
			err := consumer.Handle(s.ctx, scenario.args.event)
			scenario.expect(err)
			scenario.dependencies.inboundMock.AssertExpectations(s.T())
			scenario.dependencies.senderMock.AssertExpectations(s.T())
		})
	}
}
