//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerActivationProcessorSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^existe um token pago com assinatura e dados do cliente$`, w.givenPaidTokenWithSubscriptionExists)
	sc.Step(`^o processor de WhatsApp recebe um comando de ativação com o token atual$`, w.whenWhatsAppProcessorHandlesActivation)
	sc.Step(`^o dispatcher processa o evento onboarding\.subscription_bound$`, w.whenSubscriptionBoundEventIsDispatched)
	sc.Step(`^deve existir uma sessão de onboarding em estado "([^"]*)"$`, w.thenOnboardingSessionStateShouldBe)
	sc.Step(`^o token atual deve estar consumido pelo usuário corrente$`, w.thenCurrentTokenShouldBeConsumedByCurrentUser)
	sc.Step(`^o processor retorna a mensagem "([^"]*)"$`, w.thenProcessorReplyShouldBe)
}

func (w *onboardingWorld) givenPaidTokenWithSubscriptionExists() error {
	clearToken := newDeterministicToken('a')
	subID, err := w.seedBillingSubscription("+5511999994444", "activate@example.com")
	if err != nil {
		return err
	}
	return w.seedMagicToken(clearToken, "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: "+5511999994444",
		customerEmail:  "activate@example.com",
		externalSaleID: "sale-activate",
		paidAt:         time.Now().UTC(),
	})
}

func (w *onboardingWorld) whenWhatsAppProcessorHandlesActivation() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.runtime.deps.whatsAppProcessor.HandleActivation(ctx, "+5511999994444", w.currentTokenClear)
}

func (w *onboardingWorld) whenSubscriptionBoundEventIsDispatched() error {
	return w.runDispatcher(w.runtime.registryFactory())
}

func (w *onboardingWorld) thenOnboardingSessionStateShouldBe(expected string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := w.runtime.deps.db.QueryRowContext(ctx, `SELECT user_id, state FROM mecontrola.onboarding_sessions LIMIT 1`)
	var userID uuid.UUID
	var state string
	if err := row.Scan(&userID, &state); err != nil {
		return err
	}
	w.currentUserID = userID
	if state != expected {
		return fmt.Errorf("estado esperado %q, recebido %q", expected, state)
	}
	return nil
}

func (w *onboardingWorld) thenCurrentTokenShouldBeConsumedByCurrentUser() error {
	row, err := w.tokenRow()
	if err != nil {
		return err
	}
	if row["status"] != "CONSUMED" {
		return fmt.Errorf("status esperado CONSUMED, recebido %v", row["status"])
	}
	consumedBy, _ := row["consumed_by_user_id"].(string)
	if consumedBy == "" {
		return fmt.Errorf("consumed_by_user_id ausente")
	}
	if w.currentUserID == uuid.Nil {
		parsed, parseErr := uuid.Parse(consumedBy)
		if parseErr != nil {
			return parseErr
		}
		w.currentUserID = parsed
	}
	if consumedBy != w.currentUserID.String() {
		return fmt.Errorf("consumido por %s, esperado %s", consumedBy, w.currentUserID)
	}
	return nil
}

func (w *onboardingWorld) thenProcessorReplyShouldBe(expected string) error {
	if w.lastReply != expected {
		return fmt.Errorf("mensagem esperada %q, recebida %q", expected, w.lastReply)
	}
	return nil
}
