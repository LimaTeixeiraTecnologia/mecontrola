package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

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

	kind, ok := h.parseKind(w, r, start, kindStr)
	if !ok {
		return
	}

	in := &input.SearchDictionaryInput{Query: q, Kind: kind}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		h.handleError(ctx, w, r, start, span, kindStr, q, err)
		return
	}

	w.Header().Set("ETag", formatETag(out.Version))

	if newETagHelper().checkIfNoneMatch(r, out.Version) {
		h.writeNotModified(ctx, w, r, start, kindStr, q, out)
		return
	}

	h.writeSuccess(ctx, w, r, start, kindStr, q, out)
}

func (h *SearchDictionaryHandler) parseKind(w http.ResponseWriter, r *http.Request, start time.Time, kindStr string) (valueobjects.Kind, bool) {
	if kindStr == "" {
		d := time.Since(start)
		h.recordMetrics(r.Context(), kindStr, "invalid_kind", "", "")
		h.recordDuration(r.Context(), d, kindStr, "invalid_kind")
		h.logRequest(r, "invalid_kind", d)
		version := h.currentVersion(r.Context())
		w.Header().Set("ETag", formatETag(version))
		writeProblem(w, http.StatusUnprocessableEntity, "kind is required", "invalid_kind", version)
		return 0, false
	}

	kind, err := valueobjects.ParseKind(kindStr)
	if err != nil {
		d := time.Since(start)
		h.recordMetrics(r.Context(), kindStr, "invalid_kind", "", "")
		h.recordDuration(r.Context(), d, kindStr, "invalid_kind")
		h.logRequest(r, "invalid_kind", d)
		version := h.currentVersion(r.Context())
		w.Header().Set("ETag", formatETag(version))
		writeProblem(w, http.StatusUnprocessableEntity, "invalid kind", "invalid_kind", version)
		return 0, false
	}
	return kind, true
}

func (h *SearchDictionaryHandler) handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr, q string, err error) {
	span.RecordError(err)
	d := time.Since(start)
	if errors.Is(err, valueobjects.ErrInvalidQuery) {
		h.recordMetrics(ctx, kindStr, "invalid_query", h.calcQLenBucket(q), "")
		h.recordDuration(ctx, d, kindStr, "invalid_query")
		h.logRequest(r, "invalid_query", d)
		version := h.currentVersion(ctx)
		w.Header().Set("ETag", formatETag(version))
		writeProblem(w, http.StatusUnprocessableEntity, "invalid query", "invalid_query", version)
		return
	}
	if errors.Is(err, valueobjects.ErrInvalidKind) {
		h.recordMetrics(ctx, kindStr, "invalid_kind", "", "")
		h.recordDuration(ctx, d, kindStr, "invalid_kind")
		h.logRequest(r, "invalid_kind", d)
		version := h.currentVersion(ctx)
		w.Header().Set("ETag", formatETag(version))
		writeProblem(w, http.StatusUnprocessableEntity, "invalid kind", "invalid_kind", version)
		return
	}
	h.o11y.Logger().Error(ctx, "categories.search.failed", observability.Error(err))
	h.recordMetrics(ctx, kindStr, "error", "", "")
	h.recordDuration(ctx, d, kindStr, "error")
	h.logRequest(r, "error", d)
	version := h.currentVersion(ctx)
	w.Header().Set("ETag", formatETag(version))
	writeProblem(w, http.StatusInternalServerError, "internal error", "", version)
}

func (h *SearchDictionaryHandler) writeNotModified(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, kindStr, q string, out *output.DictionarySearchOutput) {
	d := time.Since(start)
	outcome := h.determineOutcome(out)
	h.recordMetrics(ctx, kindStr, outcome, h.calcQLenBucket(q), out.SignalTypeTop)
	h.recordDuration(ctx, d, kindStr, outcome)
	h.logRequest(r, outcome, d)
	w.WriteHeader(http.StatusNotModified)
}

func (h *SearchDictionaryHandler) writeSuccess(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, kindStr, q string, out *output.DictionarySearchOutput) {
	d := time.Since(start)
	outcome := h.determineOutcome(out)
	h.recordMetrics(ctx, kindStr, outcome, h.calcQLenBucket(q), out.SignalTypeTop)
	h.recordDuration(ctx, d, kindStr, outcome)
	h.logRequest(r, outcome, d)
	responses.JSON(w, http.StatusOK, out)
}

func (h *SearchDictionaryHandler) determineOutcome(out *output.DictionarySearchOutput) string {
	if out.Result == "no_match" {
		return "no_match"
	}
	if len(out.Candidates) == 0 {
		return "no_match"
	}
	if out.IsAmbiguous {
		return "ambiguous"
	}
	return "matched"
}

func (h *SearchDictionaryHandler) calcQLenBucket(q string) string {
	normalized := h.normalizeQuery(q)
	length := len(normalized)
	switch {
	case length >= 3 && length <= 4:
		return "3-4"
	case length >= 5 && length <= 8:
		return "5-8"
	case length >= 9 && length <= 16:
		return "9-16"
	case length >= 17 && length <= 32:
		return "17-32"
	case length >= 33:
		return "33+"
	default:
		return ""
	}
}

func (h *SearchDictionaryHandler) normalizeQuery(q string) string {
	trimmed := strings.TrimSpace(q)
	var result strings.Builder
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func (h *SearchDictionaryHandler) recordMetrics(ctx context.Context, kind, outcome, qLenBucket, signalTypeTop string) {
	fields := []observability.Field{
		observability.String("endpoint", "search_dictionary"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	}
	if qLenBucket != "" {
		fields = append(fields, observability.String("q_len_bucket", qLenBucket))
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
