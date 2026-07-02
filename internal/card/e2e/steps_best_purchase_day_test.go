//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

func registerBestPurchaseDaySteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário consulta o melhor dia de compra para banco "([^"]*)" e vencimento (\d+)$`, e.queryBestPurchaseDay)
	sc.Step(`^o usuário consulta o melhor dia de compra sem informar banco e vencimento (\d+)$`, e.queryBestPurchaseDayWithoutBank)
}

func (e *cardE2ECtx) queryBestPurchaseDay(banco string, vencimento int) error {
	path := fmt.Sprintf("/api/v1/cards/best-purchase-day?bank=%s&due_day=%d", banco, vencimento)
	return e.makeRequest(http.MethodGet, path, nil)
}

func (e *cardE2ECtx) queryBestPurchaseDayWithoutBank(vencimento int) error {
	path := fmt.Sprintf("/api/v1/cards/best-purchase-day?due_day=%d", vencimento)
	return e.makeRequest(http.MethodGet, path, nil)
}
