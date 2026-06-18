//go:build e2e

package e2e_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
)

const (
	e2eWebhookSecret    = "e2e-billing-webhook-secret"
	e2eProductMonthly   = "e2e-product-monthly"
	e2eProductQuarterly = "e2e-product-quarterly"
	e2eProductAnnual    = "e2e-product-annual"
)

func TestE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()

	cfg := &configs.Config{
		KiwifyConfig: configs.KiwifyConfig{
			APIBaseURL:             "https://example.invalid",
			AccountID:              "e2e-account",
			ClientID:               "e2e-client-id",
			ClientSecret:           "e2e-client-secret",
			ProductIDMonthly:       e2eProductMonthly,
			ProductIDQuarterly:     e2eProductQuarterly,
			ProductIDAnnual:        e2eProductAnnual,
			WebhookSecret:          e2eWebhookSecret,
			WebhookSecretNext:      "",
			OAuthTokenSafetyMargin: time.Second,
			HTTPTimeout:            time.Second,
			HTTPRetryBackoff:       time.Second,
			HTTPRetryMaxAttempts:   1,
			WebhookRateLimitPerMin: 100_000,
			WebhookRateLimitBurst:  100_000,
		},
		OutboxConfig: configs.OutboxConfig{
			RetryMaxAttempts: 3,
		},
		BillingConfig: configs.BillingConfig{
			GraceExpirationSchedule: "@every 30m",
		},
	}

	mod, err := billing.NewBillingModule(cfg, o11y, db)
	if err != nil {
		t.Fatalf("billing module: %v", err)
	}

	server := buildServer(t, mod)
	t.Cleanup(server.Close)

	suite := godog.TestSuite{
		Name: "billing-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			e := newBillingE2ECtx(server, db, mod, e2eWebhookSecret, e2eProductMonthly)
			registerSteps(sc, e)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("cenários e2e billing falharam")
	}
}

func buildServer(t *testing.T, mod billing.BillingModule) *httptest.Server {
	t.Helper()

	router := chi.NewRouter()
	if mod.WebhookRouter != nil {
		mod.WebhookRouter.Register(router)
	}
	return httptest.NewServer(router)
}

func registerSteps(sc *godog.ScenarioContext, e *billingE2ECtx) {
	registerSharedSteps(sc, e)
	registerWebhookSteps(sc, e)
	registerOutboxSteps(sc, e)
	registerConsumerSteps(sc, e)
	registerJobSteps(sc, e)
}
