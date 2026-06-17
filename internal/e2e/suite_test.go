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

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

const e2eTestUserID = "11111111-1111-1111-1111-111111111111"

func TestE2E(t *testing.T) {
	mgr, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	cfg := loadConfig(t)

	server := buildServer(t, cfg, mgr, o11y)
	t.Cleanup(server.Close)

	suite := godog.TestSuite{
		Name: "e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			e := &e2eCtx{
				server: server,
				mgr:    mgr,
				userID: uuid.MustParse(e2eTestUserID),
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
	return cfg
}

func buildServer(t *testing.T, cfg *configs.Config, mgr manager.Manager, o11y observability.Observability) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	passthrough := func(next http.Handler) http.Handler { return next }

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, passthrough)

	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, passthrough, nil, nil)
	if err != nil {
		t.Fatalf("card module: %v", err)
	}

	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, passthrough)
	if err != nil {
		t.Fatalf("transactions module: %v", err)
	}

	router := chi.NewRouter()
	router.Use(e2eAuthMiddleware)

	if categoriesModule.CategoryRouter != nil {
		categoriesModule.CategoryRouter.Register(router)
	}
	if txModule.Router != nil {
		txModule.Router.Register(router)
	}

	return httptest.NewServer(router)
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
