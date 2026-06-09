//go:build integration

package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/jobs/handlers"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const pgImageJobInteg = "postgres:16"

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
	s.mgr = setupJobIntegrationDB(s.T())
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
			ev := entities.HydrateAuthEvent(id, oldTime, nil, entities.AuthEventKindUnknownUser, entities.AuthEventSourceWhatsApp, nil)
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

func setupJobIntegrationDB(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        pgImageJobInteg,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if terr := container.Terminate(context.Background()); terr != nil {
			t.Logf("container terminate: %v", terr)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	mapped, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	portNum, err := strconv.Atoi(mapped.Port())
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	cfg := dbpostgres.PostgresConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)
	migrator, err := migration.New(mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(dsn))
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}
	if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return mgr
}
