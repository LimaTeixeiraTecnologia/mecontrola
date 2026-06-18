//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	input "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func registerOutboxPublisherSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^que existe um token PAID válido com mobile "([^"]*)" e email "([^"]*)"$`, w.givenPaidTokenWithMobileAndEmail)
	sc.Step(`^o número "([^"]*)" envia a mensagem de ativação com o token$`, w.whenMobileSendsActivationWithToken)
	sc.Step(`^a tabela outbox_events deve conter (\d+) evento[s]? "([^"]*)"$`, w.thenOutboxContainsNEventsOfType)
	sc.Step(`^o aggregate_type do evento deve ser "([^"]*)"$`, w.thenOutboxAggregateTypeIs)
	sc.Step(`^que existe um token já consumido com activation_path "([^"]*)"$`, w.givenConsumedTokenWithActivationPath)
	sc.Step(`^o número "([^"]*)" tenta reativar com o mesmo token$`, w.whenMobileTriesToReactivateWithSameToken)
}

func (w *onboardingWorld) givenPaidTokenWithMobileAndEmail(mobileE164, email string) error {
	subID, err := w.seedBillingSubscription(mobileE164, email)
	if err != nil {
		return err
	}
	clearToken := newDeterministicToken('p')
	return w.seedMagicToken(clearToken, "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: mobileE164,
		customerEmail:  email,
		externalSaleID: "sale-outbox-pub",
		paidAt:         time.Now().UTC(),
	})
}

func (w *onboardingWorld) whenMobileSendsActivationWithToken(mobileE164 string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := w.runtime.deps.consumeToken.Execute(ctx, input.ConsumeMagicTokenInput{
		Token:          w.currentTokenClear,
		FromE164:       mobileE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	})
	return err
}

func (w *onboardingWorld) thenOutboxContainsNEventsOfType(n int, eventType string) error {
	count, err := w.countOutboxEvents(eventType)
	if err != nil {
		return err
	}
	if count != n {
		return fmt.Errorf("esperados %d eventos %q na outbox_events, recebidos %d", n, eventType, count)
	}
	return nil
}

func (w *onboardingWorld) thenOutboxAggregateTypeIs(aggregateType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT aggregate_type
		  FROM mecontrola.outbox_events
		 ORDER BY created_at DESC
		 LIMIT 1
	`)
	var got string
	if err := row.Scan(&got); err != nil {
		return fmt.Errorf("aggregate_type: %w", err)
	}
	if got != aggregateType {
		return fmt.Errorf("aggregate_type esperado %q, recebido %q", aggregateType, got)
	}
	return nil
}

func (w *onboardingWorld) givenConsumedTokenWithActivationPath(activationPath string) error {
	subID, err := w.seedBillingSubscription("+5511900000099", "outbox-consumed@test.com")
	if err != nil {
		return err
	}
	clearToken := newDeterministicToken('c')
	return w.seedMagicToken(clearToken, "CONSUMED", tokenSeedOptions{
		subscriptionID:   subID,
		customerMobile:   "+5511900000099",
		customerEmail:    "outbox-consumed@test.com",
		externalSaleID:   "sale-outbox-consumed",
		paidAt:           time.Now().UTC().Add(-1 * time.Hour),
		consumedAt:       time.Now().UTC().Add(-30 * time.Minute),
		consumedByMobile: "+5511900000099",
		activationPath:   activationPath,
	})
}

func (w *onboardingWorld) whenMobileTriesToReactivateWithSameToken(mobileE164 string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = w.runtime.deps.consumeToken.Execute(ctx, input.ConsumeMagicTokenInput{
		Token:          w.currentTokenClear,
		FromE164:       mobileE164,
		ActivationPath: valueobjects.ActivationPathDirect,
	})
	return nil
}
