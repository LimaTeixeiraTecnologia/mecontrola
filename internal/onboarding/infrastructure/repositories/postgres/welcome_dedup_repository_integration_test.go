//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type WelcomeDedupRepositoryIntegrationSuite struct {
	suite.Suite
}

func TestWelcomeDedupRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(WelcomeDedupRepositoryIntegrationSuite))
}

func (s *WelcomeDedupRepositoryIntegrationSuite) TestInsertIfAbsentThenDedup() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewWelcomeDedupRepository(noop.NewProvider(), db)
	eventID := uuid.NewString()

	inserted, err := repo.InsertIfAbsent(ctx, eventID)
	s.Require().NoError(err)
	s.True(inserted)

	again, err := repo.InsertIfAbsent(ctx, eventID)
	s.Require().NoError(err)
	s.False(again)

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.onboarding_welcome_processed WHERE event_id = $1`, eventID,
	).Scan(&count))
	s.Equal(1, count)
}

func (s *WelcomeDedupRepositoryIntegrationSuite) TestDeleteCompensatesAndAllowsReinsert() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewWelcomeDedupRepository(noop.NewProvider(), db)
	eventID := uuid.NewString()

	inserted, err := repo.InsertIfAbsent(ctx, eventID)
	s.Require().NoError(err)
	s.True(inserted)

	s.Require().NoError(repo.Delete(ctx, eventID))

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.onboarding_welcome_processed WHERE event_id = $1`, eventID,
	).Scan(&count))
	s.Equal(0, count)

	reinserted, err := repo.InsertIfAbsent(ctx, eventID)
	s.Require().NoError(err)
	s.True(reinserted)
}

func (s *WelcomeDedupRepositoryIntegrationSuite) TestDeleteAbsentIsNoError() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewWelcomeDedupRepository(noop.NewProvider(), db)
	s.Require().NoError(repo.Delete(ctx, uuid.NewString()))
}
