//go:build integration

package usecases_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ActivateTelegramByTokenIntegrationSuite struct {
	suite.Suite
}

func TestActivateTelegramByTokenIntegration(t *testing.T) {
	suite.Run(t, new(ActivateTelegramByTokenIntegrationSuite))
}

func (s *ActivateTelegramByTokenIntegrationSuite) TestExecute_TokenConsumed_InsertsUserIdentityAtomically() {
	ctx := context.Background()
	mgr, dsn := testcontainer.Postgres(s.T())

	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)
	identityFactory := identityrepos.NewRepositoryFactory(o11y)
	u := uow.New[usecases.ActivateTelegramResult](mgr, uow.WithObservability(o11y))

	sut := usecases.NewActivateTelegramByToken(factory, identityFactory, u, o11y)

	userID := uuid.New()
	telegramUserID := int64(987654321)
	clearToken := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte("z"), 32))

	tok, err := valueobjects.TokenFromClear(clearToken)
	s.Require().NoError(err)

	s.seedUser(ctx, mgr.DBTX(ctx), userID)
	s.seedConsumedToken(ctx, mgr.DBTX(ctx), tok.Hash(), userID)

	res, execErr := sut.Execute(ctx, usecases.ActivateTelegramByTokenInput{
		Token:          clearToken,
		TelegramUserID: telegramUserID,
	})
	s.Require().NoError(execErr)
	s.Equal(usecases.ActivateTelegramOutcomeLinked, res.Outcome)
	s.Equal(userID, res.UserID)

	s.assertUserIdentityRow(ctx, mgr.DBTX(ctx), userID, telegramUserID)

	_ = dsn
}

func (s *ActivateTelegramByTokenIntegrationSuite) TestExecute_TokenConsumed_IdempotentSameUser() {
	ctx := context.Background()
	mgr, _ := testcontainer.Postgres(s.T())

	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)
	identityFactory := identityrepos.NewRepositoryFactory(o11y)
	u := uow.New[usecases.ActivateTelegramResult](mgr, uow.WithObservability(o11y))

	sut := usecases.NewActivateTelegramByToken(factory, identityFactory, u, o11y)

	userID := uuid.New()
	telegramUserID := int64(111222333)
	clearToken := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte("y"), 32))
	tok, err := valueobjects.TokenFromClear(clearToken)
	s.Require().NoError(err)

	s.seedUser(ctx, mgr.DBTX(ctx), userID)
	s.seedConsumedToken(ctx, mgr.DBTX(ctx), tok.Hash(), userID)

	first, err := sut.Execute(ctx, usecases.ActivateTelegramByTokenInput{Token: clearToken, TelegramUserID: telegramUserID})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeLinked, first.Outcome)

	second, err := sut.Execute(ctx, usecases.ActivateTelegramByTokenInput{Token: clearToken, TelegramUserID: telegramUserID})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeAlreadyLinked, second.Outcome,
		"segunda chamada com mesmo user deve ser idempotente")
}

func (s *ActivateTelegramByTokenIntegrationSuite) seedUser(ctx context.Context, db database.DBTX, userID uuid.UUID) {
	s.T().Helper()
	const q = `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`
	_, err := db.ExecContext(ctx, q, userID, "+5511987654321")
	s.Require().NoError(err)
}

func (s *ActivateTelegramByTokenIntegrationSuite) seedConsumedToken(ctx context.Context, db database.DBTX, hash []byte, userID uuid.UUID) {
	s.T().Helper()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	planID := uuid.New()
	subID := uuid.New()
	const q = `
		INSERT INTO mecontrola.onboarding_tokens (
			id, token_hash, status, plan_id, expires_at, created_at, paid_at, consumed_at,
			activation_token_ciphertext,
			subscription_id, customer_mobile_e164, customer_email, external_sale_id,
			consumed_by_user_id, consumed_by_mobile_e164, activation_path
		) VALUES (
			gen_random_uuid(), $1, 'CONSUMED', $2, $3, now(), now(), now(),
			'',
			$4, '+5511987654321', 'x@example.com', 'sale-1', $5, '+5511987654321', 'direct'
		)`
	_, err := db.ExecContext(ctx, q, hash, planID, expiresAt, subID, userID)
	s.Require().NoError(err)
}

func (s *ActivateTelegramByTokenIntegrationSuite) assertUserIdentityRow(ctx context.Context, db database.DBTX, userID uuid.UUID, telegramUserID int64) {
	s.T().Helper()
	const q = `
		SELECT count(*) FROM mecontrola.user_identities
		 WHERE user_id = $1 AND channel = 'telegram' AND external_id = $2 AND unlinked_at IS NULL`
	var count int
	err := db.QueryRowContext(ctx, q, userID, strconv.FormatInt(telegramUserID, 10)).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count, "user_identities deve conter exatamente 1 linha telegram para o user")
}
