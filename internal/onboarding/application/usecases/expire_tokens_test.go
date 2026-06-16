package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	usecasesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

func makePaidToken(tokenID string, paidAt time.Time, externalSaleID string) entities.MagicToken {
	hash := []byte("hash-" + tokenID)
	base := entities.HydrateMagicToken(
		tokenID, hash, valueobjects.TokenStatusExpired,
		"plan-1", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		paidAt, time.Time{}, time.Time{},
		"cipher-token", "sub-001", "+5511999999999", "user@example.com", externalSaleID,
		"", "", valueobjects.ActivationPath(0), "",
	)
	expired, _ := base.MarkExpired()
	return expired
}

func makePendingToken(tokenID string) entities.MagicToken {
	hash := []byte("hash-pending-" + tokenID)
	base := entities.HydrateMagicToken(
		tokenID, hash, valueobjects.TokenStatusExpired,
		"plan-1", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		time.Time{}, time.Time{}, time.Time{},
		"cipher-token", "", "", "", "",
		"", "", valueobjects.ActivationPath(0), "",
	)
	expired, _ := base.MarkExpired()
	return expired
}

type ExpireTokensSuite struct {
	suite.Suite
	tokenRepo  *mocks.MagicTokenRepository
	signalRepo *mocks.SupportSignalRepository
	factory    *mocks.RepositoryFactory
	idGen      id.Generator
	mgr        *usecasesmocks.FakeManager
}

func TestExpireTokens(t *testing.T) {
	suite.Run(t, new(ExpireTokensSuite))
}

func (s *ExpireTokensSuite) SetupTest() {
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
	s.signalRepo = mocks.NewSupportSignalRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.idGen = id.NewUUIDGenerator()
	s.mgr = usecasesmocks.NewFakeManager()
	s.factory.EXPECT().MagicTokenRepository(mock.Anything).Return(s.tokenRepo).Maybe()
	s.factory.EXPECT().SupportSignalRepository(mock.Anything).Return(s.signalRepo).Maybe()
}

func (s *ExpireTokensSuite) TestExecute() {
	scenarios := []struct {
		name   string
		setup  func()
		expect func(err error)
	}{
		{
			name: "deve completar sem erro quando nao ha tokens expirados",
			setup: func() {
				s.tokenRepo.EXPECT().BulkExpire(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{}, nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve emitir signal de orfao quando token PAID expira",
			setup: func() {
				paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
				token := makePaidToken("tok-paid-1", paidAt, "sale-001")
				s.tokenRepo.EXPECT().BulkExpire(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
				s.signalRepo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(sig entities.SupportSignal) bool {
					var payload map[string]any
					if err := json.Unmarshal(sig.Payload(), &payload); err != nil {
						return false
					}
					return payload["token_hash_prefix"] == "68617368" && payload["external_sale_id"] == "sale-001"
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve ignorar token PENDING expirado sem emitir signal",
			setup: func() {
				token := makePendingToken("tok-pending-1")
				s.tokenRepo.EXPECT().BulkExpire(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve parar o loop quando batch retorna menos que o limite",
			setup: func() {
				paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
				tokens := make([]entities.MagicToken, 5)
				for i := range tokens {
					tokens[i] = makePaidToken("tok-batch-small", paidAt, "sale-small")
				}
				s.tokenRepo.EXPECT().BulkExpire(mock.Anything, mock.Anything, mock.Anything).Return(tokens, nil).Once()
				s.signalRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Times(5)
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve continuar execucao quando insert de signal falha",
			setup: func() {
				paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
				token := makePaidToken("tok-signal-err", paidAt, "sale-err")
				s.tokenRepo.EXPECT().BulkExpire(mock.Anything, mock.Anything, mock.Anything).Return([]entities.MagicToken{token}, nil).Once()
				s.signalRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(errors.New("db error")).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro quando BulkExpire falha",
			setup: func() {
				s.tokenRepo.EXPECT().BulkExpire(mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db timeout")).Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorContains(err, "bulk expire")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := usecases.NewExpireTokens(s.mgr, s.factory, s.idGen, noop.NewProvider())
			err := uc.Execute(context.Background())
			scenario.expect(err)
		})
	}
}
