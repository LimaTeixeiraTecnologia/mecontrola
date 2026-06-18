package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type GetTokenStateSuite struct {
	suite.Suite
	tokenRepo *mocks.MagicTokenRepository
}

func TestGetTokenStateSuite(t *testing.T) {
	suite.Run(t, new(GetTokenStateSuite))
}

func (s *GetTokenStateSuite) SetupTest() {
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
}

func (s *GetTokenStateSuite) TestExecute() {
	scenarios := []struct {
		name   string
		setup  func() string
		expect func(result usecases.GetTokenStateResult, err error)
	}{
		{
			name: "deve retornar ReadyToActivate true quando token pago e nao expirado",
			setup: func() string {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-1", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				paid, _ := mt.MarkPaid("sub-001", "+5511999999999", "u@test.com", "sale-1", time.Now().UTC())
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, tok.Hash()).Return(paid, nil).Once()
				return tok.ClearText()
			},
			expect: func(result usecases.GetTokenStateResult, err error) {
				s.NoError(err)
				s.True(result.Output.ReadyToActivate)
				s.NotEmpty(result.Output.WaMeURL)
				s.NotEmpty(result.Output.BotNumberDisplay)
			},
		},
		{
			name: "deve retornar ReadyToActivate false quando token PENDING",
			setup: func() string {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-2", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, tok.Hash()).Return(mt, nil).Once()
				return tok.ClearText()
			},
			expect: func(result usecases.GetTokenStateResult, err error) {
				s.NoError(err)
				s.False(result.Output.ReadyToActivate)
				s.Empty(result.Output.WaMeURL)
				s.Equal(usecases.TokenStateReasonPending, result.Reason)
			},
		},
		{
			name: "deve retornar ReadyToActivate false quando token expirado",
			setup: func() string {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-3", tok.Hash(), "plan-1", time.Now().UTC().Add(-1*time.Hour))
				paid, _ := mt.MarkPaid("sub-001", "+5511999999999", "u@test.com", "sale-1", time.Now().UTC().Add(-2*time.Hour))
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, tok.Hash()).Return(paid, nil).Once()
				return tok.ClearText()
			},
			expect: func(result usecases.GetTokenStateResult, err error) {
				s.NoError(err)
				s.False(result.Output.ReadyToActivate)
				s.Empty(result.Output.WaMeURL)
				s.Equal(usecases.TokenStateReasonExpired, result.Reason)
			},
		},
		{
			name: "deve retornar ReadyToActivate false quando token nao encontrado",
			setup: func() string {
				tok, _ := valueobjects.NewToken()
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, tok.Hash()).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()
				return tok.ClearText()
			},
			expect: func(result usecases.GetTokenStateResult, err error) {
				s.NoError(err)
				s.False(result.Output.ReadyToActivate)
				s.Equal(usecases.TokenStateReasonNotFound, result.Reason)
			},
		},
		{
			name: "deve incluir token no WaMeURL quando ReadyToActivate",
			setup: func() string {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-4", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				paid, _ := mt.MarkPaid("sub-001", "+5511999999999", "u@test.com", "sale-1", time.Now().UTC())
				s.tokenRepo.EXPECT().FindByHash(mock.Anything, tok.Hash()).Return(paid, nil).Once()
				return tok.ClearText()
			},
			expect: func(result usecases.GetTokenStateResult, err error) {
				s.NoError(err)
				s.True(result.Output.ReadyToActivate)
				s.Contains(result.Output.WaMeURL, result.Output.WaMeURL)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			token := scenario.setup()
			uc := usecases.NewGetTokenState(s.tokenRepo, "+5511999999999", "+55 11 9XXXX-XXXX", "mecontrola_bot", noop.NewProvider())
			result, err := uc.Execute(context.Background(), token)
			scenario.expect(result, err)
		})
	}
}
