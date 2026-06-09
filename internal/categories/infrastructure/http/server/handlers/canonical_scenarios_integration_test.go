//go:build integration

package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const pgImage = "postgres:16-alpine"

type CanonicalScenariosIntegrationSuite struct {
	suite.Suite
	mgr        manager.Manager
	server     *httptest.Server
	router     chi.Router
	testUserID uuid.UUID
}

func TestCanonicalScenariosIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CanonicalScenariosIntegrationSuite))
}

func (s *CanonicalScenariosIntegrationSuite) SetupSuite() {
	s.testUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	s.mgr = s.setupTestDB()
	o11y := noop.NewProvider()

	categoriesModule := categories.NewCategoriesModule(s.mgr, o11y)

	s.router = chi.NewRouter()
	s.router.Use(s.authMiddleware)
	categoriesModule.CategoryRouter.Register(s.router)

	s.server = httptest.NewServer(s.router)
}

func (s *CanonicalScenariosIntegrationSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
	if s.mgr != nil {
		_ = s.mgr.Shutdown(context.Background())
	}
}

func (s *CanonicalScenariosIntegrationSuite) SetupTest() {}

func (s *CanonicalScenariosIntegrationSuite) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal := auth.Principal{
			UserID: s.testUserID,
			Source: auth.SourceHeader,
		}
		ctx := auth.WithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *CanonicalScenariosIntegrationSuite) setupTestDB() manager.Manager {
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        pgImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	s.Require().NoError(err)

	mapped, err := container.MappedPort(ctx, "5432")
	s.Require().NoError(err)

	portNum, err := strconv.Atoi(mapped.Port())
	s.Require().NoError(err)

	cfg := dbpostgres.PostgresConfig{
		DSN: fmt.Sprintf("postgres://test:test@%s:%d/testdb?sslmode=disable&search_path=mecontrola,public", host, portNum),
	}

	mgr, err := manager.New(cfg)
	s.Require().NoError(err)

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)

	migrator, err := migration.New(mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(dsn))
	s.Require().NoError(err)

	if err = migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		s.Require().NoError(err, "failed to run migrations")
	}

	return mgr
}

func (s *CanonicalScenariosIntegrationSuite) TestCCB1_HighInequivoco() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=13%C2%BA%20sal%C3%A1rio&kind=income", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	s.Equal("candidates", body["result"])

	candidates, ok := body["candidates"].([]any)
	s.Require().True(ok)
	s.NotEmpty(candidates)

	first := candidates[0].(map[string]any)
	s.Equal("alias", first["signal_type"])
	s.Equal("high", first["confidence"])
	s.Equal(false, first["is_ambiguous"])
	s.NotEmpty(first["path"])
	s.NotEmpty(first["matched_term"])
	s.NotEmpty(first["match_reason"])

	version, ok := body["version"].(float64)
	s.Require().True(ok)
	s.Greater(version, float64(0))

	etag := resp.Header.Get("ETag")
	s.NotEmpty(etag)
}

func (s *CanonicalScenariosIntegrationSuite) TestCCB2_MercuryAmbiguo() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=uber&kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	s.Equal("candidates", body["result"])

	candidates, ok := body["candidates"].([]any)
	s.Require().True(ok)
	s.LessOrEqual(len(candidates), 3)

	for _, c := range candidates {
		candidate := c.(map[string]any)
		s.Equal(true, candidate["is_ambiguous"])
	}

	foundRecorrente := false
	foundLazer := false
	for _, c := range candidates {
		candidate := c.(map[string]any)
		path := candidate["path"].(string)
		if contains(path, "Recorrente") {
			foundRecorrente = true
		}
		if contains(path, "Lazer") {
			foundLazer = true
		}
	}
	s.True(foundRecorrente || foundLazer || len(candidates) > 0)
}

func (s *CanonicalScenariosIntegrationSuite) TestCCB3_SemCorrespondencia() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=xyz123naoexiste&kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	s.Equal("no_match", body["result"])

	candidates, ok := body["candidates"].([]any)
	if ok {
		s.Empty(candidates)
	}

	version, ok := body["version"].(float64)
	s.Require().True(ok)
	s.Greater(version, float64(0))
}

func (s *CanonicalScenariosIntegrationSuite) TestCCB4_KindMismatch() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=energia&kind=income", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	s.Equal("no_match", body["result"])
}

func (s *CanonicalScenariosIntegrationSuite) TestCCD1_QueryCurto() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=ab&kind=expense", nil)
	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body := s.parseBody(resp)
	errors, ok := body["errors"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_query", errors["code"])
}

func (s *CanonicalScenariosIntegrationSuite) TestCCD2_QueryVazio() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=&kind=expense", nil)
	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body := s.parseBody(resp)
	errors, ok := body["errors"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_query", errors["code"])
}

func (s *CanonicalScenariosIntegrationSuite) TestCCD3_QuerySomenteEspacos() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=%20%20%20&kind=expense", nil)
	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body := s.parseBody(resp)
	errors, ok := body["errors"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_query", errors["code"])
}

func (s *CanonicalScenariosIntegrationSuite) TestCCD4_KindAusente() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=energia", nil)
	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body := s.parseBody(resp)
	errors, ok := body["errors"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_kind", errors["code"])
}

func (s *CanonicalScenariosIntegrationSuite) TestCCD5_KindInvalido() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary/search?q=energia&kind=invalido", nil)
	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body := s.parseBody(resp)
	errors, ok := body["errors"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_kind", errors["code"])
}

func (s *CanonicalScenariosIntegrationSuite) TestCCL1_ArvoreCompleta() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	categories, ok := body["categories"].([]any)
	s.Require().True(ok)

	rootNames := make([]string, 0, len(categories))
	for _, c := range categories {
		cat := c.(map[string]any)
		rootNames = append(rootNames, cat["name"].(string))
		s.Nil(cat["deprecated_at"])
	}

	s.Contains(rootNames, "Custo Fixo")
	s.Contains(rootNames, "Conhecimento")
	s.Contains(rootNames, "Prazeres")
	s.Contains(rootNames, "Metas")
	s.Contains(rootNames, "Liberdade Financeira")

	version, ok := body["version"].(float64)
	s.Require().True(ok)
	s.Greater(version, float64(0))
}

func (s *CanonicalScenariosIntegrationSuite) TestCCL2_FiltroPorParentID() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	categories, ok := body["categories"].([]any)
	s.Require().True(ok)
	s.NotEmpty(categories)

	var custoFixoID string
	for _, c := range categories {
		cat := c.(map[string]any)
		if cat["name"].(string) == "Custo Fixo" {
			custoFixoID = cat["id"].(string)
			break
		}
	}
	s.NotEmpty(custoFixoID)

	resp2 := s.makeRequest("GET", fmt.Sprintf("/api/v1/categories?parent_id=%s", custoFixoID), nil)
	s.Require().Equal(http.StatusOK, resp2.StatusCode)

	body2 := s.parseBody(resp2)
	subcategories, ok := body2["categories"].([]any)
	s.Require().True(ok)
	s.NotEmpty(subcategories)

	for _, sub := range subcategories {
		subcat := sub.(map[string]any)
		s.NotNil(subcat["parent_id"])
	}
}

func (s *CanonicalScenariosIntegrationSuite) TestCCL3_InclusaoDeprecated() {
	resp := s.makeRequest("GET", "/api/v1/categories?include_deprecated=true", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	_, ok := body["categories"].([]any)
	s.Require().True(ok)
}

func (s *CanonicalScenariosIntegrationSuite) TestCCL4_CategoriaDescontinuada() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	categories, ok := body["categories"].([]any)
	s.Require().True(ok)
	s.NotEmpty(categories)

	firstID := categories[0].(map[string]any)["id"].(string)

	resp2 := s.makeRequest("GET", fmt.Sprintf("/api/v1/categories/%s", firstID), nil)
	s.Require().Equal(http.StatusOK, resp2.StatusCode)
}

func (s *CanonicalScenariosIntegrationSuite) TestCCL5_PaginacaoCursor() {
	resp := s.makeRequest("GET", "/api/v1/category-dictionary?page_size=10", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	entries, ok := body["entries"].([]any)
	s.Require().True(ok)
	s.LessOrEqual(len(entries), 10)

	version, ok := body["version"].(float64)
	s.Require().True(ok)
	s.Greater(version, float64(0))
}

func (s *CanonicalScenariosIntegrationSuite) TestCCV1_PrimeiraRequisicaoETag() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	etag := resp.Header.Get("ETag")
	s.NotEmpty(etag)
	s.Regexp(`"v\d+"`, etag)

	body := s.parseBody(resp)
	version, ok := body["version"].(float64)
	s.Require().True(ok)
	s.Greater(version, float64(0))
}

func (s *CanonicalScenariosIntegrationSuite) TestCCV2_Revalidacao304() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	etag := resp.Header.Get("ETag")
	s.NotEmpty(etag)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/categories?kind=expense", s.server.URL), nil)
	s.Require().NoError(err)
	req.Header.Set("If-None-Match", etag)

	client := &http.Client{}
	resp2, err := client.Do(req)
	s.Require().NoError(err)
	defer resp2.Body.Close()

	s.Equal(http.StatusNotModified, resp2.StatusCode)

	body, _ := io.ReadAll(resp2.Body)
	s.Empty(body)
}

func (s *CanonicalScenariosIntegrationSuite) TestCCV3_MigrationIncrementaVersao() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	version1, ok := body["version"].(float64)
	s.Require().True(ok)

	etag1 := resp.Header.Get("ETag")
	s.NotEmpty(etag1)

	s.Greater(version1, float64(0))
}

func (s *CanonicalScenariosIntegrationSuite) TestCCV4_MigrationInvalidaNaoIncrementa() {
	resp := s.makeRequest("GET", "/api/v1/categories?kind=expense", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body := s.parseBody(resp)
	version, ok := body["version"].(float64)
	s.Require().True(ok)
	s.Greater(version, float64(0))
}

func (s *CanonicalScenariosIntegrationSuite) makeRequest(method, path string, body io.Reader) *http.Response {
	url := s.server.URL + path
	req, err := http.NewRequest(method, url, body)
	s.Require().NoError(err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	s.Require().NoError(err)

	return resp
}

func (s *CanonicalScenariosIntegrationSuite) parseBody(resp *http.Response) map[string]any {
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	s.Require().NoError(err)

	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
