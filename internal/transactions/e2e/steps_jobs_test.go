//go:build e2e

package transactions_e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

func registerJobSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^que existe um recurring-template ativo de (\d+) centavos com frequência "([^"]*)" para o dia de hoje$`, e.queExisteUmRecurringTemplateAtivoParaHoje)
	sc.Step(`^que o job recurring-materializer já foi executado para hoje$`, e.queOJobRecurringMaterializerJaFoiExecutadoParaHoje)
	sc.Step(`^o job recurring-materializer é executado para hoje$`, e.oJobRecurringMaterializerEExecutadoParaHoje)
	sc.Step(`^o banco deve conter pelo menos (\d+) materialização para o recurring-template no mês atual$`, e.oBancoDeveConterPeloMenosMaterializacoes)
	sc.Step(`^o banco deve conter exatamente (\d+) materialização para o recurring-template no mês atual$`, e.oBancoDeveConterExatamenteMaterializacoes)
	sc.Step(`^o job monthly-summary-reconciler é executado$`, e.oJobMonthlySummaryReconcilerEExecutado)
	sc.Step(`^a resposta do job deve ser sucesso$`, e.aRespostaDoJobDeveSerSucesso)
}

func (e *txE2ECtx) queExisteUmRecurringTemplateAtivoParaHoje(centavos int64, frequencia string) error {
	brazilLoc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return fmt.Errorf("carregar timezone Brasil: %w", err)
	}
	todayBrazil := time.Now().In(brazilLoc)
	payload := map[string]any{
		"direction":          "outcome",
		"payment_method":     "pix",
		"amount_cents":       centavos,
		"description":        "e2e job test",
		"category_id":        txE2EPrazerosRootCategoryUUID,
		"subcategory_id":     txE2EOutrosPrazeresSubcategoryUUID,
		"frequency":          frequencia,
		"day_of_month":       todayBrazil.Day(),
		"installments_total": 1,
		"started_at":         todayBrazil.AddDate(-1, 0, 0).Format(time.RFC3339),
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/recurring-templates", payload); err != nil {
		return err
	}
	if e.lastResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("esperado 201 ao criar recurring-template, recebido %d: %s", e.lastResp.StatusCode, e.lastBodyText)
	}
	if id, ok := e.lastBody["id"].(string); ok && id != "" {
		e.capturedRTID = id
	}
	return nil
}

func (e *txE2ECtx) queOJobRecurringMaterializerJaFoiExecutadoParaHoje() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return e.recurringJob.Run(ctx)
}

func (e *txE2ECtx) oJobRecurringMaterializerEExecutadoParaHoje() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e.lastJobErr = e.recurringJob.Run(ctx)
	return e.lastJobErr
}

func refMonthBrazil() (string, error) {
	brazilLoc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return "", fmt.Errorf("carregar timezone Brasil: %w", err)
	}
	return time.Now().In(brazilLoc).Format("2006-01"), nil
}

func (e *txE2ECtx) oBancoDeveConterPeloMenosMaterializacoes(minimo int) error {
	if e.capturedRTID == "" {
		return fmt.Errorf("nenhum recurring-template capturado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	refMonth, err := refMonthBrazil()
	if err != nil {
		return err
	}
	n, err := countRecurringMaterializationsForDay(ctx, e.db, e.capturedRTID, refMonth)
	if err != nil {
		return err
	}
	if n < minimo {
		return fmt.Errorf("esperado pelo menos %d materialização(ões) para template %s em %s, encontrado %d", minimo, e.capturedRTID, refMonth, n)
	}
	return nil
}

func (e *txE2ECtx) oBancoDeveConterExatamenteMaterializacoes(esperado int) error {
	if e.capturedRTID == "" {
		return fmt.Errorf("nenhum recurring-template capturado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	refMonth, err := refMonthBrazil()
	if err != nil {
		return err
	}
	n, err := countRecurringMaterializationsForDay(ctx, e.db, e.capturedRTID, refMonth)
	if err != nil {
		return err
	}
	if n != esperado {
		return fmt.Errorf("esperado exatamente %d materialização(ões) para template %s em %s, encontrado %d", esperado, e.capturedRTID, refMonth, n)
	}
	return nil
}

func (e *txE2ECtx) oJobMonthlySummaryReconcilerEExecutado() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e.lastJobErr = e.reconcilerJob.Run(ctx)
	return nil
}

func (e *txE2ECtx) aRespostaDoJobDeveSerSucesso() error {
	if e.lastJobErr != nil {
		return fmt.Errorf("job retornou erro: %w", e.lastJobErr)
	}
	return nil
}
