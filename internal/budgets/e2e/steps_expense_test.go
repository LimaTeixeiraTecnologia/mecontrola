//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const (
	e2eSubcategoryDeliveryID = "ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c"
	e2eRootSlugPrazeres      = "expense.prazeres"
)

func registerExpenseSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
	sc.Step(`^o usuário autenticado cria uma despesa de "([^"]*)" centavos na competência "([^"]*)"$`, e.whenCreateExpense)
	sc.Step(`^o usuário não autenticado tenta criar uma despesa$`, e.whenCreateExpenseWithoutAuth)
	sc.Step(`^que existe uma despesa para o usuário com versão 1$`, e.givenExpenseWithVersion1)
	sc.Step(`^o usuário autenticado exclui a despesa com versão esperada (\d+)$`, e.whenDeleteExpense)
	sc.Step(`^o usuário autenticado tenta excluir uma despesa inexistente$`, e.whenDeleteNonExistentExpense)
}

func (e *budgetsE2ECtx) whenCreateExpense(amountStr, competence string) error {
	var amount int64
	if _, err := fmt.Sscanf(amountStr, "%d", &amount); err != nil {
		return fmt.Errorf("parsear amount: %w", err)
	}

	extID := uuid.New().String()
	e.lastExternalID = extID

	now := time.Now().UTC()
	return e.post("/api/v1/budgets/expenses", map[string]any{
		"external_transaction_id": extID,
		"subcategory_id":          e2eSubcategoryDeliveryID,
		"competence":              competence,
		"amount_cents":            amount,
		"occurred_at":             now.Format(time.RFC3339),
	})
}

func (e *budgetsE2ECtx) whenCreateExpenseWithoutAuth() error {
	return e.postWithoutAuth("/api/v1/budgets/expenses", map[string]any{
		"external_transaction_id": uuid.New().String(),
		"subcategory_id":          e2eSubcategoryDeliveryID,
		"competence":              "2099-01",
		"amount_cents":            1000,
		"occurred_at":             time.Now().UTC().Format(time.RFC3339),
	})
}

func (e *budgetsE2ECtx) givenExpenseWithVersion1() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expenseID := uuid.New()
	extID := uuid.New().String()
	e.lastExternalID = extID
	e.lastExpenseID = expenseID.String()

	_, err := e.db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_expenses
		 (id, user_id, source, external_transaction_id, subcategory_id, root_slug, competence,
		  amount_cents, occurred_at, version, tombstone_version, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, 'api', $3, $4, $5, '2025-09', 5000, now(), 1, NULL, NULL, now(), now())`,
		expenseID, e2eUserID, extID, e2eSubcategoryDeliveryID, e2eRootSlugPrazeres,
	)
	if err != nil {
		return fmt.Errorf("inserir despesa: %w", err)
	}

	return nil
}

func (e *budgetsE2ECtx) whenDeleteExpense(expectedVersion int64) error {
	if e.lastExternalID == "" {
		return fmt.Errorf("lastExternalID nao preenchido")
	}

	return e.delete("/api/v1/budgets/expenses/"+e.lastExternalID, map[string]any{
		"expected_version": expectedVersion,
	})
}

func (e *budgetsE2ECtx) whenDeleteNonExistentExpense() error {
	nonExistentID := uuid.New().String()
	return e.delete("/api/v1/budgets/expenses/"+nonExistentID, map[string]any{
		"expected_version": int64(1),
	})
}
