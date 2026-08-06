//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

const e2eUserID = "11111111-1111-1111-1111-111111111111"

func TestE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	server := buildServer(t, db, o11y)
	t.Cleanup(server.Close)

	suite := godog.TestSuite{
		Name: "budgets-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			ctx := newBudgetsE2ECtx(server, db)
			registerBudgetSteps(sc, ctx)
			registerExpenseSteps(sc, ctx)
			registerSharedBudgetSteps(sc, ctx)
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

func buildServer(t *testing.T, db *sqlx.DB, o11y observability.Observability) *httptest.Server {
	t.Helper()

	catModule := categories.NewCategoriesModule(db, o11y, passthroughMiddleware)

	budgetsModule, err := budgets.NewBudgetsModule(
		testConfig(),
		o11y,
		db,
		catModule,
		passthroughMiddleware,
		newNoopChannelGateway(),
		newNoopChannelResolver(),
	)
	if err != nil {
		t.Fatalf("build budgets module: %v", err)
	}

	router := chi.NewRouter()
	if budgetsModule.BudgetsRouter != nil {
		budgetsModule.BudgetsRouter.Register(router)
	}

	return httptest.NewServer(router)
}

func passthroughMiddleware(next http.Handler) http.Handler {
	return next
}

func testConfig() *configs.Config {
	return &configs.Config{
		BudgetsConfig: configs.BudgetsConfig{
			ThresholdAlertsMode: configs.ThresholdAlertsModeLegacy,
		},
		OutboxConfig: configs.OutboxConfig{
			RetryMaxAttempts: 3,
		},
	}
}

type noopChannelGateway struct{}

func newNoopChannelGateway() notification.ChannelGateway {
	return &noopChannelGateway{}
}

func (n *noopChannelGateway) SendText(_ context.Context, _, _, _ string) error {
	return nil
}

func (n *noopChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

type noopChannelResolver struct{}

func newNoopChannelResolver() appinterfaces.UserChannelResolver {
	return &noopChannelResolver{}
}

func (n *noopChannelResolver) ResolvePreferred(_ context.Context, _ uuid.UUID) (appinterfaces.UserChannelPreference, bool, error) {
	return appinterfaces.UserChannelPreference{}, false, nil
}
