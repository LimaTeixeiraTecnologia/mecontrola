//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func e2eBudgetPayload(competence string, total int64) map[string]any {
	return map[string]any{
		"competence":  competence,
		"total_cents": total,
		"allocations": []map[string]any{
			{"root_slug": e2ePrazeresRootSlug, "basis_points": 10000},
		},
	}
}

func registerBudgetSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
	sc.Step(`^que já existe um orçamento para o usuário na competência "([^"]*)"$`, e.givenBudgetExists)
	sc.Step(`^que existe um orçamento rascunho para o usuário na competência "([^"]*)"$`, e.givenDraftBudgetExists)
	sc.Step(`^que existe um orçamento ativo para o usuário na competência "([^"]*)"$`, e.givenActiveBudgetExists)
	sc.Step(`^o usuário autenticado cria um orçamento para a competência "([^"]*)" com total de "([^"]*)" centavos$`, e.whenCreateBudget)
	sc.Step(`^o usuário não autenticado tenta criar um orçamento$`, e.whenCreateBudgetWithoutAuth)
	sc.Step(`^o usuário autenticado ativa o orçamento da competência "([^"]*)"$`, e.whenActivateBudget)
	sc.Step(`^o usuário autenticado tenta ativar o orçamento da competência "([^"]*)"$`, e.whenActivateBudget)
	sc.Step(`^o usuário autenticado exclui o rascunho da competência "([^"]*)"$`, e.whenDeleteDraftBudget)
	sc.Step(`^o usuário autenticado tenta excluir o orçamento da competência "([^"]*)"$`, e.whenDeleteDraftBudget)
	sc.Step(`^o usuário autenticado solicita o resumo da competência "([^"]*)"$`, e.whenGetMonthlySummary)
	sc.Step(`^o usuário autenticado lista os alertas$`, e.whenListAlerts)
	sc.Step(`^o usuário não autenticado lista os alertas$`, e.whenListAlertsWithoutAuth)
}

func (e *budgetsE2ECtx) givenBudgetExists(competence string) error {
	return e.insertDraftBudget(competence)
}

func (e *budgetsE2ECtx) givenDraftBudgetExists(competence string) error {
	return e.insertDraftBudget(competence)
}

func (e *budgetsE2ECtx) givenActiveBudgetExists(competence string) error {
	return e.insertActiveBudget(competence)
}

func (e *budgetsE2ECtx) whenCreateBudget(competence, totalStr string) error {
	var total int64
	if _, err := fmt.Sscanf(totalStr, "%d", &total); err != nil {
		return fmt.Errorf("parsear total: %w", err)
	}

	return e.post("/api/v1/budgets", e2eBudgetPayload(competence, total))
}

func (e *budgetsE2ECtx) whenCreateBudgetWithoutAuth() error {
	return e.postWithoutAuth("/api/v1/budgets", e2eBudgetPayload("2099-01", 100000))
}

func (e *budgetsE2ECtx) whenActivateBudget(competence string) error {
	return e.post("/api/v1/budgets/"+competence+"/activate", nil)
}

func (e *budgetsE2ECtx) whenDeleteDraftBudget(competence string) error {
	return e.delete("/api/v1/budgets/"+competence, nil)
}

func (e *budgetsE2ECtx) whenGetMonthlySummary(competence string) error {
	return e.get("/api/v1/budgets/" + competence + "/summary")
}

func (e *budgetsE2ECtx) whenListAlerts() error {
	return e.get("/api/v1/budgets/alerts")
}

func (e *budgetsE2ECtx) whenListAlertsWithoutAuth() error {
	return e.getWithoutAuth("/api/v1/budgets/alerts")
}
