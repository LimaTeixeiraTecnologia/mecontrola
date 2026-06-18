//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func registerCheckoutHTTPSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^o cliente cria uma checkout session com plan_id "([^"]*)"$`, w.whenClientCreatesCheckoutSession)
	sc.Step(`^o cliente envia um payload inválido para checkout$`, w.whenClientCreatesCheckoutWithInvalidJSON)
	sc.Step(`^o cliente cria uma checkout session com plan_id desconhecido "([^"]*)"$`, w.whenClientCreatesCheckoutWithUnknownPlan)
	sc.Step(`^o cliente faz preflight de checkout com origin permitida$`, w.whenClientPreflightsCheckoutWithAllowedOrigin)
	sc.Step(`^o token de onboarding deve estar persistido$`, w.thenOnboardingTokenShouldBePersisted)
}

func (w *onboardingWorld) whenClientCreatesCheckoutSession(planID string) error {
	if err := w.postJSON("/api/v1/onboarding/checkout", map[string]any{"plan_id": planID}, map[string]string{
		"Origin": e2eCheckoutOrigin,
	}); err != nil {
		return err
	}
	return w.extractTokenFromCheckoutURL()
}

func (w *onboardingWorld) whenClientCreatesCheckoutWithInvalidJSON() error {
	req, err := http.NewRequest(http.MethodPost, w.runtime.server.URL+"/api/v1/onboarding/checkout", strings.NewReader("not-json"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return w.do(req)
}

func (w *onboardingWorld) whenClientCreatesCheckoutWithUnknownPlan(planID string) error {
	return w.postJSON("/api/v1/onboarding/checkout", map[string]any{"plan_id": planID}, nil)
}

func (w *onboardingWorld) whenClientPreflightsCheckoutWithAllowedOrigin() error {
	return w.options("/api/v1/onboarding/checkout", map[string]string{
		"Origin":                        e2eCheckoutOrigin,
		"Access-Control-Request-Method": "POST",
	})
}

func (w *onboardingWorld) thenOnboardingTokenShouldBePersisted() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := parseClearToken(w.currentTokenClear)
	if err != nil {
		return err
	}
	var count int
	if err := w.runtime.deps.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.onboarding_tokens WHERE token_hash = $1`, token.Hash()).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("esperado 1 token persistido, recebido %d", count)
	}
	return nil
}
