//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
)

const (
	e2eTestUserID    = "11111111-1111-1111-1111-111111111111"
	e2eTestUserPhone = "+5511999990000"
)

func TestIdentityE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	o11y := noop.NewProvider()

	cfg := &configs.Config{
		IdentityConfig: configs.IdentityConfig{
			GatewaySharedSecretCurrent:     "6161616161616161616161616161616161616161616161616161616161616161",
			GatewaySharedSecretNext:        "",
			AuthEventsHousekeepingSchedule: "@daily",
			AuthEventsHousekeepingBatch:    100,
			AuthEventsRetentionDays:        90,
		},
		OutboxConfig: configs.OutboxConfig{
			RetryMaxAttempts: 3,
		},
	}

	identityModule, err := identity.NewIdentityModule(cfg, o11y, db)
	if err != nil {
		t.Fatalf("identity module: %v", err)
	}

	seedIdentityE2EUser(t, db)

	router := chi.NewRouter()
	router.Use(identityE2EAuthMiddleware)
	if identityModule.UserRouter != nil {
		identityModule.UserRouter.Register(router)
	}

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	suite := godog.TestSuite{
		Name: "identity-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			var pgNow time.Time
			_ = db.QueryRowContext(context.Background(), "SELECT now()").Scan(&pgNow)
			e := &e2eCtx{
				mgr:                   db,
				httpServer:            server,
				userID:                uuid.MustParse(e2eTestUserID),
				identityModule:        identityModule,
				identityScenarioStart: pgNow,
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
		t.Fatal("cenários e2e identity falharam")
	}
}

func identityE2EAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := auth.Principal{
			UserID: uuid.MustParse(e2eTestUserID),
			Source: auth.SourceHeader,
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}

func seedIdentityE2EUser(t *testing.T, db *sqlx.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, e2eTestUserID, e2eTestUserPhone)
	if err != nil {
		t.Fatalf("seed identity e2e user: %v", err)
	}
}
