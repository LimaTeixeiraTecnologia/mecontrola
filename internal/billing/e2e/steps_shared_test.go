//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerSharedSteps(sc *godog.ScenarioContext, e *billingE2ECtx) {
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.thenHTTPStatusShouldBe)
	sc.Step(`^o campo de erro deve ser "([^"]*)"$`, e.thenErrorCodeShouldBe)
}

func (e *billingE2ECtx) thenHTTPStatusShouldBe(expected int) error {
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if e.lastResp.StatusCode != expected {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", expected, e.lastResp.StatusCode, e.lastBodyText)
	}
	return nil
}

func (e *billingE2ECtx) thenErrorCodeShouldBe(expected string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo da resposta vazio ou nao e JSON")
	}
	errs, ok := e.lastBody["errors"].(map[string]any)
	if !ok {
		return fmt.Errorf("campo errors ausente: %v", e.lastBody)
	}
	got, _ := errs["code"].(string)
	if got != expected {
		return fmt.Errorf("codigo esperado %q, recebido %q", expected, got)
	}
	return nil
}
