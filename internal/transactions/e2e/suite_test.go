//go:build e2e

package transactions_e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
	txconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers"
)

const (
	txE2ETestUserID                    = "11111111-1111-1111-1111-111111111111"
	txE2EPrazerosRootCategoryUUID      = "ac535261-4060-56ef-b2e8-57c8cc7032d1"
	txE2EOutrosPrazeresSubcategoryUUID = "0016763e-655c-571a-90cb-bec5a18d4969"
	txE2ESalarioRootCategoryUUID       = "86dd34b0-7342-525a-9a30-b1b5a76b109f"
	txE2EDecimoTerceiroSubcategoryUUID = "98455e74-b1f3-5b9c-a8d8-05db0cdb465d"
)

type txE2ERuntime struct {
	server            *httptest.Server
	db                *sqlx.DB
	recomputeConsumer *txconsumers.MonthlySummaryRecomputeConsumer
	recurringJob      worker.Job
	reconcilerJob     worker.Job
}

type txNoOpChannelGateway struct{}

func (g *txNoOpChannelGateway) SendText(_ context.Context, _, _, _ string) error { return nil }
func (g *txNoOpChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", fmt.Errorf("not supported in tx e2e")
}

type txNoOpChannelResolver struct{}

func (r *txNoOpChannelResolver) ResolvePreferred(_ context.Context, _ uuid.UUID) (cardinterfaces.UserChannelPreference, bool, error) {
	return cardinterfaces.UserChannelPreference{}, false, nil
}

func TestE2ETransactions(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()
	cfg := loadTxE2EConfig(t)

	rt := buildTxE2EServer(t, db, cfg, o11y)
	t.Cleanup(rt.server.Close)

	suite := godog.TestSuite{
		Name: "transactions-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			e := &txE2ECtx{
				server:            rt.server,
				db:                db,
				userID:            uuid.MustParse(txE2ETestUserID),
				recomputeConsumer: rt.recomputeConsumer,
				recurringJob:      rt.recurringJob,
				reconcilerJob:     rt.reconcilerJob,
			}
			registerAllSteps(sc, e)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("cenários E2E de transactions falharam")
	}
}

func registerAllSteps(sc *godog.ScenarioContext, e *txE2ECtx) {
	registerSharedSteps(sc, e)
	registerTransactionSteps(sc, e)
	registerCreditCardSteps(sc, e)
	registerRecurringTemplateSteps(sc, e)
	registerMonthlySteps(sc, e)
	registerCardInvoiceSteps(sc, e)
	registerConsumerSteps(sc, e)
	registerJobSteps(sc, e)
}

func buildTxE2EServer(t *testing.T, db *sqlx.DB, cfg *configs.Config, o11y observability.Observability) *txE2ERuntime {
	t.Helper()

	ctx := context.Background()
	passthrough := func(next http.Handler) http.Handler { return next }

	seedTxE2EUser(t, db)

	categoriesModule := categories.NewCategoriesModule(db, o11y, passthrough)

	cardModule, err := card.NewCardModule(ctx, cfg, o11y, db, passthrough, &txNoOpChannelGateway{}, &txNoOpChannelResolver{})
	if err != nil {
		t.Fatalf("card module: %v", err)
	}

	txModule, err := transactions.NewTransactionsModule(cfg, o11y, db, cardModule, categoriesModule, passthrough)
	if err != nil {
		t.Fatalf("transactions module: %v", err)
	}

	router := chi.NewRouter()
	if cardModule.CardRouter != nil && txModule.GetCardInvoiceHandler != nil {
		cardModule.CardRouter.WithInvoiceByMonthHandler(txModule.GetCardInvoiceHandler)
	}
	if cardModule.CardRouter != nil {
		cardModule.CardRouter.Register(router)
	}
	if txModule.Router != nil {
		txModule.Router.Register(router)
	}

	return &txE2ERuntime{
		server:            httptest.NewServer(router),
		db:                db,
		recomputeConsumer: txModule.MonthlySummaryRecomputeConsumer,
		recurringJob:      txModule.RecurringMaterializerJob,
		reconcilerJob:     txModule.MonthlySummaryReconcilerJob,
	}
}

func loadTxE2EConfig(t *testing.T) *configs.Config {
	t.Helper()

	cfg, err := configs.LoadConfig("../../../")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.TransactionsConfig.Enabled = true
	cfg.TransactionsConfig.MonthlySummaryDebounceWindow = 50 * time.Millisecond
	cfg.OutboxConfig.RetryMaxAttempts = 3
	cfg.IdentityConfig.GatewaySharedSecretCurrent = "6161616161616161616161616161616161616161616161616161616161616161"
	cfg.IdentityConfig.GatewaySharedSecretNext = ""
	return cfg
}

func seedTxE2EUser(t *testing.T, db *sqlx.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, txE2ETestUserID, "+5511999990000")
	if err != nil {
		t.Fatalf("seed tx e2e user: %v", err)
	}
}
