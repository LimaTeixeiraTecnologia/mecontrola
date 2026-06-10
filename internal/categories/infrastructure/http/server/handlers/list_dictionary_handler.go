package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type listDictionaryUseCase interface {
	Execute(ctx context.Context, in *input.ListDictionaryInput) (*output.ListDictionaryOutput, error)
}

type ListDictionaryHandler struct {
	usecase         listDictionaryUseCase
	version         interfaces.VersionReader
	o11y            observability.Observability
	requestTotal    observability.Counter
	requestDuration observability.Histogram
}

func NewListDictionaryHandler(uc listDictionaryUseCase, version interfaces.VersionReader, o11y observability.Observability) *ListDictionaryHandler {
	requestTotal := o11y.Metrics().Counter(
		"category_dictionary_list_total",
		"Total de requisicoes de listagem do dicionario",
		"1",
	)
	requestDuration := o11y.Metrics().HistogramWithBuckets(
		"category_dictionary_list_duration_seconds",
		"Duracao das requisicoes de listagem do dicionario",
		"s",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	)
	return &ListDictionaryHandler{
		usecase:         uc,
		version:         version,
		o11y:            o11y,
		requestTotal:    requestTotal,
		requestDuration: requestDuration,
	}
}

func (h *ListDictionaryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, span := h.o11y.Tracer().Start(r.Context(), "categories.handler.list_dictionary")
	defer span.End()

	in, kindStr, ok := h.buildInput(w, r, start)
	if !ok {
		return
	}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		h.handleError(ctx, w, r, start, span, kindStr, err)
		return
	}

	w.Header().Set("ETag", formatETag(out.Version))

	if newETagHelper().checkIfNoneMatch(r, out.Version) {
		h.writeNotModified(ctx, w, r, start, kindStr)
		return
	}

	h.writeSuccess(ctx, w, r, start, kindStr, out)
}

func (h *ListDictionaryHandler) buildInput(w http.ResponseWriter, r *http.Request, start time.Time) (*input.ListDictionaryInput, string, bool) {
	in := &input.ListDictionaryInput{}
	var kindStr string

	if categoryID := r.URL.Query().Get("category_id"); categoryID != "" {
		in.CategoryID = &categoryID
	}

	if k := r.URL.Query().Get("kind"); k != "" {
		kindStr = k
		kind, err := valueobjects.ParseKind(kindStr)
		if err != nil {
			d := time.Since(start)
			h.recordMetrics(r.Context(), kindStr, "invalid_kind")
			h.recordDuration(r.Context(), d, kindStr, "invalid_kind")
			h.logRequest(r, "invalid_kind", d)
			version := h.currentVersion(r.Context())
			w.Header().Set("ETag", formatETag(version))
			writeProblem(w, http.StatusUnprocessableEntity, "invalid kind", "invalid_kind", version)
			return nil, "", false
		}
		in.Kind = &kind
	}

	if signalTypeStr := r.URL.Query().Get("signal_type"); signalTypeStr != "" {
		signalType, err := valueobjects.ParseSignalType(signalTypeStr)
		if err != nil {
			d := time.Since(start)
			h.recordMetrics(r.Context(), kindStr, "invalid_query")
			h.recordDuration(r.Context(), d, kindStr, "invalid_query")
			h.logRequest(r, "invalid_query", d)
			version := h.currentVersion(r.Context())
			w.Header().Set("ETag", formatETag(version))
			writeProblem(w, http.StatusUnprocessableEntity, "invalid signal_type", "invalid_query", version)
			return nil, "", false
		}
		in.SignalType = &signalType
	}

	in.Cursor = r.URL.Query().Get("cursor")

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err == nil {
			in.PageSize = pageSize
		}
	}
	return in, kindStr, true
}

func (h *ListDictionaryHandler) handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr string, err error) {
	span.RecordError(err)
	d := time.Since(start)
	h.o11y.Logger().Error(ctx, "categories.list_dictionary.failed", observability.Error(err))
	h.recordMetrics(ctx, kindStr, "error")
	h.recordDuration(ctx, d, kindStr, "error")
	h.logRequest(r, "error", d)
	version := h.currentVersion(ctx)
	w.Header().Set("ETag", formatETag(version))
	writeProblem(w, http.StatusInternalServerError, "internal error", "", version)
}

func (h *ListDictionaryHandler) writeNotModified(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, kindStr string) {
	d := time.Since(start)
	h.recordMetrics(ctx, kindStr, "matched")
	h.recordDuration(ctx, d, kindStr, "matched")
	h.logRequest(r, "matched", d)
	w.WriteHeader(http.StatusNotModified)
}

func (h *ListDictionaryHandler) writeSuccess(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, kindStr string, out *output.ListDictionaryOutput) {
	d := time.Since(start)
	h.recordMetrics(ctx, kindStr, "matched")
	h.recordDuration(ctx, d, kindStr, "matched")
	h.logRequest(r, "matched", d)
	responses.JSON(w, http.StatusOK, out)
}

func (h *ListDictionaryHandler) recordMetrics(ctx context.Context, kind, outcome string) {
	h.requestTotal.Increment(ctx,
		observability.String("endpoint", "list_dictionary"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	)
}

func (h *ListDictionaryHandler) recordDuration(ctx context.Context, duration time.Duration, kind, outcome string) {
	h.requestDuration.Record(ctx, duration.Seconds(),
		observability.String("endpoint", "list_dictionary"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	)
}

func (h *ListDictionaryHandler) logRequest(r *http.Request, outcome string, duration time.Duration) {
	h.o11y.Logger().Info(r.Context(), "categories.list_dictionary.request",
		observability.String("endpoint", "list_dictionary"),
		observability.String("method", r.Method),
		observability.String("outcome", outcome),
		observability.Int64("duration_ms", duration.Milliseconds()),
	)
}

func (h *ListDictionaryHandler) currentVersion(ctx context.Context) int64 {
	if h.version == nil {
		return 0
	}
	v, err := h.version.Current(ctx)
	if err != nil {
		return 0
	}
	return v
}
