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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
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

type mockOnboardingResolver struct {
	mock.Mock
}

func (m *mockOnboardingResolver) Execute(ctx context.Context, userID, message string) (usecases.OnboardingResult, error) {
	args := m.Called(ctx, userID, message)
	return args.Get(0).(usecases.OnboardingResult), args.Error(1)
}

type mockDestructiveConfirmResolver struct {
	mock.Mock
}

func (m *mockDestructiveConfirmResolver) Continue(ctx context.Context, userID, message string) (bool, string, error) {
	args := m.Called(ctx, userID, message)
	return args.Bool(0), args.String(1), args.Error(2)
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
		inboundMock     *mockHandleInbound
		senderMock      *mockWhatsAppSender
		onboardingMock  *mockOnboardingResolver
		destructiveMock *mockDestructiveConfirmResolver
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve processar mensagem com sucesso e enviar resposta via agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "gastei 50 reais no mercado",
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
						AgentID:    mecontrolaAgentID,
						Message:    "gastei 50 reais no mercado",
						MessageID:  "wamid-001",
					}).Return(agent.Outcome{
						RunID:   uuid.New(),
						Content: "✅ Registrei sua despesa de R$ 50,00.",
						Status:  agent.RunStatusSucceeded,
					}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "✅ Registrei sua despesa de R$ 50,00.").
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
						Text:      "quanto gastei esse mes",
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
						Text:      "oi",
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
						Text:      "quanto gastei",
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
		{
			name: "deve rotear para onboarding quando resolver retorna handled",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-new-456",
						Peer:      "+5511888888888",
						Text:      "oi",
						MessageID: "wamid-005",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511888888888", "🎯 Bem-vindo ao MeControla!").
						Return(nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-new-456", "oi").
						Return(usecases.OnboardingResult{Handled: true, Message: "🎯 Bem-vindo ao MeControla!"}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rotear para agente quando onboarding retorna nao handled",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-done-789",
						Peer:      "+5511777777777",
						Text:      "quanto gastei esse mes",
						MessageID: "wamid-006",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							Content: "📊 Você gastou R$ 1.500,00 em junho.",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511777777777", "📊 Você gastou R$ 1.500,00 em junho.").
						Return(nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-done-789", "quanto gastei esse mes").
						Return(usecases.OnboardingResult{Handled: false}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando onboarding resolver falha",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-err-999",
						Peer:      "+5511666666666",
						Text:      "oi",
						MessageID: "wamid-007",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-err-999", "oi").
						Return(usecases.OnboardingResult{}, errors.New("wm unavailable")).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "onboarding")
			},
		},
		{
			name: "deve rotear para confirmacao destrutiva quando pendente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-confirm-111",
						Peer:      "+5511555555555",
						Text:      "sim",
						MessageID: "wamid-008",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511555555555", "✅ Lançamento excluído com sucesso.").
						Return(nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-confirm-111", "sim").
						Return(true, "✅ Lançamento excluído com sucesso.", nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve continuar para onboarding quando confirmacao destrutiva nao pendente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-noconfirm-222",
						Peer:      "+5511444444444",
						Text:      "oi",
						MessageID: "wamid-009",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511444444444", "🎯 Vamos começar!").
						Return(nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-noconfirm-222", "oi").
						Return(false, "", nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-noconfirm-222", "oi").
						Return(usecases.OnboardingResult{Handled: true, Message: "🎯 Vamos começar!"}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando confirmacao destrutiva falha",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-dcerr-333",
						Peer:      "+5511333333333",
						Text:      "sim",
						MessageID: "wamid-010",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-dcerr-333", "sim").
						Return(false, "", errors.New("engine falhou")).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "confirmacao destrutiva")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			opts := []ConsumerOption{}
			if scenario.dependencies.onboardingMock != nil {
				opts = append(opts, WithOnboardingResolver(scenario.dependencies.onboardingMock))
			}
			if scenario.dependencies.destructiveMock != nil {
				opts = append(opts, WithDestructiveConfirmResolver(scenario.dependencies.destructiveMock))
			}
			consumer := NewWhatsAppInboundConsumer(
				scenario.dependencies.inboundMock,
				scenario.dependencies.senderMock,
				s.obs,
				opts...,
			)
			err := consumer.Handle(s.ctx, scenario.args.event)
			scenario.expect(err)
			scenario.dependencies.inboundMock.AssertExpectations(s.T())
			scenario.dependencies.senderMock.AssertExpectations(s.T())
			if scenario.dependencies.onboardingMock != nil {
				scenario.dependencies.onboardingMock.AssertExpectations(s.T())
			}
			if scenario.dependencies.destructiveMock != nil {
				scenario.dependencies.destructiveMock.AssertExpectations(s.T())
			}
		})
	}
}
