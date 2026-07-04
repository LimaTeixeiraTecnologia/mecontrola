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

func registerCreditCardSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^que existe um cartão configurado para o usuário$`, e.queExisteUmCartaoConfiguradoParaOUsuario)
	sc.Step(`^o usuário cria uma compra no crédito de (\d+) centavos em (\d+) parcelas no cartão em "([^"]*)"$`, e.oUsuarioCriaUmaCompraNoCredito)
	sc.Step(`^o banco deve conter (\d+) parcelas para a transação criada$`, e.oBancoDeveConterParcelasParaATransacaoCriada)
	sc.Step(`^que existe uma compra no crédito de (\d+) centavos em (\d+) parcela(?:s)? no cartão em "([^"]*)"$`, e.queExisteUmaCompraNoCreditoCriada)
	sc.Step(`^o usuário deleta a transação no crédito$`, e.oUsuarioDeletaATransacaoNoCredito)
	sc.Step(`^a transação no crédito deve ter deleted_at preenchido no banco$`, e.aTransacaoNoCreditoDeveTerDeletedAtPreenchido)
	sc.Step(`^uma requisição GET para "([^"]*)" retorna status (\d+)$`, e.umaRequisicaoGETParaRetornaStatus)
	sc.Step(`^uma requisição POST para "([^"]*)" retorna status (\d+)$`, e.umaRequisicaoPOSTParaRetornaStatus)
}

func (e *txE2ECtx) queExisteUmCartaoConfiguradoParaOUsuario() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := insertCardViaSQL(ctx, e.db, e.userID, "nubank", fmt.Sprintf("cartao-%s", uuid.NewString()[:8]), 10, 20)
	if err != nil {
		return err
	}
	e.cardID = id
	return nil
}

func (e *txE2ECtx) oUsuarioCriaUmaCompraNoCredito(amountCents, installments int, occurredAt string) error {
	cardUUID, err := uuid.Parse(e.cardID)
	if err != nil {
		return fmt.Errorf("parse card uuid: %w", err)
	}
	payload := map[string]any{
		"direction":      "outcome",
		"payment_method": "credit_card",
		"amount_cents":   int64(amountCents),
		"description":    "e2e compra no crédito",
		"category_id":    txE2EPrazerosRootCategoryUUID,
		"subcategory_id": txE2EOutrosPrazeresSubcategoryUUID,
		"card_id":        cardUUID,
		"installments":   installments,
		"occurred_at":    occurredAt + "T00:00:00Z",
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/transactions", payload); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada ao criar compra no crédito")
	}
	if e.lastResp.StatusCode == http.StatusCreated {
		if id, ok := e.lastBody["id"].(string); ok && id != "" {
			e.capturedTxID = id
			e.capturedAggregateID = id
		}
	}
	return nil
}

func (e *txE2ECtx) oBancoDeveConterParcelasParaATransacaoCriada(expected int) error {
	if e.capturedTxID == "" {
		return fmt.Errorf("transação no crédito não capturada")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := countCardInvoiceItemsForTransaction(ctx, e.db, e.capturedTxID)
	if err != nil {
		return err
	}
	if n != expected {
		return fmt.Errorf("esperado %d parcelas para transação %s, encontrado %d", expected, e.capturedTxID, n)
	}
	return nil
}

func (e *txE2ECtx) queExisteUmaCompraNoCreditoCriada(amountCents, installments int, occurredAt string) error {
	if err := e.oUsuarioCriaUmaCompraNoCredito(amountCents, installments, occurredAt); err != nil {
		return err
	}
	if e.lastResp == nil || e.lastResp.StatusCode != http.StatusCreated {
		status := 0
		if e.lastResp != nil {
			status = e.lastResp.StatusCode
		}
		return fmt.Errorf("setup: status esperado 201, recebido %d, corpo: %s", status, e.lastBodyText)
	}
	if e.capturedTxID == "" {
		return fmt.Errorf("setup: id não capturado na criação da compra no crédito")
	}
	return nil
}

func (e *txE2ECtx) oUsuarioDeletaATransacaoNoCredito() error {
	if e.capturedTxID == "" {
		return fmt.Errorf("transação no crédito não capturada")
	}
	payload := map[string]any{
		"version": int64(1),
	}
	if err := e.makeRequest(http.MethodDelete, "/api/v1/transactions/"+e.capturedTxID, payload); err != nil {
		return err
	}
	if e.lastResp != nil && e.lastResp.StatusCode == http.StatusNoContent {
		e.capturedAggregateID = e.capturedTxID
	}
	return nil
}

func (e *txE2ECtx) aTransacaoNoCreditoDeveTerDeletedAtPreenchido() error {
	if e.capturedTxID == "" {
		return fmt.Errorf("transação no crédito não capturada")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deleted, err := isTransactionSoftDeleted(ctx, e.db, e.capturedTxID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("transação %s não possui deleted_at preenchido", e.capturedTxID)
	}
	return nil
}

func (e *txE2ECtx) umaRequisicaoGETParaRetornaStatus(path string, expectedStatus int) error {
	if err := e.makeRequest(http.MethodGet, path, nil); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada para GET %s", path)
	}
	if e.lastResp.StatusCode != expectedStatus {
		return fmt.Errorf("GET %s: status esperado %d, recebido %d", path, expectedStatus, e.lastResp.StatusCode)
	}
	return nil
}

func (e *txE2ECtx) umaRequisicaoPOSTParaRetornaStatus(path string, expectedStatus int) error {
	if err := e.makeRequest(http.MethodPost, path, map[string]any{}); err != nil {
		return err
	}
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada para POST %s", path)
	}
	if e.lastResp.StatusCode != expectedStatus {
		return fmt.Errorf("POST %s: status esperado %d, recebido %d", path, expectedStatus, e.lastResp.StatusCode)
	}
	return nil
}
