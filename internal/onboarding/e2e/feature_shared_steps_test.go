//go:build e2e

package e2e_test

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func registerSharedSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^que o ambiente de teste para onboarding está pronto$`, w.givenOnboardingTestEnvironmentIsReady)
	sc.Step(`^a resposta HTTP deve ter status (\d+)$`, w.thenResponseStatusShouldBe)
	sc.Step(`^o corpo da resposta deve conter o campo "([^"]*)"$`, w.thenResponseBodyShouldContainField)
	sc.Step(`^o corpo da resposta deve conter "([^"]*)" no campo "([^"]*)"$`, w.thenResponseBodyShouldContainValueInField)
	sc.Step(`^o corpo da resposta deve ser "([^"]*)"$`, w.thenResponseBodyShouldBe)
	sc.Step(`^deve existir (\d+) evento\(s\) outbox do tipo "([^"]*)"$`, w.thenOutboxEventCountShouldBe)
	sc.Step(`^o último evento outbox "([^"]*)" deve estar com status (\d+)$`, w.thenLatestOutboxStatusShouldBe)
	sc.Step(`^o último evento outbox "([^"]*)" deve ter attempts (\d+)$`, w.thenLatestOutboxAttemptsShouldBe)
	sc.Step(`^o último evento outbox "([^"]*)" deve conter erro "([^"]*)"$`, w.thenLatestOutboxErrorShouldContain)
}

func (w *onboardingWorld) thenResponseStatusShouldBe(status int) error {
	if w.lastResp == nil {
		return fmt.Errorf("nenhuma resposta HTTP registrada")
	}
	if w.lastResp.StatusCode != status {
		return fmt.Errorf("status esperado %d, recebido %d, corpo: %s", status, w.lastResp.StatusCode, w.lastBodyText)
	}
	return nil
}

func (w *onboardingWorld) thenResponseBodyShouldContainField(field string) error {
	if w.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}
	if _, ok := w.lastBody[field]; !ok {
		return fmt.Errorf("campo %q ausente", field)
	}
	return nil
}

func (w *onboardingWorld) thenResponseBodyShouldContainValueInField(expected, field string) error {
	if w.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}
	value, ok := w.lastBody[field].(string)
	if !ok {
		return fmt.Errorf("campo %q nao e string", field)
	}
	if value != expected {
		return fmt.Errorf("valor esperado %q para %q, recebido %q", expected, field, value)
	}
	return nil
}

func (w *onboardingWorld) thenResponseBodyShouldBe(expected string) error {
	if w.lastBodyText != expected {
		return fmt.Errorf("corpo esperado %q, recebido %q", expected, w.lastBodyText)
	}
	return nil
}

func (w *onboardingWorld) thenOutboxEventCountShouldBe(expected int, eventType string) error {
	count, err := w.countOutboxEvents(eventType)
	if err != nil {
		return err
	}
	if count != expected {
		return fmt.Errorf("esperados %d eventos %q, recebidos %d", expected, eventType, count)
	}
	return nil
}

func (w *onboardingWorld) thenLatestOutboxStatusShouldBe(eventType string, expected int) error {
	status, err := w.latestOutboxStatus(eventType)
	if err != nil {
		return err
	}
	if status != expected {
		return fmt.Errorf("status esperado %d para %q, recebido %d", expected, eventType, status)
	}
	return nil
}

func (w *onboardingWorld) thenLatestOutboxAttemptsShouldBe(eventType string, expected int) error {
	attempts, _, err := w.latestOutboxDelivery(eventType)
	if err != nil {
		return err
	}
	if attempts != expected {
		return fmt.Errorf("attempts esperados %d para %q, recebidos %d", expected, eventType, attempts)
	}
	return nil
}

func (w *onboardingWorld) thenLatestOutboxErrorShouldContain(eventType string, expected string) error {
	_, lastError, err := w.latestOutboxDelivery(eventType)
	if err != nil {
		return err
	}
	if lastError == "" {
		return fmt.Errorf("erro do último evento %q ausente", eventType)
	}
	if !strings.Contains(lastError, expected) {
		return fmt.Errorf("erro esperado contendo %q para %q, recebido %q", expected, eventType, lastError)
	}
	return nil
}

func (w *onboardingWorld) givenOnboardingTestEnvironmentIsReady() error {
	return w.reset()
}
