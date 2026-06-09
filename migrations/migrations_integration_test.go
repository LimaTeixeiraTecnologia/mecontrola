//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

type MigrationSuite struct {
	suite.Suite
	ctx context.Context
	mgr manager.Manager
	dsn string
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationSuite))
}

func (s *MigrationSuite) SetupTest() {}

func (s *MigrationSuite) SetupSuite() {
	s.ctx = context.Background()

	mgr, dsn := testcontainer.Postgres(s.T())
	s.mgr = mgr
	s.dsn = dsn
}

func (s *MigrationSuite) TestUpAndDownForBillingPipelineMigrations() {
	type args struct {
		downSteps int
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(migrator migration.Migrator, downSteps int)
	}{
		{
			name:  "deve aplicar e reverter a baseline consolidada",
			args:  args{downSteps: 8},
			setup: func() {},
			expect: func(migrator migration.Migrator, downSteps int) {
				upErr := migrator.Up(s.ctx)
				s.Require().True(upErr == nil || errors.Is(upErr, migration.ErrNoChange),
					"up deve ser idempotente: %v", upErr)
				s.assertSeededPlans()
				s.assertActiveSubscriptionUniqueIndex()
				s.Require().NoError(migrator.Down(s.ctx, downSteps))
				s.assertSchemaMissing("mecontrola")
				s.assertTableMissing("mecontrola.billing_plans")
				s.assertTableMissing("mecontrola.billing_subscriptions")
				s.assertTableMissing("mecontrola.billing_processed_events")
				s.assertTableMissing("mecontrola.billing_kiwify_events")
			},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		s.Run(scenario.name, func() {
			scenario.setup()

			migrator, err := migration.New(
				s.mgr,
				migration.EmbedFS{FS: migrations.FS, Root: "."},
				migration.WithDSN(s.dsn),
			)
			s.Require().NoError(err)

			scenario.expect(migrator, scenario.args.downSteps)
		})
	}
}

func (s *MigrationSuite) TestCardAndIdempotencyMigrationsUpDownUp() {
	scenarios := []struct {
		name   string
		expect func(migrator migration.Migrator)
	}{
		{
			name: "deve aplicar, reverter e reaplicar migrations de idempotency_keys e cards",
			expect: func(migrator migration.Migrator) {
				upErr := migrator.Up(s.ctx)
				s.Require().True(upErr == nil || errors.Is(upErr, migration.ErrNoChange),
					"up deve ser idempotente: %v", upErr)

				s.assertTablePresent("mecontrola.idempotency_keys")
				s.assertTablePresent("mecontrola.cards")

				s.Require().NoError(migrator.Down(s.ctx, 2))

				s.assertTableMissing("mecontrola.idempotency_keys")
				s.assertTableMissing("mecontrola.cards")
				s.assertTablePresent("mecontrola.idempotency_keys_archived_20260609120000")
				s.assertTablePresent("mecontrola.cards_archived_20260609120000")

				s.Require().NoError(migrator.Up(s.ctx))

				s.assertTablePresent("mecontrola.idempotency_keys")
				s.assertTablePresent("mecontrola.cards")
			},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		s.Run(scenario.name, func() {
			migrator, err := migration.New(
				s.mgr,
				migration.EmbedFS{FS: migrations.FS, Root: "."},
				migration.WithDSN(s.dsn),
			)
			s.Require().NoError(err)

			scenario.expect(migrator)
		})
	}
}

func (s *MigrationSuite) TestIdempotencyKeysConstraints() {
	userID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	validHash := "a948904f2f0f479b8f936f443923b14a04f830de30e39fa93ef91b5c6a8c5e10"

	statusErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 199, '', now() + interval '1 day')
	`, "key-status-test", userID, validHash)
	s.Require().Error(statusErr, "response_status=199 deve violar check")
	s.Contains(statusErr.Error(), "idempotency_keys_status_chk")

	bodyErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 200, $4, now() + interval '1 day')
	`, "key-body-test", userID, validHash, make([]byte, 65537))
	s.Require().Error(bodyErr, "response_body > 65536 bytes deve violar check")
	s.Contains(bodyErr.Error(), "idempotency_keys_body_size_chk")

	longKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa129x"
	keyErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 200, '', now() + interval '1 day')
	`, longKey, userID, validHash)
	s.Require().Error(keyErr, "key com 133 chars deve violar check")
	s.Contains(keyErr.Error(), "idempotency_keys_key_len_chk")
}

func (s *MigrationSuite) TestCardsConstraints() {
	userID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	insertUserErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999990099', 'ACTIVE', now(), now())
	`, userID)
	s.Require().NoError(insertUserErr)

	closingZeroErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', 'mycard', 0, 5)
	`, userID)
	s.Require().Error(closingZeroErr, "closing_day=0 deve violar check")
	s.Contains(closingZeroErr.Error(), "cards_closing_day_chk")

	closing32Err := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', 'mycard', 32, 5)
	`, userID)
	s.Require().Error(closing32Err, "closing_day=32 deve violar check")
	s.Contains(closing32Err.Error(), "cards_closing_day_chk")

	emptyNicknameErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', '', 10, 15)
	`, userID)
	s.Require().Error(emptyNicknameErr, "nickname vazio deve violar check")
	s.Contains(emptyNicknameErr.Error(), "cards_nickname_len_chk")

	longNameErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, $2, 'mycard', 10, 15)
	`, userID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	s.Require().Error(longNameErr, "name com 65 chars deve violar check")
	s.Contains(longNameErr.Error(), "cards_name_len_chk")

	insertCardErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', 'mycard', 10, 15)
	`, userID)
	s.Require().NoError(insertCardErr)

	dupNicknameErr := execSQL(s.mgr, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Another Card', 'mycard', 15, 20)
	`, userID)
	s.Require().Error(dupNicknameErr, "nickname duplicado entre ativos deve violar unique index")
	s.Contains(dupNicknameErr.Error(), "cards_user_nickname_active_uniq_idx")
}

func (s *MigrationSuite) TestCardsNoPCIColumns() {
	prohibitedTerms := []string{"pan", "cvv", "cvc", "track", "pin"}
	for _, term := range prohibitedTerms {
		var count int64
		err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'mecontrola'
			  AND table_name = 'cards'
			  AND lower(column_name) LIKE '%' || $1 || '%'
		`, term).Scan(&count)
		s.Require().NoError(err)
		s.Equal(int64(0), count, "tabela cards nao deve ter coluna contendo '%s'", term)
	}
}

func (s *MigrationSuite) assertSeededPlans() {
	rows, err := s.mgr.DBTX(s.ctx).QueryContext(s.ctx, `
		SELECT code, duration_days
		FROM mecontrola.billing_plans
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
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
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
				INSERT INTO mecontrola.billing_subscriptions (
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

func (s *MigrationSuite) assertTablePresent(name string) {
	var regclass sql.NullString
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.True(regclass.Valid)
}

func (s *MigrationSuite) assertTableMissing(name string) {
	var regclass sql.NullString
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.False(regclass.Valid)
}

func (s *MigrationSuite) assertSchemaMissing(name string) {
	var exists bool
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_namespace
			WHERE nspname = $1
		)
	`, name).Scan(&exists)
	s.Require().NoError(err)
	s.False(exists)
}

func execSQL(mgr manager.Manager, ctx context.Context, query string, args ...any) error {
	_, err := mgr.DBTX(ctx).ExecContext(ctx, query, args...)
	return err
}
