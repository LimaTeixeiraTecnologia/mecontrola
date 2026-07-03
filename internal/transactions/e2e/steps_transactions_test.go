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

func registerTransactionSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^que não existe nenhuma transação para o usuário em "([^"]*)"$`, e.naoExisteNenhumaTransacaoParaOUsuarioEm)
	sc.Step(`^o usuário cria uma transação de (\d+) centavos com método "([^"]*)" e direção "([^"]*)" em "([^"]*)"$`, e.oUsuarioCriaUmaTransacao)
	sc.Step(`^o banco deve conter exatamente (\d+) transação nova para o usuário em "([^"]*)"$`, e.oBancoDeveConterExatamenteTransacaoParaOUsuario)
	sc.Step(`^que existe uma transação criada de (\d+) centavos com método "([^"]*)" e direção "([^"]*)" em "([^"]*)"$`, e.queExisteUmaTransacaoCriada)
	sc.Step(`^o usuário obtém a transação pelo ID$`, e.oUsuarioObtemATransacaoPeloID)
	sc.Step(`^o usuário tenta obter uma transação com ID inexistente$`, e.oUsuarioTentaObterTransacaoComIDInexistente)
	sc.Step(`^existem (\d+) transações criadas para o usuário em "([^"]*)"$`, e.existemNTransacoesCriadasParaOUsuario)
	sc.Step(`^que existem (\d+) transações criadas para o usuário em "([^"]*)"$`, e.existemNTransacoesCriadasParaOUsuario)
	sc.Step(`^o usuário lista transações do mês "([^"]*)"$`, e.oUsuarioListaTransacoesDoMes)
	sc.Step(`^o usuário atualiza a transação para (\d+) centavos$`, e.oUsuarioAtualizaATransacao)
	sc.Step(`^o banco deve conter a transação com valor (\d+) centavos$`, e.oBancoDeveConterATransacaoComValor)
	sc.Step(`^o usuário tenta atualizar uma transação com ID inexistente$`, e.oUsuarioTentaAtualizarTransacaoComIDInexistente)
	sc.Step(`^o usuário deleta a transação$`, e.oUsuarioDeletaATransacao)
	sc.Step(`^a transação deve ter deleted_at preenchido no banco$`, e.aTransacaoDeveTerDeletedAtPreenchido)
	sc.Step(`^a transação não deve aparecer na listagem do mês "([^"]*)"$`, e.aTransacaoNaoDeveAparecerNaListagemDoMes)
	sc.Step(`^o usuário tenta deletar uma transação com ID inexistente$`, e.oUsuarioTentaDeletarTransacaoComIDInexistente)
}

func (e *txE2ECtx) naoExisteNenhumaTransacaoParaOUsuarioEm(_ string) error {
	return nil
}

func (e *txE2ECtx) oUsuarioCriaUmaTransacao(amountCents int64, paymentMethod, direction, occurredAt string) error {
	payload := map[string]any{
		"direction":      direction,
		"payment_method": paymentMethod,
		"amount_cents":   amountCents,
		"description":    "e2e test",
		"category_id":    txE2EPrazerosRootCategoryUUID,
		"occurred_at":    occurredAt + "T00:00:00Z",
	}
	if direction == "outcome" {
		payload["subcategory_id"] = txE2EOutrosPrazeresSubcategoryUUID
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/transactions", payload); err != nil {
		return err
	}
	if e.lastResp.StatusCode == http.StatusCreated {
		if id, ok := e.lastBody["id"].(string); ok {
			e.capturedTxID = id
			e.capturedAggregateID = id
		}
	}
	return nil
}

func (e *txE2ECtx) oBancoDeveConterExatamenteTransacaoParaOUsuario(expected int, refMonth string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := countTransactionsForUser(ctx, e.db, e.userID, refMonth)
	if err != nil {
		return err
	}
	if n != expected {
		return fmt.Errorf("esperado %d transação(ões) para o usuário em %s, encontrado %d", expected, refMonth, n)
	}
	return nil
}

func (e *txE2ECtx) queExisteUmaTransacaoCriada(amountCents int64, paymentMethod, direction, occurredAt string) error {
	payload := map[string]any{
		"direction":      direction,
		"payment_method": paymentMethod,
		"amount_cents":   amountCents,
		"description":    "e2e setup",
		"category_id":    txE2EPrazerosRootCategoryUUID,
		"occurred_at":    occurredAt + "T00:00:00Z",
	}
	if direction == "outcome" {
		payload["subcategory_id"] = txE2EOutrosPrazeresSubcategoryUUID
	}
	if err := e.makeRequest(http.MethodPost, "/api/v1/transactions", payload); err != nil {
		return err
	}
	if e.lastResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("esperado 201 ao criar transação de setup, recebido %d: %s", e.lastResp.StatusCode, e.lastBodyText)
	}
	if id, ok := e.lastBody["id"].(string); ok {
		e.capturedTxID = id
		e.capturedAggregateID = id
	}
	return nil
}

func (e *txE2ECtx) oUsuarioObtemATransacaoPeloID() error {
	if e.capturedTxID == "" {
		return fmt.Errorf("nenhum ID de transação capturado")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/transactions/"+e.capturedTxID, nil)
}

func (e *txE2ECtx) oUsuarioTentaObterTransacaoComIDInexistente() error {
	return e.makeRequest(http.MethodGet, "/api/v1/transactions/"+uuid.NewString(), nil)
}

func (e *txE2ECtx) existemNTransacoesCriadasParaOUsuario(n int, occurredMonth string) error {
	for i := range n {
		day := i + 1
		occurredAt := fmt.Sprintf("%s-%02dT00:00:00Z", occurredMonth, day)
		payload := map[string]any{
			"direction":      "outcome",
			"payment_method": "pix",
			"amount_cents":   1000,
			"description":    fmt.Sprintf("e2e setup %d", i+1),
			"category_id":    txE2EPrazerosRootCategoryUUID,
			"subcategory_id": txE2EOutrosPrazeresSubcategoryUUID,
			"occurred_at":    occurredAt,
		}
		if err := e.makeRequest(http.MethodPost, "/api/v1/transactions", payload); err != nil {
			return err
		}
		if e.lastResp.StatusCode != http.StatusCreated {
			return fmt.Errorf("esperado 201 ao criar transação %d de setup, recebido %d: %s", i+1, e.lastResp.StatusCode, e.lastBodyText)
		}
		if id, ok := e.lastBody["id"].(string); ok {
			e.capturedTxID = id
			e.capturedAggregateID = id
		}
	}
	return nil
}

func (e *txE2ECtx) oUsuarioListaTransacoesDoMes(refMonth string) error {
	return e.makeRequest(http.MethodGet, "/api/v1/transactions?ref_month="+refMonth, nil)
}

func (e *txE2ECtx) oUsuarioAtualizaATransacao(amountCents int64) error {
	if e.capturedTxID == "" {
		return fmt.Errorf("nenhum ID de transação capturado")
	}

	if err := e.makeRequest(http.MethodGet, "/api/v1/transactions/"+e.capturedTxID, nil); err != nil {
		return err
	}
	if e.lastResp.StatusCode != http.StatusOK {
		return fmt.Errorf("esperado 200 ao buscar transação para update, recebido %d: %s", e.lastResp.StatusCode, e.lastBodyText)
	}

	var version int64
	if v, ok := e.lastBody["version"].(float64); ok {
		version = int64(v)
	}

	payload := map[string]any{
		"direction":      "outcome",
		"payment_method": "pix",
		"amount_cents":   amountCents,
		"description":    "updated",
		"category_id":    txE2EPrazerosRootCategoryUUID,
		"subcategory_id": txE2EOutrosPrazeresSubcategoryUUID,
		"occurred_at":    "2026-06-10T00:00:00Z",
		"version":        version,
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/transactions/"+e.capturedTxID, payload)
}

func (e *txE2ECtx) oBancoDeveConterATransacaoComValor(expectedCents int64) error {
	if e.capturedTxID == "" {
		return fmt.Errorf("nenhum ID de transação capturado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	amount, err := fetchTransactionAmountCents(ctx, e.db, e.capturedTxID)
	if err != nil {
		return err
	}
	if amount != expectedCents {
		return fmt.Errorf("esperado %d centavos no banco, encontrado %d", expectedCents, amount)
	}
	return nil
}

func (e *txE2ECtx) oUsuarioTentaAtualizarTransacaoComIDInexistente() error {
	payload := map[string]any{
		"direction":      "outcome",
		"payment_method": "pix",
		"amount_cents":   1000,
		"description":    "updated",
		"category_id":    txE2EPrazerosRootCategoryUUID,
		"subcategory_id": txE2EOutrosPrazeresSubcategoryUUID,
		"occurred_at":    "2026-06-10T00:00:00Z",
		"version":        1,
	}
	return e.makeRequest(http.MethodPatch, "/api/v1/transactions/"+uuid.NewString(), payload)
}

func (e *txE2ECtx) oUsuarioDeletaATransacao() error {
	if e.capturedTxID == "" {
		return fmt.Errorf("nenhum ID de transação capturado")
	}

	if err := e.makeRequest(http.MethodGet, "/api/v1/transactions/"+e.capturedTxID, nil); err != nil {
		return err
	}
	if e.lastResp.StatusCode != http.StatusOK {
		return fmt.Errorf("esperado 200 ao buscar transação para delete, recebido %d: %s", e.lastResp.StatusCode, e.lastBodyText)
	}

	var version int64
	if v, ok := e.lastBody["version"].(float64); ok {
		version = int64(v)
	}

	return e.makeRequest(http.MethodDelete, "/api/v1/transactions/"+e.capturedTxID, map[string]any{"version": version})
}

func (e *txE2ECtx) aTransacaoDeveTerDeletedAtPreenchido() error {
	if e.capturedTxID == "" {
		return fmt.Errorf("nenhum ID de transação capturado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deleted, err := isTransactionSoftDeleted(ctx, e.db, e.capturedTxID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("esperado deleted_at preenchido para transação %s, mas está nulo", e.capturedTxID)
	}
	return nil
}

func (e *txE2ECtx) aTransacaoNaoDeveAparecerNaListagemDoMes(refMonth string) error {
	if e.capturedTxID == "" {
		return fmt.Errorf("nenhum ID de transação capturado")
	}
	if err := e.makeRequest(http.MethodGet, "/api/v1/transactions?ref_month="+refMonth, nil); err != nil {
		return err
	}
	items, ok := e.lastBody["items"].([]any)
	if !ok {
		return nil
	}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == e.capturedTxID {
			return fmt.Errorf("transação deletada %s ainda aparece na listagem do mês %s", e.capturedTxID, refMonth)
		}
	}
	return nil
}

func (e *txE2ECtx) oUsuarioTentaDeletarTransacaoComIDInexistente() error {
	return e.makeRequest(http.MethodDelete, "/api/v1/transactions/"+uuid.NewString(), map[string]any{"version": 0})
}
