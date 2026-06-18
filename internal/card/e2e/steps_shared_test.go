//go:build e2e

package e2e_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerSharedSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^existe um usuário autenticado$`, e.existeUmUsuarioAutenticado)
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.aRespostaHTTPDeveTerStatus)
	sc.Step(`^o corpo da resposta deve conter o campo "([^"]*)"$`, e.oCorpoDeveConterOCampo)
	sc.Step(`^o campo de erro deve ser "([^"]*)"$`, e.assertErrorCode)
	sc.Step(`^o campo texto "([^"]*)" da resposta deve ser "([^"]*)"$`, e.assertTextFieldEquals)
	sc.Step(`^o campo numérico "([^"]*)" da resposta deve ser (\d+)$`, e.assertNumericFieldEquals)
	sc.Step(`^que o usuário possui um cartão criado com nome "([^"]*)", fechamento (\d+), vencimento (\d+) e limite (\d+)$`, e.cardExistsWithDetails)
}

func (e *cardE2ECtx) existeUmUsuarioAutenticado() error {
	return nil
}

func (e *cardE2ECtx) aRespostaHTTPDeveTerStatus(statusEsperado int) error {
	if e.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}

	if e.lastResp.StatusCode != statusEsperado {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", statusEsperado, e.lastResp.StatusCode, e.lastBodyText)
	}

	return nil
}

func (e *cardE2ECtx) oCorpoDeveConterOCampo(campo string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	value, ok := e.lastBody[campo]
	if !ok {
		return fmt.Errorf("campo %q ausente", campo)
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("campo %q vazio", campo)
		}
	case nil:
		return fmt.Errorf("campo %q nulo", campo)
	}

	return nil
}

func (e *cardE2ECtx) assertErrorCode(code string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	errorsBody, ok := e.lastBody["errors"].(map[string]any)
	if !ok {
		return fmt.Errorf("campo errors ausente")
	}

	got, ok := errorsBody["code"].(string)
	if !ok {
		return fmt.Errorf("codigo de erro ausente")
	}

	if got != code {
		return fmt.Errorf("codigo esperado %q, recebido %q", code, got)
	}

	return nil
}

func (e *cardE2ECtx) assertTextFieldEquals(campo, valorEsperado string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	got, ok := e.lastBody[campo].(string)
	if !ok {
		return fmt.Errorf("campo %q nao e string", campo)
	}

	if got != valorEsperado {
		return fmt.Errorf("campo %q esperado %q, recebido %q", campo, valorEsperado, got)
	}

	return nil
}

func (e *cardE2ECtx) assertNumericFieldEquals(campo string, valorEsperado int64) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}

	raw, ok := e.lastBody[campo].(float64)
	if !ok {
		return fmt.Errorf("campo %q nao e numero", campo)
	}

	got := int64(raw)
	if got != valorEsperado {
		return fmt.Errorf("campo %q esperado %d, recebido %d", campo, valorEsperado, got)
	}

	return nil
}
