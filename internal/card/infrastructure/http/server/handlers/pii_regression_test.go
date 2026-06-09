package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type logCapture struct {
	buf *bytes.Buffer
}

func newLogCapture() *logCapture { return &logCapture{buf: &bytes.Buffer{}} }

func (l *logCapture) Debug(_ context.Context, msg string, fields ...observability.Field) {
	l.write("DEBUG", msg, fields...)
}

func (l *logCapture) Info(_ context.Context, msg string, fields ...observability.Field) {
	l.write("INFO", msg, fields...)
}

func (l *logCapture) Warn(_ context.Context, msg string, fields ...observability.Field) {
	l.write("WARN", msg, fields...)
}

func (l *logCapture) Error(_ context.Context, msg string, fields ...observability.Field) {
	l.write("ERROR", msg, fields...)
}

func (l *logCapture) With(_ ...observability.Field) observability.Logger { return l }

func (l *logCapture) write(level, msg string, fields ...observability.Field) {
	l.buf.WriteString(level)
	l.buf.WriteByte(' ')
	l.buf.WriteString(msg)
	for _, f := range fields {
		l.buf.WriteByte(' ')
		l.buf.WriteString(f.Key)
		l.buf.WriteByte('=')
		l.buf.WriteString(f.StringValue())
	}
	l.buf.WriteByte('\n')
}

type captureProvider struct {
	log *logCapture
}

func newCaptureObs() *captureProvider {
	return &captureProvider{log: newLogCapture()}
}

func (p *captureProvider) Tracer() observability.Tracer     { return &captureTracer{} }
func (p *captureProvider) Logger() observability.Logger     { return p.log }
func (p *captureProvider) Metrics() observability.Metrics   { return &captureMetrics{} }
func (p *captureProvider) Shutdown(_ context.Context) error { return nil }

type captureTracer struct{}

type captureSpan struct{}

func (t *captureTracer) Start(ctx context.Context, _ string, _ ...observability.SpanOption) (context.Context, observability.Span) {
	return ctx, &captureSpan{}
}

func (t *captureTracer) SpanFromContext(_ context.Context) observability.Span { return &captureSpan{} }
func (t *captureTracer) ContextWithSpan(ctx context.Context, _ observability.Span) context.Context {
	return ctx
}

func (s *captureSpan) End()                                           {}
func (s *captureSpan) SetAttributes(_ ...observability.Field)         {}
func (s *captureSpan) SetStatus(_ observability.StatusCode, _ string) {}
func (s *captureSpan) RecordError(_ error, _ ...observability.Field)  {}
func (s *captureSpan) AddEvent(_ string, _ ...observability.Field)    {}
func (s *captureSpan) Context() observability.SpanContext             { return &captureSpanCtx{} }
func (s *captureSpan) TraceID() string                                { return "" }
func (s *captureSpan) SpanID() string                                 { return "" }
func (s *captureSpan) IsSampled() bool                                { return false }

type captureSpanCtx struct{}

func (c *captureSpanCtx) TraceID() string { return "" }
func (c *captureSpanCtx) SpanID() string  { return "" }
func (c *captureSpanCtx) IsSampled() bool { return false }

type captureMetrics struct{}

func (m *captureMetrics) Counter(_, _, _ string) observability.Counter { return &captureCounter{} }
func (m *captureMetrics) Histogram(_, _, _ string) observability.Histogram {
	return &captureHistogram{}
}
func (m *captureMetrics) HistogramWithBuckets(_, _, _ string, _ []float64) observability.Histogram {
	return &captureHistogram{}
}
func (m *captureMetrics) UpDownCounter(_, _, _ string) observability.UpDownCounter {
	return &captureUpDown{}
}
func (m *captureMetrics) Gauge(_, _, _ string, _ observability.GaugeCallback) error { return nil }

type captureCounter struct{}

func (c *captureCounter) Add(_ context.Context, _ int64, _ ...observability.Field) {}
func (c *captureCounter) Increment(_ context.Context, _ ...observability.Field)    {}

type captureHistogram struct{}

func (h *captureHistogram) Record(_ context.Context, _ float64, _ ...observability.Field) {}

type captureUpDown struct{}

func (u *captureUpDown) Add(_ context.Context, _ int64, _ ...observability.Field) {}

func TestM07_NoPIIInHandlerLogs(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New().String()

	card := output.Card{
		ID:         cardID,
		UserID:     userID.String(),
		Name:       "Nubank Secreto",
		Nickname:   "NuPrivado",
		ClosingDay: 15,
		DueDay:     22,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	assertNoPII := func(t *testing.T, logOutput string) {
		t.Helper()
		lower := strings.ToLower(logOutput)
		assert.NotContains(t, lower, "nubank secreto", "card name must not appear in logs")
		assert.NotContains(t, lower, "nuprivado", "card nickname must not appear in logs")
	}

	t.Run("create_no_pii", func(t *testing.T) {
		o11y := newCaptureObs()
		ucMock := &mockCreateCard{}
		ucMock.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
			Return(card, nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/cards",
			strings.NewReader(`{"name":"Nubank Secreto","nickname":"NuPrivado","closing_day":15,"due_day":22}`))
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID, Source: auth.SourceHeader}))
		rr := httptest.NewRecorder()
		handlers.NewCreateCardHandler(ucMock, o11y).Handle(rr, req)
		require.Equal(t, http.StatusCreated, rr.Code)
		assertNoPII(t, o11y.log.buf.String())
		ucMock.AssertExpectations(t)
	})

	t.Run("get_no_pii", func(t *testing.T) {
		o11y := newCaptureObs()
		ucMock := &mockGetCard{}
		ucMock.On("Execute", mock.Anything, mock.AnythingOfType("input.GetCard")).
			Return(card, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+cardID, nil)
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID, Source: auth.SourceHeader}))
		req = withChiParam(req, "id", cardID)
		rr := httptest.NewRecorder()
		handlers.NewGetCardHandler(ucMock, o11y).Handle(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		assertNoPII(t, o11y.log.buf.String())
		ucMock.AssertExpectations(t)
	})

	t.Run("update_no_pii", func(t *testing.T) {
		o11y := newCaptureObs()
		ucMock := &mockUpdateCard{}
		ucMock.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
			Return(card, nil).Once()

		req := httptest.NewRequest(http.MethodPut, "/api/v1/cards/"+cardID,
			strings.NewReader(`{"name":"Nubank Secreto","nickname":"NuPrivado","closing_day":15,"due_day":22}`))
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID, Source: auth.SourceHeader}))
		req = withChiParam(req, "id", cardID)
		rr := httptest.NewRecorder()
		handlers.NewUpdateCardHandler(ucMock, o11y).Handle(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		assertNoPII(t, o11y.log.buf.String())
		ucMock.AssertExpectations(t)
	})

	t.Run("delete_no_pii", func(t *testing.T) {
		o11y := newCaptureObs()
		ucMock := &mockSoftDeleteCard{}
		ucMock.On("Execute", mock.Anything, mock.AnythingOfType("input.SoftDeleteCard")).
			Return(nil).Once()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/cards/"+cardID, nil)
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID, Source: auth.SourceHeader}))
		req = withChiParam(req, "id", cardID)
		rr := httptest.NewRecorder()
		handlers.NewDeleteCardHandler(ucMock, o11y).Handle(rr, req)
		require.Equal(t, http.StatusNoContent, rr.Code)
		assertNoPII(t, o11y.log.buf.String())
		ucMock.AssertExpectations(t)
	})

	t.Run("list_no_pii", func(t *testing.T) {
		o11y := newCaptureObs()
		ucMock := &mockListCards{}
		ucMock.On("Execute", mock.Anything, mock.MatchedBy(func(any) bool { return true })).
			Return(output.CardList{Cards: []output.Card{card}}, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil)
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID, Source: auth.SourceHeader}))
		rr := httptest.NewRecorder()
		handlers.NewListCardsHandler(ucMock, o11y).Handle(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		assertNoPII(t, o11y.log.buf.String())
		ucMock.AssertExpectations(t)
	})

	t.Run("invoice_for_no_pii", func(t *testing.T) {
		o11y := newCaptureObs()
		ucMock := &mockInvoiceFor{}
		invoice := output.Invoice{
			ClosingDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			DueDate:     time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC),
		}
		ucMock.On("Execute", mock.Anything, mock.AnythingOfType("input.InvoiceFor")).
			Return(invoice, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+cardID+"/invoices?for=2024-01-10", nil)
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID, Source: auth.SourceHeader}))
		req = withChiParam(req, "id", cardID)
		rr := httptest.NewRecorder()
		handlers.NewInvoiceForHandler(ucMock, o11y).Handle(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		assertNoPII(t, o11y.log.buf.String())
		ucMock.AssertExpectations(t)
	})
}
