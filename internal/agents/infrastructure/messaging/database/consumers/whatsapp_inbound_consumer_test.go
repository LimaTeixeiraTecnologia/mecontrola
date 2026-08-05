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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const expectedWelcomeCombinedMessage = `🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar um celular novo, meta de R$ 5.000,00")`

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

type mockResumeDispatcher struct {
	mock.Mock
}

func (m *mockResumeDispatcher) Continue(ctx context.Context, resourceID, threadID, message, messageID string) (bool, string, error) {
	args := m.Called(ctx, resourceID, threadID, message, messageID)
	return args.Bool(0), args.String(1), args.Error(2)
}

type mockMessageDedup struct {
	mock.Mock
}

func (m *mockMessageDedup) InsertIfAbsent(ctx context.Context, consumer, messageID string) (bool, error) {
	args := m.Called(ctx, consumer, messageID)
	return args.Bool(0), args.Error(1)
}

func (m *mockMessageDedup) Delete(ctx context.Context, consumer, messageID string) error {
	args := m.Called(ctx, consumer, messageID)
	return args.Error(0)
}

type mockAudioProcessor struct {
	mock.Mock
}

func (m *mockAudioProcessor) Execute(ctx context.Context, in usecases.AudioInboundInput) (usecases.AudioInboundResult, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(usecases.AudioInboundResult), args.Error(1)
}

func (m *mockAudioProcessor) MarkDispatched(ctx context.Context, wamid string) error {
	args := m.Called(ctx, wamid)
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
		inboundMock    *mockHandleInbound
		senderMock     *mockWhatsAppSender
		onboardingMock *mockOnboardingResolver
		dispatcherMock *mockResumeDispatcher
		dedupMock      *mockMessageDedup
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
			name: "deve enviar fallback honesto quando run falhou sem mensagem segura de guard",
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
							Content: "",
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
			name: "deve propagar mensagem segura do guard quando run falhou com conteudo",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-uuid-123",
						Peer:      "+5511999999999",
						Text:      "gastei 150 no mercado",
						MessageID: "wamid-guard-001",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							RunID:   uuid.New(),
							Content: "Não consegui registrar. Tente novamente em breve.",
							Status:  agent.RunStatusFailed,
							Outcome: agent.ToolOutcomeUsecaseError,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "Não consegui registrar. Tente novamente em breve.").
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
							Content: "📊 Você gastou R$ 800,00.",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999999999", "📊 Você gastou R$ 800,00.").
						Return(errors.New("gateway indisponivel")).Once()
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
			name: "deve rotear para o dispatcher de resume quando ha run suspenso",
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
				dispatcherMock: func() *mockResumeDispatcher {
					m := &mockResumeDispatcher{}
					m.On("Continue", mock.Anything, "user-confirm-111", "+5511555555555", "sim", "wamid-008").
						Return(true, "✅ Lançamento excluído com sucesso.", nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve continuar para onboarding quando dispatcher nao tem run suspenso",
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
				dispatcherMock: func() *mockResumeDispatcher {
					m := &mockResumeDispatcher{}
					m.On("Continue", mock.Anything, "user-noconfirm-222", "+5511444444444", "oi", "wamid-009").
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
			name: "deve retornar erro quando dispatcher de resume falha",
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
				dispatcherMock: func() *mockResumeDispatcher {
					m := &mockResumeDispatcher{}
					m.On("Continue", mock.Anything, "user-dcerr-333", "+5511333333333", "sim", "wamid-010").
						Return(false, "", errors.New("engine falhou")).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "resume")
			},
		},
		{
			name: "deve entregar a mensagem de falha e ACK quando resume falha com reply ao usuario",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-wf-444",
						Peer:      "+5511222222222",
						Text:      "sim",
						MessageID: "wamid-011",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511222222222", "❌ Tive uma dificuldade para registrar. Pode tentar de novo?").
						Return(nil).Once()
					return m
				}(),
				dispatcherMock: func() *mockResumeDispatcher {
					m := &mockResumeDispatcher{}
					m.On("Continue", mock.Anything, "user-wf-444", "+5511222222222", "sim", "wamid-011").
						Return(true, "❌ Tive uma dificuldade para registrar. Pode tentar de novo?", errors.New("idempotent_write falhou")).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve ignorar mensagem duplicada quando dedup retorna inserted=false",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-dedup-001",
						Peer:      "+5511999990001",
						Text:      "gastei 30 no uber",
						MessageID: "wamid-dedup-001",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				dedupMock: func() *mockMessageDedup {
					m := &mockMessageDedup{}
					m.On("InsertIfAbsent", mock.Anything, whatsAppInboundConsumerName, "wamid-dedup-001").
						Return(false, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro quando InsertIfAbsent do dedup falha",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-dedup-002",
						Peer:      "+5511999990002",
						Text:      "quanto gastei",
						MessageID: "wamid-dedup-002",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: &mockHandleInbound{},
				senderMock:  &mockWhatsAppSender{},
				dedupMock: func() *mockMessageDedup {
					m := &mockMessageDedup{}
					m.On("InsertIfAbsent", mock.Anything, whatsAppInboundConsumerName, "wamid-dedup-002").
						Return(false, errors.New("db indisponivel")).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "dedup")
			},
		},
		{
			name: "deve compensar dedup com Delete quando processamento falha",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-dedup-003",
						Peer:      "+5511999990003",
						Text:      "quanto gastei",
						MessageID: "wamid-dedup-003",
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
				dedupMock: func() *mockMessageDedup {
					m := &mockMessageDedup{}
					m.On("InsertIfAbsent", mock.Anything, whatsAppInboundConsumerName, "wamid-dedup-003").
						Return(true, nil).Once()
					m.On("Delete", mock.Anything, whatsAppInboundConsumerName, "wamid-dedup-003").
						Return(nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "handle inbound")
			},
		},
		{
			name: "nao deve compensar dedup quando erro e de envio de reply",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID:    "user-dedup-004",
						Peer:      "+5511999990004",
						Text:      "quanto gastei",
						MessageID: "wamid-dedup-004",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							Content: "📊 Você gastou R$ 100,00.",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999990004", "📊 Você gastou R$ 100,00.").
						Return(errors.New("gateway indisponivel")).Once()
					return m
				}(),
				dedupMock: func() *mockMessageDedup {
					m := &mockMessageDedup{}
					m.On("InsertIfAbsent", mock.Anything, whatsAppInboundConsumerName, "wamid-dedup-004").
						Return(true, nil).Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.True(errors.Is(err, errSendReply))
			},
		},
		{
			name: "deve processar sem dedup quando MessageID vazio",
			args: args{
				event: &mockEvent{
					eventType: "agents.whatsapp.inbound.v1",
					payload: buildEnvelope(whatsAppInboundPayload{
						UserID: "user-dedup-005",
						Peer:   "+5511999990005",
						Text:   "quanto gastei",
					}),
				},
			},
			dependencies: dependencies{
				inboundMock: func() *mockHandleInbound {
					m := &mockHandleInbound{}
					m.On("Execute", mock.Anything, mock.Anything).
						Return(agent.Outcome{
							Content: "📊 Você gastou R$ 50,00.",
							Status:  agent.RunStatusSucceeded,
						}, nil).Once()
					return m
				}(),
				senderMock: func() *mockWhatsAppSender {
					m := &mockWhatsAppSender{}
					m.On("SendTextMessage", mock.Anything, "+5511999990005", "📊 Você gastou R$ 50,00.").
						Return(nil).Once()
					return m
				}(),
				dedupMock: &mockMessageDedup{},
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			opts := []ConsumerOption{}
			if scenario.dependencies.onboardingMock != nil {
				opts = append(opts, WithOnboardingResolver(scenario.dependencies.onboardingMock))
			}
			if scenario.dependencies.dispatcherMock != nil {
				opts = append(opts, WithResumeDispatcher(scenario.dependencies.dispatcherMock))
			}
			if scenario.dependencies.dedupMock != nil {
				opts = append(opts, WithMessageDedup(scenario.dependencies.dedupMock))
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
			if scenario.dependencies.dispatcherMock != nil {
				scenario.dependencies.dispatcherMock.AssertExpectations(s.T())
			}
			if scenario.dependencies.dedupMock != nil {
				scenario.dependencies.dedupMock.AssertExpectations(s.T())
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

func (s *WhatsAppInboundConsumerSuite) TestResumeDispatcherPrecedesOnboarding() {
	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildEnvelope(whatsAppInboundPayload{
			UserID:    "user-precedence-001",
			Peer:      "+5511000000001",
			Text:      "sim",
			MessageID: "wamid-precedence-001",
		}),
	}

	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511000000001", "✅ Cartão cadastrado.").
		Return(nil).Once()

	dispatcherMock := &mockResumeDispatcher{}
	dispatcherMock.On("Continue", mock.Anything, "user-precedence-001", "+5511000000001", "sim", "wamid-precedence-001").
		Return(true, "✅ Cartão cadastrado.", nil).Once()

	onboardingMock := &mockOnboardingResolver{}

	consumer := NewWhatsAppInboundConsumer(
		inboundMock,
		senderMock,
		s.obs,
		WithResumeDispatcher(dispatcherMock),
		WithOnboardingResolver(onboardingMock),
	)

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	dispatcherMock.AssertExpectations(s.T())
	onboardingMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	inboundMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func buildAudioEnvelope(p whatsAppInboundPayload) outbox.Envelope {
	p.MessageType = "audio"
	return buildEnvelope(p)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_TextRegressionUnaffectedByAudioWiring() {
	inboundMock := &mockHandleInbound{}
	inboundMock.On("Execute", mock.Anything, input.InboundInput{
		ResourceID: "user-text-regression",
		ThreadID:   "+5511900000001",
		AgentID:    mecontrolaAgentID,
		Message:    "gastei 20 no cafe",
		MessageID:  "wamid-text-regression",
	}).Return(agent.Outcome{Content: "✅ Registrado.", Status: agent.RunStatusSucceeded}, nil).Once()

	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511900000001", "✅ Registrado.").Return(nil).Once()

	audioMock := &mockAudioProcessor{}

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs, WithAudioProcessor(audioMock))

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildEnvelope(whatsAppInboundPayload{
			UserID:    "user-text-regression",
			Peer:      "+5511900000001",
			Text:      "gastei 20 no cafe",
			MessageID: "wamid-text-regression",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	inboundMock.AssertExpectations(s.T())
	senderMock.AssertExpectations(s.T())
	audioMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_ApprovedDispatchesToSameHandleInboundWithCanonicalText() {
	inboundMock := &mockHandleInbound{}
	inboundMock.On("Execute", mock.Anything, input.InboundInput{
		ResourceID: "user-audio-approved",
		ThreadID:   "+5511900000002",
		AgentID:    mecontrolaAgentID,
		Message:    "gastei 50 reais no mercado",
		MessageID:  "wamid-audio-approved",
	}).Return(agent.Outcome{Content: "✅ Registrei sua despesa.", Status: agent.RunStatusSucceeded}, nil).Once()

	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511900000002", "✅ Registrei sua despesa.").Return(nil).Once()

	audioMock := &mockAudioProcessor{}
	audioMock.On("Execute", mock.Anything, usecases.AudioInboundInput{
		WAMID: "wamid-audio-approved", UserID: "user-audio-approved", Peer: "+5511900000002",
		MediaID: "media-1", MimeType: "audio/ogg", SHA256: "sha-1", Voice: true,
	}).Return(usecases.AudioInboundResult{
		Outcome:       usecases.AudioOutcomeApproved,
		CanonicalText: "gastei 50 reais no mercado",
	}, nil).Once()
	audioMock.On("MarkDispatched", mock.Anything, "wamid-audio-approved").Return(nil).Once()

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs, WithAudioProcessor(audioMock))

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-approved", Peer: "+5511900000002", MessageID: "wamid-audio-approved",
			AudioMediaID: "media-1", AudioMimeType: "audio/ogg", AudioSHA256: "sha-1", AudioVoice: true,
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	inboundMock.AssertExpectations(s.T())
	senderMock.AssertExpectations(s.T())
	audioMock.AssertExpectations(s.T())
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_UncertainNeverCallsHandleInboundOrTools() {
	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511900000003", "pode reenviar o audio?").Return(nil).Once()

	audioMock := &mockAudioProcessor{}
	audioMock.On("Execute", mock.Anything, mock.Anything).Return(usecases.AudioInboundResult{
		Outcome:   usecases.AudioOutcomeTranscriptionUncertain,
		ReplyText: "pode reenviar o audio?",
	}, nil).Once()

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs, WithAudioProcessor(audioMock))

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-uncertain", Peer: "+5511900000003", MessageID: "wamid-audio-uncertain",
			AudioMediaID: "media-2", AudioMimeType: "audio/ogg", AudioSHA256: "sha-2",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	senderMock.AssertExpectations(s.T())
	audioMock.AssertExpectations(s.T())
	audioMock.AssertNotCalled(s.T(), "MarkDispatched", mock.Anything, mock.Anything)
	inboundMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_STTFailureNeverCallsHandleInbound() {
	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511900000004", "não consegui transcrever, tenta de novo?").Return(nil).Once()

	audioMock := &mockAudioProcessor{}
	audioMock.On("Execute", mock.Anything, mock.Anything).Return(usecases.AudioInboundResult{
		Outcome:   usecases.AudioOutcomeTranscriptionFailed,
		ReplyText: "não consegui transcrever, tenta de novo?",
	}, nil).Once()

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs, WithAudioProcessor(audioMock))

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-sttfail", Peer: "+5511900000004", MessageID: "wamid-audio-sttfail",
			AudioMediaID: "media-3", AudioMimeType: "audio/ogg", AudioSHA256: "sha-3",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	senderMock.AssertExpectations(s.T())
	audioMock.AssertExpectations(s.T())
	inboundMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_RejectedDownloadFailureNeverCallsHandleInbound() {
	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511900000005", "não consegui baixar esse audio, pode reenviar?").Return(nil).Once()

	audioMock := &mockAudioProcessor{}
	audioMock.On("Execute", mock.Anything, mock.Anything).Return(usecases.AudioInboundResult{
		Outcome:   usecases.AudioOutcomeRejected,
		ReplyText: "não consegui baixar esse audio, pode reenviar?",
	}, nil).Once()

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs, WithAudioProcessor(audioMock))

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-dlfail", Peer: "+5511900000005", MessageID: "wamid-audio-dlfail",
			AudioMediaID: "media-4", AudioMimeType: "audio/ogg", AudioSHA256: "sha-4",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	senderMock.AssertExpectations(s.T())
	audioMock.AssertExpectations(s.T())
	inboundMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_FeatureFlagOffSendsSafeReplyWithoutProcessor() {
	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}
	senderMock.On("SendTextMessage", mock.Anything, "+5511900000006", defaultAudioDisabledReply).Return(nil).Once()

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs)

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-disabled", Peer: "+5511900000006", MessageID: "wamid-audio-disabled",
			AudioMediaID: "media-5", AudioMimeType: "audio/ogg", AudioSHA256: "sha-5",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.NoError(err)
	senderMock.AssertExpectations(s.T())
	inboundMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_ProcessorInfraErrorPropagatesWithoutSendingReply() {
	inboundMock := &mockHandleInbound{}
	senderMock := &mockWhatsAppSender{}

	audioMock := &mockAudioProcessor{}
	audioMock.On("Execute", mock.Anything, mock.Anything).
		Return(usecases.AudioInboundResult{}, errors.New("audit repository indisponivel")).Once()

	consumer := NewWhatsAppInboundConsumer(inboundMock, senderMock, s.obs, WithAudioProcessor(audioMock))

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-infra", Peer: "+5511900000007", MessageID: "wamid-audio-infra",
			AudioMediaID: "media-6", AudioMimeType: "audio/ogg", AudioSHA256: "sha-6",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.Error(err)
	s.Contains(err.Error(), "process audio")
	senderMock.AssertNotCalled(s.T(), "SendTextMessage", mock.Anything, mock.Anything, mock.Anything)
	inboundMock.AssertNotCalled(s.T(), "Execute", mock.Anything, mock.Anything)
}

func (s *WhatsAppInboundConsumerSuite) TestHandleAudio_PayloadIncompletoQuandoCamposAudioAusentes() {
	consumer := NewWhatsAppInboundConsumer(&mockHandleInbound{}, &mockWhatsAppSender{}, s.obs)

	event := &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload: buildAudioEnvelope(whatsAppInboundPayload{
			UserID: "user-audio-incompleto", Peer: "+5511900000008", MessageID: "wamid-audio-incompleto",
		}),
	}

	err := consumer.Handle(s.ctx, event)
	s.Error(err)
	s.Contains(err.Error(), "payload de audio incompleto")
}
