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
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

type CategoryWriteGate000004Suite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestCategoryWriteGate000004Suite(t *testing.T) {
	suite.Run(t, new(CategoryWriteGate000004Suite))
}

func (s *CategoryWriteGate000004Suite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *CategoryWriteGate000004Suite) newMigrator() *migrate.Migrate {
	_, err := s.db.ExecContext(s.ctx, `CREATE SCHEMA IF NOT EXISTS mecontrola`)
	s.Require().NoError(err)
	driver, err := migratepgx.WithInstance(s.db.DB, &migratepgx.Config{
		MigrationsTable: migratepgx.DefaultMigrationsTable,
		SchemaName:      "mecontrola",
	})
	s.Require().NoError(err)
	src, err := iofs.New(migrations.FS, ".")
	s.Require().NoError(err)
	migrator, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	s.Require().NoError(err)
	return migrator
}

func (s *CategoryWriteGate000004Suite) applyUp(migrator *migrate.Migrate) {
	upErr := migrator.Up()
	s.Require().True(upErr == nil || errors.Is(upErr, migrate.ErrNoChange), "up idempotente: %v", upErr)
}

func (s *CategoryWriteGate000004Suite) TestCategoryWriteGateUpThenDownThenUp() {
	migrator := s.newMigrator()
	s.applyUp(migrator)

	s.assertVersion(migrator, 5)

	s.assertColumnPresent("transactions", "category_kind")
	s.assertColumnPresent("transactions", "category_path")
	s.assertColumnPresent("transactions", "category_outcome")
	s.assertColumnPresent("transactions", "category_score")
	s.assertColumnPresent("transactions", "category_confidence")
	s.assertColumnPresent("transactions", "category_match_quality")
	s.assertColumnPresent("transactions", "category_signal_type")
	s.assertColumnPresent("transactions", "category_matched_term")
	s.assertColumnPresent("transactions", "category_match_reason")
	s.assertColumnPresent("transactions", "category_decision_source")
	s.assertColumnPresent("transactions", "category_editorial_version")
	s.assertColumnPresent("transactions", "category_decided_at")

	s.assertConstraintPresent("transactions_direction_chk")
	s.assertConstraintPresent("transactions_category_name_snapshot_chk")
	s.assertConstraintPresent("transactions_subcategory_name_snapshot_chk")
	s.assertConstraintPresent("transactions_category_kind_chk")
	s.assertConstraintPresent("transactions_category_path_chk")
	s.assertConstraintPresent("transactions_category_outcome_chk")
	s.assertConstraintPresent("transactions_category_score_chk")
	s.assertConstraintPresent("transactions_category_confidence_chk")
	s.assertConstraintPresent("transactions_category_match_quality_chk")
	s.assertConstraintPresent("transactions_category_signal_type_chk")
	s.assertConstraintPresent("transactions_category_matched_term_chk")
	s.assertConstraintPresent("transactions_category_match_reason_chk")
	s.assertConstraintPresent("transactions_category_decision_source_chk")
	s.assertConstraintPresent("transactions_category_editorial_version_chk")
	s.assertConstraintPresent("transactions_subcategory_ne_category_chk")
	s.assertConstraintPresent("transactions_category_fk")
	s.assertConstraintPresent("transactions_subcategory_fk")

	s.assertTriggerPresent("mecontrola", "transactions", "transactions_category_write_gate_trg")

	s.assertColumnPresent("transactions_recurring_templates", "category_kind")
	s.assertColumnPresent("transactions_recurring_templates", "category_path")
	s.assertColumnPresent("transactions_recurring_templates", "category_outcome")
	s.assertColumnPresent("transactions_recurring_templates", "category_score")
	s.assertColumnPresent("transactions_recurring_templates", "category_confidence")
	s.assertColumnPresent("transactions_recurring_templates", "category_match_quality")
	s.assertColumnPresent("transactions_recurring_templates", "category_signal_type")
	s.assertColumnPresent("transactions_recurring_templates", "category_matched_term")
	s.assertColumnPresent("transactions_recurring_templates", "category_match_reason")
	s.assertColumnPresent("transactions_recurring_templates", "category_decision_source")
	s.assertColumnPresent("transactions_recurring_templates", "category_editorial_version")
	s.assertColumnPresent("transactions_recurring_templates", "category_decided_at")

	s.assertConstraintPresent("transactions_rt_direction_chk")
	s.assertConstraintPresent("transactions_rt_category_name_snapshot_chk")
	s.assertConstraintPresent("transactions_rt_subcategory_name_snapshot_chk")
	s.assertConstraintPresent("transactions_rt_category_kind_chk")
	s.assertConstraintPresent("transactions_rt_category_path_chk")
	s.assertConstraintPresent("transactions_rt_category_outcome_chk")
	s.assertConstraintPresent("transactions_rt_category_score_chk")
	s.assertConstraintPresent("transactions_rt_category_confidence_chk")
	s.assertConstraintPresent("transactions_rt_category_match_quality_chk")
	s.assertConstraintPresent("transactions_rt_category_signal_type_chk")
	s.assertConstraintPresent("transactions_rt_category_matched_term_chk")
	s.assertConstraintPresent("transactions_rt_category_match_reason_chk")
	s.assertConstraintPresent("transactions_rt_category_decision_source_chk")
	s.assertConstraintPresent("transactions_rt_category_editorial_version_chk")
	s.assertConstraintPresent("transactions_rt_subcategory_ne_category_chk")
	s.assertConstraintPresent("transactions_rt_category_fk")
	s.assertConstraintPresent("transactions_rt_subcategory_fk")

	s.assertTriggerPresent("mecontrola", "transactions_recurring_templates", "transactions_recurring_templates_category_write_gate_trg")

	s.assertIndexPresent("mecontrola", "transactions_category_id_idx")
	s.assertIndexPresent("mecontrola", "transactions_subcategory_id_idx")
	s.assertIndexPresent("mecontrola", "transactions_recurring_templates_category_id_idx")
	s.assertIndexPresent("mecontrola", "transactions_recurring_templates_subcategory_id_idx")

	s.assertFunctionPresent("mecontrola", "validate_category_write_gate")

	s.Require().NoError(migrator.Steps(-2))
	s.assertVersion(migrator, 3)

	s.assertColumnMissing("transactions", "category_kind")
	s.assertColumnMissing("transactions", "category_path")
	s.assertColumnMissing("transactions", "category_outcome")
	s.assertColumnMissing("transactions", "category_score")
	s.assertColumnMissing("transactions", "category_confidence")
	s.assertColumnMissing("transactions", "category_match_quality")
	s.assertColumnMissing("transactions", "category_signal_type")
	s.assertColumnMissing("transactions", "category_matched_term")
	s.assertColumnMissing("transactions", "category_match_reason")
	s.assertColumnMissing("transactions", "category_decision_source")
	s.assertColumnMissing("transactions", "category_editorial_version")
	s.assertColumnMissing("transactions", "category_decided_at")

	s.assertConstraintMissing("transactions_direction_chk")
	s.assertConstraintMissing("transactions_category_name_snapshot_chk")
	s.assertConstraintMissing("transactions_subcategory_name_snapshot_chk")
	s.assertConstraintMissing("transactions_category_kind_chk")
	s.assertConstraintMissing("transactions_category_path_chk")
	s.assertConstraintMissing("transactions_category_outcome_chk")
	s.assertConstraintMissing("transactions_category_score_chk")
	s.assertConstraintMissing("transactions_category_confidence_chk")
	s.assertConstraintMissing("transactions_category_match_quality_chk")
	s.assertConstraintMissing("transactions_category_signal_type_chk")
	s.assertConstraintMissing("transactions_category_matched_term_chk")
	s.assertConstraintMissing("transactions_category_match_reason_chk")
	s.assertConstraintMissing("transactions_category_decision_source_chk")
	s.assertConstraintMissing("transactions_category_editorial_version_chk")
	s.assertConstraintMissing("transactions_subcategory_ne_category_chk")
	s.assertConstraintMissing("transactions_category_fk")
	s.assertConstraintMissing("transactions_subcategory_fk")

	s.assertTriggerMissing("mecontrola", "transactions", "transactions_category_write_gate_trg")

	s.assertIndexMissing("mecontrola", "transactions_category_id_idx")
	s.assertIndexMissing("mecontrola", "transactions_subcategory_id_idx")
	s.assertIndexMissing("mecontrola", "transactions_recurring_templates_category_id_idx")
	s.assertIndexMissing("mecontrola", "transactions_recurring_templates_subcategory_id_idx")

	s.assertColumnMissing("transactions_recurring_templates", "category_kind")
	s.assertColumnMissing("transactions_recurring_templates", "category_path")
	s.assertColumnMissing("transactions_recurring_templates", "category_outcome")
	s.assertColumnMissing("transactions_recurring_templates", "category_score")
	s.assertColumnMissing("transactions_recurring_templates", "category_confidence")
	s.assertColumnMissing("transactions_recurring_templates", "category_match_quality")
	s.assertColumnMissing("transactions_recurring_templates", "category_signal_type")
	s.assertColumnMissing("transactions_recurring_templates", "category_matched_term")
	s.assertColumnMissing("transactions_recurring_templates", "category_match_reason")
	s.assertColumnMissing("transactions_recurring_templates", "category_decision_source")
	s.assertColumnMissing("transactions_recurring_templates", "category_editorial_version")
	s.assertColumnMissing("transactions_recurring_templates", "category_decided_at")

	s.assertConstraintMissing("transactions_rt_direction_chk")
	s.assertConstraintMissing("transactions_rt_category_name_snapshot_chk")
	s.assertConstraintMissing("transactions_rt_subcategory_name_snapshot_chk")
	s.assertConstraintMissing("transactions_rt_category_kind_chk")
	s.assertConstraintMissing("transactions_rt_category_path_chk")
	s.assertConstraintMissing("transactions_rt_category_outcome_chk")
	s.assertConstraintMissing("transactions_rt_category_score_chk")
	s.assertConstraintMissing("transactions_rt_category_confidence_chk")
	s.assertConstraintMissing("transactions_rt_category_match_quality_chk")
	s.assertConstraintMissing("transactions_rt_category_signal_type_chk")
	s.assertConstraintMissing("transactions_rt_category_matched_term_chk")
	s.assertConstraintMissing("transactions_rt_category_match_reason_chk")
	s.assertConstraintMissing("transactions_rt_category_decision_source_chk")
	s.assertConstraintMissing("transactions_rt_category_editorial_version_chk")
	s.assertConstraintMissing("transactions_rt_subcategory_ne_category_chk")
	s.assertConstraintMissing("transactions_rt_category_fk")
	s.assertConstraintMissing("transactions_rt_subcategory_fk")

	s.assertTriggerMissing("mecontrola", "transactions_recurring_templates", "transactions_recurring_templates_category_write_gate_trg")

	s.applyUp(migrator)
	s.assertVersion(migrator, 5)
	s.assertColumnPresent("transactions", "category_kind")
	s.assertConstraintPresent("transactions_category_fk")
	s.assertTriggerPresent("mecontrola", "transactions", "transactions_category_write_gate_trg")
}

func (s *CategoryWriteGate000004Suite) TestCategoryWriteGateBlocksInvalidRows() {
	migrator := s.newMigrator()
	s.applyUp(migrator)

	var expenseRootID, expenseLeafID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id IS NULL LIMIT 1`,
	).Scan(&expenseRootID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id=$1::uuid LIMIT 1`,
		expenseRootID,
	).Scan(&expenseLeafID))

	var currentVersion int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT version FROM mecontrola.category_editorial_version`,
	).Scan(&currentVersion))

	userID := "dd444444-4444-4444-4444-444444444444"
	s.Require().NoError(execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880400', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID))

	errInvalid := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategory igual raiz',
			$2::uuid, $2::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$3, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, currentVersion)
	s.Require().Error(errInvalid)
	s.Contains(errInvalid.Error(), "subcategory_must_be_direct_leaf")

	errValid := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'transacao valida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(), '2026-06', now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().NoError(errValid)
}

func (s *CategoryWriteGate000004Suite) TestCategoryWriteGateRecurringBlocksInvalidRows() {
	migrator := s.newMigrator()
	s.applyUp(migrator)

	var expenseRootID, expenseLeafID string
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id IS NULL LIMIT 1`,
	).Scan(&expenseRootID))
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT id::text FROM mecontrola.categories WHERE kind='expense' AND parent_id=$1::uuid LIMIT 1`,
		expenseRootID,
	).Scan(&expenseLeafID))

	var currentVersion int64
	s.Require().NoError(s.db.QueryRowContext(s.ctx,
		`SELECT version FROM mecontrola.category_editorial_version`,
	).Scan(&currentVersion))

	userID := "ee555555-5555-5555-5555-555555555555"
	s.Require().NoError(execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880500', 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID))

	errInvalid := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'subcategory igual raiz',
			$2::uuid, $2::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$3, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, currentVersion)
	s.Require().Error(errInvalid)
	s.Contains(errInvalid.Error(), "subcategory_must_be_direct_leaf")

	errValid := execSQL(s.db, s.ctx, `
		INSERT INTO mecontrola.transactions_recurring_templates (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			frequency, day_of_month, installments_total,
			started_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 1, 1000, 'recorrencia valida',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense/root', 'matched', 0.9,
			'high', 'exact', 'canonical_name',
			'ifood', 'exact match', 'auto_matched',
			$4, now(),
			1, 1, 1,
			now(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().NoError(errValid)
}

func (s *CategoryWriteGate000004Suite) assertVersion(migrator *migrate.Migrate, want uint) {
	version, dirty, err := migrator.Version()
	s.Require().NoError(err)
	s.False(dirty)
	s.Equal(want, version)
}

func (s *CategoryWriteGate000004Suite) assertColumnPresent(table, column string) {
	s.Equal(int64(1), s.countColumn(table, column), "coluna %s.%s ausente", table, column)
}

func (s *CategoryWriteGate000004Suite) assertColumnMissing(table, column string) {
	s.Equal(int64(0), s.countColumn(table, column), "coluna %s.%s presente", table, column)
}

func (s *CategoryWriteGate000004Suite) countColumn(table, column string) int64 {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema = 'mecontrola' AND table_name = $1 AND column_name = $2
	`, table, column).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *CategoryWriteGate000004Suite) assertConstraintPresent(name string) {
	s.Equal(int64(1), s.countConstraint(name), "constraint %s ausente", name)
}

func (s *CategoryWriteGate000004Suite) assertConstraintMissing(name string) {
	s.Equal(int64(0), s.countConstraint(name), "constraint %s presente", name)
}

func (s *CategoryWriteGate000004Suite) countConstraint(name string) int64 {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM pg_constraint WHERE conname = $1`, name).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *CategoryWriteGate000004Suite) assertTriggerPresent(schema, table, trigger string) {
	s.Equal(int64(1), s.countTrigger(schema, table, trigger), "trigger %s.%s.%s ausente", schema, table, trigger)
}

func (s *CategoryWriteGate000004Suite) assertTriggerMissing(schema, table, trigger string) {
	s.Equal(int64(0), s.countTrigger(schema, table, trigger), "trigger %s.%s.%s presente", schema, table, trigger)
}

func (s *CategoryWriteGate000004Suite) countTrigger(schema, table, trigger string) int64 {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM pg_trigger t
		JOIN pg_class c ON c.oid = t.tgrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND t.tgname = $3 AND NOT t.tgisinternal
	`, schema, table, trigger).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *CategoryWriteGate000004Suite) assertIndexPresent(schema, index string) {
	s.Equal(int64(1), s.countIndex(schema, index), "indice %s.%s ausente", schema, index)
}

func (s *CategoryWriteGate000004Suite) assertIndexMissing(schema, index string) {
	s.Equal(int64(0), s.countIndex(schema, index), "indice %s.%s presente", schema, index)
}

func (s *CategoryWriteGate000004Suite) countIndex(schema, index string) int64 {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE schemaname = $1 AND indexname = $2
	`, schema, index).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *CategoryWriteGate000004Suite) assertFunctionPresent(schema, function string) {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1 AND p.proname = $2
	`, schema, function).Scan(&count)
	s.Require().NoError(err)
	s.Equal(int64(1), count, "funcao %s.%s ausente", schema, function)
}

func (s *CategoryWriteGate000004Suite) regclass(name string) sql.NullString {
	var rc sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&rc)
	s.Require().NoError(err)
	return rc
}
