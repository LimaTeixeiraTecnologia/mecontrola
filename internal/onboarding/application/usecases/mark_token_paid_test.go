package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	usecasesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MarkTokenPaidSuite struct {
	suite.Suite
	tokenRepo *mocks.MagicTokenRepository
	factory   *mocks.RepositoryFactory
	mgr       *usecasesmocks.FakeManager
}

func TestMarkTokenPaidSuite(t *testing.T) {
	suite.Run(t, new(MarkTokenPaidSuite))
}

func (s *MarkTokenPaidSuite) SetupTest() {
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.mgr = usecasesmocks.NewFakeManager()
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
}

func (s *MarkTokenPaidSuite) TestExecute() {
	scenarios := []struct {
		name   string
		setup  func() input.MarkTokenPaidInput
		expect func(in input.MarkTokenPaidInput, err error)
	}{
		{
			name: "deve usar hash decodificado e armazenar subscription ID com sucesso",
			setup: func() input.MarkTokenPaidInput {
				clearToken, _ := valueobjects.NewToken()
				expectedHash := clearToken.Hash()
				token, _ := entities.NewMagicToken("tok-id", expectedHash, "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				token, _ = token.WithActivationTokenCiphertext("cipher-token")
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, expectedHash).Return(token, nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkPaid(mock.Anything, mock.MatchedBy(func(t entities.MagicToken) bool {
					return t.SubscriptionID() == "sub-001"
				})).Return(nil).Once()
				return input.MarkTokenPaidInput{
					SubscriptionID:     "sub-001",
					FunnelToken:        clearToken.ClearText(),
					CustomerMobileE164: "+5511999999999",
					CustomerEmail:      "user@example.com",
					ExternalSaleID:     "sale-001",
					PaidAt:             time.Now().UTC(),
				}
			},
			expect: func(in input.MarkTokenPaidInput, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando token nao for encontrado",
			setup: func() input.MarkTokenPaidInput {
				clearToken, _ := valueobjects.NewToken()
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
				return input.MarkTokenPaidInput{
					SubscriptionID:     "sub-002",
					FunnelToken:        clearToken.ClearText(),
					CustomerMobileE164: "+5511999999999",
					CustomerEmail:      "user@example.com",
					ExternalSaleID:     "sale-002",
					PaidAt:             time.Now().UTC(),
				}
			},
			expect: func(in input.MarkTokenPaidInput, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			in := scenario.setup()
			uc := usecases.NewMarkTokenPaid(s.mgr, s.factory, noop.NewProvider())
			err := uc.Execute(context.Background(), in)
			scenario.expect(in, err)
		})
	}
}
