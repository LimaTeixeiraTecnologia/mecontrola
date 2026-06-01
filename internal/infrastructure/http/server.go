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

// serverBuilder constrói as opções do servidor Chi a partir de cfg e DB.
// Separado de Deps para que NewServer não precise de funções standalone.
type serverBuilder struct {
	cfg *configs.Config
	db  *database.Manager
}

// buildOptions monta o slice de opções Chi com timeouts, middlewares e health checks.
func (b *serverBuilder) buildOptions() []chiserver.Option {
	port := fmt.Sprintf(":%d", b.cfg.HTTPConfig.Port)

	opts := []chiserver.Option{
		chiserver.WithPort(port),
		chiserver.WithServiceName(b.cfg.HTTPConfig.ServiceNameAPI),
		chiserver.WithServiceVersion(b.cfg.O11yConfig.ServiceVersion),
		chiserver.WithEnvironment(b.cfg.AppConfig.Environment),
		chiserver.WithReadTimeout(defaultTimeout),
		chiserver.WithWriteTimeout(defaultTimeout),
		chiserver.WithBodyLimit(defaultBodyLimit),
		chiserver.WithTracing(),
		chiserver.WithOTelMetrics(),
		chiserver.WithHealthChecks(b.buildHealthChecks()),
	}

	if origins := strings.TrimSpace(b.cfg.HTTPConfig.CORSAllowedOrigins); origins != "" {
		opts = append(opts, chiserver.WithCORS(origins))
	}

	return opts
}

// buildHealthChecks mapeia os checks de health das dependências injetadas.
func (b *serverBuilder) buildHealthChecks() map[string]common.HealthCheckFunc {
	checks := make(map[string]common.HealthCheckFunc)

	if b.db != nil {
		checks["database"] = b.db.HealthCheck
	}

	return checks
}

// NewServer constrói o servidor HTTP com o stack completo de middlewares (ADR-008):
// RequestID, Recovery, Timeout (25s), BodyLimit (1 MiB), SecurityHeaders, CORS estrito, OTel.
// Registra os health endpoints /health, /live e /ready consumindo deps.DB.HealthCheck.
func NewServer(cfg *configs.Config, deps Deps) (*chiserver.Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("http: cfg não pode ser nil")
	}

	o11y := deps.Provider.Observability()
	builder := &serverBuilder{cfg: cfg, db: deps.DB}

	srv, err := chiserver.New(o11y, builder.buildOptions()...)
	if err != nil {
		return nil, fmt.Errorf("http: inicializando servidor: %w", err)
	}

	return srv, nil
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
