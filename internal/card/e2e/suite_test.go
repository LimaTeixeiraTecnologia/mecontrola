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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

const (
	e2eUserID    = "11111111-1111-1111-1111-111111111111"
	e2eUserPhone = "+5511999990000"
)

type cardE2ERuntime struct {
	server         *httptest.Server
	invoiceDueJob  worker.Job
	channelGateway *cardE2EChannelGateway
	eventHandlers  map[string]platformevents.Handler
}

type cardE2EChannelGateway struct {
	mu       sync.Mutex
	messages []cardE2ESentMessage
}

type cardE2ESentMessage struct {
	Channel    string
	ExternalID string
	Text       string
}

func (g *cardE2EChannelGateway) SendText(_ context.Context, channel, externalID, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = append(g.messages, cardE2ESentMessage{Channel: channel, ExternalID: externalID, Text: text})
	return nil
}

func (g *cardE2EChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", fmt.Errorf("unsupported in card e2e")
}

type cardE2EUserChannelResolver struct{}

func (r *cardE2EUserChannelResolver) ResolvePreferred(_ context.Context, userID uuid.UUID) (cardinterfaces.UserChannelPreference, bool, error) {
	if userID.String() == e2eUserID {
		return cardinterfaces.UserChannelPreference{
			Channel:    notification.ChannelWhatsApp,
			ExternalID: e2eUserPhone,
		}, true, nil
	}
	return cardinterfaces.UserChannelPreference{}, false, nil
}

func TestE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()

	runtime := buildCardServer(t, db, o11y)
	t.Cleanup(runtime.server.Close)

	suite := godog.TestSuite{
		Name: "card-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			e := newCardE2ECtx(runtime, db)
			sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				runtime.channelGateway.mu.Lock()
				runtime.channelGateway.messages = nil
				runtime.channelGateway.mu.Unlock()
				return ctx, nil
			})
			registerSteps(sc, e)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("cenários e2e card falharam")
	}
}

func buildCardServer(t *testing.T, db *sqlx.DB, o11y observability.Observability) *cardE2ERuntime {
	t.Helper()

	ctx := context.Background()
	passthrough := func(next http.Handler) http.Handler { return next }

	seedCardE2EUser(t, db)

	cfg := &configs.Config{
		CardConfig: configs.CardConfig{
			InvoiceDueAlertsEnabled: true,
			InvoiceDueWindowDays:    3,
			InvoiceDueScanLimit:     100,
		},
		AuthRateLimit: configs.AuthRateLimitConfig{
			PerUserPerMin: 60000,
			PerUserBurst:  60000,
		},
		OutboxConfig: configs.OutboxConfig{
			RetryMaxAttempts: 3,
		},
	}

	channelGateway := &cardE2EChannelGateway{}
	channelResolver := &cardE2EUserChannelResolver{}

	mod, err := card.NewCardModule(ctx, cfg, o11y, db, passthrough, channelGateway, channelResolver)
	if err != nil {
		t.Fatalf("card module: %v", err)
	}

	router := chi.NewRouter()
	if mod.CardRouter != nil {
		mod.CardRouter.Register(router)
	}

	handlers := make(map[string]platformevents.Handler, len(mod.EventHandlers))
	for _, reg := range mod.EventHandlers {
		handlers[reg.EventType] = reg.Handler
	}

	return &cardE2ERuntime{
		server:         httptest.NewServer(router),
		invoiceDueJob:  mod.InvoiceDueAlertsJob,
		channelGateway: channelGateway,
		eventHandlers:  handlers,
	}
}

func seedCardE2EUser(t *testing.T, db *sqlx.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
        INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
        VALUES ($1, $2, 'ACTIVE', now(), now())
        ON CONFLICT (id) DO NOTHING
    `, e2eUserID, e2eUserPhone)
	if err != nil {
		t.Fatalf("seed e2e user: %v", err)
	}
}

func registerSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	registerSharedSteps(sc, e)
	registerCreateSteps(sc, e)
	registerReadListSteps(sc, e)
	registerUpdateSteps(sc, e)
	registerDeleteSteps(sc, e)
	registerInvoiceSteps(sc, e)
	registerConsumerSteps(sc, e)
}
