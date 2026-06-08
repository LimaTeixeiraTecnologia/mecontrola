package usecases_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	apperrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	usecasesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

func buildPaidToken(mobile string) entities.MagicToken {
	t, _ := entities.NewMagicToken("tok-id", []byte("hash"), "plan-id-1", time.Now().Add(7*24*time.Hour))
	t, _ = t.WithActivationTokenCiphertext("cipher-token")
	t, _ = t.MarkPaid("sub-001", mobile, "test@example.com", "sale-001", time.Now().Add(-3*time.Hour))
	return t
}

type SendOutreachSuite struct {
	suite.Suite
	tokenRepo *mocks.MagicTokenRepository
	factory   *mocks.RepositoryFactory
	gateway   *mocks.WhatsAppGateway
	cipher    *mocks.TokenCipher
	mgr       *usecasesmocks.FakeManager
}

func TestSendOutreach(t *testing.T) {
	suite.Run(t, new(SendOutreachSuite))
}

func (s *SendOutreachSuite) SetupTest() {
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.gateway = mocks.NewWhatsAppGateway(s.T())
	s.cipher = mocks.NewTokenCipher(s.T())
	s.mgr = usecasesmocks.NewFakeManager()
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
}

func (s *SendOutreachSuite) TestExecute() {
	scenarios := []struct {
		name   string
		setup  func()
		expect func(err error)
	}{
		{
			name: "deve enviar mensagem para todos candidatos com sucesso",
			setup: func() {
				mobile := "+5511999990001"
				token := buildPaidToken(mobile)
				s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
				s.gateway.EXPECT().SendActivationTemplate(mock.Anything, mobile, mock.Anything, "clear-token").Return("wamid.test", nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve completar sem erro quando nao ha candidatos",
			setup: func() {
				s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{}, nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve marcar outreach_sent_at e nao resetar quando gateway retorna erro 4xx",
			setup: func() {
				mobile := "+5511999990002"
				token := buildPaidToken(mobile)
				s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
				s.gateway.EXPECT().SendActivationTemplate(mock.Anything, mobile, mock.Anything, "clear-token").Return("", fmt.Errorf("gateway error: %w", apperrors.ErrWhatsAppClientError)).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve resetar outreach_sent_at quando gateway retorna erro 5xx",
			setup: func() {
				mobile := "+5511999990003"
				token := buildPaidToken(mobile)
				s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
				s.gateway.EXPECT().SendActivationTemplate(mock.Anything, mobile, mock.Anything, "clear-token").Return("", fmt.Errorf("gateway error: %w", apperrors.ErrWhatsAppServerError)).Once()
				s.tokenRepo.EXPECT().UpdateMarkOutreachReset(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando FindPaidForOutreach falha",
			setup: func() {
				s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db unavailable")).Once()
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve pular envio quando UpdateMarkOutreachSent falha",
			setup: func() {
				mobile1 := "+5511999990011"
				mobile2 := "+5511999990012"
				tokens := []entities.MagicToken{buildPaidToken(mobile1), buildPaidToken(mobile2)}
				s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return(tokens, nil).Once()
				s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db write failed")).Times(2)
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := usecases.NewSendOutreach(
				s.mgr,
				s.factory,
				s.gateway,
				s.cipher,
				id.NewUUIDGenerator(),
				"activation_reminder",
				2*time.Hour,
				noop.NewProvider(),
			)
			err := uc.Execute(context.Background())
			scenario.expect(err)
		})
	}
}
