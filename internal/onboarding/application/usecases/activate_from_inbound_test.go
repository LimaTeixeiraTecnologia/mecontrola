package usecases

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type testTokenActivator struct {
	mock.Mock
}

func (m *testTokenActivator) Execute(ctx context.Context, in input.ConsumeMagicTokenInput) (ConsumeResult, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(ConsumeResult), args.Error(1)
}

func newTestTokenActivator(t *testing.T) *testTokenActivator {
	m := &testTokenActivator{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

type unitOfWorkActivate struct{}

func (u *unitOfWorkActivate) DBTX() database.DBTX { return nil }

func (u *unitOfWorkActivate) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func validTokenStr() string {
	b := make([]byte, 32)
	return base64.RawURLEncoding.EncodeToString(b)
}

func buildActivablePaidToken(fromE164 string) entities.MagicToken {
	hash := []byte("hash-activable-paid-1234567890ab")
	return entities.HydrateMagicToken(
		"tok-activable", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-1*time.Hour), time.Time{}, time.Time{},
		"cipher", "sub-1", fromE164, "user@example.com", "sale-1",
		"", "", valueobjects.ActivationPath(0),
	)
}

type ActivateFromInboundSuite struct {
	suite.Suite
	ctx          context.Context
	obs          observability.Observability
	tokenRepo    *mocks.MagicTokenRepository
	factory      *mocks.RepositoryFactory
	identityGW   *mocks.IdentityGateway
	binder       *mocks.SubscriptionBinder
	publisher    *outboxmocks.Publisher
	throttle     *mocks.NoMatchThrottle
	gateway      *mocks.WhatsAppGateway
	consumeToken *testTokenActivator
}

func TestActivateFromInbound(t *testing.T) {
	suite.Run(t, new(ActivateFromInboundSuite))
}

func (s *ActivateFromInboundSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.identityGW = mocks.NewIdentityGateway(s.T())
	s.binder = mocks.NewSubscriptionBinder(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.throttle = mocks.NewNoMatchThrottle(s.T())
	s.gateway = mocks.NewWhatsAppGateway(s.T())
	s.consumeToken = newTestTokenActivator(s.T())
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
}

func (s *ActivateFromInboundSuite) TestValidate() {
	type args struct {
		in *input.ActivateFromInboundInput
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name: "deve retornar erro quando peer_e164 vazio",
			args: args{in: &input.ActivateFromInboundInput{PeerE164: "", Text: "Oi", MessageID: "wamid-1"}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, input.ErrPeerE164Required)
			},
		},
		{
			name: "deve retornar erro quando message_id vazio",
			args: args{in: &input.ActivateFromInboundInput{PeerE164: "+5511999990001", Text: "Oi", MessageID: ""}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, input.ErrMessageIDRequired)
			},
		},
		{
			name: "deve retornar ambos os erros quando peer e message_id vazios",
			args: args{in: &input.ActivateFromInboundInput{PeerE164: "", Text: "Oi", MessageID: ""}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, input.ErrPeerE164Required)
				s.ErrorIs(err, input.ErrMessageIDRequired)
			},
		},
		{
			name: "deve retornar nil quando valido",
			args: args{in: &input.ActivateFromInboundInput{PeerE164: "+5511999990001", Text: "Oi", MessageID: "wamid-1"}},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := scenario.args.in.Validate()
			scenario.expect(err)
		})
	}
}

func (s *ActivateFromInboundSuite) TestExecute() {
	type dependencies struct {
		tokenRepo    *mocks.MagicTokenRepository
		factory      *mocks.RepositoryFactory
		identityGW   *mocks.IdentityGateway
		binder       *mocks.SubscriptionBinder
		publisher    *outboxmocks.Publisher
		throttle     *mocks.NoMatchThrottle
		gateway      *mocks.WhatsAppGateway
		consumeToken *testTokenActivator
		in           input.ActivateFromInboundInput
	}

	const peer = "+5511999990001"
	const msgID = "wamid-test-001"
	validToken := validTokenStr()

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result ActivateFromInboundResult, err error)
	}{
		{
			name: "deve retornar PhoneMatched quando token encontrado por telefone",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindActivableByMobile(mock.Anything, peer, mock.Anything).Return(buildActivablePaidToken(peer), nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkActivationStartedAt(mock.Anything, "tok-activable", mock.Anything).Return(nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkConsumed(mock.Anything, mock.Anything).Return(nil).Once()
					return s.tokenRepo
				}(),
				factory: s.factory,
				identityGW: func() *mocks.IdentityGateway {
					s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, peer, "user@example.com").Return(interfaces.UpsertUserResult{UserID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}, nil).Once()
					return s.identityGW
				}(),
				binder: func() *mocks.SubscriptionBinder {
					s.binder.EXPECT().BindUser(mock.Anything, "sub-1", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa").Return(nil).Once()
					return s.binder
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
				throttle:     s.throttle,
				gateway:      s.gateway,
				consumeToken: s.consumeToken,
				in:           input.ActivateFromInboundInput{PeerE164: peer, Text: "Oi", MessageID: msgID},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.NoError(err)
				s.Equal(ActivateOutcomePhoneMatched, result.Outcome)
				s.Equal("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", result.UserID)
			},
		},
		{
			name: "deve retornar AlreadyActive quando corrida na ativacao por telefone",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindActivableByMobile(mock.Anything, peer, mock.Anything).Return(buildActivablePaidToken(peer), nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkActivationStartedAt(mock.Anything, "tok-activable", mock.Anything).Return(nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkConsumed(mock.Anything, mock.Anything).Return(domain.ErrTokenAlreadyConsumedRace).Once()
					return s.tokenRepo
				}(),
				factory: s.factory,
				identityGW: func() *mocks.IdentityGateway {
					s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, peer, "user@example.com").Return(interfaces.UpsertUserResult{UserID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}, nil).Once()
					return s.identityGW
				}(),
				binder: func() *mocks.SubscriptionBinder {
					s.binder.EXPECT().BindUser(mock.Anything, "sub-1", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb").Return(nil).Once()
					return s.binder
				}(),
				publisher:    s.publisher,
				throttle:     s.throttle,
				gateway:      s.gateway,
				consumeToken: s.consumeToken,
				in:           input.ActivateFromInboundInput{PeerE164: peer, Text: "Oi", MessageID: msgID},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.NoError(err)
				s.Equal(ActivateOutcomeAlreadyActive, result.Outcome)
			},
		},
		{
			name: "deve retornar TokenMatched quando texto contem token valido",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindActivableByMobile(mock.Anything, peer, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
					return s.tokenRepo
				}(),
				factory:    s.factory,
				identityGW: s.identityGW,
				binder:     s.binder,
				publisher:  s.publisher,
				throttle:   s.throttle,
				gateway:    s.gateway,
				consumeToken: func() *testTokenActivator {
					s.consumeToken.On("Execute", mock.Anything, mock.MatchedBy(func(in input.ConsumeMagicTokenInput) bool {
						return in.Token == validToken && in.FromE164 == peer && in.ActivationPath == valueobjects.ActivationPathDirect
					})).Return(ConsumeResult{Outcome: ConsumeOutcomeActivated, UserID: "user-uuid-3"}, nil).Once()
					return s.consumeToken
				}(),
				in: input.ActivateFromInboundInput{PeerE164: peer, Text: validToken, MessageID: msgID},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.NoError(err)
				s.Equal(ActivateOutcomeTokenMatched, result.Outcome)
				s.Equal("user-uuid-3", result.UserID)
			},
		},
		{
			name: "deve retornar AlreadyActive quando token ja consumido",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindActivableByMobile(mock.Anything, peer, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
					return s.tokenRepo
				}(),
				factory:    s.factory,
				identityGW: s.identityGW,
				binder:     s.binder,
				publisher:  s.publisher,
				throttle:   s.throttle,
				gateway:    s.gateway,
				consumeToken: func() *testTokenActivator {
					s.consumeToken.On("Execute", mock.Anything, mock.Anything).Return(ConsumeResult{Outcome: ConsumeOutcomeAlreadyActive}, nil).Once()
					return s.consumeToken
				}(),
				in: input.ActivateFromInboundInput{PeerE164: peer, Text: validToken, MessageID: msgID},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.NoError(err)
				s.Equal(ActivateOutcomeAlreadyActive, result.Outcome)
			},
		},
		{
			name: "deve retornar NoMatch e enviar mensagem quando throttle permite",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindActivableByMobile(mock.Anything, peer, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
					s.tokenRepo.EXPECT().HasConsumedByMobile(mock.Anything, peer).Return(false, nil).Once()
					return s.tokenRepo
				}(),
				factory:      s.factory,
				identityGW:   s.identityGW,
				binder:       s.binder,
				publisher:    s.publisher,
				consumeToken: s.consumeToken,
				throttle: func() *mocks.NoMatchThrottle {
					s.throttle.EXPECT().AllowReply(mock.Anything, peer, mock.Anything).Return(true, nil).Once()
					return s.throttle
				}(),
				gateway: func() *mocks.WhatsAppGateway {
					s.gateway.EXPECT().SendTextMessage(mock.Anything, peer, "Nao encontramos sua conta.").Return(nil).Once()
					return s.gateway
				}(),
				in: input.ActivateFromInboundInput{PeerE164: peer, Text: "Oi", MessageID: msgID},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.NoError(err)
				s.Equal(ActivateOutcomeNoMatch, result.Outcome)
			},
		},
		{
			name: "deve retornar NoMatch sem enviar mensagem quando throttle bloqueia",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindActivableByMobile(mock.Anything, peer, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
					s.tokenRepo.EXPECT().HasConsumedByMobile(mock.Anything, peer).Return(false, nil).Once()
					return s.tokenRepo
				}(),
				factory:      s.factory,
				identityGW:   s.identityGW,
				binder:       s.binder,
				publisher:    s.publisher,
				consumeToken: s.consumeToken,
				throttle: func() *mocks.NoMatchThrottle {
					s.throttle.EXPECT().AllowReply(mock.Anything, peer, mock.Anything).Return(false, nil).Once()
					return s.throttle
				}(),
				gateway: s.gateway,
				in:      input.ActivateFromInboundInput{PeerE164: peer, Text: "Oi", MessageID: msgID},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.NoError(err)
				s.Equal(ActivateOutcomeNoMatch, result.Outcome)
			},
		},
		{
			name: "deve retornar erro quando input invalido",
			dependencies: dependencies{
				tokenRepo:    s.tokenRepo,
				factory:      s.factory,
				identityGW:   s.identityGW,
				binder:       s.binder,
				publisher:    s.publisher,
				throttle:     s.throttle,
				gateway:      s.gateway,
				consumeToken: s.consumeToken,
				in:           input.ActivateFromInboundInput{PeerE164: "", Text: "Oi", MessageID: ""},
			},
			expect: func(result ActivateFromInboundResult, err error) {
				s.Error(err)
				s.ErrorIs(err, input.ErrPeerE164Required)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			idGen := id.NewUUIDGenerator()
			bind := binding.NewSubscriptionBindingService(scenario.dependencies.identityGW, scenario.dependencies.binder, services.NewMagicTokenWorkflow(), scenario.dependencies.publisher, idGen)
			uc := NewActivateFromInbound(
				&unitOfWorkActivate{},
				scenario.dependencies.factory,
				bind,
				scenario.dependencies.consumeToken,
				scenario.dependencies.gateway,
				scenario.dependencies.throttle,
				24*time.Hour,
				"Nao encontramos sua conta.",
				s.obs,
			)
			result, err := uc.Execute(s.ctx, scenario.dependencies.in)
			scenario.expect(result, err)
		})
	}
}
