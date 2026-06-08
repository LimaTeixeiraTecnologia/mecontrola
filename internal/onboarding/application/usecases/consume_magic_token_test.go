package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeConsumeUoW struct{}

func (u *fakeConsumeUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) (ConsumeInternalResult, error), _ ...uow.Option) (ConsumeInternalResult, error) {
	return fn(ctx, nil)
}

type fakeConsumeOutboxPublisher struct {
	published  int
	publishErr error
	events     []outbox.Event
}

func (p *fakeConsumeOutboxPublisher) Publish(_ context.Context, event outbox.Event) error {
	p.published++
	p.events = append(p.events, event)
	return p.publishErr
}

type fakeConsumeIdentityGW struct {
	upsertResult appinterfaces.UpsertUserResult
	upsertErr    error
}

func (g *fakeConsumeIdentityGW) UpsertUserByWhatsApp(_ context.Context, _, _ string) (appinterfaces.UpsertUserResult, error) {
	return g.upsertResult, g.upsertErr
}

type fakeConsumeSubscriptionBinder struct {
	boundSubscriptionID string
	boundUserID         string
	err                 error
}

func (b *fakeConsumeSubscriptionBinder) BindUser(_ context.Context, subscriptionID string, userID string) error {
	b.boundSubscriptionID = subscriptionID
	b.boundUserID = userID
	return b.err
}

type fakeConsumeMagicTokenRepo struct {
	token     entities.MagicToken
	findErr   error
	updated   bool
	updateErr error
}

func (r *fakeConsumeMagicTokenRepo) Insert(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeConsumeMagicTokenRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return r.token, r.findErr
}
func (r *fakeConsumeMagicTokenRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeConsumeMagicTokenRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeConsumeMagicTokenRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeConsumeMagicTokenRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	r.updated = true
	return r.updateErr
}
func (r *fakeConsumeMagicTokenRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *fakeConsumeMagicTokenRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error {
	return nil
}
func (r *fakeConsumeMagicTokenRepo) CountPaidUnconsumed(_ context.Context) (int64, error) {
	return 0, nil
}
func (r *fakeConsumeMagicTokenRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}

type fakeConsumeSignalRepo struct {
	inserted int
}

func (r *fakeConsumeSignalRepo) Insert(_ context.Context, _ entities.SupportSignal) error {
	r.inserted++
	return nil
}

type fakeConsumeFactory struct {
	tokenRepo  appinterfaces.MagicTokenRepository
	signalRepo appinterfaces.SupportSignalRepository
}

func (f *fakeConsumeFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.tokenRepo
}
func (f *fakeConsumeFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return f.signalRepo
}
func (f *fakeConsumeFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *fakeConsumeFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

func buildConsumeUC(tokenRepo *fakeConsumeMagicTokenRepo, signalRepo *fakeConsumeSignalRepo, identityGW appinterfaces.IdentityGateway, pubErr error) *ConsumeMagicToken {
	factory := &fakeConsumeFactory{tokenRepo: tokenRepo, signalRepo: signalRepo}
	publisher := &fakeConsumeOutboxPublisher{publishErr: pubErr}
	binder := &fakeConsumeSubscriptionBinder{}
	idGen := id.NewUUIDGenerator()
	if identityGW == nil {
		identityGW = &fakeConsumeIdentityGW{upsertResult: appinterfaces.UpsertUserResult{UserID: "uid-test"}}
	}
	return NewConsumeMagicToken(&fakeConsumeUoW{}, factory, identityGW, binder, publisher, idGen, noop.NewProvider())
}

func buildConsumeUCWithDeps(
	tokenRepo *fakeConsumeMagicTokenRepo,
	signalRepo *fakeConsumeSignalRepo,
	identityGW appinterfaces.IdentityGateway,
	binder appinterfaces.SubscriptionBinder,
	publisher *fakeConsumeOutboxPublisher,
) *ConsumeMagicToken {
	factory := &fakeConsumeFactory{tokenRepo: tokenRepo, signalRepo: signalRepo}
	if identityGW == nil {
		identityGW = &fakeConsumeIdentityGW{upsertResult: appinterfaces.UpsertUserResult{UserID: "uid-test"}}
	}
	return NewConsumeMagicToken(&fakeConsumeUoW{}, factory, identityGW, binder, publisher, id.NewUUIDGenerator(), noop.NewProvider())
}

func buildPaidToken(fromE164, email string) entities.MagicToken {
	hash := []byte("hash-paid-tok-1234567890123456")
	tok := entities.HydrateMagicToken(
		"tok-paid-1", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-2*time.Hour),
		time.Now().UTC().Add(-1*time.Hour), time.Time{}, time.Time{},
		"cipher-token", "sub-001", fromE164, email, "sale-001",
		"", "", 0,
	)
	return tok
}

func buildConsumedToken(consumedByE164 string) entities.MagicToken {
	hash := []byte("hash-consumed-tok-12345678901234")
	return entities.HydrateMagicToken(
		"tok-consumed-1", hash, valueobjects.TokenStatusConsumed,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC().Add(-3*time.Hour),
		time.Now().UTC().Add(-2*time.Hour), time.Now().UTC().Add(-1*time.Hour), time.Time{},
		"cipher-token", "sub-001", consumedByE164, "user@test.com", "sale-001",
		"user-id-1", consumedByE164, valueobjects.ActivationPathDirect,
	)
}

func buildPendingToken() entities.MagicToken {
	hash := []byte("hash-pending-tok-123456789012345")
	return entities.HydrateMagicToken(
		"tok-pending", hash, valueobjects.TokenStatusPending,
		"plan-1", time.Now().UTC().Add(7*24*time.Hour), time.Now().UTC(),
		time.Time{}, time.Time{}, time.Time{},
		"cipher-token", "", "", "", "",
		"", "", 0,
	)
}

func buildExpiredToken() entities.MagicToken {
	hash := []byte("hash-expired-tok-123456789012345")
	return entities.HydrateMagicToken(
		"tok-expired", hash, valueobjects.TokenStatusExpired,
		"plan-1", time.Now().UTC().Add(-24*time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		time.Time{}, time.Time{}, time.Time{},
		"cipher-token", "", "", "", "",
		"", "", 0,
	)
}

type ConsumeMagicTokenSuite struct {
	suite.Suite
}

func TestConsumeMagicToken(t *testing.T) {
	suite.Run(t, new(ConsumeMagicTokenSuite))
}

func (s *ConsumeMagicTokenSuite) validInput(fromE164 string) input.ConsumeMagicTokenInput {
	return input.ConsumeMagicTokenInput{
		Token:          "validtokenstring123456789012345678901234567",
		FromE164:       fromE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	}
}

func (s *ConsumeMagicTokenSuite) TestTokenNotFound() {
	tokenRepo := &fakeConsumeMagicTokenRepo{findErr: domain.ErrTokenNotFound}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	result, err := uc.Execute(context.Background(), s.validInput("+5511999999999"))

	s.NoError(err)
	s.Equal(ConsumeOutcomeNotFound, result.Outcome)
}

func (s *ConsumeMagicTokenSuite) TestInvalidTokenFormat() {
	tokenRepo := &fakeConsumeMagicTokenRepo{}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	in := input.ConsumeMagicTokenInput{Token: "!!invalid!!", FromE164: "+5511999999999", ActivationPath: valueobjects.ActivationPathDirect}
	result, err := uc.Execute(context.Background(), in)

	s.NoError(err)
	s.Equal(ConsumeOutcomeNotFound, result.Outcome)
}

func (s *ConsumeMagicTokenSuite) TestTokenPending_ReturnsNotYetPaid() {
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildPendingToken()}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	result, err := uc.Execute(context.Background(), s.validInput("+5511999999999"))

	s.NoError(err)
	s.Equal(ConsumeOutcomeNotYetPaid, result.Outcome)
}

func (s *ConsumeMagicTokenSuite) TestTokenExpired_ReturnsExpired() {
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildExpiredToken()}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	result, err := uc.Execute(context.Background(), s.validInput("+5511999999999"))

	s.NoError(err)
	s.Equal(ConsumeOutcomeExpired, result.Outcome)
}

func (s *ConsumeMagicTokenSuite) TestTokenPaidButExpiredByTimestamp_ReturnsExpired() {
	hash := []byte("hash-paid-expired-tok-123456789012")
	expiredPaid := entities.HydrateMagicToken(
		"tok-paid-expired", hash, valueobjects.TokenStatusPaid,
		"plan-1", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-8*24*time.Hour),
		time.Now().UTC().Add(-3*24*time.Hour), time.Time{}, time.Time{},
		"cipher-token", "sub-expired", "+5511999999999", "e@test.com", "sale-x",
		"", "", 0,
	)
	tokenRepo := &fakeConsumeMagicTokenRepo{token: expiredPaid}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	result, err := uc.Execute(context.Background(), s.validInput("+5511999999999"))

	s.NoError(err)
	s.Equal(ConsumeOutcomeExpired, result.Outcome)
}

func (s *ConsumeMagicTokenSuite) TestTokenConsumedSameNumber_AlreadyActive() {
	fromE164 := "+5511999999999"
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildConsumedToken(fromE164)}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	result, err := uc.Execute(context.Background(), s.validInput(fromE164))

	s.NoError(err)
	s.Equal(ConsumeOutcomeAlreadyActive, result.Outcome)
}

func (s *ConsumeMagicTokenSuite) TestTokenConsumedDifferentNumber_InsertsSignal() {
	consumedByE164 := "+5511999999999"
	fromE164 := "+5521888888888"
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildConsumedToken(consumedByE164)}
	signalRepo := &fakeConsumeSignalRepo{}
	uc := buildConsumeUC(tokenRepo, signalRepo, nil, nil)

	result, err := uc.Execute(context.Background(), input.ConsumeMagicTokenInput{
		Token:          "validtokenstring123456789012345678901234567",
		FromE164:       fromE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	})

	s.NoError(err)
	s.Equal(ConsumeOutcomeReuseOtherAccount, result.Outcome)
	s.Equal(1, signalRepo.inserted)
}

func (s *ConsumeMagicTokenSuite) TestTokenPaid_SuccessfulActivation() {
	fromE164 := "+5511999999999"
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildPaidToken(fromE164, "u@test.com")}
	identityGW := &fakeConsumeIdentityGW{upsertResult: appinterfaces.UpsertUserResult{UserID: "user-123"}}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, identityGW, nil)

	result, err := uc.Execute(context.Background(), s.validInput(fromE164))

	s.NoError(err)
	s.Equal(ConsumeOutcomeActivated, result.Outcome)
	s.True(tokenRepo.updated)
}

func (s *ConsumeMagicTokenSuite) TestTokenPaid_BindsSubscriptionAndPublishesSubscriptionID() {
	fromE164 := "+5511999999999"
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildPaidToken(fromE164, "u@test.com")}
	identityGW := &fakeConsumeIdentityGW{upsertResult: appinterfaces.UpsertUserResult{UserID: "user-123"}}
	binder := &fakeConsumeSubscriptionBinder{}
	publisher := &fakeConsumeOutboxPublisher{}
	uc := buildConsumeUCWithDeps(tokenRepo, &fakeConsumeSignalRepo{}, identityGW, binder, publisher)

	result, err := uc.Execute(context.Background(), s.validInput(fromE164))

	s.NoError(err)
	s.Equal(ConsumeOutcomeActivated, result.Outcome)
	s.Equal("sub-001", binder.boundSubscriptionID)
	s.Equal("user-123", binder.boundUserID)
	s.Require().Len(publisher.events, 1)
	var payload map[string]any
	s.Require().NoError(json.Unmarshal(publisher.events[0].Payload, &payload))
	s.Equal("sub-001", payload["subscription_id"])
}

func (s *ConsumeMagicTokenSuite) TestTokenPaid_IdentityGatewayFails_ReturnsError() {
	fromE164 := "+5511999999999"
	tokenRepo := &fakeConsumeMagicTokenRepo{token: buildPaidToken(fromE164, "u@test.com")}
	identityGW := &fakeConsumeIdentityGW{upsertErr: errors.New("identity unavailable")}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, identityGW, nil)

	_, err := uc.Execute(context.Background(), s.validInput(fromE164))

	s.Error(err)
}

func (s *ConsumeMagicTokenSuite) TestTokenPaid_UnsupportedCountry_FindError() {
	tokenRepo := &fakeConsumeMagicTokenRepo{findErr: domain.ErrUnsupportedCountry}
	uc := buildConsumeUC(tokenRepo, &fakeConsumeSignalRepo{}, nil, nil)

	_, err := uc.Execute(context.Background(), s.validInput("+12125551234"))

	s.Error(err)
}
