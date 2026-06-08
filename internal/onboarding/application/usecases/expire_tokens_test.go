package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type fakeMagicTokenRepo struct {
	bulkExpireResult []entities.MagicToken
	bulkExpireErr    error
	bulkExpireCalls  int
}

func (r *fakeMagicTokenRepo) Insert(_ context.Context, _ entities.MagicToken) error { return nil }
func (r *fakeMagicTokenRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeMagicTokenRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeMagicTokenRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeMagicTokenRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeMagicTokenRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeMagicTokenRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *fakeMagicTokenRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error { return nil }
func (r *fakeMagicTokenRepo) CountPaidUnconsumed(_ context.Context) (int64, error)      { return 0, nil }
func (r *fakeMagicTokenRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	r.bulkExpireCalls++
	if r.bulkExpireErr != nil {
		return nil, r.bulkExpireErr
	}
	res := r.bulkExpireResult
	r.bulkExpireResult = nil
	return res, nil
}

type fakeSupportSignalRepo struct {
	insertCalls int
	insertErr   error
	lastSignal  entities.SupportSignal
}

func (r *fakeSupportSignalRepo) Insert(_ context.Context, signal entities.SupportSignal) error {
	r.insertCalls++
	r.lastSignal = signal
	return r.insertErr
}

type fakeRepositoryFactory struct {
	tokenRepo  appinterfaces.MagicTokenRepository
	signalRepo appinterfaces.SupportSignalRepository
}

func (f *fakeRepositoryFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.tokenRepo
}
func (f *fakeRepositoryFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return f.signalRepo
}
func (f *fakeRepositoryFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *fakeRepositoryFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

type fakeManager struct{}

func (m *fakeManager) Driver() database.Driver              { return "" }
func (m *fakeManager) DBTX(_ context.Context) database.DBTX { return nil }
func (m *fakeManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (m *fakeManager) Ping(_ context.Context) error     { return nil }
func (m *fakeManager) Shutdown(_ context.Context) error { return nil }

type fakeIDGen struct{ val string }

func (g *fakeIDGen) NewID() string {
	if g.val != "" {
		return g.val
	}
	return "test-id"
}

func makePaidToken(tokenID string, paidAt time.Time, externalSaleID string) entities.MagicToken {
	hash := []byte("hash-" + tokenID)
	base := entities.HydrateMagicToken(
		tokenID, hash, valueobjects.TokenStatusExpired,
		"plan-1", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		paidAt, time.Time{}, time.Time{},
		"cipher-token", "sub-001", "+5511999999999", "user@example.com", externalSaleID,
		"", "", valueobjects.ActivationPath(0),
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
		"", "", valueobjects.ActivationPath(0),
	)
	expired, _ := base.MarkExpired()
	return expired
}

type ExpireTokensSuite struct {
	suite.Suite
	tokenRepo  *fakeMagicTokenRepo
	signalRepo *fakeSupportSignalRepo
	factory    *fakeRepositoryFactory
	idGen      id.Generator
	uc         *usecases.ExpireTokens
}

func TestExpireTokens(t *testing.T) {
	suite.Run(t, new(ExpireTokensSuite))
}

func (s *ExpireTokensSuite) SetupTest() {
	s.tokenRepo = &fakeMagicTokenRepo{}
	s.signalRepo = &fakeSupportSignalRepo{}
	s.factory = &fakeRepositoryFactory{tokenRepo: s.tokenRepo, signalRepo: s.signalRepo}
	s.idGen = &fakeIDGen{val: "signal-id-1"}
	s.uc = usecases.NewExpireTokens(&fakeManager{}, s.factory, s.idGen, noop.NewProvider())
}

func (s *ExpireTokensSuite) TestNoExpiredTokens() {
	s.tokenRepo.bulkExpireResult = nil

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(1, s.tokenRepo.bulkExpireCalls)
	s.Equal(0, s.signalRepo.insertCalls)
}

func (s *ExpireTokensSuite) TestPAIDExpiredTokenEmitsOrphanSignal() {
	paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
	token := makePaidToken("tok-paid-1", paidAt, "sale-001")
	s.tokenRepo.bulkExpireResult = []entities.MagicToken{token}

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(1, s.signalRepo.insertCalls)
	var payload map[string]any
	s.Require().NoError(json.Unmarshal(s.signalRepo.lastSignal.Payload(), &payload))
	s.Equal("68617368", payload["token_hash_prefix"])
	s.Equal("sale-001", payload["external_sale_id"])
}

func (s *ExpireTokensSuite) TestPENDINGExpiredTokenDoesNotEmitSignal() {
	token := makePendingToken("tok-pending-1")
	s.tokenRepo.bulkExpireResult = []entities.MagicToken{token}

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(0, s.signalRepo.insertCalls)
}

func (s *ExpireTokensSuite) TestCONSUMEDTokenIgnoredByBulkExpire() {
	s.tokenRepo.bulkExpireResult = nil

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(0, s.signalRepo.insertCalls)
}

func (s *ExpireTokensSuite) TestEXPIREDNoOpWhenNoBatch() {
	s.tokenRepo.bulkExpireResult = nil

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(1, s.tokenRepo.bulkExpireCalls)
}

func (s *ExpireTokensSuite) TestBatchLoopStopsWhenFewerThanBatchSize() {
	paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
	tokens := make([]entities.MagicToken, 5)
	for i := range tokens {
		tokens[i] = makePaidToken("tok-batch-small", paidAt, "sale-small")
	}
	s.tokenRepo.bulkExpireResult = tokens

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(1, s.tokenRepo.bulkExpireCalls)
	s.Equal(5, s.signalRepo.insertCalls)
}

func (s *ExpireTokensSuite) TestSignalInsertErrorDoesNotAbortJob() {
	paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
	token := makePaidToken("tok-signal-err", paidAt, "sale-err")
	s.tokenRepo.bulkExpireResult = []entities.MagicToken{token}
	s.signalRepo.insertErr = errors.New("db error")

	err := s.uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Equal(1, s.signalRepo.insertCalls)
}

func (s *ExpireTokensSuite) TestBulkExpireErrorPropagates() {
	s.tokenRepo.bulkExpireErr = errors.New("db timeout")

	err := s.uc.Execute(context.Background())

	s.Require().Error(err)
	s.ErrorContains(err, "bulk expire")
}

func (s *ExpireTokensSuite) TestRepeatExecutionIdempotent() {
	paidAt := time.Now().UTC().Add(-2 * 24 * time.Hour)
	token := makePaidToken("tok-repeat", paidAt, "sale-repeat")

	s.tokenRepo.bulkExpireResult = []entities.MagicToken{token}
	err := s.uc.Execute(context.Background())
	s.Require().NoError(err)
	s.Equal(1, s.signalRepo.insertCalls)

	s.tokenRepo.bulkExpireResult = nil
	err = s.uc.Execute(context.Background())
	s.Require().NoError(err)
	s.Equal(1, s.signalRepo.insertCalls)
}
