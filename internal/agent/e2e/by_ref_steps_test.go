//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerByRefSteps(sc *godog.ScenarioContext, e *agentE2ECtx) {
	sc.Step(`^o usuário possui um lançamento "([^"]*)" de (\d+) centavos$`, e.givenTransactionWithDescription)
}

func (e *agentE2ECtx) givenTransactionWithDescription(description string, amountCents int) error {
	if err := e.givenUserIsActive(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	refMonth := now.Format("2006-01")
	id := uuid.New()

	var categoryID uuid.UUID
	if err := e.db.QueryRowContext(
		ctx,
		`SELECT id FROM mecontrola.categories WHERE kind = 'expense' AND parent_id IS NULL AND slug = 'custo-fixo'`,
	).Scan(&categoryID); err != nil {
		return fmt.Errorf("seed transaction %q: resolver categoria raiz: %w", description, err)
	}

	if _, err := e.db.ExecContext(
		ctx,
		`INSERT INTO mecontrola.transactions
		        (id, user_id, direction, payment_method, amount_cents, description,
		         category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		         ref_month, occurred_at, version, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, 2, 1, $3, $4, $5, NULL, 'Custo Fixo', '', $6, $7, 1, NULL, $7, $7)`,
		id, e.userID, int64(amountCents), description, categoryID, refMonth, now,
	); err != nil {
		return fmt.Errorf("seed transaction %q: %w", description, err)
	}
	time.Sleep(2 * time.Millisecond)
	return nil
}
