package observability

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

// Provider é o adaptador de infraestrutura que compõe devkit-go/pkg/observability/otel
// com a configuração da aplicação, expondo traces, métricas e logs correlacionados.
//
// Invariantes:
//   - Criado via NewProvider; nunca via literal de struct.
//   - shutdown(ctx) drena spans/logs/metrics pendentes; idempotente.
//   - Redaction de PII ativa (Sanitize=true); campos listados em redaction.go são mascarados.
type Provider struct {
	inner *otel.Provider
}

// Observability retorna a interface Observability do devkit-go para consumo pelos subsistemas.
func (p *Provider) Observability() observability.Observability {
	return p.inner
}

// Shutdown drena e fecha todos os exporters (traces, métricas, logs).
// Deve ser chamado no SIGTERM antes de o processo sair.
// É idempotente: chamadas repetidas ou concorrentes são seguras.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.inner.Shutdown(ctx)
}

// NewProvider constrói e inicializa o provider OTel com os exporters OTLP gRPC configurados.
//
// Resource attributes obrigatórios derivados do cfg:
//   - service.name = "mecontrola"
//   - service.version = cfg.O11yConfig.ServiceVersion
//   - deployment.environment = cfg.AppConfig.Environment
//
// O exporter usa:
//   - Endpoint: cfg.O11yConfig.OTLPEndpoint (obrigatório)
//   - Headers: env OTEL_EXPORTER_OTLP_HEADERS (Basic Auth Grafana Cloud)
//   - Protocol: grpc (padrão; ou http quando OTLPEndpoint tiver schema http)
//   - Insecure: false em produção (bloqueado pelo devkit); true permitido em local/staging
//   - TraceSampleRate: cfg.O11yConfig.TraceSampleRate (default 1.0)
//   - Sanitize: true — redaction ativa para os campos de PIIFields
func NewProvider(cfg *configs.Config) (*Provider, func(context.Context) error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("observability: cfg não pode ser nil")
	}

	if cfg.O11yConfig.OTLPEndpoint == "" {
		return nil, nil, fmt.Errorf("observability: OTEL_EXPORTER_OTLP_ENDPOINT é obrigatório")
	}

	otelCfg := buildOtelConfig(cfg)

	inner, err := otel.NewProvider(context.Background(), otelCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("observability: inicializando provider: %w", err)
	}

	slog.SetDefault(slog.New(NewRedactingSlogHandler(slog.Default().Handler())))

	provider := &Provider{inner: inner}
	shutdown := provider.Shutdown

	return provider, shutdown, nil
}

// buildOtelConfig mapeia configs.Config para otel.Config do devkit-go.
func buildOtelConfig(cfg *configs.Config) *otel.Config {
	otelCfg := otel.DefaultConfig("mecontrola")

	otelCfg.ServiceVersion = cfg.O11yConfig.ServiceVersion
	otelCfg.Environment = cfg.AppConfig.Environment
	otelCfg.OTLPEndpoint = cfg.O11yConfig.OTLPEndpoint
	otelCfg.OTLPProtocol = otel.ProtocolGRPC
	otelCfg.Insecure = cfg.AppConfig.Environment != "production"
	otelCfg.TraceSampleRate = cfg.O11yConfig.TraceSampleRate
	otelCfg.LogLevel = observability.LogLevel(cfg.O11yConfig.LogLevel)
	otelCfg.LogFormat = observability.LogFormat(cfg.O11yConfig.LogFormat)

	// Redaction de PII obrigatória: Sanitize=true ativa mascaramento de campos sensíveis
	// (password, token, card_number, etc.) no otelLogger do devkit-go.
	// PIIFields adicionais são registrados no ResourceAttributes para rastreabilidade.
	otelCfg.Sanitize = true

	return otelCfg
}
