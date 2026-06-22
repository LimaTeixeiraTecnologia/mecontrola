package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type unitOfWorkFallback struct{}

func (u *unitOfWorkFallback) DBTX() database.DBTX { return nil }

func (u *unitOfWorkFallback) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func buildPaidTokenWithOutreach(fromE164 string) entities.MagicToken {
	hash := []byte("hash-paid-outreach-12345678901234")
	return entities.HydrateMagicToken(
		"tok-paid-outreach", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-2*time.Hour), time.Time{}, time.Now().UTC().Add(-30*time.Minute),
		"cipher-token", "sub-fallback", fromE164, "test@example.com", "sale-002",
		"", "", 0, "",
	)
}

func buildPaidTokenWithoutOutreach(fromE164 string) entities.MagicToken {
	hash := []byte("hash-paid-no-outreach-1234567890")
	return entities.HydrateMagicToken(
		"tok-paid-no-outreach", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-2*time.Hour), time.Time{}, time.Time{},
		"cipher-token", "sub-no-outreach", fromE164, "test@example.com", "sale-003",
		"", "", 0, "",
	)
}

func buildExpiredPaidTokenWithOutreach(fromE164 string) entities.MagicToken {
	hash := []byte("hash-expired-outreach-1234567890")
	return entities.HydrateMagicToken(
		"tok-expired-outreach", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		time.Now().UTC().Add(-5*24*time.Hour), time.Time{}, time.Now().UTC().Add(-4*24*time.Hour),
		"cipher-token", "sub-expired", fromE164, "test@example.com", "sale-004",
		"", "", 0, "",
	)
}

type TryFallbackActivationSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	tokenRepo  *mocks.MagicTokenRepository
	factory    *mocks.RepositoryFactory
	identityGW *mocks.IdentityGateway
	binder     *mocks.SubscriptionBinder
	publisher  *outboxmocks.Publisher
	signalRepo *mocks.SupportSignalRepository
}

func TestTryFallbackActivation(t *testing.T) {
	suite.Run(t, new(TryFallbackActivationSuite))
}

func (s *TryFallbackActivationSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.identityGW = mocks.NewIdentityGateway(s.T())
	s.binder = mocks.NewSubscriptionBinder(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.signalRepo = mocks.NewSupportSignalRepository(s.T())
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
	s.factory.EXPECT().SupportSignalRepository(mock.Anything).Return(s.signalRepo).Maybe()
}

func (s *TryFallbackActivationSuite) TestExecute() {
	type dependencies struct {
		tokenRepo  *mocks.MagicTokenRepository
		factory    *mocks.RepositoryFactory
		identityGW *mocks.IdentityGateway
		binder     *mocks.SubscriptionBinder
		publisher  *outboxmocks.Publisher
		fromE164   string
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result FallbackResult, err error)
	}{
		{
			name: "deve retornar NoMatch quando token nao for encontrado",
			dependencies: func() dependencies {
				fromE164 := "+5511999990001"
				s.tokenRepo.EXPECT().FindPaidByMobileForFallback(mock.Anything, fromE164).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
				return dependencies{tokenRepo: s.tokenRepo, factory: s.factory, identityGW: s.identityGW, binder: s.binder, publisher: s.publisher, fromE164: fromE164}
			}(),
			expect: func(result FallbackResult, err error) {
				s.NoError(err)
				s.Equal(FallbackOutcomeNoMatch, result.Outcome)
			},
		},
		{
			name: "deve retornar OutreachRequired quando token sem outreach",
			dependencies: func() dependencies {
				fromE164 := "+5511999990002"
				token := buildPaidTokenWithoutOutreach(fromE164)
				s.tokenRepo.EXPECT().FindPaidByMobileForFallback(mock.Anything, fromE164).Return(token, nil).Once()
				return dependencies{tokenRepo: s.tokenRepo, factory: s.factory, identityGW: s.identityGW, binder: s.binder, publisher: s.publisher, fromE164: fromE164}
			}(),
			expect: func(result FallbackResult, err error) {
				s.NoError(err)
				s.Equal(FallbackOutcomeOutreachRequired, result.Outcome)
			},
		},
		{
			name: "deve ativar quando token com outreach",
			dependencies: func() dependencies {
				fromE164 := "+5511999990003"
				token := buildPaidTokenWithOutreach(fromE164)
				s.tokenRepo.EXPECT().FindPaidByMobileForFallback(mock.Anything, fromE164).Return(token, nil).Once()
				s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, mock.Anything, mock.Anything).Return(interfaces.UpsertUserResult{UserID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}, nil).Once()
				s.binder.EXPECT().BindUser(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkConsumed(mock.Anything, mock.Anything).Return(nil).Once()
				s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
				return dependencies{tokenRepo: s.tokenRepo, factory: s.factory, identityGW: s.identityGW, binder: s.binder, publisher: s.publisher, fromE164: fromE164}
			}(),
			expect: func(result FallbackResult, err error) {
				s.NoError(err)
				s.Equal(FallbackOutcomeActivated, result.Outcome)
			},
		},
		{
			name: "deve retornar NoMatch quando token expirado",
			dependencies: func() dependencies {
				fromE164 := "+5511999990004"
				token := buildExpiredPaidTokenWithOutreach(fromE164)
				s.tokenRepo.EXPECT().FindPaidByMobileForFallback(mock.Anything, fromE164).Return(token, nil).Once()
				return dependencies{tokenRepo: s.tokenRepo, factory: s.factory, identityGW: s.identityGW, binder: s.binder, publisher: s.publisher, fromE164: fromE164}
			}(),
			expect: func(result FallbackResult, err error) {
				s.NoError(err)
				s.Equal(FallbackOutcomeNoMatch, result.Outcome)
			},
		},
		{
			name: "deve retornar erro quando identity gateway falha",
			dependencies: func() dependencies {
				fromE164 := "+5511999990005"
				token := buildPaidTokenWithOutreach(fromE164)
				s.tokenRepo.EXPECT().FindPaidByMobileForFallback(mock.Anything, fromE164).Return(token, nil).Once()
				s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, mock.Anything, mock.Anything).Return(interfaces.UpsertUserResult{}, errors.New("identity unavailable")).Once()
				return dependencies{tokenRepo: s.tokenRepo, factory: s.factory, identityGW: s.identityGW, binder: s.binder, publisher: s.publisher, fromE164: fromE164}
			}(),
			expect: func(result FallbackResult, err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro quando find falha",
			dependencies: func() dependencies {
				fromE164 := "+5511999990006"
				s.tokenRepo.EXPECT().FindPaidByMobileForFallback(mock.Anything, fromE164).Return(entities.MagicToken{}, errors.New("db error")).Once()
				return dependencies{tokenRepo: s.tokenRepo, factory: s.factory, identityGW: s.identityGW, binder: s.binder, publisher: s.publisher, fromE164: fromE164}
			}(),
			expect: func(result FallbackResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			idGen := id.NewUUIDGenerator()
			bind := binding.NewSubscriptionBindingService(scenario.dependencies.identityGW, scenario.dependencies.binder, services.NewMagicTokenWorkflow(), scenario.dependencies.publisher, idGen)
			uc := NewTryFallbackActivation(&unitOfWorkFallback{}, scenario.dependencies.factory, bind, s.obs)
			result, err := uc.Execute(s.ctx, scenario.dependencies.fromE164)
			scenario.expect(result, err)
		})
	}
}
