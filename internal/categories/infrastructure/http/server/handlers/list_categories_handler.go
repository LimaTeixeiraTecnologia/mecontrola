package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type listCategoriesUseCase interface {
	Execute(ctx context.Context, in *input.ListCategoriesInput) (*output.ListCategoriesOutput, error)
}

type ListCategoriesHandler struct {
	usecase         listCategoriesUseCase
	version         interfaces.VersionReader
	o11y            observability.Observability
	requestTotal    observability.Counter
	requestDuration observability.Histogram
}

func NewListCategoriesHandler(uc listCategoriesUseCase, version interfaces.VersionReader, o11y observability.Observability) *ListCategoriesHandler {
	requestTotal := o11y.Metrics().Counter(
		"categories_list_total",
		"Total de requisicoes de listagem de categorias",
		"1",
	)
	requestDuration := o11y.Metrics().HistogramWithBuckets(
		"categories_list_duration_seconds",
		"Duracao das requisicoes de listagem de categorias",
		"s",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	)
	return &ListCategoriesHandler{
		usecase:         uc,
		version:         version,
		o11y:            o11y,
		requestTotal:    requestTotal,
		requestDuration: requestDuration,
	}
}

func (h *ListCategoriesHandler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, span := h.o11y.Tracer().Start(r.Context(), "categories.handler.list")
	defer span.End()

	version := h.extractVersion(r)
	if version > 0 {
		w.Header().Set("ETag", formatETag(version))
	}

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

func (h *ListCategoriesHandler) buildInput(w http.ResponseWriter, r *http.Request, start time.Time) (*input.ListCategoriesInput, string, bool) {
	in := &input.ListCategoriesInput{}
	var kindStr string

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

	if parentIDStr := r.URL.Query().Get("parent_id"); parentIDStr != "" {
		parentID, err := uuid.Parse(parentIDStr)
		if err != nil {
			d := time.Since(start)
			h.recordMetrics(r.Context(), kindStr, "invalid_query")
			h.recordDuration(r.Context(), d, kindStr, "invalid_query")
			h.logRequest(r, "invalid_query", d)
			version := h.currentVersion(r.Context())
			w.Header().Set("ETag", formatETag(version))
			writeProblem(w, http.StatusUnprocessableEntity, "invalid parent_id", "invalid_query", version)
			return nil, "", false
		}
		in.ParentID = &parentID
	}

	if r.URL.Query().Get("include_deprecated") == "true" {
		in.IncludeDeprecated = true
	}
	return in, kindStr, true
}

func (h *ListCategoriesHandler) handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, span observability.Span, kindStr string, err error) {
	span.RecordError(err)
	d := time.Since(start)
	h.o11y.Logger().Error(ctx, "categories.list.failed", observability.Error(err))
	h.recordMetrics(ctx, kindStr, "error")
	h.recordDuration(ctx, d, kindStr, "error")
	h.logRequest(r, "error", d)
	version := h.currentVersion(ctx)
	w.Header().Set("ETag", formatETag(version))
	writeProblem(w, http.StatusInternalServerError, "internal error", "", version)
}

func (h *ListCategoriesHandler) writeNotModified(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, kindStr string) {
	d := time.Since(start)
	h.recordMetrics(ctx, kindStr, "matched")
	h.recordDuration(ctx, d, kindStr, "matched")
	h.logRequest(r, "matched", d)
	w.WriteHeader(http.StatusNotModified)
}

func (h *ListCategoriesHandler) writeSuccess(ctx context.Context, w http.ResponseWriter, r *http.Request, start time.Time, kindStr string, out *output.ListCategoriesOutput) {
	d := time.Since(start)
	h.recordMetrics(ctx, kindStr, "matched")
	h.recordDuration(ctx, d, kindStr, "matched")
	h.logRequest(r, "matched", d)
	responses.JSON(w, http.StatusOK, out)
}

func (h *ListCategoriesHandler) recordMetrics(ctx context.Context, kind, outcome string) {
	h.requestTotal.Increment(ctx,
		observability.String("endpoint", "list_categories"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	)
}

func (h *ListCategoriesHandler) recordDuration(ctx context.Context, duration time.Duration, kind, outcome string) {
	h.requestDuration.Record(ctx, duration.Seconds(),
		observability.String("endpoint", "list_categories"),
		observability.String("kind", kind),
		observability.String("outcome", outcome),
	)
}

func (h *ListCategoriesHandler) logRequest(r *http.Request, outcome string, duration time.Duration) {
	h.o11y.Logger().Info(r.Context(), "categories.list.request",
		observability.String("endpoint", "list_categories"),
		observability.String("method", r.Method),
		observability.String("outcome", outcome),
		observability.Int64("duration_ms", duration.Milliseconds()),
	)
}

func (h *ListCategoriesHandler) currentVersion(ctx context.Context) int64 {
	if h.version == nil {
		return 0
	}
	v, err := h.version.Current(ctx)
	if err != nil {
		return 0
	}
	return v
}

func (h *ListCategoriesHandler) extractVersion(r *http.Request) int64 {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return 0
	}
	return newETagHelper().parse(ifNoneMatch)
}
