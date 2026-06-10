package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
)

type getCategoryUseCase interface {
	Execute(ctx context.Context, in *input.GetCategoryInput) (*output.CategoryDetailOutput, error)
}

type GetCategoryHandler struct {
	usecase         getCategoryUseCase
	version         interfaces.VersionReader
	o11y            observability.Observability
	requestTotal    observability.Counter
	requestDuration observability.Histogram
}

func NewGetCategoryHandler(uc getCategoryUseCase, version interfaces.VersionReader, o11y observability.Observability) *GetCategoryHandler {
	requestTotal := o11y.Metrics().Counter(
		"categories_get_total",
		"Total de requisicoes de obtencao de categoria",
		"1",
	)
	requestDuration := o11y.Metrics().HistogramWithBuckets(
		"categories_get_duration_seconds",
		"Duracao das requisicoes de obtencao de categoria",
		"s",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	)
	return &GetCategoryHandler{
		usecase:         uc,
		version:         version,
		o11y:            o11y,
		requestTotal:    requestTotal,
		requestDuration: requestDuration,
	}
}

func (h *GetCategoryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, span := h.o11y.Tracer().Start(r.Context(), "categories.handler.get")
	defer span.End()

	in, ok := h.buildInput(w, r, start)
	if !ok {
		return
	}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		h.handleError(ctx, w, r, start, span, err)
		return
	}

	w.Header().Set("ETag", formatETag(out.Version))

	if newETagHelper().checkIfNoneMatch(r, out.Version) {
		h.writeNotModified(ctx, w, r, start)
		return
	}

	h.writeSuccess(ctx, w, r, start, out)
}

func (h *GetCategoryHandler) buildInput(w http.ResponseWriter, r *http.Request, start time.Time) (*input.GetCategoryInput, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		d := time.Since(start)
		h.recordMetrics(r.Context(), "invalid_query")
		h.recordDuration(r.Context(), d, "invalid_query")
		h.logRequest(r, "invalid_query", d)
		version := h.currentVersion(r.Context())
		w.Header().Set("ETag", formatETag(version))
		writeProblem(w, http.StatusUnprocessableEntity, "invalid id", "invalid_query", version)
		return nil, false
	}

	in := &input.GetCategoryInput{ID: id}
	if r.URL.Query().Get("include_deprecated") == "true" {
		in.IncludeDeprecated = true
	}
	return in, true
}

func (h *GetCategoryHandler) handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, err error) {
	span.RecordError(err)
	d := time.Since(start)
	if errors.Is(err, usecases.ErrCategoryNotFound) {
		h.recordMetrics(ctx, "not_found")
		h.recordDuration(ctx, d, "not_found")
		h.logRequest(r, "not_found", d)
		version := h.currentVersion(ctx)
		w.Header().Set("ETag", formatETag(version))
		writeProblem(w, http.StatusNotFound, "category not found", "not_found", version)
		return
	}
	h.o11y.Logger().Error(ctx, "categories.get.failed", observability.Error(err))
	h.recordMetrics(ctx, "error")
	h.recordDuration(ctx, d, "error")
	h.logRequest(r, "error", d)
	version := h.currentVersion(ctx)
	w.Header().Set("ETag", formatETag(version))
	writeProblem(w, http.StatusInternalServerError, "internal error", "", version)
}

func (h *GetCategoryHandler) writeNotModified(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time) {
	d := time.Since(start)
	h.recordMetrics(ctx, "matched")
	h.recordDuration(ctx, d, "matched")
	h.logRequest(r, "matched", d)
	w.WriteHeader(http.StatusNotModified)
}

func (h *GetCategoryHandler) writeSuccess(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, out *output.CategoryDetailOutput) {
	d := time.Since(start)
	h.recordMetrics(ctx, "matched")
	h.recordDuration(ctx, d, "matched")
	h.logRequest(r, "matched", d)
	responses.JSON(w, http.StatusOK, out)
}

func (h *GetCategoryHandler) recordMetrics(ctx context.Context, outcome string) {
	h.requestTotal.Increment(ctx,
		observability.String("endpoint", "get_category"),
		observability.String("outcome", outcome),
	)
}

func (h *GetCategoryHandler) recordDuration(ctx context.Context, duration time.Duration, outcome string) {
	h.requestDuration.Record(ctx, duration.Seconds(),
		observability.String("endpoint", "get_category"),
		observability.String("outcome", outcome),
	)
}

func (h *GetCategoryHandler) logRequest(r *http.Request, outcome string, duration time.Duration) {
	h.o11y.Logger().Info(r.Context(), "categories.get.request",
		observability.String("endpoint", "get_category"),
		observability.String("method", r.Method),
		observability.String("outcome", outcome),
		observability.Int64("duration_ms", duration.Milliseconds()),
	)
}

func (h *GetCategoryHandler) currentVersion(ctx context.Context) int64 {
	if h.version == nil {
		return 0
	}
	v, err := h.version.Current(ctx)
	if err != nil {
		return 0
	}
	return v
}
