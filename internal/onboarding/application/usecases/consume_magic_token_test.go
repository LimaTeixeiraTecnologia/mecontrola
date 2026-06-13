package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type unitOfWorkConsume struct{}

func (u *unitOfWorkConsume) Do(ctx context.Context, fn func(context.Context, database.DBTX) (usecases.ConsumeInternalResult, error), _ ...uow.Option) (usecases.ConsumeInternalResult, error) {
	return fn(ctx, nil)
}

func buildConsumePaidToken(fromE164, email string) entities.MagicToken {
	hash := []byte("hash-paid-tok-1234567890123456")
	return entities.HydrateMagicToken(
		"tok-paid-1", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-2*time.Hour),
		time.Now().UTC().Add(-1*time.Hour), time.Time{}, time.Time{},
		"cipher-token", "sub-001", fromE164, email, "sale-001",
		"", "", 0,
	)
}

func buildConsumedToken(consumedByE164 string) entities.MagicToken {
	hash := []byte("hash-consumed-tok-12345678901234")
	return entities.HydrateMagicToken(
		"tok-consumed-1", hash, valueobjects.TokenStatusConsumed,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-2*time.Hour), time.Now().UTC().Add(-1*time.Hour), time.Time{},
		"cipher-token", "sub-001", consumedByE164, "user@test.com", "sale-001",
		"user-id-1", consumedByE164, valueobjects.ActivationPathDirect,
	)
}

func buildPendingToken() entities.MagicToken {
	hash := []byte("hash-pending-tok-123456789012345")
	return entities.HydrateMagicToken(
		"tok-pending", hash, valueobjects.TokenStatusPending,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC(),
		time.Time{}, time.Time{}, time.Time{},
		"cipher-token", "", "", "", "",
		"", "", 0,
	)
}

func buildExpiredToken() entities.MagicToken {
	hash := []byte("hash-expired-tok-123456789012345")
	return entities.HydrateMagicToken(
		"tok-expired", hash, valueobjects.TokenStatusExpired,
		"plan-1", time.Now().UTC().Add(-24*time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		time.Time{}, time.Time{}, time.Time{},
		"cipher-token", "", "", "", "",
		"", "", 0,
	)
}

type ConsumeMagicTokenSuite struct {
	suite.Suite
	tokenRepo  *mocks.MagicTokenRepository
	signalRepo *mocks.SupportSignalRepository
	factory    *mocks.RepositoryFactory
	identityGW *mocks.IdentityGateway
	binder     *mocks.SubscriptionBinder
	publisher  *outboxmocks.Publisher
	events     []outbox.Event
}

func TestConsumeMagicToken(t *testing.T) {
	suite.Run(t, new(ConsumeMagicTokenSuite))
}

func (s *ConsumeMagicTokenSuite) SetupTest() {
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.signalRepo = mocks.NewSupportSignalRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.identityGW = mocks.NewIdentityGateway(s.T())
	s.binder = mocks.NewSubscriptionBinder(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.events = nil
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
	s.factory.EXPECT().SupportSignalRepository(mock.Anything).Return(s.signalRepo).Maybe()
}

func (s *ConsumeMagicTokenSuite) validInput(fromE164 string) input.ConsumeMagicTokenInput {
	return input.ConsumeMagicTokenInput{
		Token:          "validtokenstring123456789012345678901234567",
		FromE164:       fromE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	}
}

func (s *ConsumeMagicTokenSuite) TestExecute() {
	scenarios := []struct {
		name   string
		setup  func() input.ConsumeMagicTokenInput
		expect func(result usecases.ConsumeResult, err error)
	}{
		{
			name: "deve retornar NotFound quando token nao for encontrado",
			setup: func() input.ConsumeMagicTokenInput {
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
				return s.validInput("+5511999999999")
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeNotFound, result.Outcome)
			},
		},
		{
			name: "deve retornar NotFound quando formato do token for invalido",
			setup: func() input.ConsumeMagicTokenInput {
				return input.ConsumeMagicTokenInput{
					Token:          "!!invalid!!",
					FromE164:       "+5511999999999",
					ActivationPath: valueobjects.ActivationPathDirect,
				}
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeNotFound, result.Outcome)
			},
		},
		{
			name: "deve retornar NotYetPaid quando token PENDING",
			setup: func() input.ConsumeMagicTokenInput {
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildPendingToken(), nil).Once()
				return s.validInput("+5511999999999")
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeNotYetPaid, result.Outcome)
			},
		},
		{
			name: "deve retornar Expired quando token expirado",
			setup: func() input.ConsumeMagicTokenInput {
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildExpiredToken(), nil).Once()
				return s.validInput("+5511999999999")
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeExpired, result.Outcome)
			},
		},
		{
			name: "deve retornar AlreadyActive quando token ja consumido pelo mesmo numero",
			setup: func() input.ConsumeMagicTokenInput {
				fromE164 := "+5511999999999"
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildConsumedToken(fromE164), nil).Once()
				return s.validInput(fromE164)
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeAlreadyActive, result.Outcome)
			},
		},
		{
			name: "deve inserir signal quando token consumido por numero diferente",
			setup: func() input.ConsumeMagicTokenInput {
				consumedByE164 := "+5511999999999"
				fromE164 := "+5521888888888"
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildConsumedToken(consumedByE164), nil).Once()
				s.signalRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
				return input.ConsumeMagicTokenInput{
					Token:          "validtokenstring123456789012345678901234567",
					FromE164:       fromE164,
					ActivationPath: valueobjects.ActivationPathDirect,
				}
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeReuseOtherAccount, result.Outcome)
			},
		},
		{
			name: "deve ativar com sucesso quando token pago",
			setup: func() input.ConsumeMagicTokenInput {
				fromE164 := "+5511999999999"
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildConsumePaidToken(fromE164, "u@test.com"), nil).Once()
				s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, mock.Anything, mock.Anything).Return(interfaces.UpsertUserResult{UserID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}, nil).Once()
				s.binder.EXPECT().BindUser(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkConsumed(mock.Anything, mock.Anything).Return(nil).Once()
				s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
				return s.validInput(fromE164)
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeActivated, result.Outcome)
			},
		},
		{
			name: "deve vincular subscription e publicar subscription_id no evento",
			setup: func() input.ConsumeMagicTokenInput {
				fromE164 := "+5511999999999"
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildConsumePaidToken(fromE164, "u@test.com"), nil).Once()
				s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, mock.Anything, mock.Anything).Return(interfaces.UpsertUserResult{UserID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}, nil).Once()
				s.binder.EXPECT().BindUser(mock.Anything, "sub-001", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa").Return(nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkConsumed(mock.Anything, mock.Anything).Return(nil).Once()
				s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, event outbox.Event) error {
					s.events = append(s.events, event)
					return nil
				}).Once()
				return s.validInput(fromE164)
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.NoError(err)
				s.Equal(usecases.ConsumeOutcomeActivated, result.Outcome)
				s.Require().Len(s.events, 1)
				var payload map[string]any
				s.Require().NoError(json.Unmarshal(s.events[0].Payload, &payload))
				s.Equal("sub-001", payload["subscription_id"])
			},
		},
		{
			name: "deve retornar erro quando identity gateway falha",
			setup: func() input.ConsumeMagicTokenInput {
				fromE164 := "+5511999999999"
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(buildConsumePaidToken(fromE164, "u@test.com"), nil).Once()
				s.identityGW.EXPECT().UpsertUserByWhatsApp(mock.Anything, mock.Anything, mock.Anything).Return(interfaces.UpsertUserResult{}, errors.New("identity unavailable")).Once()
				return s.validInput(fromE164)
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro quando pais nao suportado",
			setup: func() input.ConsumeMagicTokenInput {
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(entities.MagicToken{}, domain.ErrUnsupportedCountry).Once()
				return s.validInput("+12125551234")
			},
			expect: func(result usecases.ConsumeResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			in := scenario.setup()
			idGen := id.NewUUIDGenerator()
			bind := binding.NewSubscriptionBindingService(s.identityGW, s.binder, services.NewMagicTokenWorkflow(), s.publisher, idGen)
			uc := usecases.NewConsumeMagicToken(&unitOfWorkConsume{}, s.factory, bind, idGen, noop.NewProvider())
			result, err := uc.Execute(context.Background(), in)
			scenario.expect(result, err)
		})
	}
}
