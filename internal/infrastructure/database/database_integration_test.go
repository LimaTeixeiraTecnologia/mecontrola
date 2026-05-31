//go:build integration

package database_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	devkitdb "github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
)

type DatabaseIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	cfg *configs.Config
	mgr *dbpkg.Manager
}

func TestDatabaseIntegration(t *testing.T) {
	suite.Run(t, new(DatabaseIntegrationSuite))
}

func (s *DatabaseIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	s.cfg = s.startPostgres()

	mgr, err := dbpkg.NewManager(s.cfg)
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *DatabaseIntegrationSuite) TearDownTest() {
	_ = s.mgr.Shutdown(context.Background())
}

func (s *DatabaseIntegrationSuite) startPostgres() *configs.Config {
	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)

	mappedPort, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(mappedPort.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 5,
			MinConns: 1,
		},
	}
}

func (s *DatabaseIntegrationSuite) TestPoolStartup() {
	scenarios := []struct {
		name string
	}{
		{name: "deve conectar ao postgres e responder ao ping com sucesso"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			defer cancel()

			err := s.mgr.Inner().Ping(ctx)
			s.NoError(err)
		})
	}
}

func (s *DatabaseIntegrationSuite) TestMigrateUpDown() {
	scenarios := []struct {
		name string
	}{
		{name: "deve aplicar e reverter migrations sem erro e tolerar chamadas idempotentes"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			err := dbpkg.RunMigrations(s.ctx, s.mgr)
			s.NoError(err)

			err = dbpkg.RunMigrations(s.ctx, s.mgr)
			s.NoError(err)

			err = dbpkg.RunMigrationsDown(s.ctx, s.mgr)
			s.NoError(err)

			err = dbpkg.RunMigrationsDown(s.ctx, s.mgr)
			s.NoError(err)
		})
	}
}

func (s *DatabaseIntegrationSuite) TestHealthCheck() {
	scenarios := []struct {
		name string
	}{
		{name: "deve retornar nil com tabela health_probe presente e ErrConnection após remoção"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			defer cancel()

			s.Require().NoError(dbpkg.RunMigrations(ctx, s.mgr))

			err := s.mgr.HealthCheck(ctx)
			s.NoError(err)

			s.Require().NoError(dbpkg.RunMigrationsDown(ctx, s.mgr))

			err = s.mgr.HealthCheck(ctx)
			s.Error(err)
			s.True(errors.Is(err, dbpkg.ErrConnection))
		})
	}
}

func (s *DatabaseIntegrationSuite) TestUoWCommit() {
	scenarios := []struct {
		name string
	}{
		{name: "deve commitar transação e tornar dados visíveis"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))

			uow := dbpkg.NewUnitOfWork[string](s.mgr)

			result, err := uow.Do(s.ctx, func(ctx context.Context, tx devkitdb.DBTX) (string, error) {
				row := tx.QueryRowContext(ctx, "SELECT note FROM health_probe LIMIT 1")
				var note string
				if scanErr := row.Scan(&note); scanErr != nil {
					return "", fmt.Errorf("scan: %w", scanErr)
				}
				return note, nil
			})

			s.NoError(err)
			s.Equal("ok", result)
		})
	}
}

func (s *DatabaseIntegrationSuite) TestUoWRollback() {
	scenarios := []struct {
		name string
	}{
		{name: "deve reverter INSERT quando função retorna erro"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))

			rowsBefore := s.countHealthProbeRows()

			uow := dbpkg.NewUnitOfWork[struct{}](s.mgr)

			_, err := uow.Do(s.ctx, func(ctx context.Context, tx devkitdb.DBTX) (struct{}, error) {
				if _, execErr := tx.ExecContext(ctx, "INSERT INTO health_probe (note) VALUES ($1)", "rollback-test"); execErr != nil {
					return struct{}{}, fmt.Errorf("insert: %w", execErr)
				}
				return struct{}{}, errors.New("rollback intencional")
			})
			s.Error(err)
			s.EqualError(err, "rollback intencional")

			rowsAfter := s.countHealthProbeRows()
			s.Equal(rowsBefore, rowsAfter)
		})
	}
}

func (s *DatabaseIntegrationSuite) countHealthProbeRows() int {
	row := s.mgr.Inner().DBTX(s.ctx).QueryRowContext(s.ctx, "SELECT COUNT(*) FROM health_probe")
	var n int
	s.Require().NoError(row.Scan(&n))
	return n
}
