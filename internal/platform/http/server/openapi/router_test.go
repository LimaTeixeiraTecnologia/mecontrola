package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func TestNewRouterIfEnabled(t *testing.T) {
	t.Run("deve habilitar docs apenas em local", func(t *testing.T) {
		router, err := NewRouterIfEnabled(&configs.Config{
			AppConfig: configs.AppConfig{Environment: "local"},
		})
		if err != nil {
			t.Fatalf("NewRouterIfEnabled() error = %v", err)
		}
		if router == nil {
			t.Fatal("esperava router local de docs")
		}
	})

	for _, env := range []string{"staging", "production"} {
		t.Run("deve desabilitar docs em "+env, func(t *testing.T) {
			router, err := NewRouterIfEnabled(&configs.Config{
				AppConfig: configs.AppConfig{Environment: env},
			})
			if err != nil {
				t.Fatalf("NewRouterIfEnabled() error = %v", err)
			}
			if router != nil {
				t.Fatalf("esperava router nil para %s", env)
			}
		})
	}
}

func TestDocsRouter_HTML(t *testing.T) {
	router := newDocsTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/__docs?module=categories", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"MeControla OpenAPI Docs",
		"MeControla Categories API",
		"/api/v1/categories",
		"/__docs/openapi/categories.yaml",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("html docs must contain %q", expected)
		}
	}
}

func TestDocsRouter_IndexJSON(t *testing.T) {
	router := newDocsTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/__docs/openapi/index.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var catalog []catalogItem
	if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(catalog) != len(specDefinitions) {
		t.Fatalf("catalog len = %d, want %d", len(catalog), len(specDefinitions))
	}

	expectedModules := []string{"identity", "billing", "categories", "onboarding", "card", "budgets", "transactions"}
	for i, module := range expectedModules {
		if catalog[i].Module != module {
			t.Fatalf("catalog[%d].Module = %q, want %q", i, catalog[i].Module, module)
		}
		if catalog[i].RawURL == "" || catalog[i].DocsURL == "" {
			t.Fatalf("catalog[%d] missing urls: %+v", i, catalog[i])
		}
	}
}

func TestDocsRouter_RawYAML(t *testing.T) {
	router := newDocsTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/__docs/openapi/card.yaml", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"openapi: 3.2.0",
		"title: MeControla Cards API",
		"/api/v1/cards",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("raw yaml must contain %q", expected)
		}
	}
}

func TestLoadSpecs_ValidatePaths(t *testing.T) {
	specs, err := loadSpecs()
	if err != nil {
		t.Fatalf("loadSpecs() error = %v", err)
	}

	expected := map[string]map[string][]string{
		"identity": {
			"/api/v1/identity/users": {http.MethodPost},
		},
		"billing": {
			"/api/v1/billing/webhooks/kiwify": {http.MethodPost},
		},
		"categories": {
			"/api/v1/categories":                 {http.MethodGet},
			"/api/v1/categories/{id}":            {http.MethodGet},
			"/api/v1/category-dictionary":        {http.MethodGet},
			"/api/v1/category-dictionary/search": {http.MethodGet},
		},
		"onboarding": {
			"/api/v1/onboarding/checkout":             {http.MethodPost},
			"/api/v1/onboarding/tokens/{token}/state": {http.MethodGet},
		},
		"card": {
			"/api/v1/cards":               {http.MethodGet, http.MethodPost},
			"/api/v1/cards/{id}":          {http.MethodGet, http.MethodPut, http.MethodDelete},
			"/api/v1/cards/{id}/invoices": {http.MethodGet},
		},
		"budgets": {
			"/api/v1/budgets":                       {http.MethodPost},
			"/api/v1/budgets/recurrence":            {http.MethodPost},
			"/api/v1/budgets/alerts":                {http.MethodGet},
			"/api/v1/budgets/expenses":              {http.MethodPost},
			"/api/v1/budgets/expenses/{id}":         {http.MethodPatch, http.MethodDelete},
			"/api/v1/budgets/{competence}/activate": {http.MethodPost},
			"/api/v1/budgets/{competence}":          {http.MethodDelete},
			"/api/v1/budgets/{competence}/summary":  {http.MethodGet},
		},
		"transactions": {
			"/api/v1/transactions":                         {http.MethodGet, http.MethodPost},
			"/api/v1/transactions/{id}":                    {http.MethodGet, http.MethodPatch, http.MethodDelete},
			"/api/v1/card-purchases":                       {http.MethodGet, http.MethodPost},
			"/api/v1/card-purchases/{id}":                  {http.MethodGet, http.MethodPatch, http.MethodDelete},
			"/api/v1/cards/{card_id}/invoices/{ref_month}": {http.MethodGet},
			"/api/v1/recurring-templates":                  {http.MethodGet, http.MethodPost},
			"/api/v1/recurring-templates/{id}":             {http.MethodGet, http.MethodPatch, http.MethodDelete},
			"/api/v1/months/{ref_month}":                   {http.MethodGet},
			"/api/v1/months/{ref_month}/entries":           {http.MethodGet},
		},
	}

	for module, paths := range expected {
		spec := specs[module]
		pathMap := map[string]map[string]bool{}
		for _, op := range spec.Operations {
			if pathMap[op.Path] == nil {
				pathMap[op.Path] = map[string]bool{}
			}
			pathMap[op.Path][op.Method] = true
		}

		for path, methods := range paths {
			registered, ok := pathMap[path]
			if !ok {
				t.Fatalf("module %s missing path %s", module, path)
			}
			for _, method := range methods {
				if !registered[method] {
					t.Fatalf("module %s path %s missing method %s", module, path, method)
				}
			}
		}
	}
}

func newDocsTestRouter(t *testing.T) http.Handler {
	t.Helper()

	docsRouter, err := NewRouter()
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	router := chi.NewRouter()
	docsRouter.Register(router)
	return router
}
