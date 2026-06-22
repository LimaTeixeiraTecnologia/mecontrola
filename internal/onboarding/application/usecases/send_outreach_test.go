package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	apperrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func buildPaidToken(mobile string) entities.MagicToken {
	t, _ := entities.NewMagicToken("tok-id", []byte("hash"), "plan-id-1", time.Now().Add(7*24*time.Hour))
	t, _ = t.WithActivationTokenCiphertext("cipher-token")
	t, _ = t.MarkPaid("sub-001", mobile, "test@example.com", "sale-001", time.Now().Add(-3*time.Hour))
	return t
}

func buildPaidTokenTelegramOnly(telegramID string) entities.MagicToken {
	t, _ := entities.NewMagicToken("tok-id-tg", []byte("hash-tg"), "plan-id-1", time.Now().Add(7*24*time.Hour))
	t, _ = t.WithActivationTokenCiphertext("cipher-token")
	t, _ = t.MarkPaid("sub-002", "", "tg@example.com", "sale-002", time.Now().Add(-3*time.Hour))
	t, _ = t.LinkTelegramExternalID(telegramID)
	return t
}

func buildPaidTokenNoChannel() entities.MagicToken {
	t, _ := entities.NewMagicToken("tok-id-none", []byte("hash-none"), "plan-id-1", time.Now().Add(7*24*time.Hour))
	t, _ = t.WithActivationTokenCiphertext("cipher-token")
	t, _ = t.MarkPaid("sub-003", "", "none@example.com", "sale-003", time.Now().Add(-3*time.Hour))
	return t
}

type SendOutreachSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	tokenRepo *mocks.MagicTokenRepository
	gateway   *mocks.OutreachChannelGateway
	cipher    *mocks.TokenCipher
}

func TestSendOutreach(t *testing.T) {
	suite.Run(t, new(SendOutreachSuite))
}

func (s *SendOutreachSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.gateway = mocks.NewOutreachChannelGateway(s.T())
	s.cipher = mocks.NewTokenCipher(s.T())
}

func (s *SendOutreachSuite) TestExecute() {
	type dependencies struct {
		tokenRepo *mocks.MagicTokenRepository
		gateway   *mocks.OutreachChannelGateway
		cipher    *mocks.TokenCipher
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve enviar mensagem para todos candidatos com sucesso",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					mobile := "+5511999990001"
					token := buildPaidToken(mobile)
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
					return s.tokenRepo
				}(),
				gateway: func() *mocks.OutreachChannelGateway {
					mobile := "+5511999990001"
					s.gateway.EXPECT().SendActivationTemplate(mock.Anything, "whatsapp", mobile, mock.Anything, "clear-token").Return("wamid.test", nil).Once()
					return s.gateway
				}(),
				cipher: func() *mocks.TokenCipher {
					s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
					return s.cipher
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve completar sem erro quando nao ha candidatos",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{}, nil).Once()
					return s.tokenRepo
				}(),
				gateway: s.gateway,
				cipher:  s.cipher,
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve marcar outreach_sent_at e nao resetar quando gateway retorna erro 4xx",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					mobile := "+5511999990002"
					token := buildPaidToken(mobile)
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
					return s.tokenRepo
				}(),
				gateway: func() *mocks.OutreachChannelGateway {
					mobile := "+5511999990002"
					s.gateway.EXPECT().SendActivationTemplate(mock.Anything, "whatsapp", mobile, mock.Anything, "clear-token").Return("", fmt.Errorf("gateway error: %w", apperrors.ErrWhatsAppClientError)).Once()
					return s.gateway
				}(),
				cipher: func() *mocks.TokenCipher {
					s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
					return s.cipher
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve resetar outreach_sent_at quando gateway retorna erro 5xx",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					mobile := "+5511999990003"
					token := buildPaidToken(mobile)
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachReset(mock.Anything, mock.Anything).Return(nil).Once()
					return s.tokenRepo
				}(),
				gateway: func() *mocks.OutreachChannelGateway {
					mobile := "+5511999990003"
					s.gateway.EXPECT().SendActivationTemplate(mock.Anything, "whatsapp", mobile, mock.Anything, "clear-token").Return("", fmt.Errorf("gateway error: %w", apperrors.ErrWhatsAppServerError)).Once()
					return s.gateway
				}(),
				cipher: func() *mocks.TokenCipher {
					s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
					return s.cipher
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando FindPaidForOutreach falha",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db unavailable")).Once()
					return s.tokenRepo
				}(),
				gateway: s.gateway,
				cipher:  s.cipher,
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve enviar texto telegram quando token tem telegram_external_id e nao tem whatsapp",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					telegramID := "987654321"
					token := buildPaidTokenTelegramOnly(telegramID)
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
					return s.tokenRepo
				}(),
				gateway: func() *mocks.OutreachChannelGateway {
					telegramID := "987654321"
					s.gateway.EXPECT().SendText(mock.Anything, "telegram", telegramID, mock.MatchedBy(func(text string) bool {
						return text != "" && containsString(text, "clear-token")
					})).Return(nil).Once()
					return s.gateway
				}(),
				cipher: func() *mocks.TokenCipher {
					s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
					return s.cipher
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve resetar outreach_sent_at quando gateway telegram falha",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					telegramID := "111222333"
					token := buildPaidTokenTelegramOnly(telegramID)
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachReset(mock.Anything, mock.Anything).Return(nil).Once()
					return s.tokenRepo
				}(),
				gateway: func() *mocks.OutreachChannelGateway {
					telegramID := "111222333"
					s.gateway.EXPECT().SendText(mock.Anything, "telegram", telegramID, mock.Anything).Return(errors.New("telegram api 500")).Once()
					return s.gateway
				}(),
				cipher: func() *mocks.TokenCipher {
					s.cipher.EXPECT().Decrypt(mock.Anything, "cipher-token").Return("clear-token", nil).Once()
					return s.cipher
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve pular token sem whatsapp e sem telegram_external_id",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					token := buildPaidTokenNoChannel()
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
					return s.tokenRepo
				}(),
				gateway: s.gateway,
				cipher:  s.cipher,
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve pular envio quando UpdateMarkOutreachSent falha",
			dependencies: dependencies{
				tokenRepo: func() *mocks.MagicTokenRepository {
					mobile1 := "+5511999990011"
					mobile2 := "+5511999990012"
					tokens := []entities.MagicToken{buildPaidToken(mobile1), buildPaidToken(mobile2)}
					s.tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).Return(tokens, nil).Once()
					s.tokenRepo.EXPECT().UpdateMarkOutreachSent(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db write failed")).Times(2)
					return s.tokenRepo
				}(),
				gateway: s.gateway,
				cipher:  s.cipher,
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewSendOutreach(
				scenario.dependencies.tokenRepo,
				scenario.dependencies.gateway,
				scenario.dependencies.cipher,
				id.NewUUIDGenerator(),
				"activation_reminder",
				2*time.Hour,
				s.obs,
			)
			err := uc.Execute(s.ctx)
			scenario.expect(err)
		})
	}
}
