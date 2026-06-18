//go:build e2e

package transactions_e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerRecurringTemplateSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^o usuário cria um recurring-template de (\d+) centavos com frequência "([^"]*)" no dia (\d+) e direção "([^"]*)"$`, e.oUsuarioCriaRecurringTemplate)
	sc.Step(`^o banco deve conter (\d+) recurring-template novo para o usuário$`, e.oBancoDeveConterRecurringTemplates)
	sc.Step(`^que existe um recurring-template criado de (\d+) centavos com frequência "([^"]*)" no dia (\d+) e direção "([^"]*)"$`, e.queExisteUmRecurringTemplateCriado)
	sc.Step(`^o usuário obtém o recurring-template pelo ID$`, e.oUsuarioObtemRecurringTemplatePeloID)
	sc.Step(`^o usuário tenta obter um recurring-template com ID inexistente$`, e.oUsuarioTentaObterRecurringTemplateInexistente)
	sc.Step(`^existem (\d+) recurring-templates criados para o usuário$`, e.existemNRecurringTemplatesCriadosParaOUsuario)
	sc.Step(`^que existem (\d+) recurring-templates criados para o usuário$`, e.existemNRecurringTemplatesCriadosParaOUsuario)
	sc.Step(`^o usuário lista recurring-templates$`, e.oUsuarioListaRecurringTemplates)
	sc.Step(`^o usuário atualiza o recurring-template para (\d+) centavos$`, e.oUsuarioAtualizaRecurringTemplate)
	sc.Step(`^o usuário tenta atualizar um recurring-template com ID inexistente$`, e.oUsuarioTentaAtualizarRecurringTemplateInexistente)
	sc.Step(`^o usuário deleta o recurring-template$`, e.oUsuarioDeletaRecurringTemplate)
	sc.Step(`^o recurring-template deve ter deleted_at preenchido no banco$`, e.oRecurringTemplateDeveTerDeletedAtPreenchido)
	sc.Step(`^o usuário tenta deletar um recurring-template com ID inexistente$`, e.oUsuarioTentaDeletarRecurringTemplateInexistente)
}

func (e *txE2ECtx) oUsuarioCriaRecurringTemplate(amountCents int, frequency string, dayOfMonth int, direction string) error {
	payload := map[string]any{
		"direction":          direction,
		"payment_method":     "pix",
		"amount_cents":       amountCents,
		"description":        "e2e recorrente",
		"category_id":        txE2EPrazerosRootCategoryUUID,
		"frequency":          frequency,
		"day_of_month":       dayOfMonth,
		"installments_total": 1,
		"started_at":         "2026-06-01T00:00:00Z",
	}

	if err := e.makeRequest(http.MethodPost, "/api/v1/recurring-templates", payload); err != nil {
		return err
	}

	if e.lastResp.StatusCode == http.StatusCreated && e.lastBody != nil {
		if id, ok := e.lastBody["id"].(string); ok {
			e.capturedRTID = id
			e.capturedAggregateID = id
		}
	}

	return nil
}

func (e *txE2ECtx) oBancoDeveConterRecurringTemplates(expected int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := countRecurringTemplatesForUser(ctx, e.db, e.userID)
	if err != nil {
		return err
	}

	if n != expected {
		return fmt.Errorf("esperado %d recurring-template(s) no banco, encontrado %d", expected, n)
	}

	return nil
}

func (e *txE2ECtx) queExisteUmRecurringTemplateCriado(amountCents int, frequency string, dayOfMonth int, direction string) error {
	payload := map[string]any{
		"direction":          direction,
		"payment_method":     "pix",
		"amount_cents":       amountCents,
		"description":        "e2e recorrente",
		"category_id":        txE2EPrazerosRootCategoryUUID,
		"frequency":          frequency,
		"day_of_month":       dayOfMonth,
		"installments_total": 1,
		"started_at":         "2026-06-01T00:00:00Z",
	}

	if err := e.makeRequest(http.MethodPost, "/api/v1/recurring-templates", payload); err != nil {
		return fmt.Errorf("setup recurring-template: %w", err)
	}

	if e.lastResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("setup recurring-template: status esperado 201, recebido %d, corpo: %s", e.lastResp.StatusCode, e.lastBodyText)
	}

	if e.lastBody != nil {
		if id, ok := e.lastBody["id"].(string); ok {
			e.capturedRTID = id
			e.capturedAggregateID = id
		}
	}

	e.capturedVersion = 1

	return nil
}

func (e *txE2ECtx) oUsuarioObtemRecurringTemplatePeloID() error {
	if e.capturedRTID == "" {
		return fmt.Errorf("capturedRTID não preenchido")
	}

	return e.makeRequest(http.MethodGet, "/api/v1/recurring-templates/"+e.capturedRTID, nil)
}

func (e *txE2ECtx) oUsuarioTentaObterRecurringTemplateInexistente() error {
	return e.makeRequest(http.MethodGet, "/api/v1/recurring-templates/"+uuid.NewString(), nil)
}

func (e *txE2ECtx) existemNRecurringTemplatesCriadosParaOUsuario(n int) error {
	for i := range n {
		payload := map[string]any{
			"direction":          "outcome",
			"payment_method":     "pix",
			"amount_cents":       1000 + i*100,
			"description":        fmt.Sprintf("e2e recorrente %d", i+1),
			"category_id":        txE2EPrazerosRootCategoryUUID,
			"frequency":          "monthly",
			"day_of_month":       i + 1,
			"installments_total": 1,
			"started_at":         "2026-06-01T00:00:00Z",
		}

		if err := e.makeRequest(http.MethodPost, "/api/v1/recurring-templates", payload); err != nil {
			return fmt.Errorf("setup recurring-template %d: %w", i+1, err)
		}

		if e.lastResp.StatusCode != http.StatusCreated {
			return fmt.Errorf("setup recurring-template %d: status esperado 201, recebido %d, corpo: %s", i+1, e.lastResp.StatusCode, e.lastBodyText)
		}
	}

	return nil
}

func (e *txE2ECtx) oUsuarioListaRecurringTemplates() error {
	return e.makeRequest(http.MethodGet, "/api/v1/recurring-templates", nil)
}

func (e *txE2ECtx) oUsuarioAtualizaRecurringTemplate(amountCents int) error {
	if e.capturedRTID == "" {
		return fmt.Errorf("capturedRTID não preenchido")
	}

	payload := map[string]any{
		"direction":          "outcome",
		"payment_method":     "pix",
		"amount_cents":       amountCents,
		"description":        "e2e recorrente atualizado",
		"category_id":        txE2EPrazerosRootCategoryUUID,
		"frequency":          "monthly",
		"day_of_month":       5,
		"installments_total": 1,
		"started_at":         "2026-06-01T00:00:00Z",
		"version":            e.capturedVersion,
	}

	if err := e.makeRequest(http.MethodPatch, "/api/v1/recurring-templates/"+e.capturedRTID, payload); err != nil {
		return err
	}

	if e.lastResp.StatusCode == http.StatusOK && e.lastBody != nil {
		if id, ok := e.lastBody["id"].(string); ok {
			e.capturedAggregateID = id
		}
	}

	return nil
}

func (e *txE2ECtx) oUsuarioTentaAtualizarRecurringTemplateInexistente() error {
	payload := map[string]any{
		"direction":          "outcome",
		"payment_method":     "pix",
		"amount_cents":       1200,
		"description":        "e2e recorrente atualizado",
		"category_id":        txE2EPrazerosRootCategoryUUID,
		"frequency":          "monthly",
		"day_of_month":       5,
		"installments_total": 1,
		"started_at":         "2026-06-01T00:00:00Z",
		"version":            1,
	}

	return e.makeRequest(http.MethodPatch, "/api/v1/recurring-templates/"+uuid.NewString(), payload)
}

func (e *txE2ECtx) oUsuarioDeletaRecurringTemplate() error {
	if e.capturedRTID == "" {
		return fmt.Errorf("capturedRTID não preenchido")
	}

	payload := map[string]any{
		"version": e.capturedVersion,
	}

	return e.makeRequest(http.MethodDelete, "/api/v1/recurring-templates/"+e.capturedRTID, payload)
}

func (e *txE2ECtx) oRecurringTemplateDeveTerDeletedAtPreenchido() error {
	if e.capturedRTID == "" {
		return fmt.Errorf("capturedRTID não preenchido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deleted, err := isRecurringTemplateSoftDeleted(ctx, e.db, e.capturedRTID)
	if err != nil {
		return err
	}

	if !deleted {
		return fmt.Errorf("recurring-template %s não tem deleted_at preenchido", e.capturedRTID)
	}

	return nil
}

func (e *txE2ECtx) oUsuarioTentaDeletarRecurringTemplateInexistente() error {
	payload := map[string]any{
		"version": 1,
	}

	return e.makeRequest(http.MethodDelete, "/api/v1/recurring-templates/"+uuid.NewString(), payload)
}
