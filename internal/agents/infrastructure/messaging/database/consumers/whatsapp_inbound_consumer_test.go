package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const expectedWelcomeCombinedMessage = `🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar uma casa, meta de R$ 400.000,00")`

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

func (m *mockOnboardingResolver) Execute(ctx context.Context, userID, peer, message string) (usecases.OnboardingResult, error) {
	args := m.Called(ctx, userID, peer, message)
	return args.Get(0).(usecases.OnboardingResult), args.Error(1)
}

type mockDestructiveConfirmResolver struct {
	mock.Mock
}

func (m *mockDestructiveConfirmResolver) Continue(ctx context.Context, userID, message string) (bool, string, error) {
	args := m.Called(ctx, userID, message)
	return args.Bool(0), args.String(1), args.Error(2)
}

type mockPendingEntryContinuerResolver struct {
	mock.Mock
}

func (m *mockPendingEntryContinuerResolver) Continue(ctx context.Context, userID, peer, message, messageID string) (workflows.PendingEntryResult, error) {
	args := m.Called(ctx, userID, peer, message, messageID)
	return args.Get(0).(workflows.PendingEntryResult), args.Error(1)
}

type mockCardCreateResolver struct {
	mock.Mock
}

func (m *mockCardCreateResolver) Continue(ctx context.Context, resourceID, peer, message, messageID string) (bool, string, error) {
	args := m.Called(ctx, resourceID, peer, message, messageID)
	return args.Bool(0), args.String(1), args.Error(2)
}

type mockBudgetCreationResolver struct {
	mock.Mock
}

func (m *mockBudgetCreationResolver) Continue(ctx context.Context, resourceID, text, messageID string) (bool, string, error) {
	args := m.Called(ctx, resourceID, text, messageID)
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
			name: "deve enviar fallback honesto quando reply vazio e nunca chamar gateway com vazio",
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
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", fallbackReply).
						Return(nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve suprimir confirmacao alucinada quando run falhou e enviar fallback honesto",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "gastei 150 no mercado",
						MessageID: "wamid-halluc-001",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Despesa registrada com sucesso ✅ R$150,00",
							Status:  agent.RunStatusFailed,
							Outcome: agent.ToolOutcomeUsecaseError,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", fallbackReply).
						Return(nil).Once()
					return m
				}(),
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
			name: "deve rotear para onboarding e enviar primeira mensagem combinada em unica resposta",
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
					m.On("SendTextMessage", mock.Anything, "+5511888888888", mock.MatchedBy(func(text string) bool {
						return strings.Contains(text, "🎉 Bem-vindo ao MeControla! 🎉") &&
							strings.Contains(text, "Vamos começar?") &&
							strings.Contains(text, "objetivo financeiro")
					})).Return(nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-new-456", "+5511888888888", "oi").
						Return(usecases.OnboardingResult{
							Handled: true,
							Message: expectedWelcomeCombinedMessage,
						}, nil).Once()
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
					m.On("Execute", mock.Anything, "user-done-789", "+5511777777777", "quanto gastei esse mes").
						Return(usecases.OnboardingResult{Handled: false}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve normalizar resposta do agente para whatsapp antes de enviar",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-format-321",
						Peer:      "+5511222222222",
						Text:      "ative meu orçamento",
						MessageID: "wamid-010",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							Content: "### Resumo de Onboarding\n\n- **Custo Fixo**: R$2.400,00\n\nVocê confirma que deseja ativar este orçamento?",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511222222222", "### 📊 Resumo de Onboarding\n\n- *Custo Fixo*: R$2.400,00\n\n✅ Você confirma que deseja ativar este orçamento?").
						Return(nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-format-321", "+5511222222222", "ative meu orçamento").
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
					m.On("Execute", mock.Anything, "user-err-999", "+5511666666666", "oi").
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
					m.On("Execute", mock.Anything, "user-noconfirm-222", "+5511444444444", "oi").
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

func (s *WhatsAppInboundConsumerSuite) TestHandleInboundTimeoutCancelsLLMCall() {
	blockingInbound := &blockingHandleInbound{}
	senderMock := &mockWhatsAppSender{}

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildEnvelope(whatsAppInboundPayload{
			UserID:    "user-timeout-001",
			Peer:      "+5511111111111",
			Text:      "quanto gastei?",
			MessageID: "wamid-timeout-001",
		}),
	}

	consumer := NewWhatsAppInboundConsumer(
		blockingInbound,
		senderMock,
		s.obs,
		WithInboundTimeout(5*time.Millisecond),
	)

	err := consumer.Handle(s.ctx, event)
	s.Error(err)
	s.Contains(err.Error(), "handle inbound")
	s.True(errors.Is(err, context.DeadlineExceeded))
}

type blockingHandleInbound struct{}

func (b *blockingHandleInbound) Execute(ctx context.Context, _ input.InboundInput) (agent.Outcome, error) {
	<-ctx.Done()
	return agent.Outcome{}, ctx.Err()
}

func (s *WhatsAppInboundConsumerSuite) TestHandle_RestoresTraceparentFromMetadata() {
	gotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))
	defer gotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	const traceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}

	inboundMock.On("Execute", mock.Anything, mock.AnythingOfType("input.InboundInput")).
		Return(agent.Outcome{Content: "ok", Status: agent.RunStatusSucceeded}, nil).Once()
	senderMock.On("SendTextMessage", mock.Anything, "+5511999999999", "ok").
		Return(nil).Once()

	env := outbox.Envelope{
		ID:       uuid.NewString(),
		Metadata: map[string]string{"traceparent": traceParent},
	}
	payload, _ := json.Marshal(whatsAppInboundPayload{
		UserID: "user-trace-001", Peer: "+5511999999999", Text: "olá", MessageID: "wamid-trace-001",
	})
	env.Payload = payload

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs)
	err := consumer.Handle(s.ctx, &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload:   env,
	})
	s.NoError(err)
	inboundMock.AssertExpectations(s.T())
	senderMock.AssertExpectations(s.T())
}

func (s *WhatsAppInboundConsumerSuite) TestPendingEntryOrdering() {
	type args struct {
		event *mockEvent
	}
	type dependencies struct {
		inboundMock     *mockHandleInbound
		senderMock      *mockWhatsAppSender
		pendingMock     *mockPendingEntryContinuerResolver
		destructiveMock *mockDestructiveConfirmResolver
		onboardingMock  *mockOnboardingResolver
	}

	validPayload := buildEnvelope(whatsAppInboundPayload{
		UserID:    "user-ord-001",
		Peer:      "+5511999999999",
		Text:      "sim confirmo",
		MessageID: "wamid-ord-001",
	})

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "pendencia handled=true encerra sem chamar destructive nem agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "Despesa registrada ✅").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-ord-001", "+5511999999999", "sim confirmo", "wamid-ord-001").
						Return(workflows.PendingEntryResult{
							Handled: true,
							Message: "Despesa registrada ✅",
							Mode:    workflows.PendingEntryModeCompleted,
						}, nil).Once()
					return m
				}(),
				destructiveMock: &mockDestructiveConfirmResolver{},
				onboardingMock:  &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "pendencia replaced handled=false passa para agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Registrei nova despesa.",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "Registrei nova despesa.").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-ord-001", "+5511999999999", "sim confirmo", "wamid-ord-001").
						Return(workflows.PendingEntryResult{
							Handled: false,
							Mode:    workflows.PendingEntryModeReplaced,
						}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-ord-001", "sim confirmo").
						Return(false, "", nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-ord-001", "+5511999999999", "sim confirmo").
						Return(usecases.OnboardingResult{Handled: false}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "sem pendencia ativa passa para destructive e agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Ok!",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "Ok!").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-ord-001", "+5511999999999", "sim confirmo", "wamid-ord-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-ord-001", "sim confirmo").
						Return(false, "", nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-ord-001", "+5511999999999", "sim confirmo").
						Return(usecases.OnboardingResult{Handled: false}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "erro no continuer de pendencia retorna erro sem chamar agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-ord-001", "+5511999999999", "sim confirmo", "wamid-ord-001").
						Return(workflows.PendingEntryResult{}, errors.New("engine falhou")).Once()
					return m
				}(),
				destructiveMock: &mockDestructiveConfirmResolver{},
				onboardingMock:  &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "pendencia de lancamento")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			opts := []ConsumerOption{}
			if scenario.dependencies.pendingMock != nil {
				opts = append(opts, WithPendingEntryContinuer(scenario.dependencies.pendingMock))
			}
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
		})
	}
}

func (s *WhatsAppInboundConsumerSuite) TestCardCreateOrdering() {
	type args struct {
		event *mockEvent
	}
	type dependencies struct {
		inboundMock     *mockHandleInbound
		senderMock      *mockWhatsAppSender
		pendingMock     *mockPendingEntryContinuerResolver
		destructiveMock *mockDestructiveConfirmResolver
		cardCreateMock  *mockCardCreateResolver
		onboardingMock  *mockOnboardingResolver
	}

	validPayload := buildEnvelope(whatsAppInboundPayload{
		UserID:    "user-card-001",
		Peer:      "+5511988888888",
		Text:      "sim",
		MessageID: "wamid-card-001",
	})

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "card-create handled=true responde e nao chama onboarding nem agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511988888888", "✅ Cartão *Nubank* cadastrado com sucesso.").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(true, "✅ Cartão *Nubank* cadastrado com sucesso.", nil).Once()
					return m
				}(),
				onboardingMock: &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "destructive suspenso consome mensagem e nao inicia card-create (exclusao mutua RF-18)",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511988888888", "Cartão excluído.").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "sim").
						Return(true, "Cartão excluído.", nil).Once()
					return m
				}(),
				cardCreateMock: &mockCardCreateResolver{},
				onboardingMock: &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "sem pendencias ativas passa para onboarding e agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Ok!",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511988888888", "Ok!").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(false, "", nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-card-001", "+5511988888888", "sim").
						Return(usecases.OnboardingResult{Handled: false}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "erro no continuer de card-create retorna erro sem chamar onboarding",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-card-001", "+5511988888888", "sim", "wamid-card-001").
						Return(false, "", errors.New("engine falhou")).Once()
					return m
				}(),
				onboardingMock: &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "confirmacao de cadastro de cartao")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			opts := []ConsumerOption{}
			if scenario.dependencies.pendingMock != nil {
				opts = append(opts, WithPendingEntryContinuer(scenario.dependencies.pendingMock))
			}
			if scenario.dependencies.destructiveMock != nil {
				opts = append(opts, WithDestructiveConfirmResolver(scenario.dependencies.destructiveMock))
			}
			if scenario.dependencies.cardCreateMock != nil {
				opts = append(opts, WithCardCreateResolver(scenario.dependencies.cardCreateMock))
			}
			if scenario.dependencies.onboardingMock != nil {
				opts = append(opts, WithOnboardingResolver(scenario.dependencies.onboardingMock))
			}
			consumer := NewWhatsAppInboundConsumer(
				scenario.dependencies.inboundMock,
				scenario.dependencies.senderMock,
				s.obs,
				opts...,
			)
			err := consumer.Handle(s.ctx, scenario.args.event)
			scenario.expect(err)
		})
	}
}

func (s *WhatsAppInboundConsumerSuite) TestBudgetCreationOrdering() {
	type args struct {
		event *mockEvent
	}
	type dependencies struct {
		inboundMock        *mockHandleInbound
		senderMock         *mockWhatsAppSender
		pendingMock        *mockPendingEntryContinuerResolver
		destructiveMock    *mockDestructiveConfirmResolver
		cardCreateMock     *mockCardCreateResolver
		budgetCreationMock *mockBudgetCreationResolver
		onboardingMock     *mockOnboardingResolver
	}

	validPayload := buildEnvelope(whatsAppInboundPayload{
		UserID:    "user-budget-001",
		Peer:      "+5511977777777",
		Text:      "sim",
		MessageID: "wamid-budget-001",
	})

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "budget-creation handled=true responde e nao chama onboarding nem agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511977777777", "🎉 Orçamento de junho de 2026 criado e ativado com sucesso!").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(false, "", nil).Once()
					return m
				}(),
				budgetCreationMock: func() *mockBudgetCreationResolver {
					m := &mockBudgetCreationResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim", "wamid-budget-001").
						Return(true, "🎉 Orçamento de junho de 2026 criado e ativado com sucesso!", nil).Once()
					return m
				}(),
				onboardingMock: &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "card-create suspenso consome mensagem e nao chama budget-creation (exclusao mutua)",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511977777777", "✅ Cartão cadastrado.").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(true, "✅ Cartão cadastrado.", nil).Once()
					return m
				}(),
				budgetCreationMock: &mockBudgetCreationResolver{},
				onboardingMock:     &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "falha de persistencia devolve mensagem especifica distinta do fallback generico",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511977777777", "Não consegui criar o orçamento. Tente novamente em breve.").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(false, "", nil).Once()
					return m
				}(),
				budgetCreationMock: func() *mockBudgetCreationResolver {
					m := &mockBudgetCreationResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim", "wamid-budget-001").
						Return(true, "Não consegui criar o orçamento. Tente novamente em breve.", nil).Once()
					return m
				}(),
				onboardingMock: &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "sem pendencias ativas passa para onboarding e agente",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Ok!",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511977777777", "Ok!").
						Return(nil).Once()
					return m
				}(),
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(false, "", nil).Once()
					return m
				}(),
				budgetCreationMock: func() *mockBudgetCreationResolver {
					m := &mockBudgetCreationResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim", "wamid-budget-001").
						Return(false, "", nil).Once()
					return m
				}(),
				onboardingMock: func() *mockOnboardingResolver {
					m := &mockOnboardingResolver{}
					m.On("Execute", mock.Anything, "user-budget-001", "+5511977777777", "sim").
						Return(usecases.OnboardingResult{Handled: false}, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "erro no continuer de budget-creation retorna erro sem chamar onboarding",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload:   validPayload,
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				pendingMock: func() *mockPendingEntryContinuerResolver {
					m := &mockPendingEntryContinuerResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(workflows.PendingEntryResult{Handled: false}, nil).Once()
					return m
				}(),
				destructiveMock: func() *mockDestructiveConfirmResolver {
					m := &mockDestructiveConfirmResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim").
						Return(false, "", nil).Once()
					return m
				}(),
				cardCreateMock: func() *mockCardCreateResolver {
					m := &mockCardCreateResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "+5511977777777", "sim", "wamid-budget-001").
						Return(false, "", nil).Once()
					return m
				}(),
				budgetCreationMock: func() *mockBudgetCreationResolver {
					m := &mockBudgetCreationResolver{}
					m.On("Continue", mock.Anything, "user-budget-001", "sim", "wamid-budget-001").
						Return(false, "", errors.New("infra indisponivel")).Once()
					return m
				}(),
				onboardingMock: &mockOnboardingResolver{},
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "criacao de orcamento")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			opts := []ConsumerOption{}
			if scenario.dependencies.pendingMock != nil {
				opts = append(opts, WithPendingEntryContinuer(scenario.dependencies.pendingMock))
			}
			if scenario.dependencies.destructiveMock != nil {
				opts = append(opts, WithDestructiveConfirmResolver(scenario.dependencies.destructiveMock))
			}
			if scenario.dependencies.cardCreateMock != nil {
				opts = append(opts, WithCardCreateResolver(scenario.dependencies.cardCreateMock))
			}
			if scenario.dependencies.budgetCreationMock != nil {
				opts = append(opts, WithBudgetCreationResolver(scenario.dependencies.budgetCreationMock))
			}
			if scenario.dependencies.onboardingMock != nil {
				opts = append(opts, WithOnboardingResolver(scenario.dependencies.onboardingMock))
			}
			consumer := NewWhatsAppInboundConsumer(
				scenario.dependencies.inboundMock,
				scenario.dependencies.senderMock,
				s.obs,
				opts...,
			)
			err := consumer.Handle(s.ctx, scenario.args.event)
			scenario.expect(err)
		})
	}
}
