package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeFallbackUoW struct{}

func (u *fakeFallbackUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) (ConsumeInternalResult, error), _ ...uow.Option) (ConsumeInternalResult, error) {
	return fn(ctx, nil)
}

type fakeFallbackOutboxPublisher struct {
	published  int
	publishErr error
}

func (p *fakeFallbackOutboxPublisher) Publish(_ context.Context, _ outbox.Event) error {
	p.published++
	return p.publishErr
}

type fakeFallbackIdentityGW struct {
	upsertResult appinterfaces.UpsertUserResult
	upsertErr    error
}

type fakeFallbackSubscriptionBinder struct {
	boundSubscriptionID string
	boundUserID         string
	err                 error
}

func (b *fakeFallbackSubscriptionBinder) BindUser(_ context.Context, subscriptionID string, userID string) error {
	b.boundSubscriptionID = subscriptionID
	b.boundUserID = userID
	return b.err
}

func (g *fakeFallbackIdentityGW) UpsertUserByWhatsApp(_ context.Context, _, _ string) (appinterfaces.UpsertUserResult, error) {
	return g.upsertResult, g.upsertErr
}

type fakeFallbackMagicTokenRepo struct {
	token     entities.MagicToken
	findErr   error
	updated   bool
	updateErr error
}

func (r *fakeFallbackMagicTokenRepo) Insert(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeFallbackMagicTokenRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeFallbackMagicTokenRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return r.token, r.findErr
}
func (r *fakeFallbackMagicTokenRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeFallbackMagicTokenRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeFallbackMagicTokenRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	r.updated = true
	return r.updateErr
}
func (r *fakeFallbackMagicTokenRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *fakeFallbackMagicTokenRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error {
	return nil
}
func (r *fakeFallbackMagicTokenRepo) CountPaidUnconsumed(_ context.Context) (int64, error) {
	return 0, nil
}
func (r *fakeFallbackMagicTokenRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}

type fakeFallbackSignalRepo struct{}

func (r *fakeFallbackSignalRepo) Insert(_ context.Context, _ entities.SupportSignal) error {
	return nil
}

type fakeFallbackFactory struct {
	tokenRepo appinterfaces.MagicTokenRepository
}

func (f *fakeFallbackFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.tokenRepo
}
func (f *fakeFallbackFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return &fakeFallbackSignalRepo{}
}
func (f *fakeFallbackFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *fakeFallbackFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

func buildFallbackUC(
	tokenRepo *fakeFallbackMagicTokenRepo,
	identityGW appinterfaces.IdentityGateway,
	publishErr error,
) *TryFallbackActivation {
	factory := &fakeFallbackFactory{tokenRepo: tokenRepo}
	publisher := &fakeFallbackOutboxPublisher{publishErr: publishErr}
	binder := &fakeFallbackSubscriptionBinder{}
	idGen := id.NewUUIDGenerator()
	if identityGW == nil {
		identityGW = &fakeFallbackIdentityGW{upsertResult: appinterfaces.UpsertUserResult{UserID: "uid-fallback"}}
	}
	return NewTryFallbackActivation(&fakeFallbackUoW{}, factory, identityGW, binder, publisher, idGen, noop.NewProvider())
}

func buildPaidTokenWithOutreach(fromE164 string) entities.MagicToken {
	hash := []byte("hash-paid-outreach-12345678901234")
	return entities.HydrateMagicToken(
		"tok-paid-outreach", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-2*time.Hour), time.Time{}, time.Now().UTC().Add(-30*time.Minute),
		"cipher-token", "sub-fallback", fromE164, "test@example.com", "sale-002",
		"", "", 0,
	)
}

func buildPaidTokenWithoutOutreach(fromE164 string) entities.MagicToken {
	hash := []byte("hash-paid-no-outreach-1234567890")
	return entities.HydrateMagicToken(
		"tok-paid-no-outreach", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-2*time.Hour), time.Time{}, time.Time{},
		"cipher-token", "sub-no-outreach", fromE164, "test@example.com", "sale-003",
		"", "", 0,
	)
}

func buildExpiredPaidTokenWithOutreach(fromE164 string) entities.MagicToken {
	hash := []byte("hash-expired-outreach-1234567890")
	return entities.HydrateMagicToken(
		"tok-expired-outreach", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		time.Now().UTC().Add(-5*24*time.Hour), time.Time{}, time.Now().UTC().Add(-4*24*time.Hour),
		"cipher-token", "sub-expired", fromE164, "test@example.com", "sale-004",
		"", "", 0,
	)
}

type TryFallbackActivationSuite struct {
	suite.Suite
}

func TestTryFallbackActivation(t *testing.T) {
	suite.Run(t, new(TryFallbackActivationSuite))
}

func (s *TryFallbackActivationSuite) TestNoMatch_TokenNotFound() {
	fromE164 := "+5511999990001"
	tokenRepo := &fakeFallbackMagicTokenRepo{findErr: domain.ErrTokenNotFound}
	uc := buildFallbackUC(tokenRepo, nil, nil)

	result, err := uc.Execute(context.Background(), fromE164)

	s.NoError(err)
	s.Equal(FallbackOutcomeNoMatch, result.Outcome)
}

func (s *TryFallbackActivationSuite) TestNoOutreach_ReturnsOutreachRequired() {
	fromE164 := "+5511999990002"
	tokenRepo := &fakeFallbackMagicTokenRepo{token: buildPaidTokenWithoutOutreach(fromE164)}
	uc := buildFallbackUC(tokenRepo, nil, nil)

	result, err := uc.Execute(context.Background(), fromE164)

	s.NoError(err)
	s.Equal(FallbackOutcomeOutreachRequired, result.Outcome)
}

func (s *TryFallbackActivationSuite) TestWithOutreach_Activates() {
	fromE164 := "+5511999990003"
	tokenRepo := &fakeFallbackMagicTokenRepo{token: buildPaidTokenWithOutreach(fromE164)}
	uc := buildFallbackUC(tokenRepo, nil, nil)

	result, err := uc.Execute(context.Background(), fromE164)

	s.NoError(err)
	s.Equal(FallbackOutcomeActivated, result.Outcome)
	s.True(tokenRepo.updated)
}

func (s *TryFallbackActivationSuite) TestTokenExpired_ReturnsNoMatch() {
	fromE164 := "+5511999990004"
	tokenRepo := &fakeFallbackMagicTokenRepo{token: buildExpiredPaidTokenWithOutreach(fromE164)}
	uc := buildFallbackUC(tokenRepo, nil, nil)

	result, err := uc.Execute(context.Background(), fromE164)

	s.NoError(err)
	s.Equal(FallbackOutcomeNoMatch, result.Outcome)
}

func (s *TryFallbackActivationSuite) TestIdentityGatewayFails_ReturnsError() {
	fromE164 := "+5511999990005"
	tokenRepo := &fakeFallbackMagicTokenRepo{token: buildPaidTokenWithOutreach(fromE164)}
	identityGW := &fakeFallbackIdentityGW{upsertErr: errors.New("identity unavailable")}
	uc := buildFallbackUC(tokenRepo, identityGW, nil)

	_, err := uc.Execute(context.Background(), fromE164)

	s.Error(err)
}

func (s *TryFallbackActivationSuite) TestFindError_ReturnsError() {
	fromE164 := "+5511999990006"
	tokenRepo := &fakeFallbackMagicTokenRepo{findErr: errors.New("db error")}
	uc := buildFallbackUC(tokenRepo, nil, nil)

	_, err := uc.Execute(context.Background(), fromE164)

	s.Error(err)
}

func (s *TryFallbackActivationSuite) TestWithOutreach_PublishesEvent() {
	fromE164 := "+5511999990007"
	tokenRepo := &fakeFallbackMagicTokenRepo{token: buildPaidTokenWithOutreach(fromE164)}
	publisher := &fakeFallbackOutboxPublisher{}
	factory := &fakeFallbackFactory{tokenRepo: tokenRepo}
	identityGW := &fakeFallbackIdentityGW{upsertResult: appinterfaces.UpsertUserResult{UserID: "user-fallback"}}
	binder := &fakeFallbackSubscriptionBinder{}
	uc := NewTryFallbackActivation(&fakeFallbackUoW{}, factory, identityGW, binder, publisher, id.NewUUIDGenerator(), noop.NewProvider())

	result, err := uc.Execute(context.Background(), fromE164)

	s.NoError(err)
	s.Equal(FallbackOutcomeActivated, result.Outcome)
	s.Equal(1, publisher.published)
}
