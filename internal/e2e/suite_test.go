//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
	txhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server/handlers"
)

const e2eTestUserID = "11111111-1111-1111-1111-111111111111"
const e2eTestUserPhone = "+5511999990000"
const e2eBillingWebhookSecret = "e2e-billing-secret"
const e2eBillingProductIDMonthly = "e2e-kiwify-monthly"

type e2eRuntime struct {
	server           *httptest.Server
	invoiceDueAlerts worker.Job
	channelGateway   *e2eChannelGateway
}

type e2eChannelGateway struct {
	mu       sync.Mutex
	messages []e2eSentMessage
}

type e2eSentMessage struct {
	Channel    string
	ExternalID string
	Text       string
}

func (g *e2eChannelGateway) SendText(_ context.Context, channel, externalID, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.messages = append(g.messages, e2eSentMessage{
		Channel:    channel,
		ExternalID: externalID,
		Text:       text,
	})
	return nil
}

func (g *e2eChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", fmt.Errorf("template sending unsupported in e2e")
}

type e2eUserChannelResolver struct {
	preferences map[uuid.UUID]cardinterfaces.UserChannelPreference
}

func (r *e2eUserChannelResolver) ResolvePreferred(_ context.Context, userID uuid.UUID) (cardinterfaces.UserChannelPreference, bool, error) {
	pref, ok := r.preferences[userID]
	return pref, ok, nil
}

func TestE2E(t *testing.T) {
	mgr, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	cfg := loadConfig(t)

	runtime := buildServer(t, cfg, mgr, o11y)
	t.Cleanup(runtime.server.Close)

	suite := godog.TestSuite{
		Name: "e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			e := &e2eCtx{
				server:               runtime.server,
				mgr:                  mgr,
				userID:               uuid.MustParse(e2eTestUserID),
				invoiceDueAlertsJob:  runtime.invoiceDueAlerts,
				channelGateway:       runtime.channelGateway,
				billingWebhookSecret: e2eBillingWebhookSecret,
				billingProductID:     e2eBillingProductIDMonthly,
			}
			registerSteps(sc, e)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("cenários BDD falharam")
	}
}

func loadConfig(t *testing.T) *configs.Config {
	t.Helper()

	cfg, err := configs.LoadConfig("../../")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.TransactionsConfig.Enabled = true
	cfg.CardConfig.InvoiceDueAlertsEnabled = true
	cfg.CardConfig.InvoiceDueWindowDays = 3
	cfg.CardConfig.InvoiceDueScanLimit = 100
	cfg.KiwifyConfig.APIBaseURL = "https://example.com"
	cfg.KiwifyConfig.AccountID = "e2e-account"
	cfg.KiwifyConfig.ClientID = "e2e-client"
	cfg.KiwifyConfig.ClientSecret = "e2e-secret"
	cfg.KiwifyConfig.ProductIDMonthly = e2eBillingProductIDMonthly
	cfg.KiwifyConfig.ProductIDQuarterly = "e2e-kiwify-quarterly"
	cfg.KiwifyConfig.ProductIDAnnual = "e2e-kiwify-annual"
	cfg.KiwifyConfig.WebhookSecret = e2eBillingWebhookSecret
	cfg.KiwifyConfig.WebhookSecretNext = ""
	cfg.KiwifyConfig.OAuthTokenSafetyMargin = time.Second
	cfg.KiwifyConfig.HTTPTimeout = time.Second
	cfg.KiwifyConfig.HTTPRetryBackoff = time.Second
	cfg.OutboxConfig.RetryMaxAttempts = 3
	return cfg
}

func buildServer(t *testing.T, cfg *configs.Config, mgr *sqlx.DB, o11y observability.Observability) *e2eRuntime {
	t.Helper()

	ctx := context.Background()
	passthrough := func(next http.Handler) http.Handler { return next }

	seedE2EUser(t, mgr)

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, passthrough)

	channelGateway := &e2eChannelGateway{}
	channelResolver := &e2eUserChannelResolver{
		preferences: map[uuid.UUID]cardinterfaces.UserChannelPreference{
			uuid.MustParse(e2eTestUserID): {
				Channel:    notification.ChannelWhatsApp,
				ExternalID: e2eTestUserPhone,
			},
		},
	}

	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, passthrough, channelGateway, channelResolver)
	if err != nil {
		t.Fatalf("card module: %v", err)
	}

	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, passthrough)
	if err != nil {
		t.Fatalf("transactions module: %v", err)
	}

	billingModule, err := billing.NewBillingModule(cfg, o11y, mgr)
	if err != nil {
		t.Fatalf("billing module: %v", err)
	}

	router := chi.NewRouter()
	router.Use(e2eAuthMiddleware)

	if categoriesModule.CategoryRouter != nil {
		categoriesModule.CategoryRouter.Register(router)
	}
	if cardModule.CardRouter != nil {
		cardModule.CardRouter.Register(router)
	}
	registerTransactionE2ERoutes(router, txModule, o11y)
	if billingModule.WebhookRouter != nil {
		billingModule.WebhookRouter.Register(router)
	}

	return &e2eRuntime{
		server:           httptest.NewServer(router),
		invoiceDueAlerts: cardModule.InvoiceDueAlertsJob,
		channelGateway:   channelGateway,
	}
}

func e2eAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := auth.Principal{
			UserID: uuid.MustParse(e2eTestUserID),
			Source: auth.SourceHeader,
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}

func seedE2EUser(t *testing.T, mgr *sqlx.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := mgr.ExecContext(ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, e2eTestUserID, e2eTestUserPhone)
	if err != nil {
		t.Fatalf("seed e2e user: %v", err)
	}
}

func registerTransactionE2ERoutes(router chi.Router, txModule transactions.TransactionsModule, o11y observability.Observability) {
	if txModule.CreateTransactionUC == nil {
		return
	}

	createTransaction := txhandlers.NewCreateTransactionHandler(txModule.CreateTransactionUC, o11y)
	router.Route("/api/v1/transactions", func(sub chi.Router) {
		sub.Post("/", createTransaction.Handle)
	})
}
