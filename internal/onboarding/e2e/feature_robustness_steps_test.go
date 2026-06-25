//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"

	onboardingapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
)

func registerRobustnessSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^existe um token pago elegível para fallback por WhatsApp$`, w.givenPaidTokenEligibleForWhatsAppFallbackExists)
	sc.Step(`^existe um token já consumido por outro usuário$`, w.givenTokenAlreadyConsumedByAnotherUserExists)
	sc.Step(`^o processor de WhatsApp recebe uma tentativa de fallback do número atual$`, w.whenWhatsAppProcessorHandlesFallback)
	sc.Step(`^o processor de WhatsApp recebe uma tentativa de ativação com reutilização do token$`, w.whenWhatsAppProcessorHandlesTokenReuseAttempt)
	sc.Step(`^o dispatcher do outbox é executado sem handlers registrados$`, w.whenOutboxDispatcherRunsWithoutHandlers)
	sc.Step(`^o dispatcher do outbox é executado com handler que falha$`, w.whenOutboxDispatcherRunsWithFailingHandler)
	sc.Step(`^o gateway de outreach responde erro 4xx$`, w.givenOutreachGatewayReturns4xx)
	sc.Step(`^o gateway de outreach responde erro 5xx$`, w.givenOutreachGatewayReturns5xx)
	sc.Step(`^a última mensagem WhatsApp enviada deve ser "([^"]*)"$`, w.thenLatestWhatsAppMessageShouldBe)
	sc.Step(`^o token atual deve permanecer com status "([^"]*)"$`, w.thenCurrentTokenStatusShouldBe)
	sc.Step(`^o token atual deve ter outreach_sent_at preenchido$`, w.thenCurrentTokenShouldHaveOutreachSentAt)
	sc.Step(`^o token atual deve ter outreach_sent_at nulo$`, w.thenCurrentTokenShouldNotHaveOutreachSentAt)
	sc.Step(`^o cliente envia requisições em excesso para o endpoint de checkout$`, w.whenClientSendsExcessRequestsToCheckout)
	sc.Step(`^pelo menos uma resposta deve ter status (\d+)$`, w.thenAtLeastOneResponseHasStatus)
}

func (w *onboardingWorld) givenPaidTokenEligibleForWhatsAppFallbackExists() error {
	subID, err := w.seedBillingSubscription("+5511999997777", "fallback@example.com")
	if err != nil {
		return err
	}
	return w.seedMagicToken(newDeterministicToken('f'), "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: "+5511999997777",
		customerEmail:  "fallback@example.com",
		externalSaleID: "sale-fallback",
		paidAt:         time.Now().UTC().Add(-48 * time.Hour),
		outreachSentAt: time.Now().UTC().Add(-24 * time.Hour),
	})
}

func (w *onboardingWorld) givenTokenAlreadyConsumedByAnotherUserExists() error {
	if err := w.ensureCurrentUserByWhatsApp("+5511999998888", "owner@example.com"); err != nil {
		return err
	}
	subID, err := w.seedBillingSubscription("+5511999998888", "owner@example.com")
	if err != nil {
		return err
	}
	return w.seedMagicToken(newDeterministicToken('u'), "CONSUMED", tokenSeedOptions{
		subscriptionID:   subID,
		customerMobile:   "+5511999998888",
		customerEmail:    "owner@example.com",
		externalSaleID:   "sale-reuse",
		paidAt:           time.Now().UTC().Add(-48 * time.Hour),
		consumedAt:       time.Now().UTC().Add(-24 * time.Hour),
		consumedByUserID: w.currentUserID.String(),
		consumedByMobile: "+5511999998888",
		activationPath:   "direct",
	})
}

func (w *onboardingWorld) whenWhatsAppProcessorHandlesFallback() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.runtime.deps.whatsAppProcessor.HandleFallback(ctx, "+5511999997777")
}

func (w *onboardingWorld) whenWhatsAppProcessorHandlesTokenReuseAttempt() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.runtime.deps.whatsAppProcessor.HandleActivation(ctx, "+5511999990000", w.currentTokenClear)
}

func (w *onboardingWorld) whenOutboxDispatcherRunsWithoutHandlers() error {
	return w.runDispatcher(newEventRegistry())
}

func (w *onboardingWorld) whenOutboxDispatcherRunsWithFailingHandler() error {
	w.runtime.failingHandler.configure(fmt.Errorf("forced dispatch failure"))
	registry := newEventRegistry()
	if err := registry.Register("billing.subscription.activated_without_token", w.runtime.failingHandler); err != nil {
		return err
	}
	return w.runDispatcher(registry)
}

func (w *onboardingWorld) givenOutreachGatewayReturns4xx() error {
	w.runtime.outreachGateway.templateErr = fmt.Errorf("gateway 4xx: %w", onboardingapp.ErrWhatsAppClientError)
	return nil
}

func (w *onboardingWorld) givenOutreachGatewayReturns5xx() error {
	w.runtime.outreachGateway.templateErr = fmt.Errorf("gateway 5xx")
	return nil
}

func (w *onboardingWorld) thenLatestWhatsAppMessageShouldBe(expected string) error {
	w.runtime.metaGateway.mu.Lock()
	defer w.runtime.metaGateway.mu.Unlock()
	if len(w.runtime.metaGateway.messages) == 0 {
		return fmt.Errorf("nenhuma mensagem WhatsApp enviada")
	}
	got := w.runtime.metaGateway.messages[len(w.runtime.metaGateway.messages)-1].Text
	if got != expected {
		return fmt.Errorf("mensagem esperada %q, recebida %q", expected, got)
	}
	return nil
}

func (w *onboardingWorld) thenCurrentTokenStatusShouldBe(expected string) error {
	row, err := w.tokenRow()
	if err != nil {
		return err
	}
	if row["status"] != expected {
		return fmt.Errorf("status esperado %q, recebido %v", expected, row["status"])
	}
	return nil
}

func (w *onboardingWorld) thenCurrentTokenShouldHaveOutreachSentAt() error {
	sent, err := w.tokenOutreachSentAt()
	if err != nil {
		return err
	}
	if !sent {
		return fmt.Errorf("outreach_sent_at deveria estar preenchido")
	}
	return nil
}

func (w *onboardingWorld) thenCurrentTokenShouldNotHaveOutreachSentAt() error {
	sent, err := w.tokenOutreachSentAt()
	if err != nil {
		return err
	}
	if sent {
		return fmt.Errorf("outreach_sent_at deveria estar nulo")
	}
	return nil
}

func (w *onboardingWorld) whenClientSendsExcessRequestsToCheckout() error {
	const totalRequests = 12
	statuses := make([]int, 0, totalRequests)
	payload := []byte(`{"plan_id":"monthly"}`)
	for i := 0; i < totalRequests; i++ {
		req, err := http.NewRequest(http.MethodPost, w.runtime.server.URL+"/api/v1/onboarding/checkout", bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("criar request %d: %w", i, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := w.runtime.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("executar request %d: %w", i, err)
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			return fmt.Errorf("fechar body request %d: %w", i, closeErr)
		}
		statuses = append(statuses, resp.StatusCode)
	}
	w.lastStatusCodes = statuses
	return nil
}

func (w *onboardingWorld) thenAtLeastOneResponseHasStatus(expected int) error {
	for _, code := range w.lastStatusCodes {
		if code == expected {
			return nil
		}
	}
	return fmt.Errorf("nenhuma resposta com status %d encontrada, recebidos: %v", expected, w.lastStatusCodes)
}
