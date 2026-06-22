package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MarkTokenPaidSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	tokenRepo *mocks.MagicTokenRepository
}

func TestMarkTokenPaidSuite(t *testing.T) {
	suite.Run(t, new(MarkTokenPaidSuite))
}

func (s *MarkTokenPaidSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
}

func (s *MarkTokenPaidSuite) TestExecute() {
	type dependencies struct {
		tokenRepo *mocks.MagicTokenRepository
		in        input.MarkTokenPaidInput
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(in input.MarkTokenPaidInput, err error)
	}{
		{
			name: "deve usar hash decodificado e armazenar subscription ID com sucesso",
			dependencies: func() dependencies {
				clearToken, _ := valueobjects.NewToken()
				expectedHash := clearToken.Hash()
				token, _ := entities.NewMagicToken("tok-id", expectedHash, "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				token, _ = token.WithActivationTokenCiphertext("cipher-token")
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, expectedHash).Return(token, nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkPaid(mock.Anything, mock.MatchedBy(func(t entities.MagicToken) bool {
					return t.SubscriptionID() == "sub-001"
				})).Return(nil).Once()
				return dependencies{
					tokenRepo: s.tokenRepo,
					in: input.MarkTokenPaidInput{
						SubscriptionID:     "sub-001",
						FunnelToken:        clearToken.ClearText(),
						CustomerMobileE164: "+5511999999999",
						CustomerEmail:      "user@example.com",
						ExternalSaleID:     "sale-001",
						PaidAt:             time.Now().UTC(),
					},
				}
			}(),
			expect: func(in input.MarkTokenPaidInput, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando token nao for encontrado",
			dependencies: func() dependencies {
				clearToken, _ := valueobjects.NewToken()
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
				return dependencies{
					tokenRepo: s.tokenRepo,
					in: input.MarkTokenPaidInput{
						SubscriptionID:     "sub-002",
						FunnelToken:        clearToken.ClearText(),
						CustomerMobileE164: "+5511999999999",
						CustomerEmail:      "user@example.com",
						ExternalSaleID:     "sale-002",
						PaidAt:             time.Now().UTC(),
					},
				}
			}(),
			expect: func(in input.MarkTokenPaidInput, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewMarkTokenPaid(scenario.dependencies.tokenRepo, services.NewMagicTokenWorkflow(), s.obs)
			err := uc.Execute(s.ctx, scenario.dependencies.in)
			scenario.expect(scenario.dependencies.in, err)
		})
	}
}
