package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type searchDictionaryUseCase interface {
	Execute(ctx context.Context, in *input.SearchDictionaryInput) (*output.DictionarySearchOutput, error)
}

type SearchDictionaryHandler struct {
	usecase         searchDictionaryUseCase
	version         interfaces.VersionReader
	o11y            observability.Observability
	requestTotal    observability.Counter
	requestDuration observability.Histogram
}

func NewSearchDictionaryHandler(uc searchDictionaryUseCase, version interfaces.VersionReader, o11y observability.Observability) *SearchDictionaryHandler {
	requestTotal := o11y.Metrics().Counter(
		"category_dictionary_search_total",
		"Total de requisicoes de busca no dicionario",
		"1",
	)
	requestDuration := o11y.Metrics().HistogramWithBuckets(
		"category_dictionary_search_duration_seconds",
		"Duracao das requisicoes de busca no dicionario",
		"s",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	)
	return &SearchDictionaryHandler{
		usecase:         uc,
		version:         version,
		o11y:            o11y,
		requestTotal:    requestTotal,
		requestDuration: requestDuration,
	}
}

func (h *SearchDictionaryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, span := h.o11y.Tracer().Start(r.Context(), "categories.handler.search")
	defer span.End()

	q := r.URL.Query().Get("q")
	kindStr := r.URL.Query().Get("kind")

	kind, ok := h.parseKind(ctx, w, r, start, span, kindStr)
	if !ok {
		return
	}

	in := &input.SearchDictionaryInput{Query: q, Kind: kind}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		h.handleError(ctx, w, r, start, span, kindStr, in, err)
		return
	}

	w.Header().Set("ETag", formatETag(out.Version))

	if newETagHelper().checkIfNoneMatch(r, out.Version) {
		h.writeNotModified(ctx, w, r, start, span, kindStr, in, out)
		return
	}

	h.writeSuccess(ctx, w, r, start, span, kindStr, in, out)
}

func (h *SearchDictionaryHandler) parseKind(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr string) (valueobjects.Kind, bool) {
	if kindStr == "" {
		h.respondProblem(ctx, w, r, start, span, kindStr, "", "", "invalid_kind", http.StatusUnprocessableEntity, "kind is required")
		return 0, false
	}

	kind, err := valueobjects.ParseKind(kindStr)
	if err != nil {
		h.respondProblem(ctx, w, r, start, span, kindStr, "", "", "invalid_kind", http.StatusUnprocessableEntity, "invalid kind")
		return 0, false
	}
	return kind, true
}

func (h *SearchDictionaryHandler) handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr string, in *input.SearchDictionaryInput, err error) {
	span.RecordError(err)
	bucket := qLenBucket(in.NormalizedQuery())

	switch {
	case errors.Is(err, valueobjects.ErrInvalidQuery):
		h.respondProblem(ctx, w, r, start, span, kindStr, bucket, "", "invalid_query", http.StatusUnprocessableEntity, "invalid query")
	case errors.Is(err, valueobjects.ErrInvalidKind):
		h.respondProblem(ctx, w, r, start, span, kindStr, "", "", "invalid_kind", http.StatusUnprocessableEntity, "invalid kind")
	default:
		h.o11y.Logger().Error(ctx, "categories.search.failed", observability.Error(err))
		h.respondProblem(ctx, w, r, start, span, kindStr, "", "", "error", http.StatusInternalServerError, "internal error")
	}
}

func (h *SearchDictionaryHandler) respondProblem(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr, bucket, signalTypeTop, outcome string, status int, detail string) {
	d := time.Since(start)
	span.SetAttributes(observability.String("outcome", outcome))
	h.recordMetrics(ctx, kindStr, outcome, bucket, signalTypeTop)
	h.recordDuration(ctx, d, kindStr, outcome)
	h.logRequest(r, outcome, d)
	version := h.currentVersion(ctx)
	w.Header().Set("ETag", formatETag(version))
	writeProblem(w, status, detail, problemCode(outcome), version)
}

func (h *SearchDictionaryHandler) writeNotModified(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr string, in *input.SearchDictionaryInput, out *output.DictionarySearchOutput) {
	d := time.Since(start)
	outcome := resolveOutcomeLabel(out)
	span.SetAttributes(observability.String("outcome", outcome))
	h.recordMetrics(ctx, kindStr, outcome, qLenBucket(in.NormalizedQuery()), out.SignalTypeTop)
	h.recordDuration(ctx, d, kindStr, outcome)
	h.logRequest(r, outcome, d)
	w.WriteHeader(http.StatusNotModified)
}

func (h *SearchDictionaryHandler) writeSuccess(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr string, in *input.SearchDictionaryInput, out *output.DictionarySearchOutput) {
	d := time.Since(start)
	outcome := resolveOutcomeLabel(out)
	span.SetAttributes(observability.String("outcome", outcome))
	h.recordMetrics(ctx, kindStr, outcome, qLenBucket(in.NormalizedQuery()), out.SignalTypeTop)
	h.recordDuration(ctx, d, kindStr, outcome)
	h.logRequest(r, outcome, d)
	responses.JSON(w, http.StatusOK, out)
}

func resolveOutcomeLabel(out *output.DictionarySearchOutput) string {
	if out.Outcome.IsValid() {
		return out.Outcome.String()
	}
	return valueobjects.ClassifyOutcome(len(out.Candidates)).String()
}

func (h *SearchDictionaryHandler) recordMetrics(ctx context.Context, kind, outcome, qLenBucketLabel, signalTypeTop string) {
	fields := []observability.Field{
		observability.String("endpoint", "search_dictionary"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	}
	if qLenBucketLabel != "" {
		fields = append(fields, observability.String("q_len_bucket", qLenBucketLabel))
	}
	if signalTypeTop != "" {
		fields = append(fields, observability.String("signal_type_top", signalTypeTop))
	}
	h.requestTotal.Increment(ctx, fields...)
}

func (h *SearchDictionaryHandler) recordDuration(ctx context.Context, duration time.Duration, kind, outcome string) {
	h.requestDuration.Record(ctx, duration.Seconds(),
		observability.String("endpoint", "search_dictionary"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	)
}

func (h *SearchDictionaryHandler) logRequest(r *http.Request, outcome string, duration time.Duration) {
	h.o11y.Logger().Info(r.Context(), "categories.search.request",
		observability.String("endpoint", "search_dictionary"),
		observability.String("method", r.Method),
		observability.String("outcome", outcome),
		observability.Int64("duration_ms", duration.Milliseconds()),
	)
}

func (h *SearchDictionaryHandler) currentVersion(ctx context.Context) int64 {
	if h.version == nil {
		return 0
	}
	v, err := h.version.Current(ctx)
	if err != nil {
		return 0
	}
	return v
}

func problemCode(outcome string) string {
	switch outcome {
	case "invalid_query", "invalid_kind":
		return outcome
	default:
		return ""
	}
}
