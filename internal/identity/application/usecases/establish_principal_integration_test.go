//go:build integration

package usecases_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const integrationPgImage = "postgres:16"

type EstablishPrincipalIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	mgr  manager.Manager
	o11y *noop.Provider
}

func TestEstablishPrincipalIntegration(t *testing.T) {
	suite.Run(t, new(EstablishPrincipalIntegrationSuite))
}

func (s *EstablishPrincipalIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *EstablishPrincipalIntegrationSuite) SetupSuite() {
	s.mgr = setupEstablishTestDB(s.T())
	s.o11y = noop.NewProvider()
}

func setupEstablishTestDB(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        integrationPgImage,
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
		DSN: fmt.Sprintf("postgres://test:test@%s:%d/testdb?sslmode=disable&search_path=mecontrola,public", host, portNum),
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

func (s *EstablishPrincipalIntegrationSuite) outboxCfg() configs.OutboxConfig {
	return configs.OutboxConfig{RetryMaxAttempts: 3}
}

func (s *EstablishPrincipalIntegrationSuite) newPublisher() outbox.Publisher {
	storage := outbox.NewPostgresStorage(s.mgr.DBTX(s.ctx))
	return outbox.NewPostgresPublisher(storage, s.outboxCfg())
}

func (s *EstablishPrincipalIntegrationSuite) seedActiveUser(wa string) entities.User {
	s.T().Helper()
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.mgr.DBTX(s.ctx))
	waNum, err := valueobjects.NewWhatsAppNumber(wa)
	s.Require().NoError(err)
	candidate := entities.New(waNum)
	user, err := repo.UpsertByWhatsAppNumber(s.ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	return user
}

func (s *EstablishPrincipalIntegrationSuite) countOutboxByType(eventType string) int {
	var total int
	err := s.mgr.DBTX(s.ctx).QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`,
		eventType,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *EstablishPrincipalIntegrationSuite) newSUT() *usecases.EstablishPrincipal {
	factory := repositories.NewRepositoryFactory(s.o11y)
	u := uow.New[usecases.EstablishResult](s.mgr, uow.WithObservability(s.o11y))
	return usecases.NewEstablishPrincipal(u, factory, s.newPublisher(), s.o11y)
}

func (s *EstablishPrincipalIntegrationSuite) TestEstablishPrincipal() {
	type args struct {
		waNumber string
	}

	scenarios := []struct {
		name   string
		setup  func() args
		expect func(auth.Principal, error)
	}{
		{
			name: "usuario ativo: retorna Principal e linha outbox auth.principal_established",
			setup: func() args {
				const wa = "+5511900000001"
				s.seedActiveUser(wa)
				return args{waNumber: wa}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.False(p.IsZero())
				s.Equal(auth.SourceWhatsApp, p.Source)
				s.GreaterOrEqual(s.countOutboxByType("auth.principal_established"), 1)
			},
		},
		{
			name: "usuario inexistente: retorna ErrUnknownUser e linha outbox auth.unknown_user",
			setup: func() args {
				return args{waNumber: "+5511900000099"}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
				s.GreaterOrEqual(s.countOutboxByType("auth.unknown_user"), 1)
			},
		},
		{
			name: "rollback observavel: outbox invalido causa erro e nenhuma linha adicional e inserida",
			setup: func() args {
				const wa = "+5511900000002"
				s.seedActiveUser(wa)
				return args{waNumber: wa}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.False(p.IsZero())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.setup()
			sut := s.newSUT()
			p, err := sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: a.waNumber})
			scenario.expect(p, err)
		})
	}
}
