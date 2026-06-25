//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/jmoiron/sqlx"
)

type MagicTokenRepositorySuite struct {
	suite.Suite
	db *sqlx.DB
}

func TestMagicTokenRepositorySuite(t *testing.T) {
	suite.Run(t, new(MagicTokenRepositorySuite))
}

func (s *MagicTokenRepositorySuite) SetupTest() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *MagicTokenRepositorySuite) newToken(planID string, expiresAt time.Time) entities.MagicToken {
	hash := make([]byte, 32)
	_, err := rand.Read(hash)
	s.Require().NoError(err)
	token, err := entities.NewMagicToken(uuid.NewString(), hash, planID, expiresAt)
	s.Require().NoError(err)
	return token
}

func (s *MagicTokenRepositorySuite) insertToken(ctx context.Context, repo interface {
	Insert(context.Context, entities.MagicToken) error
}, token entities.MagicToken) {
	s.Require().NoError(repo.Insert(ctx, token))
}

func (s *MagicTokenRepositorySuite) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *MagicTokenRepositorySuite) TestInsertAndFindByHash() {
	ctx := context.Background()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	token := s.newToken("plan-abc", time.Now().UTC().Add(24*time.Hour))
	s.insertToken(ctx, repo, token)

	var count int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM mecontrola.onboarding_tokens WHERE id = $1`, token.ID()).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)

	found, err := repo.FindByHash(ctx, token.TokenHash())
	s.Require().NoError(err)
	s.Equal(token.ID(), found.ID())
	s.Equal(valueobjects.TokenStatusPending, found.Status())
	s.Equal(token.PlanID(), found.PlanID())
	s.WithinDuration(token.ExpiresAt(), found.ExpiresAt(), time.Second)

	randomHash := make([]byte, 32)
	_, err = rand.Read(randomHash)
	s.Require().NoError(err)
	_, err = repo.FindByHash(ctx, randomHash)
	s.Require().ErrorIs(err, domain.ErrTokenNotFound)
}

func (s *MagicTokenRepositorySuite) TestUpdateMarkPaid() {
	ctx := context.Background()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	token := s.newToken("plan-paid", time.Now().UTC().Add(24*time.Hour))
	s.insertToken(ctx, repo, token)

	subID := uuid.NewString()
	paid, err := token.MarkPaid(subID, "+5511999990001", "user@example.com", "sale-001", time.Now().UTC())
	s.Require().NoError(err)

	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))

	var (
		status         string
		paidAt         sql.NullTime
		subscriptionID sql.NullString
		mobileE164     sql.NullString
		email          sql.NullString
	)
	err = s.queryRow(ctx,
		`SELECT status, paid_at, subscription_id, customer_mobile_e164, customer_email
		   FROM mecontrola.onboarding_tokens WHERE id = $1`,
		token.ID(),
	).Scan(&status, &paidAt, &subscriptionID, &mobileE164, &email)
	s.Require().NoError(err)
	s.Equal("PAID", status)
	s.True(paidAt.Valid)
	s.Equal(subID, subscriptionID.String)
	s.Equal("+5511999990001", mobileE164.String)
	s.Equal("user@example.com", email.String)

	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))

	var status2 string
	err = s.queryRow(ctx, `SELECT status FROM mecontrola.onboarding_tokens WHERE id = $1`, token.ID()).Scan(&status2)
	s.Require().NoError(err)
	s.Equal("PAID", status2)
}

func (s *MagicTokenRepositorySuite) TestUpdateMarkConsumed() {
	ctx := context.Background()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	token := s.newToken("plan-consume", time.Now().UTC().Add(24*time.Hour))
	s.insertToken(ctx, repo, token)

	paid, err := token.MarkPaid(uuid.NewString(), "+5511999990002", "consumer@example.com", "sale-002", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))

	userID := uuid.NewString()
	consumed, err := paid.MarkConsumed(userID, "+5511999990002", valueobjects.ActivationPathDirect, time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkConsumed(ctx, consumed))

	var (
		status         string
		consumedAt     sql.NullTime
		consumedByUser sql.NullString
		activationPath sql.NullString
	)
	err = s.queryRow(ctx,
		`SELECT status, consumed_at, consumed_by_user_id, activation_path
		   FROM mecontrola.onboarding_tokens WHERE id = $1`,
		token.ID(),
	).Scan(&status, &consumedAt, &consumedByUser, &activationPath)
	s.Require().NoError(err)
	s.Equal("CONSUMED", status)
	s.True(consumedAt.Valid)
	s.Equal(userID, consumedByUser.String)
	s.Equal("direct", activationPath.String)
}

func (s *MagicTokenRepositorySuite) TestBulkExpire() {
	ctx := context.Background()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	now := time.Now().UTC()

	expiredPending := s.newToken("plan-bulk", now.Add(-2*time.Hour))
	s.insertToken(ctx, repo, expiredPending)

	hash2 := make([]byte, 32)
	_, _ = rand.Read(hash2)
	expiredPaid, err := entities.NewMagicToken(uuid.NewString(), hash2, "plan-bulk", now.Add(-2*time.Hour))
	s.Require().NoError(err)
	s.insertToken(ctx, repo, expiredPaid)
	paidToken, err := expiredPaid.MarkPaid(uuid.NewString(), "+5511999991002", "bulk2@example.com", "sale-bulk-2", now.Add(-3*time.Hour))
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkPaid(ctx, paidToken))

	hash3 := make([]byte, 32)
	_, _ = rand.Read(hash3)
	notExpired, err := entities.NewMagicToken(uuid.NewString(), hash3, "plan-bulk", now.Add(24*time.Hour))
	s.Require().NoError(err)
	s.insertToken(ctx, repo, notExpired)

	expired, err := repo.BulkExpire(ctx, now, 10)
	s.Require().NoError(err)
	s.Len(expired, 2)

	expiredIDs := make(map[string]bool)
	for _, t := range expired {
		expiredIDs[t.ID()] = true
		s.Equal(valueobjects.TokenStatusExpired, t.Status())
	}
	s.True(expiredIDs[expiredPending.ID()])
	s.True(expiredIDs[expiredPaid.ID()])

	var statusExpiredPending string
	err = s.queryRow(ctx, `SELECT status FROM mecontrola.onboarding_tokens WHERE id = $1`, expiredPending.ID()).Scan(&statusExpiredPending)
	s.Require().NoError(err)
	s.Equal("EXPIRED", statusExpiredPending)

	var statusExpiredPaid string
	err = s.queryRow(ctx, `SELECT status FROM mecontrola.onboarding_tokens WHERE id = $1`, expiredPaid.ID()).Scan(&statusExpiredPaid)
	s.Require().NoError(err)
	s.Equal("EXPIRED", statusExpiredPaid)

	var statusNotExpired string
	err = s.queryRow(ctx, `SELECT status FROM mecontrola.onboarding_tokens WHERE id = $1`, notExpired.ID()).Scan(&statusNotExpired)
	s.Require().NoError(err)
	s.Equal("PENDING", statusNotExpired)
}

func (s *MagicTokenRepositorySuite) TestOutreachFlow() {
	ctx := context.Background()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	now := time.Now().UTC()

	token := s.newToken("plan-outreach", now.Add(24*time.Hour))
	s.insertToken(ctx, repo, token)

	paid, err := token.MarkPaid(uuid.NewString(), "+5511999992001", "out@example.com", "sale-out-1", now.Add(-2*time.Hour))
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))

	tokens, err := repo.FindPaidForOutreach(ctx, now, 10)
	s.Require().NoError(err)
	found := false
	for _, t := range tokens {
		if t.ID() == token.ID() {
			found = true
			break
		}
	}
	s.True(found)

	sentAt := now
	s.Require().NoError(repo.UpdateMarkOutreachSent(ctx, token.ID(), sentAt))

	var outreachSentAt sql.NullTime
	err = s.queryRow(ctx, `SELECT outreach_sent_at FROM mecontrola.onboarding_tokens WHERE id = $1`, token.ID()).Scan(&outreachSentAt)
	s.Require().NoError(err)
	s.True(outreachSentAt.Valid)

	tokens2, err := repo.FindPaidForOutreach(ctx, now, 10)
	s.Require().NoError(err)
	for _, t := range tokens2 {
		s.NotEqual(token.ID(), t.ID())
	}

	s.Require().NoError(repo.UpdateMarkOutreachReset(ctx, token.ID()))

	var outreachAfterReset sql.NullTime
	err = s.queryRow(ctx, `SELECT outreach_sent_at FROM mecontrola.onboarding_tokens WHERE id = $1`, token.ID()).Scan(&outreachAfterReset)
	s.Require().NoError(err)
	s.False(outreachAfterReset.Valid)
}

func (s *MagicTokenRepositorySuite) TestFindPaidByMobileForFallback() {
	ctx := context.Background()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	now := time.Now().UTC()
	mobile := "+5511988776655"

	token := s.newToken("plan-fallback", now.Add(24*time.Hour))
	s.insertToken(ctx, repo, token)

	paid, err := token.MarkPaid(uuid.NewString(), mobile, "fallback@example.com", "sale-fallback-1", now.Add(-time.Hour))
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))
	s.Require().NoError(repo.UpdateMarkOutreachSent(ctx, token.ID(), now))

	found, err := repo.FindPaidByMobileForFallback(ctx, mobile)
	s.Require().NoError(err)
	s.Equal(token.ID(), found.ID())
	s.Equal(valueobjects.TokenStatusPaid, found.Status())

	_, err = repo.FindPaidByMobileForFallback(ctx, "+5511000000000")
	s.Require().ErrorIs(err, domain.ErrTokenNotFound)
}

func (s *MagicTokenRepositorySuite) TestCountPaidUnconsumed() {
	ctx := context.Background()

	planID := "plan-count-" + uuid.NewString()
	repo := postgres.NewMagicTokenRepository(noop.NewProvider(), s.db)

	count0, err := repo.CountPaidUnconsumed(ctx)
	s.Require().NoError(err)
	s.Equal(int64(0), count0)

	now := time.Now().UTC()

	mobiles := []string{"+5511999993001", "+5511999993002"}
	for _, mobile := range mobiles {
		h := make([]byte, 32)
		_, _ = rand.Read(h)
		t, err := entities.NewMagicToken(uuid.NewString(), h, planID, now.Add(24*time.Hour))
		s.Require().NoError(err)
		s.Require().NoError(repo.Insert(ctx, t))
		paid, err := t.MarkPaid(uuid.NewString(), mobile, "count@example.com", "sale-count", now.Add(-time.Hour))
		s.Require().NoError(err)
		s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))
	}

	h3 := make([]byte, 32)
	_, _ = rand.Read(h3)
	t3, err := entities.NewMagicToken(uuid.NewString(), h3, planID, now.Add(24*time.Hour))
	s.Require().NoError(err)
	s.Require().NoError(repo.Insert(ctx, t3))
	paid3, err := t3.MarkPaid(uuid.NewString(), "+5511999993003", "count3@example.com", "sale-count-3", now.Add(-time.Hour))
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid3))
	consumed, err := paid3.MarkConsumed(uuid.NewString(), "+5511999993003", valueobjects.ActivationPathDirect, now)
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkConsumed(ctx, consumed))

	count2, err := repo.CountPaidUnconsumed(ctx)
	s.Require().NoError(err)
	s.Equal(int64(2), count2)
}
