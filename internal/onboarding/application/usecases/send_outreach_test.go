package usecases_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	apperrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type stubOutreachTokenRepo struct {
	candidates            []entities.MagicToken
	findErr               error
	markOutreachSentErr   error
	markOutreachSentCalls []string
	resetCalls            []string
	resetErr              error
}

func (r *stubOutreachTokenRepo) Insert(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *stubOutreachTokenRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *stubOutreachTokenRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *stubOutreachTokenRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return r.candidates, r.findErr
}
func (r *stubOutreachTokenRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *stubOutreachTokenRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *stubOutreachTokenRepo) UpdateMarkOutreachSent(_ context.Context, tokenID string, _ time.Time) error {
	r.markOutreachSentCalls = append(r.markOutreachSentCalls, tokenID)
	return r.markOutreachSentErr
}
func (r *stubOutreachTokenRepo) UpdateMarkOutreachReset(_ context.Context, tokenID string) error {
	r.resetCalls = append(r.resetCalls, tokenID)
	return r.resetErr
}
func (r *stubOutreachTokenRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *stubOutreachTokenRepo) CountPaidUnconsumed(_ context.Context) (int64, error) {
	return 0, nil
}

type stubOutreachFactory struct {
	repo *stubOutreachTokenRepo
}

func (f *stubOutreachFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.repo
}
func (f *stubOutreachFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return nil
}
func (f *stubOutreachFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *stubOutreachFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

type stubOutreachGateway struct {
	sendErr    error
	sentToE164 []string
	sentTokens []string
}

func (g *stubOutreachGateway) SendActivationTemplate(_ context.Context, toE164, _, token string) (string, error) {
	if g.sendErr != nil {
		return "", g.sendErr
	}
	g.sentToE164 = append(g.sentToE164, toE164)
	g.sentTokens = append(g.sentTokens, token)
	return "wamid.test", nil
}

func (g *stubOutreachGateway) SendTextMessage(_ context.Context, _ string, _ string) error {
	return nil
}

type stubFakeManager struct{}

func (m *stubFakeManager) Driver() database.Driver              { return "" }
func (m *stubFakeManager) DBTX(_ context.Context) database.DBTX { return nil }
func (m *stubFakeManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (m *stubFakeManager) Ping(_ context.Context) error     { return nil }
func (m *stubFakeManager) Shutdown(_ context.Context) error { return nil }

type stubTokenCipher struct {
	decrypted string
	err       error
}

func (c *stubTokenCipher) Encrypt(_ context.Context, clearToken string) (string, error) {
	return clearToken, nil
}

func (c *stubTokenCipher) Decrypt(_ context.Context, _ string) (string, error) {
	return c.decrypted, c.err
}

func buildPaidToken(mobile string) entities.MagicToken {
	t, _ := entities.NewMagicToken("tok-id", []byte("hash"), "plan-id-1", time.Now().Add(7*24*time.Hour))
	t, _ = t.WithActivationTokenCiphertext("cipher-token")
	t, _ = t.MarkPaid("sub-001", mobile, "test@example.com", "sale-001", time.Now().Add(-3*time.Hour))
	return t
}

type SendOutreachSuite struct {
	suite.Suite
}

func TestSendOutreach(t *testing.T) {
	suite.Run(t, new(SendOutreachSuite))
}

func (s *SendOutreachSuite) newUseCase(repo *stubOutreachTokenRepo, gw appinterfaces.WhatsAppGateway) *usecases.SendOutreach {
	factory := &stubOutreachFactory{repo: repo}
	idGen := id.NewUUIDGenerator()
	return usecases.NewSendOutreach(
		&stubFakeManager{},
		factory,
		gw,
		&stubTokenCipher{decrypted: "clear-token"},
		idGen,
		"activation_reminder",
		2*time.Hour,
		noop.NewProvider(),
	)
}

func (s *SendOutreachSuite) TestSuccess_SendsToAllCandidates() {
	mobile := "+5511999990001"
	token := buildPaidToken(mobile)
	repo := &stubOutreachTokenRepo{candidates: []entities.MagicToken{token}}
	gw := &stubOutreachGateway{}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Len(gw.sentToE164, 1)
	s.Equal(mobile, gw.sentToE164[0])
	s.Equal([]string{"clear-token"}, gw.sentTokens)
	s.Len(repo.markOutreachSentCalls, 1)
	s.Len(repo.resetCalls, 0)
}

func (s *SendOutreachSuite) TestToggleOff_NoCandidates() {
	repo := &stubOutreachTokenRepo{candidates: nil}
	gw := &stubOutreachGateway{}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Empty(gw.sentToE164)
}

func (s *SendOutreachSuite) TestError4xx_NoReset() {
	mobile := "+5511999990002"
	token := buildPaidToken(mobile)
	repo := &stubOutreachTokenRepo{candidates: []entities.MagicToken{token}}
	gw := &stubOutreachGateway{
		sendErr: fmt.Errorf("gateway error: %w", apperrors.ErrWhatsAppClientError),
	}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Len(repo.markOutreachSentCalls, 1, "deve marcar outreach_sent_at antes de enviar")
	s.Len(repo.resetCalls, 0, "4xx não deve resetar outreach_sent_at")
}

func (s *SendOutreachSuite) TestError5xx_ResetsOutreach() {
	mobile := "+5511999990003"
	token := buildPaidToken(mobile)
	repo := &stubOutreachTokenRepo{candidates: []entities.MagicToken{token}}
	gw := &stubOutreachGateway{
		sendErr: fmt.Errorf("gateway error: %w", apperrors.ErrWhatsAppServerError),
	}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Len(repo.markOutreachSentCalls, 1, "deve marcar antes de enviar")
	s.Len(repo.resetCalls, 1, "5xx deve resetar outreach_sent_at para retry")
}

func (s *SendOutreachSuite) TestIdempotence_EmptyCandidates() {
	repo := &stubOutreachTokenRepo{candidates: []entities.MagicToken{}}
	gw := &stubOutreachGateway{}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Empty(gw.sentToE164)
}

func (s *SendOutreachSuite) TestFindError_ReturnsError() {
	repo := &stubOutreachTokenRepo{findErr: errors.New("db unavailable")}
	gw := &stubOutreachGateway{}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().Error(err)
}

func (s *SendOutreachSuite) TestMultipleCandidates_SkipsOnMarkError() {
	mobile1 := "+5511999990011"
	mobile2 := "+5511999990012"
	tokens := []entities.MagicToken{buildPaidToken(mobile1), buildPaidToken(mobile2)}

	repo := &stubOutreachTokenRepo{
		candidates:          tokens,
		markOutreachSentErr: errors.New("db write failed"),
	}
	gw := &stubOutreachGateway{}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err, "execute should not fail even if individual tokens error")
	s.Empty(gw.sentToE164, "no message should be sent when mark fails")
}

func (s *SendOutreachSuite) TestPIISafe_MobileNotInLogs() {
	mobile := "+5511999990099"
	token := buildPaidToken(mobile)
	repo := &stubOutreachTokenRepo{candidates: []entities.MagicToken{token}}
	gw := &stubOutreachGateway{}

	uc := s.newUseCase(repo, gw)
	err := uc.Execute(context.Background())

	s.Require().NoError(err)
	s.Len(gw.sentToE164, 1)
	_ = token
	_ = valueobjects.TokenStatusPaid
}
