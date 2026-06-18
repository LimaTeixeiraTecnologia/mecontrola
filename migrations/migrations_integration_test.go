//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

type MigrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
	dsn string
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationSuite))
}

func (s *MigrationSuite) SetupSuite() {
	s.ctx = context.Background()

	db, dsn := testcontainer.Postgres(s.T())
	s.db = db
	s.dsn = dsn
}

func (s *MigrationSuite) SetupTest() {}

func (s *MigrationSuite) newMigrator() *migrate.Migrate {
	driver, err := migratepgx.WithInstance(s.db.DB, &migratepgx.Config{
		MigrationsTable: migratepgx.DefaultMigrationsTable,
		SchemaName:      "public",
	})
	s.Require().NoError(err)
	src, err := iofs.New(migrations.FS, ".")
	s.Require().NoError(err)
	migrator, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	s.Require().NoError(err)
	return migrator
}

func (s *MigrationSuite) applyBaseline(migrator *migrate.Migrate) {
	upErr := migrator.Up()
	s.Require().True(
		upErr == nil || errors.Is(upErr, migrate.ErrNoChange),
		"up deve ser idempotente: %v",
		upErr,
	)
}

func (s *MigrationSuite) downToVersion(migrator *migrate.Migrate, target uint) {
	for {
		version, dirty, err := migrator.Version()
		if err != nil {
			return
		}
		s.False(dirty)
		if version <= target {
			return
		}
		s.Require().NoError(migrator.Steps(-1))
	}
}

func (s *MigrationSuite) TestBaselineUpDownUp() {
	migrator := s.newMigrator()

	s.applyBaseline(migrator)

	s.assertTablePresent("mecontrola.billing_plans")
	s.assertTablePresent("mecontrola.categories")
	s.assertTablePresent("mecontrola.category_dictionary")
	s.assertTablePresent("mecontrola.idempotency_keys")
	s.assertTablePresent("mecontrola.cards")
	s.assertTablePresent("mecontrola.budgets")
	s.assertTablePresent("mecontrola.transactions")
	s.assertTablePresent("mecontrola.user_identities")
	s.assertTablePresent("mecontrola.channel_processed_messages")
	s.assertTablePresent("mecontrola.budget_alerts_sent")
	s.assertTablePresent("mecontrola.onboarding_sessions")

	s.assertTableMissing("mecontrola.meta_processed_messages")
	s.assertTableMissing("mecontrola.telegram_processed_updates")

	s.assertSeededPlans()
	s.assertExpenseCategoriesCount()
	s.assertIncomeCategoriesCount()
	s.assertDictionaryCanonicalsCount()
	s.assertDictionaryAliasesCount()
	s.assertUnaccentAvailable()

	s.downToVersion(migrator, 0)

	s.assertSchemaMissing("mecontrola")

	s.applyBaseline(migrator)

	s.assertTablePresent("mecontrola.billing_plans")
	s.assertTablePresent("mecontrola.channel_processed_messages")
}

func (s *MigrationSuite) TestFinalSchemaColumnsAndConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertColumnPresent("mecontrola.outbox_events", "aggregate_user_id")
	s.assertColumnPresent("mecontrola.auth_events", "request_id")
	s.assertColumnPresent("mecontrola.auth_events", "client_ip")
	s.assertColumnPresent("mecontrola.cards", "limit_cents")
	s.assertColumnPresent("mecontrola.cards", "version")
	s.assertColumnPresent("mecontrola.onboarding_tokens", "telegram_external_id")

	invalidSourceErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, occurred_at)
		VALUES ($1, 'principal_established', 'telegram', now())
	`, uuid.NewString())
	s.Require().Error(invalidSourceErr)
	s.Contains(invalidSourceErr.Error(), "auth_events_source_check")

	validGatewayErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.auth_events (id, kind, source, reason, request_id, client_ip, occurred_at)
		VALUES ($1, 'failed', 'gateway', 'stale_webhook', 'req-1', '127.0.0.1', now())
	`, uuid.NewString())
	s.Require().NoError(validGatewayErr)
}

func (s *MigrationSuite) TestChannelDedupAndUserIdentitiesConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	insertMessageErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ('whatsapp', 'wamid-1', now())
	`)
	s.Require().NoError(insertMessageErr)

	dupMessageErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ('whatsapp', 'wamid-1', now())
	`)
	s.Require().Error(dupMessageErr)
	s.Contains(dupMessageErr.Error(), "channel_processed_messages_pkey")

	userID := "12121212-1212-1212-1212-121212121212"
	insertUserErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999991212', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID)
	s.Require().NoError(insertUserErr)

	insertIdentityErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.user_identities (id, user_id, channel, external_id, verified_at, created_at)
		VALUES ($1, $2, 'telegram', 'external-1', now(), now())
	`, uuid.NewString(), userID)
	s.Require().NoError(insertIdentityErr)

	dupIdentityErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.user_identities (id, user_id, channel, external_id, verified_at, created_at)
		VALUES ($1, $2, 'telegram', 'external-1', now(), now())
	`, uuid.NewString(), userID)
	s.Require().Error(dupIdentityErr)
	s.Contains(dupIdentityErr.Error(), "user_identities_channel_external_active_uniq_idx")
}

func (s *MigrationSuite) TestIdempotencyKeysConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	validHash := "a948904f2f0f479b8f936f443923b14a04f830de30e39fa93ef91b5c6a8c5e10"

	statusErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 199, '', now() + interval '1 day')
	`, "key-status-test", userID, validHash)
	s.Require().Error(statusErr)
	s.Contains(statusErr.Error(), "idempotency_keys_status_chk")

	bodyErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 200, $4, now() + interval '1 day')
	`, "key-body-test", userID, validHash, make([]byte, 65537))
	s.Require().Error(bodyErr)
	s.Contains(bodyErr.Error(), "idempotency_keys_body_size_chk")

	longKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa129x"
	keyErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.idempotency_keys (scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ('card', $1, $2, $3, 200, '', now() + interval '1 day')
	`, longKey, userID, validHash)
	s.Require().Error(keyErr)
	s.Contains(keyErr.Error(), "idempotency_keys_key_len_chk")
}

func (s *MigrationSuite) TestCardsConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	insertUserErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999990099', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID)
	s.Require().NoError(insertUserErr)

	closingZeroErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', 'mycard', 0, 5)
	`, userID)
	s.Require().Error(closingZeroErr)
	s.Contains(closingZeroErr.Error(), "cards_closing_day_chk")

	closing32Err := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', 'mycard', 32, 5)
	`, userID)
	s.Require().Error(closing32Err)
	s.Contains(closing32Err.Error(), "cards_closing_day_chk")

	emptyNicknameErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', '', 10, 15)
	`, userID)
	s.Require().Error(emptyNicknameErr)
	s.Contains(emptyNicknameErr.Error(), "cards_nickname_len_chk")

	longNameErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, $2, 'mycard', 10, 15)
	`, userID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	s.Require().Error(longNameErr)
	s.Contains(longNameErr.Error(), "cards_name_len_chk")

	insertCardErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Valid Card', 'mycard', 10, 15)
	`, userID)
	s.Require().NoError(insertCardErr)

	dupNicknameErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day)
		VALUES (gen_random_uuid(), $1, 'Another Card', 'mycard', 15, 20)
	`, userID)
	s.Require().Error(dupNicknameErr)
	s.Contains(dupNicknameErr.Error(), "cards_user_nickname_active_uniq_idx")
}

func (s *MigrationSuite) TestCardsNoPCIColumns() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	prohibitedTerms := []string{"pan", "cvv", "cvc", "track", "pin"}
	for _, term := range prohibitedTerms {
		var count int64
		err := s.db.QueryRowContext(s.ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'mecontrola'
			  AND table_name = 'cards'
			  AND lower(column_name) LIKE '%' || $1 || '%'
		`, term).Scan(&count)
		s.Require().NoError(err)
		s.Equal(int64(0), count)
	}
}

func (s *MigrationSuite) TestCategoriesAndDictionarySeed() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertExpenseCategoriesCount()
	s.assertIncomeCategoriesCount()
	s.assertDeterministicCategoryIDs()
	s.assertAllocationTypesCorrect()
	s.assertMaxDepthTwo()
	s.assertEditorialVersionIncremented()
	s.assertDictionaryCanonicalsCount()
	s.assertDictionaryAliasesCount()
	s.assertDictionaryUniqueness()
	s.assertDictionaryUnaccentNormalization()
}

func (s *MigrationSuite) TestCategoryDictionarySeedV2_aliases() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	probes := []string{"ifood", "mercado", "uber", "gasolina", "curso", "viagem"}

	var presentCount int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary
		WHERE deprecated_at IS NULL
		  AND term_normalized = ANY($1)
	`, probes).Scan(&presentCount)
	s.Require().NoError(err)
	s.GreaterOrEqual(presentCount, int64(6))

	for _, term := range probes {
		var categoryID sql.NullString
		queryErr := s.db.QueryRowContext(s.ctx, `
			SELECT category_id::text FROM mecontrola.category_dictionary
			WHERE deprecated_at IS NULL AND term_normalized = $1
			ORDER BY confidence DESC
			LIMIT 1
		`, term).Scan(&categoryID)
		s.Require().NoErrorf(queryErr, "term=%s", term)
		s.Truef(categoryID.Valid, "term=%s sem category_id", term)
		s.NotEmptyf(categoryID.String, "term=%s mapeamento vazio", term)
	}

	var seedV2Count int64
	seedErr := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary
		WHERE id::text LIKE 'a1b00001-0000-5007-0000-%'
	`).Scan(&seedV2Count)
	s.Require().NoError(seedErr)
	s.GreaterOrEqualf(seedV2Count, int64(60), "seed v2 inseriu %d entradas", seedV2Count)
	s.LessOrEqualf(seedV2Count, int64(200), "seed v2 inseriu %d entradas", seedV2Count)

	var dupCount int64
	dupErr := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM (
			SELECT matched_term FROM (
				SELECT term_normalized AS matched_term, COUNT(*) AS c
				FROM mecontrola.category_dictionary
				WHERE deprecated_at IS NULL
				GROUP BY kind, category_id, term_normalized
				HAVING COUNT(*) > 1
			) inner_t
		) outer_t
	`).Scan(&dupCount)
	s.Require().NoError(dupErr)
	s.Equal(int64(0), dupCount)
}

func (s *MigrationSuite) TestTransactionsConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	userID := "aaaaaaaa-1111-1111-1111-aaaaaaaaaaaa"
	insertUserErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880010', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID)
	s.Require().NoError(insertUserErr)

	invalidAmountErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, category_name_snapshot, ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 1, 1, 0, 'teste',
			gen_random_uuid(), 'cat', '2026-06', now(), now(), now()
		)
	`, userID)
	s.Require().Error(invalidAmountErr)

	cardID := uuid.New()
	cardInvoiceID := uuid.New()
	insertInvoiceErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_card_invoices (
			id, user_id, card_id, ref_month, closing_at, due_at, created_at, updated_at
		) VALUES ($1, $2, $3, '2026-06', now(), now(), now(), now())
	`, cardInvoiceID, userID, cardID)
	s.Require().NoError(insertInvoiceErr)

	dupInvoiceErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_card_invoices (
			id, user_id, card_id, ref_month, closing_at, due_at, created_at, updated_at
		) VALUES (gen_random_uuid(), $1, $2, '2026-06', now(), now(), now(), now())
	`, userID, cardID)
	s.Require().Error(dupInvoiceErr)
	s.Contains(dupInvoiceErr.Error(), "transactions_card_invoices_uk")
}

func (s *MigrationSuite) TestBudgetsConstraints() {
	migrator := s.newMigrator()
	s.applyBaseline(migrator)

	s.assertBudgetsUniqueConstraint()
	s.assertBudgetsExpensesIdentityUnique()
	s.assertBudgetsExpensesPartialIndex()
	s.assertBudgetsThresholdStatesConstraints()
	s.assertBudgetsPendingEventIdempotency()
}

func (s *MigrationSuite) assertSeededPlans() {
	rows, err := s.db.QueryContext(s.ctx, `
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

func (s *MigrationSuite) assertTablePresent(name string) {
	var regclass sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.True(regclass.Valid)
}

func (s *MigrationSuite) assertTableMissing(name string) {
	var regclass sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&regclass)
	s.Require().NoError(err)
	s.False(regclass.Valid)
}

func (s *MigrationSuite) assertSchemaMissing(name string) {
	var exists bool
	err := s.db.QueryRowContext(s.ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_namespace
			WHERE nspname = $1
		)
	`, name).Scan(&exists)
	s.Require().NoError(err)
	s.False(exists)
}

func (s *MigrationSuite) assertColumnPresent(table, column string) {
	parts := splitTableName(table)
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND column_name = $3
	`, parts[0], parts[1], column).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(1), count)
}

func (s *MigrationSuite) assertUnaccentAvailable() {
	var result bool
	err := s.db.QueryRowContext(s.ctx, `SELECT unaccent('á') = 'a'`).Scan(&result)
	s.Require().NoError(err)
	s.True(result)
}

func (s *MigrationSuite) assertExpenseCategoriesCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.categories WHERE kind = 'expense'`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(88))
}

func (s *MigrationSuite) assertIncomeCategoriesCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM mecontrola.categories WHERE kind = 'income'`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(21))
}

func (s *MigrationSuite) assertDeterministicCategoryIDs() {
	rows, err := s.db.QueryContext(s.ctx, `SELECT slug, kind, id FROM mecontrola.categories`)
	s.Require().NoError(err)
	defer rows.Close()

	for rows.Next() {
		var slug string
		var kind string
		var id string
		s.Require().NoError(rows.Scan(&slug, &kind, &id))
	}
	s.Require().NoError(rows.Err())
}

func (s *MigrationSuite) assertAllocationTypesCorrect() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.categories
		WHERE kind = 'expense' AND parent_id IS NOT NULL AND allocation_type = 'asset_allocation'
	`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(1))
}

func (s *MigrationSuite) assertMaxDepthTwo() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.categories c1
		JOIN mecontrola.categories c2 ON c1.parent_id = c2.id
		WHERE c2.parent_id IS NOT NULL
	`).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(0), count)
}

func (s *MigrationSuite) assertEditorialVersionIncremented() {
	var version int64
	err := s.db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version`).Scan(&version)
	s.Require().NoError(err)
	s.GreaterOrEqual(version, int64(4))
}

func (s *MigrationSuite) assertDictionaryCanonicalsCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary WHERE signal_type = 'canonical_name'
	`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(60))
}

func (s *MigrationSuite) assertDictionaryAliasesCount() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM mecontrola.category_dictionary WHERE signal_type IN ('alias', 'phrase')
	`).Scan(&count)
	s.Require().NoError(err)
	s.GreaterOrEqual(count, int64(10))
}

func (s *MigrationSuite) assertDictionaryUniqueness() {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM (
			SELECT kind, category_id, term_normalized, COUNT(*) AS c
			FROM mecontrola.category_dictionary
			WHERE deprecated_at IS NULL
			GROUP BY kind, category_id, term_normalized
			HAVING COUNT(*) > 1
		) t
	`).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(0), count)
}

func (s *MigrationSuite) assertDictionaryUnaccentNormalization() {
	var result string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT term_normalized FROM mecontrola.category_dictionary WHERE term = 'gás encanado' LIMIT 1
	`).Scan(&result)
	s.Require().NoError(err)
	s.Equal("gas encanado", result)
}

func (s *MigrationSuite) assertBudgetsUniqueConstraint() {
	userID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, '2026-01', 0, 1, now(), now())
	`, userID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, '2026-01', 0, 1, now(), now())
	`, userID)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "budgets_user_comp_uk")
}

func (s *MigrationSuite) assertBudgetsExpensesIdentityUnique() {
	userID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expenses (
			id, user_id, source, external_transaction_id, subcategory_id,
			root_slug, competence, amount_cents, occurred_at, version,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 'api', 'ext-001',
			gen_random_uuid(), 'expense.custo_fixo', '2026-01',
			1000, now(), 1, now(), now()
		)
	`, userID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expenses (
			id, user_id, source, external_transaction_id, subcategory_id,
			root_slug, competence, amount_cents, occurred_at, version,
			created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 'api', 'ext-001',
			gen_random_uuid(), 'expense.custo_fixo', '2026-01',
			2000, now(), 1, now(), now()
		)
	`, userID)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "budgets_expenses_identity_uk")
}

func (s *MigrationSuite) assertBudgetsExpensesPartialIndex() {
	rows, err := s.db.QueryContext(s.ctx, `
		EXPLAIN SELECT root_slug, SUM(amount_cents)
		FROM mecontrola.budgets_expenses
		WHERE user_id = 'dddddddd-dddd-dddd-dddd-dddddddddddd'
		  AND competence = '2026-01'
		  AND deleted_at IS NULL
		GROUP BY root_slug
	`)
	s.Require().NoError(err)
	defer rows.Close()

	var hasRows bool
	for rows.Next() {
		hasRows = true
	}
	s.Require().NoError(rows.Err())
	s.True(hasRows)
}

func (s *MigrationSuite) assertBudgetsThresholdStatesConstraints() {
	userID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_threshold_states (
			user_id, competence, root_slug, threshold, currently_crossed, version
		) VALUES ($1, '2026-01', 'expense.custo_fixo', 80, false, 0)
	`, userID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_threshold_states (
			user_id, competence, root_slug, threshold, currently_crossed, version
		) VALUES ($1, '2026-01', 'expense.custo_fixo', 80, false, 0)
	`, userID)
	s.Require().Error(dupErr)

	invalidThresholdErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_threshold_states (
			user_id, competence, root_slug, threshold, currently_crossed, version
		) VALUES ($1, '2026-02', 'expense.custo_fixo', 50, false, 0)
	`, userID)
	s.Require().Error(invalidThresholdErr)
	s.Contains(invalidThresholdErr.Error(), "budgets_threshold_states_threshold_chk")
}

func (s *MigrationSuite) assertBudgetsPendingEventIdempotency() {
	eventID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	insertErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expense_events_pending (
			id, event_id, source, user_id, external_transaction_id,
			expected_version, mutation_kind, payload, state, received_at
		) VALUES (
			gen_random_uuid(), $1, 'api',
			'aaaaaaaa-0000-0000-0000-000000000001', 'ext-pending-001',
			1, 1, '{}', 1, now()
		)
	`, eventID)
	s.Require().NoError(insertErr)

	dupErr := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.budgets_expense_events_pending (
			id, event_id, source, user_id, external_transaction_id,
			expected_version, mutation_kind, payload, state, received_at
		) VALUES (
			gen_random_uuid(), $1, 'api',
			'aaaaaaaa-0000-0000-0000-000000000001', 'ext-pending-001',
			1, 1, '{}', 1, now()
		)
	`, eventID)
	s.Require().Error(dupErr)
	s.Contains(dupErr.Error(), "budgets_expense_events_pending_event_uk")
}

func splitTableName(qualified string) [2]string {
	for i := 0; i < len(qualified); i++ {
		if qualified[i] == '.' {
			return [2]string{qualified[:i], qualified[i+1:]}
		}
	}
	return [2]string{"public", qualified}
}

func execSQL(db database.DBTX, ctx context.Context, query string, args ...any) error {
	_, err := db.ExecContext(ctx, query, args...)
	return err
}
