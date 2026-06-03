package outbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

// MetricsSuite cobre os cenários obrigatórios de OutboxMetrics (RF-21).
type MetricsSuite struct {
	suite.Suite
	obs     *noop.Provider
	metrics *outbox.OutboxMetrics
}

func TestMetricsSuite(t *testing.T) {
	suite.Run(t, new(MetricsSuite))
}

func (s *MetricsSuite) SetupTest() {
	s.obs = noop.NewProvider()
	var err error
	s.metrics, err = outbox.NewOutboxMetrics(s.obs)
	s.Require().NoError(err)
}

// Cenário 5: bucketização de error_class — 5 valores fixos (R-OBS-001).
// Table-driven conforme R-TEST-001.
func (s *MetricsSuite) TestClassifyError_Buckets() {
	type testCase struct {
		name     string
		err      error
		expected string
	}

	// ErrPermanent personalizado para teste.
	permanentErr := errors.New("connection refused")

	cases := []testCase{
		{
			name:     "transient: erro genérico de rede",
			err:      errors.New("connection reset by peer"),
			expected: "transient",
		},
		{
			name:     "transient: erro genérico de timeout customizado",
			err:      errors.New("i/o timeout"),
			expected: "transient",
		},
		{
			name:     "timeout: context.DeadlineExceeded",
			err:      context.DeadlineExceeded,
			expected: "timeout",
		},
		{
			name:     "timeout: context.Canceled",
			err:      context.Canceled,
			expected: "timeout",
		},
		{
			name:     "permanent: ErrPermanent direto",
			err:      outbox.ErrPermanent,
			expected: "permanent",
		},
		{
			name:     "permanent: wrappado com fmt.Errorf",
			err:      errors.Join(outbox.ErrPermanent, errors.New("schema inválido")),
			expected: "permanent",
		},
		{
			name:     "transient: erro desconhecido sem ErrPermanent",
			err:      permanentErr,
			expected: "transient",
		},
		{
			name:     "timeout: DeadlineExceeded wrappado",
			err:      errors.Join(context.DeadlineExceeded, errors.New("outer")),
			expected: "timeout",
		},
	}

	ctx := context.Background()
	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Usa RecordFailed como proxy para ClassifyError (método privado).
			// O teste verifica que não há panic e que o método aceita o erro.
			// Verificação de bucket via ClassifyErrorForTest (exportado para testes).
			bucket := outbox.ClassifyErrorForTest(tc.err)
			s.Equal(tc.expected, bucket, "error_class incorreto para erro: %v", tc.err)

			// Verifica que RecordFailed não panics e aceita o erro.
			s.NotPanics(func() {
				s.metrics.RecordFailed(ctx, "test-subscription", tc.err)
			})
		})
	}
}

// Cenário 6: nenhum dos métodos de Metrics aceita payload como parâmetro.
// Verifica via inspeção de assinatura (compilação garante ausência de json.RawMessage).
// O compilador Go rejeita código que passe json.RawMessage para os métodos abaixo —
// esta suite confirma que os métodos existem com as assinaturas corretas.
func (s *MetricsSuite) TestMetrics_NoPayloadParameter() {
	ctx := context.Background()
	// Todos os métodos públicos de OutboxMetrics são chamados sem payload.
	// Se qualquer assinatura aceitasse json.RawMessage, este arquivo não compilaria.
	s.NotPanics(func() {
		s.metrics.RecordPublished(ctx, "order.placed")
		s.metrics.RecordProcessed(ctx, "test-sub", 150.0)
		s.metrics.RecordFailed(ctx, "test-sub", errors.New("err"))
		s.metrics.RecordDLQ(ctx, "test-sub")
		s.metrics.RecordPoll(ctx, 12.5, 10)
		s.metrics.RecordReaperReleased(ctx, 3)
		s.metrics.RecordHousekeepingDeleted(ctx, 100)
		s.metrics.SetPending(ctx, "test-sub", 5)
	})
}

// Cenário: NewOutboxMetrics retorna erro se Observability for nil.
func (s *MetricsSuite) TestNewOutboxMetrics_Error_NilObs() {
	m, err := outbox.NewOutboxMetrics(nil)
	s.Error(err)
	s.Nil(m)
}

// Cenário: Tracer() retorna tracer não-nil.
func (s *MetricsSuite) TestTracer_ReturnsNonNil() {
	tracer := s.metrics.Tracer()
	s.NotNil(tracer, "Tracer() deve retornar tracer não-nil")
}

// Cenário: RecordPublished não panics com tipos variados de event_type.
func (s *MetricsSuite) TestRecordPublished_NoPanic() {
	ctx := context.Background()
	s.NotPanics(func() {
		s.metrics.RecordPublished(ctx, "order.placed")
		s.metrics.RecordPublished(ctx, "payment.processed")
		s.metrics.RecordPublished(ctx, "")
	})
}

// Cenário: RecordProcessed registra com latência correta.
func (s *MetricsSuite) TestRecordProcessed_NoPanic() {
	ctx := context.Background()
	s.NotPanics(func() {
		s.metrics.RecordProcessed(ctx, "order-handler", 0.0)
		s.metrics.RecordProcessed(ctx, "order-handler", 999.9)
	})
}

// Cenário: SetPending aceita deltas negativos (gauge via UpDownCounter).
func (s *MetricsSuite) TestSetPending_AcceptsNegativeDelta() {
	ctx := context.Background()
	s.NotPanics(func() {
		s.metrics.SetPending(ctx, "order-handler", 10)
		s.metrics.SetPending(ctx, "order-handler", -3)
		s.metrics.SetPending(ctx, "order-handler", 0)
	})
}

// Cenário: RecordReaperReleased com n=0 não panics.
func (s *MetricsSuite) TestRecordReaperReleased_ZeroValue() {
	ctx := context.Background()
	s.NotPanics(func() {
		s.metrics.RecordReaperReleased(ctx, 0)
		s.metrics.RecordReaperReleased(ctx, 100)
	})
}

// Cenário: RecordHousekeepingDeleted com valores variados não panics.
func (s *MetricsSuite) TestRecordHousekeepingDeleted_Values() {
	ctx := context.Background()
	s.NotPanics(func() {
		s.metrics.RecordHousekeepingDeleted(ctx, 0)
		s.metrics.RecordHousekeepingDeleted(ctx, 1000)
	})
}
