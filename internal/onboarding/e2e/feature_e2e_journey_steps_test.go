//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const (
	e2eUnknownMobile      = "+5511900000000"
	e2eExpiredTokenMobile = "+5511999990099"
)

func registerE2EJourneySteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^o dispatcher do outbox é executado com handlers reais e handlers da jornada completa$`, w.whenJourneyDispatcherRuns)
	sc.Step(`^o corpo da resposta deve ter status (\d+)$`, w.thenResponseStatusShouldBe)
	sc.Step(`^a resposta deve conter wa_me_url com texto "([^"]*)"$`, w.thenResponseContainsWaMeURLWithText)
	sc.Step(`^o evento de tentativa de ativação é enfileirado para o número do cliente$`, w.whenActivationAttemptQueuedForClient)
	sc.Step(`^o token atual deve estar consumido pelo usuário corrente$`, w.thenCurrentTokenShouldBeConsumed)
	sc.Step(`^deve ter sido enviadas (\d+) mensagens? de boas-vindas para o número do cliente$`, w.thenWelcomeMessageCountForClient)
	sc.Step(`^o mesmo evento de tentativa de ativação é reenviado$`, w.whenSameActivationAttemptResent)
	sc.Step(`^o evento de tentativa de ativação é enfileirado para um número sem sessão PAID$`, w.whenActivationAttemptQueuedForUnknownNumber)
	sc.Step(`^deve ter sido enviada (\d+) mensagem de no-match para o número sem sessão$`, w.thenNoMatchMessageCountForUnknownNumber)
	sc.Step(`^existe um token com pagamento expirado há mais de 24 horas$`, w.givenPaymentWindowExpiredTokenExists)
	sc.Step(`^o evento de tentativa de ativação é enfileirado para o número do token expirado$`, w.whenActivationAttemptQueuedForExpiredToken)
	sc.Step(`^o token deve permanecer com status "([^"]*)"$`, w.thenTokenStatusShouldBe)
	sc.Step(`^nenhuma mensagem de boas-vindas deve ter sido enviada$`, w.thenNoWelcomeMessagesSent)
}

func (w *onboardingWorld) whenJourneyDispatcherRuns() error {
	return w.runDispatcher(w.runtime.journeyRegistryFactory())
}

func (w *onboardingWorld) thenResponseContainsWaMeURLWithText(text string) error {
	if w.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}
	value, ok := w.lastBody["wa_me_url"].(string)
	if !ok {
		return fmt.Errorf("campo wa_me_url ausente ou não é string")
	}
	if !strings.Contains(value, text) {
		return fmt.Errorf("wa_me_url %q não contém %q", value, text)
	}
	return nil
}

func (w *onboardingWorld) whenActivationAttemptQueuedForClient() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mobile, err := w.mobileForCurrentSubscription(ctx)
	if err != nil {
		return err
	}
	w.lastActivationMobile = mobile
	w.lastActivationMessageID = uuid.NewString()

	payload := map[string]any{
		"peer_e164":  mobile,
		"text":       "Oi",
		"message_id": w.lastActivationMessageID,
	}
	eventID, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		"onboarding.activation.attempted.v1",
		"onboarding_activation",
		mobile,
		"",
		payload,
	)
	if err != nil {
		return err
	}
	w.lastOutboxEventID = eventID
	return nil
}

func (w *onboardingWorld) thenCurrentTokenShouldBeConsumed() error {
	row, err := w.tokenRow()
	if err != nil {
		return err
	}
	if row["status"] != "CONSUMED" {
		return fmt.Errorf("status esperado CONSUMED, recebido %v", row["status"])
	}
	return nil
}

func (w *onboardingWorld) thenWelcomeMessageCountForClient(expected int) error {
	w.runtime.metaGateway.mu.Lock()
	defer w.runtime.metaGateway.mu.Unlock()
	count := 0
	for _, msg := range w.runtime.metaGateway.messages {
		if msg.To == w.lastActivationMobile {
			count++
		}
	}
	if count != expected {
		return fmt.Errorf("esperadas %d mensagens para %s, enviadas %d", expected, w.lastActivationMobile, count)
	}
	return nil
}

func (w *onboardingWorld) whenSameActivationAttemptResent() error {
	if w.lastActivationMobile == "" || w.lastActivationMessageID == "" {
		return fmt.Errorf("nenhuma tentativa de ativação anterior registrada")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := map[string]any{
		"peer_e164":  w.lastActivationMobile,
		"text":       "Oi",
		"message_id": w.lastActivationMessageID,
	}
	_, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		"onboarding.activation.attempted.v1",
		"onboarding_activation",
		w.lastActivationMobile,
		"",
		payload,
	)
	return err
}

func (w *onboardingWorld) whenActivationAttemptQueuedForUnknownNumber() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := map[string]any{
		"peer_e164":  e2eUnknownMobile,
		"text":       "Oi",
		"message_id": uuid.NewString(),
	}
	_, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		"onboarding.activation.attempted.v1",
		"onboarding_activation",
		e2eUnknownMobile,
		"",
		payload,
	)
	return err
}

func (w *onboardingWorld) thenNoMatchMessageCountForUnknownNumber(expected int) error {
	w.runtime.metaGateway.mu.Lock()
	defer w.runtime.metaGateway.mu.Unlock()
	count := 0
	for _, msg := range w.runtime.metaGateway.messages {
		if msg.To == e2eUnknownMobile {
			count++
		}
	}
	if count != expected {
		return fmt.Errorf("esperadas %d mensagens no-match para %s, enviadas %d", expected, e2eUnknownMobile, count)
	}
	return nil
}

func (w *onboardingWorld) givenPaymentWindowExpiredTokenExists() error {
	subID, err := w.seedBillingSubscription(e2eExpiredTokenMobile, "expired-window@example.com")
	if err != nil {
		return err
	}
	return w.seedMagicToken(newDeterministicToken('q'), "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: e2eExpiredTokenMobile,
		customerEmail:  "expired-window@example.com",
		paidAt:         time.Now().UTC().Add(-25 * time.Hour),
	})
}

func (w *onboardingWorld) whenActivationAttemptQueuedForExpiredToken() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := map[string]any{
		"peer_e164":  e2eExpiredTokenMobile,
		"text":       "Oi",
		"message_id": uuid.NewString(),
	}
	_, err := insertOutboxEnvelope(
		ctx,
		w.runtime.deps.db,
		w.runtime.deps.outboxCfg,
		w.runtime.deps.o11y,
		"onboarding.activation.attempted.v1",
		"onboarding_activation",
		e2eExpiredTokenMobile,
		"",
		payload,
	)
	return err
}

func (w *onboardingWorld) thenTokenStatusShouldBe(expected string) error {
	row, err := w.tokenRow()
	if err != nil {
		return err
	}
	if row["status"] != expected {
		return fmt.Errorf("status esperado %q, recebido %v", expected, row["status"])
	}
	return nil
}

func (w *onboardingWorld) thenNoWelcomeMessagesSent() error {
	w.runtime.metaGateway.mu.Lock()
	defer w.runtime.metaGateway.mu.Unlock()
	for _, msg := range w.runtime.metaGateway.messages {
		if msg.Text == "wa-welcome" || msg.Text == "wa-onboarding-intro" {
			return fmt.Errorf("mensagem de boas-vindas inesperada enviada para %s", msg.To)
		}
	}
	return nil
}

func (w *onboardingWorld) mobileForCurrentSubscription(ctx context.Context) (string, error) {
	row := w.runtime.deps.db.QueryRowContext(ctx, `
		SELECT customer_mobile_e164
		  FROM mecontrola.billing_subscriptions
		 WHERE id = $1
	`, w.currentSubscriptionID)
	var mobile string
	if err := row.Scan(&mobile); err != nil {
		return "", fmt.Errorf("lookup subscription mobile: %w", err)
	}
	return mobile, nil
}
