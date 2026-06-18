//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerBillingOutboxSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^existe um token pendente com assinatura billing associada$`, w.givenPendingTokenWithBillingSubscriptionExists)
	sc.Step(`^o evento "([^"]*)" é enfileirado na outbox de integração$`, w.whenIntegrationEventIsQueuedInOutbox)
	sc.Step(`^o dispatcher do outbox é executado com handlers reais$`, w.whenOutboxDispatcherRunsWithRealHandlers)
	sc.Step(`^o token atual deve estar marcado como pago$`, w.thenCurrentTokenShouldBeMarkedPaid)
	sc.Step(`^deve existir um support signal do tipo "([^"]*)"$`, w.thenSupportSignalOfKindShouldExist)
	sc.Step(`^deve ter sido enviado (\d+) email\(s\) de ativação$`, w.thenActivationEmailCountShouldBe)
	sc.Step(`^que existe um token PENDING para o plano mensal$`, w.givenPendingTokenForMonthlyPlan)
	sc.Step(`^que um evento billing\.subscription\.activated sem customer_email é disparado para o token$`, w.givenBillingEventWithoutCustomerEmail)
	sc.Step(`^que um evento billing\.subscription\.activated com customer_email "([^"]*)" é disparado para o token$`, w.givenBillingEventWithCustomerEmail)
	sc.Step(`^o consumer de email processa o evento$`, w.whenActivationEmailConsumerProcessesEvent)
	sc.Step(`^nenhum email deve ter sido enviado pelo gateway de email$`, w.thenNoEmailWasSent)
	sc.Step(`^exatamente (\d+) email de ativação deve ter sido enviado$`, w.thenExactlyNActivationEmailsSent)
}

func (w *onboardingWorld) givenPendingTokenWithBillingSubscriptionExists() error {
	subID, err := w.seedBillingSubscription("+5511999993333", "billing@example.com")
	if err != nil {
		return err
	}
	return w.seedMagicToken(newDeterministicToken('b'), "PENDING", tokenSeedOptions{
		subscriptionID: subID,
	})
}

func (w *onboardingWorld) whenIntegrationEventIsQueuedInOutbox(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if w.currentSubscriptionID == "" {
		subID, err := w.seedBillingSubscription("+5511999993333", "billing@example.com")
		if err != nil {
			return err
		}
		w.currentSubscriptionID = subID
	}

	payload := map[string]any{
		"subscription_id":      w.currentSubscriptionID,
		"external_sale_id":     "sale-billing",
		"customer_mobile_e164": "+5511999993333",
		"customer_email":       "billing@example.com",
		"paid_at":              time.Now().UTC(),
		"occurred_at":          time.Now().UTC(),
	}
	if eventType == "billing.subscription.activated" {
		payload["funnel_token"] = w.currentTokenClear
	}

	eventID, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		eventType,
		"billing_subscription",
		w.currentSubscriptionID,
		"",
		payload,
	)
	if err != nil {
		return err
	}
	w.lastOutboxEventID = eventID
	return nil
}

func (w *onboardingWorld) whenOutboxDispatcherRunsWithRealHandlers() error {
	return w.runDispatcher(w.runtime.registryFactory())
}

func (w *onboardingWorld) thenCurrentTokenShouldBeMarkedPaid() error {
	row, err := w.tokenRow()
	if err != nil {
		return err
	}
	if row["status"] != "PAID" {
		return fmt.Errorf("status esperado PAID, recebido %v", row["status"])
	}
	return nil
}

func (w *onboardingWorld) thenSupportSignalOfKindShouldExist(kind string) error {
	count, err := w.supportSignalCount(kind)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("esperado 1 support signal %q, recebido %d", kind, count)
	}
	return nil
}

func (w *onboardingWorld) thenActivationEmailCountShouldBe(expected int) error {
	got := len(w.runtime.emailSender.messages)
	if got != expected {
		return fmt.Errorf("esperados %d emails de ativação, recebidos %d", expected, got)
	}
	return nil
}

func (w *onboardingWorld) givenPendingTokenForMonthlyPlan() error {
	subID, err := w.seedBillingSubscription("+5511988880001", "monthly@e2e.test")
	if err != nil {
		return err
	}
	return w.seedMagicToken(newDeterministicToken('m'), "PENDING", tokenSeedOptions{
		subscriptionID: subID,
	})
}

func (w *onboardingWorld) givenBillingEventWithoutCustomerEmail() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := map[string]any{
		"subscription_id":      w.currentSubscriptionID,
		"external_sale_id":     "sale-no-email",
		"customer_mobile_e164": "+5511988880001",
		"customer_email":       "",
		"funnel_token":         w.currentTokenClear,
		"paid_at":              time.Now().UTC(),
		"occurred_at":          time.Now().UTC(),
	}

	eventID, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		"billing.subscription.activated",
		"billing_subscription",
		w.currentSubscriptionID,
		"",
		payload,
	)
	if err != nil {
		return err
	}
	w.lastOutboxEventID = eventID
	return nil
}

func (w *onboardingWorld) givenBillingEventWithCustomerEmail(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := map[string]any{
		"subscription_id":      w.currentSubscriptionID,
		"external_sale_id":     "sale-with-email",
		"customer_mobile_e164": "+5511988880001",
		"customer_email":       email,
		"funnel_token":         w.currentTokenClear,
		"paid_at":              time.Now().UTC(),
		"occurred_at":          time.Now().UTC(),
	}

	eventID, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		"billing.subscription.activated",
		"billing_subscription",
		w.currentSubscriptionID,
		"",
		payload,
	)
	if err != nil {
		return err
	}
	w.lastOutboxEventID = eventID
	return nil
}

func (w *onboardingWorld) whenActivationEmailConsumerProcessesEvent() error {
	registry := newEventRegistry()
	if err := registry.Register("billing.subscription.activated", w.runtime.deps.activationEmailConsumer); err != nil {
		return err
	}
	return w.runDispatcher(registry)
}

func (w *onboardingWorld) thenNoEmailWasSent() error {
	w.runtime.emailSender.mu.Lock()
	defer w.runtime.emailSender.mu.Unlock()
	got := len(w.runtime.emailSender.messages)
	if got != 0 {
		return fmt.Errorf("esperado 0 emails, enviados %d", got)
	}
	return nil
}

func (w *onboardingWorld) thenExactlyNActivationEmailsSent(expected int) error {
	w.runtime.emailSender.mu.Lock()
	defer w.runtime.emailSender.mu.Unlock()
	got := len(w.runtime.emailSender.messages)
	if got != expected {
		return fmt.Errorf("esperados %d emails de ativação, enviados %d", expected, got)
	}
	return nil
}
