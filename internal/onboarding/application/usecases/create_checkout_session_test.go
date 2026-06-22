package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type unitOfWorkCheckout struct{}

func (u *unitOfWorkCheckout) DBTX() database.DBTX { return nil }

func (u *unitOfWorkCheckout) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type CreateCheckoutSessionSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	tokenRepo *mocks.MagicTokenRepository
	factory   *mocks.RepositoryFactory
	builder   *mocks.CheckoutURLBuilder
	cipher    *mocks.TokenCipher
}

func TestCreateCheckoutSessionSuite(t *testing.T) {
	suite.Run(t, new(CreateCheckoutSessionSuite))
}

func (s *CreateCheckoutSessionSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.builder = mocks.NewCheckoutURLBuilder(s.T())
	s.cipher = mocks.NewTokenCipher(s.T())
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
}

func (s *CreateCheckoutSessionSuite) TestExecute() {
	type dependencies struct {
		tokenRepo *mocks.MagicTokenRepository
		factory   *mocks.RepositoryFactory
		builder   *mocks.CheckoutURLBuilder
		cipher    *mocks.TokenCipher
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(out output.CreateCheckoutSessionOutput, err error)
	}{
		{
			name: "deve persistir token criptografado para outreach",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(t entities.MagicToken) bool {
						return t.ActivationTokenCiphertext() != ""
					})).Return(nil).Once()
					return s.tokenRepo
				}(),
				factory: s.factory,
				builder: func() *mocks.CheckoutURLBuilder {
					s.builder.EXPECT().Build(mock.Anything, mock.Anything, mock.Anything).Return("https://pay.kiwify.com.br/checkout?sck=token", nil).Once()
					return s.builder
				}(),
				cipher: func() *mocks.TokenCipher {
					s.cipher.EXPECT().Encrypt(mock.Anything, mock.Anything).Return("cipher:token", nil).Once()
					return s.cipher
				}(),
			},
			expect: func(out output.CreateCheckoutSessionOutput, err error) {
				s.NoError(err)
				s.NotEmpty(out.CheckoutURL)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewCreateCheckoutSession(
				&unitOfWorkCheckout{},
				scenario.dependencies.factory,
				scenario.dependencies.builder,
				scenario.dependencies.cipher,
				id.NewUUIDGenerator(),
				7*24*time.Hour,
				s.obs,
			)
			out, err := uc.Execute(s.ctx, input.CreateCheckoutSessionInput{PlanID: "plan-1"})
			scenario.expect(out, err)
		})
	}
}
