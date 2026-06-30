//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerWebhookRegressionSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^nenhum token deve estar com status PAID$`, w.thenNoTokenShouldBePaid)
	sc.Step(`^o evento "([^"]*)" é enfileirado novamente com o mesmo ID$`, w.whenSameBillingEventRequeued)
}

func (w *onboardingWorld) thenNoTokenShouldBePaid() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.onboarding_tokens WHERE status = 'PAID'`)
	var count int
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("esperados 0 tokens PAID, encontrados %d", count)
	}
	return nil
}

func (w *onboardingWorld) whenSameBillingEventRequeued(eventType string) error {
	return w.whenIntegrationEventIsQueuedInOutbox(eventType)
}
