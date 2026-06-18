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

func registerCardInvoiceSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^que existe um cartão configurado para o usuário com fechamento no dia (\d+)$`, e.queExisteUmCartaoConfiguradoParaOUsuarioComFechamentoNoDia)
	sc.Step(`^que existe uma card-purchase de (\d+) centavos em (\d+) parcela no cartão em "([^"]*)"$`, e.queExisteUmaCardPurchaseDeNParcelaNoCartaoEm)
	sc.Step(`^o usuário obtém a fatura do cartão para "([^"]*)"$`, e.oUsuarioObtemAFaturaDoCartaoPara)
}

func (e *txE2ECtx) queExisteUmCartaoConfiguradoParaOUsuarioComFechamentoNoDia(closingDay int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dueDay := closingDay + 10
	if dueDay > 28 {
		dueDay = 28
	}
	id, err := insertCardViaSQL(ctx, e.db, e.userID, "Cartão Invoice E2E", fmt.Sprintf("invoice-e2e-%s", uuid.NewString()[:8]), closingDay, dueDay)
	if err != nil {
		return err
	}
	e.cardID = id
	return nil
}

func (e *txE2ECtx) queExisteUmaCardPurchaseDeNParcelaNoCartaoEm(totalAmountCents, installmentsTotal int, purchasedAt string) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID não capturado — crie o cartão antes da card-purchase")
	}
	cardUUID, err := uuid.Parse(e.cardID)
	if err != nil {
		return fmt.Errorf("parse card uuid: %w", err)
	}
	categoryUUID, err := uuid.Parse(txE2EPrazerosRootCategoryUUID)
	if err != nil {
		return fmt.Errorf("parse category uuid: %w", err)
	}
	payload := map[string]any{
		"card_id":            cardUUID,
		"total_amount_cents": int64(totalAmountCents),
		"installments_total": installmentsTotal,
		"description":        "e2e invoice setup",
		"category_id":        categoryUUID,
		"purchased_at":       purchasedAt + "T00:00:00Z",
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/card-purchases", payload); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada ao criar card-purchase")
	}
	if e.lastResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("status esperado 201 ao criar card-purchase, recebido %d: %s", e.lastResp.StatusCode, e.lastBodyText)
	}
	if id, ok := e.lastBody["id"].(string); ok && id != "" {
		e.capturedCPID = id
	}
	return nil
}

func (e *txE2ECtx) oUsuarioObtemAFaturaDoCartaoPara(refMonth string) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID não capturado — crie o cartão antes de consultar a fatura")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+e.cardID+"/invoices/"+refMonth, nil)
}
