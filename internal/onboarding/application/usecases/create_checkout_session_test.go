package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
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
	tokenRepo *mocks.MagicTokenRepository
	factory   *mocks.RepositoryFactory
	builder   *mocks.CheckoutURLBuilder
	cipher    *mocks.TokenCipher
}

func TestCreateCheckoutSessionSuite(t *testing.T) {
	suite.Run(t, new(CreateCheckoutSessionSuite))
}

func (s *CreateCheckoutSessionSuite) SetupTest() {
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.builder = mocks.NewCheckoutURLBuilder(s.T())
	s.cipher = mocks.NewTokenCipher(s.T())
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
}

func (s *CreateCheckoutSessionSuite) TestExecute() {
	scenarios := []struct {
		name   string
		setup  func()
		expect func(out output.CreateCheckoutSessionOutput, err error)
	}{
		{
			name: "deve persistir token criptografado para outreach",
			setup: func() {
				s.tokenRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(t entities.MagicToken) bool {
					return t.ActivationTokenCiphertext() != ""
				})).Return(nil).Once()
				s.cipher.EXPECT().Encrypt(mock.Anything, mock.Anything).Return("cipher:token", nil).Once()
				s.builder.EXPECT().Build(mock.Anything, mock.Anything, mock.Anything).Return("https://pay.kiwify.com.br/checkout?sck=token", nil).Once()
			},
			expect: func(out output.CreateCheckoutSessionOutput, err error) {
				s.NoError(err)
				s.NotEmpty(out.CheckoutURL)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := usecases.NewCreateCheckoutSession(
				&unitOfWorkCheckout{},
				s.factory,
				s.builder,
				s.cipher,
				id.NewUUIDGenerator(),
				7*24*time.Hour,
				noop.NewProvider(),
			)
			out, err := uc.Execute(context.Background(), input.CreateCheckoutSessionInput{PlanID: "plan-1"})
			scenario.expect(out, err)
		})
	}
}
