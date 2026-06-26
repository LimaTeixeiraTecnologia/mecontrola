//go:build e2e

package e2e_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cucumber/godog"

	agentservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
)

func TestE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	runtime := buildOnboardingRuntime(t, db)
	t.Cleanup(runtime.server.Close)

	suite := godog.TestSuite{
		Name: "onboarding-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			world := newOnboardingWorld(t, runtime)
			registerSharedSteps(sc, world)
			registerCheckoutHTTPSteps(sc, world)
			registerTokenStateHTTPSteps(sc, world)
			registerBillingOutboxSteps(sc, world)
			registerActivationProcessorSteps(sc, world)
			registerJobSteps(sc, world)
			registerRobustnessSteps(sc, world)
			registerOutboxPublisherSteps(sc, world)
			registerOnboardingConversationalSteps(sc, world)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("cenarios e2e falharam")
	}
}

type onboardingRuntime struct {
	server          *httptest.Server
	httpClient      *http.Client
	deps            *onboardingDependencies
	metaGateway     *recordingWhatsAppGateway
	outreachGateway *recordingOutreachGateway
	emailSender     *recordingEmailSender
	failingHandler  *forcedFailureHandler
	registryFactory func() *eventRegistry
	onboardingAgent *agentservices.OnboardingAgent
}
