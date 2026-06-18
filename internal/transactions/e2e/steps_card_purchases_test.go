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

func registerCardPurchaseSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^que existe um cartão configurado para o usuário$`, e.queExisteUmCartaoConfiguradoParaOUsuario)
	sc.Step(`^o usuário cria uma compra de (\d+) centavos em (\d+) parcelas no cartão em "([^"]*)"$`, e.oUsuarioCriaUmaCompra)
	sc.Step(`^o banco deve conter (\d+) parcelas para a card-purchase criada$`, e.oBancoDeveConterParcelasParaACardPurchaseCriada)
	sc.Step(`^que existe uma card-purchase criada de (\d+) centavos em (\d+) parcela(?:s)? no cartão em "([^"]*)"$`, e.queExisteUmaCardPurchaseCriada)
	sc.Step(`^o usuário obtém a card-purchase pelo ID$`, e.oUsuarioObtemACardPurchasePeloID)
	sc.Step(`^o usuário tenta obter uma card-purchase com ID inexistente$`, e.oUsuarioTentaObterUmaCardPurchaseComIDInexistente)
	sc.Step(`^que existem (\d+) card-purchases criadas para o usuário$`, e.queExistemNCardPurchasesCriadasParaOUsuario)
	sc.Step(`^o usuário lista card-purchases$`, e.oUsuarioListaCardPurchases)
	sc.Step(`^o usuário atualiza a descrição da card-purchase$`, e.oUsuarioAtualizaADescricaoDaCardPurchase)
	sc.Step(`^o usuário tenta atualizar uma card-purchase com ID inexistente$`, e.oUsuarioTentaAtualizarUmaCardPurchaseComIDInexistente)
	sc.Step(`^o usuário deleta a card-purchase$`, e.oUsuarioDeletaACardPurchase)
	sc.Step(`^a card-purchase deve ter deleted_at preenchido no banco$`, e.aCardPurchaseDeveTerDeletedAtPreenchidoNoBanco)
	sc.Step(`^o usuário tenta deletar uma card-purchase com ID inexistente$`, e.oUsuarioTentaDeletarUmaCardPurchaseComIDInexistente)
}

func (e *txE2ECtx) queExisteUmCartaoConfiguradoParaOUsuario() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := insertCardViaSQL(ctx, e.db, e.userID, "Cartão E2E", fmt.Sprintf("cartao-%s", uuid.NewString()[:8]), 10, 20)
	if err != nil {
		return err
	}
	e.cardID = id
	return nil
}

func (e *txE2ECtx) oUsuarioCriaUmaCompra(totalAmountCents, installmentsTotal int, purchasedAt string) error {
	categoryUUID, err := uuid.Parse(txE2EPrazerosRootCategoryUUID)
	if err != nil {
		return fmt.Errorf("parse category uuid: %w", err)
	}
	cardUUID, err := uuid.Parse(e.cardID)
	if err != nil {
		return fmt.Errorf("parse card uuid: %w", err)
	}
	payload := map[string]any{
		"card_id":            cardUUID,
		"total_amount_cents": int64(totalAmountCents),
		"installments_total": installmentsTotal,
		"description":        "e2e compra",
		"category_id":        categoryUUID,
		"purchased_at":       purchasedAt + "T00:00:00Z",
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/card-purchases", payload); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada ao criar card-purchase")
	}
	if e.lastResp.StatusCode == http.StatusCreated {
		e.captureCardPurchaseIDFromBody()
	}
	return nil
}

func (e *txE2ECtx) captureCardPurchaseIDFromBody() {
	if id, ok := e.lastBody["id"].(string); ok && id != "" {
		e.capturedCPID = id
		e.capturedAggregateID = id
	}
}

func (e *txE2ECtx) oBancoDeveConterParcelasParaACardPurchaseCriada(expected int) error {
	if e.capturedCPID == "" {
		return fmt.Errorf("card-purchase ID não capturado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := countCardInvoiceItemsForPurchase(ctx, e.db, e.capturedCPID)
	if err != nil {
		return err
	}
	if n != expected {
		return fmt.Errorf("esperado %d parcelas para card-purchase %s, encontrado %d", expected, e.capturedCPID, n)
	}
	return nil
}

func (e *txE2ECtx) queExisteUmaCardPurchaseCriada(totalAmountCents, installmentsTotal int, purchasedAt string) error {
	if err := e.oUsuarioCriaUmaCompra(totalAmountCents, installmentsTotal, purchasedAt); err != nil {
		return err
	}
	if e.lastResp == nil || e.lastResp.StatusCode != http.StatusCreated {
		status := 0
		if e.lastResp != nil {
			status = e.lastResp.StatusCode
		}
		return fmt.Errorf("setup: status esperado 201, recebido %d, corpo: %s", status, e.lastBodyText)
	}
	e.captureCardPurchaseIDFromBody()
	if e.capturedCPID == "" {
		return fmt.Errorf("setup: id não capturado na criação da card-purchase")
	}
	return nil
}

func (e *txE2ECtx) oUsuarioObtemACardPurchasePeloID() error {
	if e.capturedCPID == "" {
		return fmt.Errorf("card-purchase ID não capturado")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/card-purchases/"+e.capturedCPID, nil)
}

func (e *txE2ECtx) oUsuarioTentaObterUmaCardPurchaseComIDInexistente() error {
	return e.makeRequest(http.MethodGet, "/api/v1/card-purchases/"+uuid.NewString(), nil)
}

func (e *txE2ECtx) queExistemNCardPurchasesCriadasParaOUsuario(n int) error {
	for i := range n {
		purchasedAt := fmt.Sprintf("2026-06-%02dT00:00:00Z", 10+i)
		if err := e.oUsuarioCriaUmaCompra(5000, 2, fmt.Sprintf("2026-06-%02d", 10+i)); err != nil {
			return fmt.Errorf("criar card-purchase %d: %w", i+1, err)
		}
		_ = purchasedAt
		if e.lastResp == nil || e.lastResp.StatusCode != http.StatusCreated {
			status := 0
			if e.lastResp != nil {
				status = e.lastResp.StatusCode
			}
			return fmt.Errorf("criar card-purchase %d: status esperado 201, recebido %d, corpo: %s", i+1, status, e.lastBodyText)
		}
	}
	return nil
}

func (e *txE2ECtx) oUsuarioListaCardPurchases() error {
	return e.makeRequest(http.MethodGet, "/api/v1/card-purchases", nil)
}

func (e *txE2ECtx) oUsuarioAtualizaADescricaoDaCardPurchase() error {
	if e.capturedCPID == "" {
		return fmt.Errorf("card-purchase ID não capturado")
	}
	categoryUUID, err := uuid.Parse(txE2EPrazerosRootCategoryUUID)
	if err != nil {
		return fmt.Errorf("parse category uuid: %w", err)
	}
	payload := map[string]any{
		"total_amount_cents": int64(6000),
		"installments_total": 2,
		"description":        "e2e compra atualizada",
		"category_id":        categoryUUID,
		"purchased_at":       "2026-06-10T00:00:00Z",
		"version":            int64(1),
	}
	if err := e.makeRequest(http.MethodPatch, "/api/v1/card-purchases/"+e.capturedCPID, payload); err != nil {
		return err
	}
	if e.lastResp != nil && e.lastResp.StatusCode == http.StatusOK {
		if id, ok := e.lastBody["id"].(string); ok && id != "" {
			e.capturedAggregateID = id
		}
	}
	return nil
}

func (e *txE2ECtx) oUsuarioTentaAtualizarUmaCardPurchaseComIDInexistente() error {
	categoryUUID, err := uuid.Parse(txE2EPrazerosRootCategoryUUID)
	if err != nil {
		return fmt.Errorf("parse category uuid: %w", err)
	}
	payload := map[string]any{
		"total_amount_cents": int64(5000),
		"installments_total": 1,
		"description":        "inexistente",
		"category_id":        categoryUUID,
		"purchased_at":       "2026-06-10T00:00:00Z",
		"version":            int64(1),
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/card-purchases/"+uuid.NewString(), payload)
}

func (e *txE2ECtx) oUsuarioDeletaACardPurchase() error {
	if e.capturedCPID == "" {
		return fmt.Errorf("card-purchase ID não capturado")
	}
	payload := map[string]any{
		"version": int64(1),
	}
	if err := e.makeRequest(http.MethodDelete, "/api/v1/card-purchases/"+e.capturedCPID, payload); err != nil {
		return err
	}
	if e.lastResp != nil && e.lastResp.StatusCode == http.StatusNoContent {
		e.capturedAggregateID = e.capturedCPID
	}
	return nil
}

func (e *txE2ECtx) aCardPurchaseDeveTerDeletedAtPreenchidoNoBanco() error {
	if e.capturedCPID == "" {
		return fmt.Errorf("card-purchase ID não capturado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deleted, err := isCardPurchaseSoftDeleted(ctx, e.db, e.capturedCPID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("card-purchase %s não possui deleted_at preenchido", e.capturedCPID)
	}
	return nil
}

func (e *txE2ECtx) oUsuarioTentaDeletarUmaCardPurchaseComIDInexistente() error {
	payload := map[string]any{
		"version": int64(1),
	}
	return e.makeRequest(http.MethodDelete, "/api/v1/card-purchases/"+uuid.NewString(), payload)
}
