//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/jobs/handlers"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type AuthEventsHousekeepingIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	mgr manager.Manager
}

func TestAuthEventsHousekeepingIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AuthEventsHousekeepingIntegrationSuite))
}

func (s *AuthEventsHousekeepingIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthEventsHousekeepingIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	s.mgr = mgr
}

func (s *AuthEventsHousekeepingIntegrationSuite) newJob(cfg configs.IdentityConfig) *jobhandlers.AuthEventsHousekeepingJob {
	o11y := noop.NewProvider()
	factory := identityrepos.NewRepositoryFactory(o11y)
	cleanup := usecases.NewCleanupAuthEvents(s.mgr, factory, cfg, o11y)
	return jobhandlers.NewAuthEventsHousekeepingJob(cleanup, cfg)
}

func (s *AuthEventsHousekeepingIntegrationSuite) TestRunDeletes25kIn3Batches() {
	s.Run("deve apagar 25k linhas em 3 lotes verificaveis", func() {
		o11y := noop.NewProvider()
		factory := identityrepos.NewRepositoryFactory(o11y)
		repo := factory.AuthEventsRepository(s.mgr.DBTX(s.ctx))

		oldTime := time.Now().UTC().Add(-200 * 24 * time.Hour) // 200 dias atrás (além dos 180)

		for range 25_000 {
			id, err := uuid.NewV7()
			s.Require().NoError(err)
			ev := entities.HydrateAuthEvent(id, oldTime, nil, entities.AuthEventKindUnknownUser, entities.AuthEventSourceWhatsApp, nil, "", "")
			s.Require().NoError(repo.Insert(s.ctx, ev))
		}

		cfg := configs.IdentityConfig{
			AuthEventsRetentionDays:        180,
			AuthEventsHousekeepingBatch:    10_000,
			AuthEventsHousekeepingSchedule: "@monthly",
		}
		job := s.newJob(cfg)

		err := job.Run(s.ctx)
		s.Require().NoError(err)

		var count int
		err = s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM auth_events WHERE occurred_at < $1",
			time.Now().UTC().Add(-180*24*time.Hour),
		).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count, "nao deve restar linhas mais antigas que 180 dias")
	})
}

func (s *AuthEventsHousekeepingIntegrationSuite) TestRunIdempotent() {
	s.Run("segunda execucao e no-op", func() {
		cfg := configs.IdentityConfig{
			AuthEventsRetentionDays:        180,
			AuthEventsHousekeepingBatch:    10_000,
			AuthEventsHousekeepingSchedule: "@monthly",
		}
		job := s.newJob(cfg)

		s.Require().NoError(job.Run(s.ctx))
		s.Require().NoError(job.Run(s.ctx))
	})
}
