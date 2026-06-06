//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const pgImage = "postgres:16"

type MigrationSuite struct {
	suite.Suite
	ctx      context.Context
	mgr      manager.Manager
	dsn      string
	migrator migration.Migrator
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationSuite))
}

func (s *MigrationSuite) SetupSuite() {
	s.ctx = context.Background()

	req := tc.ContainerRequest{
		Image:        pgImage,
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

	container, err := tc.GenericContainer(s.ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		s.Require().NoError(container.Terminate(context.Background()))
	})

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)

	mapped, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	portNum, err := strconv.Atoi(mapped.Port())
	s.Require().NoError(err)

	cfg := dbpostgres.PostgresConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
	}

	mgr, err := manager.New(cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.T().Cleanup(func() {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	})

	s.dsn = fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)

	migrator, err := migration.New(s.mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(s.dsn))
	s.Require().NoError(err)
	s.migrator = migrator
}

func (s *MigrationSuite) TestUpAndDownForBillingPipelineMigrations() {
	s.Require().NoError(s.migrator.Up(s.ctx))

	s.assertSeededPlans()
	s.assertActiveSubscriptionUniqueIndex()

	s.Require().NoError(s.migrator.Down(s.ctx, 6))

	s.assertBillingTablesRemoved()
}

func (s *MigrationSuite) assertSeededPlans() {
	rows, err := s.mgr.DBTX(s.ctx).QueryContext(s.ctx, `
		SELECT code, duration_days
		FROM billing_plans
		ORDER BY duration_days
	`)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		s.Require().NoError(rows.Close())
	})

	type planRow struct {
		code         string
		durationDays int
	}

	var plans []planRow
	for rows.Next() {
		var row planRow
		s.Require().NoError(rows.Scan(&row.code, &row.durationDays))
		plans = append(plans, row)
	}
	s.Require().NoError(rows.Err())
	s.Require().Equal([]planRow{
		{code: "MONTHLY", durationDays: 30},
		{code: "QUARTERLY", durationDays: 90},
		{code: "ANNUAL", durationDays: 365},
	}, plans)
}

func (s *MigrationSuite) assertActiveSubscriptionUniqueIndex() {
	userID := "11111111-1111-1111-1111-111111111111"
	_, err := s.mgr.DBTX(s.ctx).ExecContext(s.ctx, `
		INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999990001', 'ACTIVE', now(), now())
	`, userID)
	s.Require().NoError(err)

	start := make(chan struct{})
	results := make(chan error, 2)
	inputs := [][4]string{
		{"22222222-2222-2222-2222-222222222222", "token-1", "order-1", "ACTIVE"},
		{"33333333-3333-3333-3333-333333333333", "token-2", "order-2", "PAST_DUE"},
	}

	for _, in := range inputs {
		go func(values [4]string) {
			<-start
			_, execErr := s.mgr.DBTX(s.ctx).ExecContext(s.ctx, `
				INSERT INTO billing_subscriptions (
					id, funnel_token, user_id, kiwify_order_id, plan_code, status,
					period_start, period_end, grace_end, last_event_at, created_at, updated_at
				) VALUES ($1, $2, $3, $4, 'MONTHLY', $5,
					now(), now() + interval '30 days', NULL, now(), now(), now())
			`, values[0], values[1], userID, values[2], values[3])
			results <- execErr
		}(in)
	}
	close(start)

	var successCount, uniqueViolationCount int
	for range 2 {
		resultErr := <-results
		if resultErr == nil {
			successCount++
			continue
		}
		var pgErr *pgconn.PgError
		if errors.As(resultErr, &pgErr) &&
			pgErr.Code == pgerrcode.UniqueViolation &&
			pgErr.ConstraintName == "billing_subscriptions_user_active_uniq_idx" {
			uniqueViolationCount++
		}
	}
	s.Equal(1, successCount)
	s.Equal(1, uniqueViolationCount)
}

func (s *MigrationSuite) assertBillingTablesRemoved() {
	s.assertTableMissing("billing_plans")
	s.assertTableMissing("billing_subscriptions")
	s.assertTableMissing("billing_processed_events")
	s.assertTableMissing("billing_kiwify_events")
	s.assertTableMissing("billing_reconciliation_checkpoints")
	s.assertTableMissing("identity_entitlements")
	s.assertTableMissing("identity_entitlements_pending")
}

func (s *MigrationSuite) assertTableMissing(name string) {
	var regclass sql.NullString
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.False(regclass.Valid)
}

func (s *MigrationSuite) TearDownSuite() {
	if s.migrator == nil {
		return
	}

	err := s.migrator.Down(s.ctx, 6)
	if err != nil && !errors.Is(err, migration.ErrNoChange) {
		s.T().Logf("cleanup migrations down: %v", err)
	}
}
