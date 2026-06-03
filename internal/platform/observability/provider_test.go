package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	infrao11y "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

func makeLocalConfig(endpoint string) *configs.Config {
	return &configs.Config{
		AppConfig: configs.AppConfig{
			Environment: "local",
		},
		O11yConfig: configs.O11yConfig{
			OTLPEndpoint:    endpoint,
			TraceSampleRate: 1.0,
			LogLevel:        "info",
			LogFormat:       "json",
			ServiceVersion:  "test-sha",
		},
		HTTPConfig: configs.HTTPConfig{
			Port: 8080,
		},
	}
}

// ObservabilityProviderSuite testa o Provider OTel, redaction de PII, métricas e spans.
type ObservabilityProviderSuite struct {
	suite.Suite
	ctx context.Context
}

func TestObservabilityProvider(t *testing.T) {
	suite.Run(t, new(ObservabilityProviderSuite))
}

func (s *ObservabilityProviderSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ObservabilityProviderSuite) TestNewProvider() {
	scenarios := []struct {
		name    string
		cfg     *configs.Config
		wantErr bool
		expect  func(provider *infrao11y.Provider, shutdown func(context.Context) error, err error)
	}{
		{
			name:    "deve retornar erro com cfg nil",
			cfg:     nil,
			wantErr: true,
			expect: func(provider *infrao11y.Provider, shutdown func(context.Context) error, err error) {
				s.Error(err)
				s.Nil(provider)
				s.Nil(shutdown)
			},
		},
		{
			name:    "deve retornar erro com endpoint vazio",
			cfg:     makeLocalConfig(""),
			wantErr: true,
			expect: func(provider *infrao11y.Provider, shutdown func(context.Context) error, err error) {
				s.Error(err)
				s.Nil(provider)
				s.Nil(shutdown)
			},
		},
		{
			name:    "deve criar provider com configuração válida",
			cfg:     makeLocalConfig("localhost:4317"),
			wantErr: false,
			expect: func(provider *infrao11y.Provider, shutdown func(context.Context) error, err error) {
				s.NoError(err)
				s.NotNil(provider)
				s.NotNil(shutdown)
				ctx, cancel := context.WithTimeout(s.ctx, 0)
				defer cancel()
				_ = shutdown(ctx)
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			provider, shutdown, err := infrao11y.NewProvider(sc.cfg)
			sc.expect(provider, shutdown, err)
		})
	}
}

func (s *ObservabilityProviderSuite) TestShutdownIdempotent() {
	s.Run("deve suportar múltiplos shutdowns sem pânico", func() {
		_, shutdown, err := infrao11y.NewProvider(makeLocalConfig("localhost:4317"))
		s.Require().NoError(err)
		_ = shutdown(s.ctx)
		_ = shutdown(s.ctx)
	})
}

func (s *ObservabilityProviderSuite) TestPIIFields() {
	scenarios := []struct {
		name   string
		expect func()
	}{
		{
			name: "deve conter ao menos um campo de PII",
			expect: func() {
				s.NotEmpty(infrao11y.PIIFields)
			},
		},
		{
			name: "deve conter todos os campos obrigatórios de PII",
			expect: func() {
				required := []string{"phone", "password", "token", "card_number", "amount"}
				for _, field := range required {
					s.Contains(infrao11y.PIIFields, field, "PIIFields deve conter %q", field)
				}
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect()
		})
	}
}

func (s *ObservabilityProviderSuite) TestPIIRedactionFakeLogger() {
	s.Run("deve capturar entrada de log com campo phone via FakeLogger", func() {
		fakeProvider := devkitfake.NewProvider()
		fakeLogger, ok := fakeProvider.Logger().(*devkitfake.FakeLogger)
		s.Require().True(ok, "Logger deve ser *fake.FakeLogger")

		fakeLogger.Info(s.ctx, "evento de teste", observability.String("phone", "+5511999999999"))

		entries := fakeLogger.GetEntries()
		s.Require().Len(entries, 1)
		s.Equal("evento de teste", entries[0].Message)
		s.Contains(infrao11y.PIIFields, "phone", "phone deve estar na lista de redaction da infra")
	})
}

func (s *ObservabilityProviderSuite) TestRegisterFoundationMetrics() {
	scenarios := []struct {
		name    string
		metrics observability.Metrics
		wantErr bool
		expect  func(fm *infrao11y.FoundationMetrics, err error)
	}{
		{
			name:    "deve retornar erro com provider nil",
			metrics: nil,
			wantErr: true,
			expect: func(fm *infrao11y.FoundationMetrics, err error) {
				s.Error(err)
				s.Nil(fm)
			},
		},
		{
			name:    "deve criar instrumentos com FakeMetrics",
			metrics: devkitfake.NewFakeMetrics(),
			wantErr: false,
			expect: func(fm *infrao11y.FoundationMetrics, err error) {
				s.NoError(err)
				s.Require().NotNil(fm)
				s.NotNil(fm.BootstrapDuration)
				s.NotNil(fm.EventsPublished)
				s.NotNil(fm.HealthProbeStatus)
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			fm, err := infrao11y.RegisterFoundationMetrics(sc.metrics)
			sc.expect(fm, err)
		})
	}
}

func (s *ObservabilityProviderSuite) TestFoundationMetricsRecord() {
	s.Run("deve registrar bootstrap duration no histograma", func() {
		fakeMetrics := devkitfake.NewFakeMetrics()
		fm, err := infrao11y.RegisterFoundationMetrics(fakeMetrics)
		s.Require().NoError(err)

		fm.RecordBootstrapDuration(s.ctx, 1.23)

		hist := fakeMetrics.GetHistogram("bootstrap_duration_seconds")
		s.Require().NotNil(hist)
		values := hist.GetValues()
		s.Require().Len(values, 1)
		s.InDelta(1.23, values[0].Value, 0.001)
	})

	s.Run("deve incrementar counter de eventos publicados com labels corretos", func() {
		fakeMetrics := devkitfake.NewFakeMetrics()
		fm, err := infrao11y.RegisterFoundationMetrics(fakeMetrics)
		s.Require().NoError(err)

		fm.IncrementEventsPublished(s.ctx, "identity.user-created", "success")
		fm.IncrementEventsPublished(s.ctx, "identity.user-created", "success")

		counter := fakeMetrics.GetCounter("events_published_total")
		s.Require().NotNil(counter)
		values := counter.GetValues()
		s.Require().Len(values, 2)
		for _, v := range values {
			s.Equal(int64(1), v.Value)
			fieldMap := make(map[string]string)
			for _, f := range v.Fields {
				if f.Kind() == observability.FieldKindString {
					fieldMap[f.Key] = f.StringValue()
				}
			}
			s.Equal("identity.user-created", fieldMap["event_name"])
			s.Equal("success", fieldMap["outcome"])
		}
	})

	s.Run("deve registrar status de health probe no up-down counter", func() {
		fakeMetrics := devkitfake.NewFakeMetrics()
		fm, err := infrao11y.RegisterFoundationMetrics(fakeMetrics)
		s.Require().NoError(err)

		fm.SetHealthProbeStatus(s.ctx, "db_ping", true)
		fm.SetHealthProbeStatus(s.ctx, "db_select", false)

		counter := fakeMetrics.GetUpDownCounter("health_probe_status")
		s.Require().NotNil(counter)
		values := counter.GetValues()
		s.Require().Len(values, 2)
		s.Equal(int64(1), values[0].Value)
		s.Equal(int64(-1), values[1].Value)
	})

	s.Run("deve reutilizar instrumento ao registrar métricas duas vezes", func() {
		fakeMetrics := devkitfake.NewFakeMetrics()
		fm1, err := infrao11y.RegisterFoundationMetrics(fakeMetrics)
		s.Require().NoError(err)
		fm2, err := infrao11y.RegisterFoundationMetrics(fakeMetrics)
		s.Require().NoError(err)

		fm1.RecordBootstrapDuration(s.ctx, 0.5)
		fm2.RecordBootstrapDuration(s.ctx, 1.0)

		hist := fakeMetrics.GetHistogram("bootstrap_duration_seconds")
		s.Require().NotNil(hist)
		s.Len(hist.GetValues(), 2)
	})
}

func (s *ObservabilityProviderSuite) TestPIIHandlerMasks() {
	s.Run("deve mascarar phone e amount para [REDACTED]", func() {
		var buf bytes.Buffer
		inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := infrao11y.NewRedactingSlogHandler(inner)
		logger := slog.New(handler)

		logger.Info("pagamento",
			slog.String("phone", "+5511999999999"),
			slog.String("amount", "R$1234,56"),
			slog.String("name", "João"),
		)

		var record map[string]any
		s.Require().NoError(json.Unmarshal(buf.Bytes(), &record))
		s.Equal("[REDACTED]", record["phone"])
		s.Equal("[REDACTED]", record["amount"])
		s.Equal("João", record["name"])
	})

	s.Run("deve mascarar todos os campos declarados em PIIFields", func() {
		for _, field := range infrao11y.PIIFields {
			s.Run(field, func() {
				var buf bytes.Buffer
				inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
				handler := infrao11y.NewRedactingSlogHandler(inner)
				logger := slog.New(handler)

				logger.Info("evento", slog.String(field, "valor-sensivel"))

				var record map[string]any
				s.Require().NoError(json.Unmarshal(buf.Bytes(), &record),
					"output deve ser JSON válido para campo %q", field)
				s.Equal("[REDACTED]", record[field],
					"campo %q deve ser mascarado para [REDACTED]", field)
			})
		}
	})
}

func (s *ObservabilityProviderSuite) TestSpanResourceAttributes() {
	s.Run("deve conter service.name, service.version e deployment.environment nos resource attributes", func() {
		const (
			wantServiceName    = "mecontrola"
			wantServiceVersion = "v1.2.3-test"
			wantEnvironment    = "staging"
		)

		res, err := resource.New(s.ctx,
			resource.WithAttributes(
				semconv.ServiceName(wantServiceName),
				semconv.ServiceVersion(wantServiceVersion),
				semconv.DeploymentEnvironment(wantEnvironment),
			),
		)
		s.Require().NoError(err)

		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSyncer(exporter),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		)
		defer func() { _ = tp.Shutdown(s.ctx) }()

		tracer := tp.Tracer("test-tracer")
		_, span := tracer.Start(s.ctx, "test-span")
		span.End()

		spans := exporter.GetSpans()
		s.Require().Len(spans, 1)

		spanResource := spans[0].Resource
		s.Require().NotNil(spanResource)

		attrs := spanResource.Attributes()
		assertResourceAttr(s.T(), attrs, string(semconv.ServiceNameKey), wantServiceName)
		assertResourceAttr(s.T(), attrs, string(semconv.ServiceVersionKey), wantServiceVersion)
		assertResourceAttr(s.T(), attrs, string(semconv.DeploymentEnvironmentKey), wantEnvironment)
	})
}

func assertResourceAttr(t *testing.T, attrs []attribute.KeyValue, key, want string) {
	t.Helper()
	for _, kv := range attrs {
		if string(kv.Key) == key {
			require.Equal(t, want, kv.Value.AsString(),
				"resource attribute %q deve ser %q", key, want)
			return
		}
	}
	t.Errorf("resource attribute %q não encontrado; atributos: %v", key, attrs)
}
