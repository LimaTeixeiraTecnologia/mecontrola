package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	chiserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"
	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	infraerrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/errors"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/observability"
)

const (
	defaultTimeout   = 25 * time.Second
	defaultBodyLimit = 1 * 1024 * 1024 // 1 MiB
)

// Deps agrupa as dependências necessárias para montar o servidor HTTP.
type Deps struct {
	DB       *database.Manager
	Provider *observability.Provider
}

// NewServer constrói o servidor HTTP com o stack completo de middlewares (ADR-008):
// RequestID, Recovery, Timeout (25s), BodyLimit (1 MiB), SecurityHeaders, CORS estrito, OTel.
// Registra os health endpoints /health, /live e /ready consumindo deps.DB.HealthCheck.
func NewServer(cfg *configs.Config, deps Deps) (*chiserver.Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("http: cfg não pode ser nil")
	}

	o11y := deps.Provider.Observability()

	opts := buildOptions(cfg, deps.DB)
	srv, err := chiserver.New(o11y, opts...)
	if err != nil {
		return nil, fmt.Errorf("http: inicializando servidor: %w", err)
	}

	return srv, nil
}

func buildOptions(cfg *configs.Config, mgr *database.Manager) []chiserver.Option {
	port := fmt.Sprintf(":%d", cfg.HTTPConfig.Port)

	opts := []chiserver.Option{
		chiserver.WithPort(port),
		chiserver.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		chiserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
		chiserver.WithEnvironment(cfg.AppConfig.Environment),
		chiserver.WithReadTimeout(defaultTimeout),
		chiserver.WithWriteTimeout(defaultTimeout),
		chiserver.WithBodyLimit(defaultBodyLimit),
		chiserver.WithTracing(),
		chiserver.WithOTelMetrics(),
		chiserver.WithHealthChecks(buildHealthChecks(mgr)),
	}

	if origins := strings.TrimSpace(cfg.HTTPConfig.CORSAllowedOrigins); origins != "" {
		opts = append(opts, chiserver.WithCORS(origins))
	}

	return opts
}

func buildHealthChecks(mgr *database.Manager) map[string]common.HealthCheckFunc {
	checks := make(map[string]common.HealthCheckFunc)

	if mgr != nil {
		checks["database"] = mgr.HealthCheck
	}

	return checks
}

// WriteJSON escreve uma resposta JSON com o status e o payload fornecidos.
// Usado pelos handlers de health quando é necessário customizar o payload.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteProblem escreve uma resposta application/problem+json derivada do erro.
func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	pd, status := infraerrors.ToProblemDetails(err)
	pd.Instance = r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(pd)
}
