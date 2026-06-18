//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

func registerSharedBudgetSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
	sc.Step(`^que o ambiente de teste para budgets está pronto$`, e.givenEnvReady)
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.thenStatusIs)
	sc.Step(`^a tabela outbox_events deve conter (\d+) evento(?:s)? do tipo "([^"]*)"$`, e.thenOutboxCount)
	sc.Step(`^a resposta deve conter o campo "([^"]*)" com valor "([^"]*)"$`, e.thenBodyFieldEquals)
	sc.Step(`^o banco deve conter (\d+) orçamento(?:s)? para o usuário na competência "([^"]*)"$`, e.thenBudgetCountForCompetence)
	sc.Step(`^o banco não deve conter orçamento para o usuário na competência "([^"]*)"$`, e.thenNoBudgetForCompetence)
	sc.Step(`^o banco deve conter o orçamento da competência "([^"]*)" com estado "([^"]*)"$`, e.thenBudgetStateIs)
	sc.Step(`^o banco deve conter (\d+) despesa(?:s)? para o usuário$`, e.thenExpenseCount)
	sc.Step(`^o banco deve conter a despesa com deleted_at preenchido$`, e.thenExpenseSoftDeleted)
	sc.Step(`^o banco deve conter tombstone para a despesa$`, e.thenTombstoneExists)
}

func (e *budgetsE2ECtx) givenEnvReady() error {
	return nil
}

func (e *budgetsE2ECtx) thenStatusIs(expected int) error {
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}

	if e.lastResp.StatusCode != expected {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", expected, e.lastResp.StatusCode, e.mustMarshalBody())
	}

	return nil
}

func (e *budgetsE2ECtx) thenOutboxCount(expected int, eventType string) error {
	n, err := e.countOutboxByType(eventType)
	if err != nil {
		return fmt.Errorf("contar outbox_events: %w", err)
	}

	if n != expected {
		return fmt.Errorf("outbox_events tipo %q: esperado %d, recebido %d", eventType, expected, n)
	}

	return nil
}

func (e *budgetsE2ECtx) thenBodyFieldEquals(field, expected string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	value, ok := e.lastBody[field].(string)
	if !ok {
		return fmt.Errorf("campo %q ausente ou nao e string", field)
	}

	if value != expected {
		return fmt.Errorf("campo %q esperado %q, recebido %q", field, expected, value)
	}

	return nil
}

func (e *budgetsE2ECtx) thenBudgetCountForCompetence(expected int, competence string) error {
	n, err := e.countBudgets(e2eUserID, competence)
	if err != nil {
		return fmt.Errorf("contar orçamentos: %w", err)
	}

	if n != expected {
		return fmt.Errorf("orçamentos na competência %q: esperado %d, recebido %d", competence, expected, n)
	}

	return nil
}

func (e *budgetsE2ECtx) thenNoBudgetForCompetence(competence string) error {
	return e.thenBudgetCountForCompetence(0, competence)
}

func (e *budgetsE2ECtx) thenBudgetStateIs(competence, stateName string) error {
	state, err := e.budgetState(e2eUserID, competence)
	if err != nil {
		return fmt.Errorf("obter estado do orçamento: %w", err)
	}

	expected := 1
	if stateName == "active" {
		expected = int(entities.BudgetStateActive)
	}

	if state != expected {
		return fmt.Errorf("estado do orçamento %q: esperado %d (%s), recebido %d", competence, expected, stateName, state)
	}

	return nil
}

func (e *budgetsE2ECtx) thenExpenseCount(expected int) error {
	n, err := e.countExpenses(e2eUserID)
	if err != nil {
		return fmt.Errorf("contar despesas: %w", err)
	}

	if n != expected {
		return fmt.Errorf("despesas: esperado %d, recebido %d", expected, n)
	}

	return nil
}

func (e *budgetsE2ECtx) thenExpenseSoftDeleted() error {
	if e.lastExternalID == "" {
		return fmt.Errorf("lastExternalID nao preenchido")
	}

	deleted, err := e.expenseDeletedAt(e2eUserID, "api", e.lastExternalID)
	if err != nil {
		return fmt.Errorf("verificar deleted_at: %w", err)
	}

	if !deleted {
		return fmt.Errorf("despesa %q nao foi soft-deleted", e.lastExternalID)
	}

	return nil
}

func (e *budgetsE2ECtx) thenTombstoneExists() error {
	if e.lastExternalID == "" {
		return fmt.Errorf("lastExternalID nao preenchido")
	}

	n, err := e.countTombstones(e2eUserID, "api", e.lastExternalID)
	if err != nil {
		return fmt.Errorf("contar tombstones: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("tombstone nao encontrado para despesa %q", e.lastExternalID)
	}

	return nil
}

func (e *budgetsE2ECtx) insertDraftBudget(competence string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	budgetID := uuid.New()
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, auto_draft, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, false, now(), now())
		 ON CONFLICT DO NOTHING`,
		budgetID, e2eUserID, competence, 100000, int(entities.BudgetStateDraft),
	)
	if err != nil {
		return fmt.Errorf("inserir orçamento rascunho: %w", err)
	}

	e.lastBudgetID = budgetID.String()
	e.lastCompetence = competence
	return nil
}

func (e *budgetsE2ECtx) insertActiveBudget(competence string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	budgetID := uuid.New()
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, auto_draft, activated_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, false, now(), now(), now())
		 ON CONFLICT DO NOTHING`,
		budgetID, e2eUserID, competence, 100000, int(entities.BudgetStateActive),
	)
	if err != nil {
		return fmt.Errorf("inserir orçamento ativo: %w", err)
	}

	e.lastBudgetID = budgetID.String()
	e.lastCompetence = competence
	return nil
}
