//go:build e2e

package e2e_test

import (
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerTokenStateHTTPSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^existe um token pago pronto para ativação$`, w.givenReadyToActivateTokenExists)
	sc.Step(`^existe um token pendente$`, w.givenPendingTokenExists)
	sc.Step(`^existe um token expirado$`, w.givenExpiredTokenExists)
	sc.Step(`^o cliente consulta o estado do token atual$`, w.whenClientGetsCurrentTokenState)
	sc.Step(`^o cliente consulta o estado de um token inexistente$`, w.whenClientGetsNonexistentTokenState)
	sc.Step(`^o corpo da resposta deve indicar ready_to_activate verdadeiro$`, w.thenResponseShouldIndicateReadyToActivate)
	sc.Step(`^o corpo da resposta deve indicar ready_to_activate falso$`, w.thenResponseShouldIndicateNotReadyToActivate)
}

func (w *onboardingWorld) givenReadyToActivateTokenExists() error {
	subID, err := w.seedBillingSubscription("+5511999992222", "buyer@example.com")
	if err != nil {
		return err
	}
	return w.seedMagicToken(newDeterministicToken('r'), "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: "+5511999992222",
		customerEmail:  "buyer@example.com",
		externalSaleID: "sale-ready",
		paidAt:         time.Now().UTC(),
	})
}

func (w *onboardingWorld) givenPendingTokenExists() error {
	return w.seedMagicToken(newDeterministicToken('p'), "PENDING", tokenSeedOptions{})
}

func (w *onboardingWorld) givenExpiredTokenExists() error {
	return w.seedMagicToken(newDeterministicToken('e'), "EXPIRED", tokenSeedOptions{
		expired: true,
	})
}

func (w *onboardingWorld) whenClientGetsCurrentTokenState() error {
	return w.get("/api/v1/onboarding/tokens/" + w.currentTokenClear + "/state")
}

func (w *onboardingWorld) whenClientGetsNonexistentTokenState() error {
	return w.get("/api/v1/onboarding/tokens/" + newDeterministicToken('x') + "/state")
}

func (w *onboardingWorld) thenResponseShouldIndicateReadyToActivate() error {
	value, ok := w.lastBody["ready_to_activate"].(bool)
	if !ok || !value {
		return fmt.Errorf("ready_to_activate deveria ser true")
	}
	return nil
}

func (w *onboardingWorld) thenResponseShouldIndicateNotReadyToActivate() error {
	value, ok := w.lastBody["ready_to_activate"].(bool)
	if !ok {
		return fmt.Errorf("campo ready_to_activate ausente")
	}
	if value {
		return fmt.Errorf("ready_to_activate deveria ser false")
	}
	return nil
}
