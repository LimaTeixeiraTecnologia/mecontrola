//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type OnboardingSessionRepositorySuite struct {
	suite.Suite
}

func TestOnboardingSessionRepositorySuite(t *testing.T) {
	suite.Run(t, new(OnboardingSessionRepositorySuite))
}

func (s *OnboardingSessionRepositorySuite) insertUser(ctx context.Context, db interface {
	ExecContext(ctx context.Context, sql string, args ...any) (any, error)
}) uuid.UUID {
	return uuid.New()
}

func (s *OnboardingSessionRepositorySuite) TestUpsertFindAndMarkActive() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	userID := uuid.New()
	number := "+5511" + uuid.New().String()[:9]
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)

	repo := postgres.NewOnboardingSessionRepository(noop.NewProvider(), db)

	_, err = repo.Find(ctx, userID)
	s.Require().ErrorIs(err, appinterfaces.ErrOnboardingSessionNotFound)

	initial, err := entities.NewOnboardingSession(
		userID,
		entities.OnboardingChannelWhatsApp,
		time.Now().UTC(),
	)
	s.Require().NoError(err)
	s.Require().NoError(repo.Upsert(ctx, initial))

	got, err := repo.Find(ctx, userID)
	s.Require().NoError(err)
	s.Equal(userID, got.UserID())
	s.Equal(entities.OnboardingChannelWhatsApp, got.Channel())
	s.False(got.IsActive())

	income, err := valueobjects.NewMonthlyIncome(350000)
	s.Require().NoError(err)
	updated := initial.WithIncome(income, time.Now().UTC())
	s.Require().NoError(repo.Upsert(ctx, updated))

	got2, err := repo.Find(ctx, userID)
	s.Require().NoError(err)
	s.False(got2.IsActive())
	s.Equal(int64(350000), got2.Payload().IncomeCents)

	completed := updated.WithCompletion(time.Now().UTC())
	s.Require().NoError(repo.Upsert(ctx, completed))

	got3, err := repo.Find(ctx, userID)
	s.Require().NoError(err)
	s.True(got3.IsActive())
	s.NotNil(got3.Payload().CompletedAt)
}
