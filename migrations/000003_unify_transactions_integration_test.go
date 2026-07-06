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

type Unify000003Suite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestUnify000003Suite(t *testing.T) {
	suite.Run(t, new(Unify000003Suite))
}

func (s *Unify000003Suite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *Unify000003Suite) newMigrator() *migrate.Migrate {
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

func (s *Unify000003Suite) applyUp(migrator *migrate.Migrate) {
	upErr := migrator.Up()
	s.Require().True(upErr == nil || errors.Is(upErr, migrate.ErrNoChange), "up idempotente: %v", upErr)
}

func (s *Unify000003Suite) TestUnifyUpThenDownThenUp() {
	migrator := s.newMigrator()
	s.applyUp(migrator)

	s.assertVersion(migrator, 4)

	s.assertColumnPresent("transactions", "card_id")
	s.assertColumnPresent("transactions", "installments_total")
	s.assertColumnPresent("transactions", "card_closing_day")
	s.assertColumnPresent("transactions", "card_due_day")
	s.assertConstraintPresent("transactions_card_completeness_chk")
	s.assertConstraintPresent("transactions_installments_range_chk")
	s.assertConstraintPresent("transactions_card_closing_day_chk")
	s.assertConstraintPresent("transactions_card_due_day_chk")
	s.assertConstraintPresent("transactions_card_fk")

	s.assertColumnPresent("transactions_card_invoice_items", "transaction_id")
	s.assertColumnMissing("transactions_card_invoice_items", "purchase_id")
	s.assertConstraintPresent("transactions_card_invoice_items_transaction_uk")
	s.assertConstraintPresent("transactions_card_invoice_items_transaction_fk")

	s.assertColumnMissing("transactions_recurring_materializations", "materialized_purchase_id")
	s.assertColumnPresent("transactions_recurring_materializations", "materialized_transaction_id")

	s.assertTableMissing("mecontrola.transactions_card_purchases")
	s.assertTablePresent("mecontrola.transactions_card_invoices")

	s.assertCompletenessCheckRejectsPartialCard()

	s.Require().NoError(migrator.Steps(-2))
	s.assertVersion(migrator, 2)

	s.assertColumnMissing("transactions", "card_id")
	s.assertColumnMissing("transactions", "installments_total")
	s.assertColumnMissing("transactions", "card_closing_day")
	s.assertColumnMissing("transactions", "card_due_day")
	s.assertConstraintMissing("transactions_card_completeness_chk")
	s.assertConstraintMissing("transactions_card_fk")

	s.assertTablePresent("mecontrola.transactions_card_purchases")
	s.assertColumnPresent("transactions_card_invoice_items", "purchase_id")
	s.assertColumnMissing("transactions_card_invoice_items", "transaction_id")
	s.assertColumnPresent("transactions_recurring_materializations", "materialized_purchase_id")

	s.applyUp(migrator)
	s.assertVersion(migrator, 4)
	s.assertColumnPresent("transactions", "card_id")
	s.assertColumnPresent("transactions_card_invoice_items", "transaction_id")
	s.assertTableMissing("mecontrola.transactions_card_purchases")
}

func (s *Unify000003Suite) assertCompletenessCheckRejectsPartialCard() {
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

	userID := "aaaaaaaa-2222-2222-2222-aaaaaaaaaaaa"
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.transactions (
			id, user_id, direction, payment_method, amount_cents, description,
			category_id, subcategory_id,
			category_name_snapshot, subcategory_name_snapshot,
			category_kind, category_path, category_outcome, category_score,
			category_confidence, category_match_quality, category_signal_type,
			category_matched_term, category_match_reason, category_decision_source,
			category_editorial_version, category_decided_at,
			ref_month, occurred_at,
			card_id, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 2, 7, 1000, 'compra incompleta',
			$2::uuid, $3::uuid,
			'categoria', 'subcategoria',
			'expense', 'expense.root/leaf', 'matched', 1.0,
			'high', 'exact', 'canonical_name',
			'term', 'exact match', 'auto_matched',
			$4, now(),
			'2026-06', now(),
			gen_random_uuid(), now(), now()
		)
	`, userID, expenseRootID, expenseLeafID, currentVersion)
	s.Require().Error(err)
	s.Contains(err.Error(), "transactions_card_completeness_chk")
}

func (s *Unify000003Suite) assertVersion(migrator *migrate.Migrate, want uint) {
	version, dirty, err := migrator.Version()
	s.Require().NoError(err)
	s.False(dirty)
	s.Equal(want, version)
}

func (s *Unify000003Suite) assertColumnPresent(table, column string) {
	s.Equal(int64(1), s.countColumn(table, column), "coluna %s.%s ausente", table, column)
}

func (s *Unify000003Suite) assertColumnMissing(table, column string) {
	s.Equal(int64(0), s.countColumn(table, column), "coluna %s.%s presente", table, column)
}

func (s *Unify000003Suite) countColumn(table, column string) int64 {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema = 'mecontrola' AND table_name = $1 AND column_name = $2
	`, table, column).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *Unify000003Suite) assertConstraintPresent(name string) {
	s.Equal(int64(1), s.countConstraint(name), "constraint %s ausente", name)
}

func (s *Unify000003Suite) assertConstraintMissing(name string) {
	s.Equal(int64(0), s.countConstraint(name), "constraint %s presente", name)
}

func (s *Unify000003Suite) countConstraint(name string) int64 {
	var count int64
	err := s.db.QueryRowContext(s.ctx, `SELECT COUNT(*) FROM pg_constraint WHERE conname = $1`, name).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *Unify000003Suite) assertTablePresent(name string) {
	s.True(s.regclass(name).Valid, "tabela %s ausente", name)
}

func (s *Unify000003Suite) assertTableMissing(name string) {
	s.False(s.regclass(name).Valid, "tabela %s presente", name)
}

func (s *Unify000003Suite) regclass(name string) sql.NullString {
	var rc sql.NullString
	err := s.db.QueryRowContext(s.ctx, `SELECT to_regclass($1)`, name).Scan(&rc)
	s.Require().NoError(err)
	return rc
}
