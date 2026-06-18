//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type OnboardingCleanupRepositoryIntegrationSuite struct {
	suite.Suite
}

func TestOnboardingCleanupRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(OnboardingCleanupRepositoryIntegrationSuite))
}

func (s *OnboardingCleanupRepositoryIntegrationSuite) TestDeleteMetaProcessedOlderThan() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewOnboardingCleanupRepository(noop.NewProvider(), db)

	old := time.Now().UTC().Add(-2 * time.Hour)
	for i := 0; i < 3; i++ {
		_, err := db.ExecContext(ctx,
			`INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at) VALUES ($1, $2, $3)`,
			"whatsapp", uuid.NewString(), old,
		)
		s.Require().NoError(err)
	}

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	deleted, err := repo.DeleteMetaProcessedOlderThan(ctx, cutoff, 10)
	s.Require().NoError(err)
	s.Equal(int64(3), deleted)

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.channel_processed_messages`,
	).Scan(&count))
	s.Equal(0, count)

	for i := 0; i < 2; i++ {
		_, err := db.ExecContext(ctx,
			`INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at) VALUES ($1, $2, $3)`,
			"whatsapp", uuid.NewString(), old,
		)
		s.Require().NoError(err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at) VALUES ($1, $2, $3)`,
		"whatsapp", uuid.NewString(), time.Now().UTC().Add(1*time.Hour),
	)
	s.Require().NoError(err)

	deleted2, err := repo.DeleteMetaProcessedOlderThan(ctx, cutoff, 10)
	s.Require().NoError(err)
	s.Equal(int64(2), deleted2)

	var remaining int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.channel_processed_messages`,
	).Scan(&remaining))
	s.Equal(1, remaining)
}

func (s *OnboardingCleanupRepositoryIntegrationSuite) TestDeleteMetaProcessedOlderThan_Limit() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewOnboardingCleanupRepository(noop.NewProvider(), db)

	old := time.Now().UTC().Add(-2 * time.Hour)
	for i := 0; i < 5; i++ {
		_, err := db.ExecContext(ctx,
			`INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at) VALUES ($1, $2, $3)`,
			"whatsapp", uuid.NewString(), old,
		)
		s.Require().NoError(err)
	}

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	deleted, err := repo.DeleteMetaProcessedOlderThan(ctx, cutoff, 2)
	s.Require().NoError(err)
	s.Equal(int64(2), deleted)

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.channel_processed_messages`,
	).Scan(&count))
	s.Equal(3, count)
}

func (s *OnboardingCleanupRepositoryIntegrationSuite) TestDeleteMetaProcessedOlderThan_Empty() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewOnboardingCleanupRepository(noop.NewProvider(), db)

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	deleted, err := repo.DeleteMetaProcessedOlderThan(ctx, cutoff, 10)
	s.Require().NoError(err)
	s.Equal(int64(0), deleted)
}

func (s *OnboardingCleanupRepositoryIntegrationSuite) TestDeleteConsumerLookupAttemptsOlderThan() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewOnboardingCleanupRepository(noop.NewProvider(), db)

	old := time.Now().UTC().Add(-2 * time.Hour)
	for i := 0; i < 2; i++ {
		_, err := db.ExecContext(ctx,
			`INSERT INTO mecontrola.consumer_lookup_attempts (event_id, attempts, last_attempt_at) VALUES ($1, $2, $3)`,
			uuid.NewString(), 1, old,
		)
		s.Require().NoError(err)
	}

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	deleted, err := repo.DeleteConsumerLookupAttemptsOlderThan(ctx, cutoff, 10)
	s.Require().NoError(err)
	s.Equal(int64(2), deleted)

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.consumer_lookup_attempts`,
	).Scan(&count))
	s.Equal(0, count)

	_, err = db.ExecContext(ctx,
		`INSERT INTO mecontrola.consumer_lookup_attempts (event_id, attempts, last_attempt_at) VALUES ($1, $2, $3)`,
		uuid.NewString(), 1, old,
	)
	s.Require().NoError(err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO mecontrola.consumer_lookup_attempts (event_id, attempts, last_attempt_at) VALUES ($1, $2, $3)`,
		uuid.NewString(), 1, time.Now().UTC().Add(1*time.Hour),
	)
	s.Require().NoError(err)

	deleted2, err := repo.DeleteConsumerLookupAttemptsOlderThan(ctx, cutoff, 10)
	s.Require().NoError(err)
	s.Equal(int64(1), deleted2)

	var remaining int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.consumer_lookup_attempts`,
	).Scan(&remaining))
	s.Equal(1, remaining)
}

func (s *OnboardingCleanupRepositoryIntegrationSuite) TestDeleteConsumerLookupAttemptsOlderThan_Empty() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewOnboardingCleanupRepository(noop.NewProvider(), db)

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	deleted, err := repo.DeleteConsumerLookupAttemptsOlderThan(ctx, cutoff, 10)
	s.Require().NoError(err)
	s.Equal(int64(0), deleted)
}
