//go:build e2e

package e2e_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cucumber/godog"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
)

const e2eUserID = "11111111-1111-1111-1111-111111111111"

func TestE2E(t *testing.T) {
	db, _ := postgres.NewTestDatabase(t)
	runtime := buildServer(db, noop.NewProvider())
	t.Cleanup(runtime.server.Close)

	suite := godog.TestSuite{
		Name: "categories-e2e",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			registerSteps(sc, newCategoriesE2ECtx(runtime.server, db))
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

type categoriesRuntime struct {
	server *httptest.Server
}

func buildServer(db *sqlx.DB, o11y observability.Observability) *categoriesRuntime {
	module := categories.NewCategoriesModule(db, o11y, passthroughMiddleware)

	router := chi.NewRouter()
	if module.CategoryRouter != nil {
		module.CategoryRouter.Register(router)
	}

	return &categoriesRuntime{
		server: httptest.NewServer(router),
	}
}

func passthroughMiddleware(next http.Handler) http.Handler {
	return next
}

func registerSteps(sc *godog.ScenarioContext, e *categoriesE2ECtx) {
	registerCategoriesListSteps(sc, e)
	registerCategoryGetSteps(sc, e)
	registerDictionaryListSteps(sc, e)
	registerDictionarySearchSteps(sc, e)
	registerSharedSteps(sc, e)
}
