//go:build e2e

package transactions_e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

func registerSharedSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	sc.Step(`^que o ambiente E2E de transactions está pronto$`, e.ambienteE2EDeTransactionsEstaPronto)
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.aRespostaHTTPDeveTerStatus)
	sc.Step(`^o corpo da resposta deve conter o campo "([^"]*)"$`, e.oCorpoDeveConterOCampo)
	sc.Step(`^a tabela outbox_events deve conter (\d+) evento(?:s)? com event_type "([^"]*)"$`, e.aTabelaOutboxDeveConterEventoComEventType)
	sc.Step(`^o usuário envia uma requisição POST para "([^"]*)" com payload inválido$`, e.oUsuarioEnviaPostComPayloadInvalido)
	sc.Step(`^uma requisição não autenticada envia POST para "([^"]*)"$`, e.requisicaoNaoAutenticadaEnviaPost)
}

func (e *txE2ECtx) ambienteE2EDeTransactionsEstaPronto() error {
	e.lastResp = nil
	e.lastBody = nil
	e.lastBodyText = ""
	e.capturedTxID = ""
	e.capturedCPID = ""
	e.capturedRTID = ""
	e.capturedAggregateID = ""
	e.capturedVersion = 0
	e.cardID = ""
	return nil
}

func (e *txE2ECtx) aRespostaHTTPDeveTerStatus(statusEsperado int) error {
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if e.lastResp.StatusCode != statusEsperado {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", statusEsperado, e.lastResp.StatusCode, e.lastBodyText)
	}
	return nil
}

func (e *txE2ECtx) oCorpoDeveConterOCampo(campo string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo da resposta está vazio ou não é JSON")
	}
	val, ok := e.lastBody[campo]
	if !ok {
		return fmt.Errorf("campo %q ausente no corpo: %s", campo, e.lastBodyText)
	}
	if val == nil || val == "" {
		return fmt.Errorf("campo %q presente mas vazio", campo)
	}
	return nil
}

func (e *txE2ECtx) aTabelaOutboxDeveConterEventoComEventType(qtd int, eventType string) error {
	if e.capturedAggregateID == "" {
		return fmt.Errorf("aggregate_id não capturado — nenhum recurso criado neste cenário")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := countOutboxByEventType(ctx, e.db, eventType, e.capturedAggregateID)
	if err != nil {
		return err
	}
	if n != qtd {
		return fmt.Errorf("esperado %d evento(s) %q no outbox para aggregate %s, encontrado %d", qtd, eventType, e.capturedAggregateID, n)
	}
	return nil
}

func (e *txE2ECtx) oUsuarioEnviaPostComPayloadInvalido(path string) error {
	return e.makeRequest(http.MethodPost, path, "invalid-json-payload")
}

func (e *txE2ECtx) requisicaoNaoAutenticadaEnviaPost(path string) error {
	return e.makeUnauthenticatedRequest(http.MethodPost, path, map[string]any{
		"direction":      "outcome",
		"payment_method": "pix",
		"amount_cents":   1000,
	})
}
