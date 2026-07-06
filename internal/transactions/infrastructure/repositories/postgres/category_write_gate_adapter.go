package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	catinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	catinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	catvos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type categoryWriteGateAdapter struct {
	resolveForWrite *catusecases.ResolveCategoryForWrite
	versionReader   catinterfaces.VersionReader
	o11y            observability.Observability
	gateTotal       observability.Counter
	versionDrift    observability.Counter
	persisted       observability.Counter
}

func NewCategoryWriteGateAdapter(
	resolveForWrite *catusecases.ResolveCategoryForWrite,
	versionReader catinterfaces.VersionReader,
	o11y observability.Observability,
) interfaces.CategoryWriteGate {
	return &categoryWriteGateAdapter{
		resolveForWrite: resolveForWrite,
		versionReader:   versionReader,
		o11y:            o11y,
		gateTotal: o11y.Metrics().Counter(
			"category_write_gate_total",
			"Total de execucoes do gate de escrita de categoria",
			"1",
		),
		versionDrift: o11y.Metrics().Counter(
			"category_write_version_drift_total",
			"Total de bloqueios por drift de versao editorial de categoria",
			"1",
		),
		persisted: o11y.Metrics().Counter(
			"category_write_persisted_total",
			"Total de escritas de categoria aprovadas pelo gate",
			"1",
		),
	}
}

func (a *categoryWriteGateAdapter) Approve(ctx context.Context, in interfaces.CategoryWriteGateInput) (valueobjects.CategoryWriteEvidence, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "transactions.category_write_gate.approve")
	defer span.End()

	kind, err := kindFromDirection(in.Direction)
	if err != nil {
		span.RecordError(err)
		a.gateTotal.Add(ctx, 1,
			observability.String("status", "blocked"),
			observability.String("reason", "kind_direction_mismatch"),
			observability.String("source", in.Source.String()),
			observability.String("kind", kindStringFromDirection(in.Direction)),
			observability.String("surface", in.Surface),
		)
		return valueobjects.CategoryWriteEvidence{}, fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryKindDirectionMismatch)
	}

	expectedVersion, err := a.resolveExpectedVersion(ctx, in)
	if err != nil {
		span.RecordError(err)
		a.gateTotal.Add(ctx, 1,
			observability.String("status", "error"),
			observability.String("reason", "version_read_error"),
			observability.String("source", in.Source.String()),
			observability.String("kind", kind.String()),
			observability.String("surface", in.Surface),
		)
		return valueobjects.CategoryWriteEvidence{}, fmt.Errorf("transactions/category_write_gate: ler versao editorial: %w", err)
	}

	out, err := a.resolveForWrite.Execute(ctx, &catinput.ResolveCategoryForWriteInput{
		RootCategoryID:  in.RootCategoryID,
		SubcategoryID:   in.SubcategoryID,
		Kind:            kind,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		span.RecordError(err)
		reason := gateBlockReason(err)
		if errors.Is(err, catusecases.ErrVersionDrift) {
			a.versionDrift.Add(ctx, 1,
				observability.String("kind", kind.String()),
				observability.String("surface", in.Surface),
			)
		}
		a.gateTotal.Add(ctx, 1,
			observability.String("status", "blocked"),
			observability.String("reason", reason),
			observability.String("source", in.Source.String()),
			observability.String("kind", kind.String()),
			observability.String("surface", in.Surface),
		)
		return valueobjects.CategoryWriteEvidence{}, mapGateResolveError(err)
	}

	evidenceIn := buildGateEvidence(in, out.SubcategorySlug, out.Path, out.EditorialVersion)
	evidence, err := valueobjects.NewCategoryWriteEvidence(evidenceIn)
	if err != nil {
		span.RecordError(err)
		a.gateTotal.Add(ctx, 1,
			observability.String("status", "blocked"),
			observability.String("reason", "evidence_invalid"),
			observability.String("source", in.Source.String()),
			observability.String("kind", kind.String()),
			observability.String("surface", in.Surface),
		)
		return valueobjects.CategoryWriteEvidence{}, fmt.Errorf("transactions/category_write_gate: construir evidencia: %w", err)
	}

	a.gateTotal.Add(ctx, 1,
		observability.String("status", "approved"),
		observability.String("reason", "ok"),
		observability.String("source", in.Source.String()),
		observability.String("kind", kind.String()),
		observability.String("surface", in.Surface),
	)
	a.persisted.Add(ctx, 1,
		observability.String("source", in.Source.String()),
		observability.String("kind", kind.String()),
		observability.String("surface", in.Surface),
	)

	return evidence, nil
}

func (a *categoryWriteGateAdapter) resolveExpectedVersion(ctx context.Context, in interfaces.CategoryWriteGateInput) (int64, error) {
	if in.Source == valueobjects.CategoryDecisionSourceManualCanonicalID {
		return a.versionReader.Current(ctx)
	}
	if in.ExpectedVersion <= 0 {
		return 0, valueobjects.ErrCategoryVersionChanged
	}
	return in.ExpectedVersion, nil
}

func kindFromDirection(direction string) (catvos.Kind, error) {
	switch direction {
	case "income":
		return catvos.KindIncome, nil
	case "outcome":
		return catvos.KindExpense, nil
	default:
		return 0, fmt.Errorf("transactions/category_write_gate: direction invalida %q", direction)
	}
}

func gateBlockReason(err error) string {
	switch {
	case errors.Is(err, catusecases.ErrVersionDrift):
		return "version_drift"
	case errors.Is(err, catusecases.ErrCategoryDeprecated):
		return "deprecated"
	case errors.Is(err, catusecases.ErrKindMismatch):
		return "kind_mismatch"
	case errors.Is(err, catusecases.ErrLeafNotFromRoot):
		return "leaf_not_from_root"
	case errors.Is(err, catusecases.ErrRootWithoutLeaf):
		return "root_without_leaf"
	case errors.Is(err, catusecases.ErrRootCategoryNotFound),
		errors.Is(err, catusecases.ErrSubcategoryNotFound):
		return "not_found"
	default:
		return "resolve_error"
	}
}

func mapGateResolveError(err error) error {
	switch {
	case errors.Is(err, catusecases.ErrVersionDrift):
		return fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryVersionChanged)
	case errors.Is(err, catusecases.ErrCategoryDeprecated):
		return fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryDeprecated)
	case errors.Is(err, catusecases.ErrKindMismatch):
		return fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryKindDirectionMismatch)
	case errors.Is(err, catusecases.ErrLeafNotFromRoot):
		return fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryWriteBlocked)
	case errors.Is(err, catusecases.ErrRootWithoutLeaf):
		return fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryRootWithoutLeaf)
	case errors.Is(err, catusecases.ErrRootCategoryNotFound),
		errors.Is(err, catusecases.ErrSubcategoryNotFound):
		return fmt.Errorf("transactions/category_write_gate: %w", valueobjects.ErrCategoryWriteBlocked)
	default:
		return fmt.Errorf("transactions/category_write_gate: resolver categoria: %w", err)
	}
}

func buildGateEvidence(in interfaces.CategoryWriteGateInput, subcategorySlug, path string, editorialVersion int64) valueobjects.CategoryWriteEvidenceInput {
	if in.Source == valueobjects.CategoryDecisionSourceManualCanonicalID {
		return valueobjects.CategoryWriteEvidenceInput{
			RootCategoryID:   in.RootCategoryID,
			SubcategoryID:    in.SubcategoryID,
			Kind:             kindStringFromDirection(in.Direction),
			Path:             path,
			Outcome:          "matched",
			Score:            1.0,
			Confidence:       "manual_confirmed",
			Quality:          "manual_canonical",
			SignalType:       "manual_canonical",
			MatchedTerm:      subcategorySlug,
			MatchReason:      "manual canonical id validated",
			Source:           in.Source,
			EditorialVersion: editorialVersion,
			DecidedAt:        time.Now().UTC(),
		}
	}
	return valueobjects.CategoryWriteEvidenceInput{
		RootCategoryID:   in.RootCategoryID,
		SubcategoryID:    in.SubcategoryID,
		Kind:             kindStringFromDirection(in.Direction),
		Path:             path,
		Outcome:          in.Outcome,
		Score:            in.Score,
		Confidence:       in.Confidence,
		Quality:          in.Quality,
		SignalType:       in.SignalType,
		MatchedTerm:      in.MatchedTerm,
		MatchReason:      in.MatchReason,
		Source:           in.Source,
		EditorialVersion: editorialVersion,
		DecidedAt:        time.Now().UTC(),
	}
}

func kindStringFromDirection(direction string) string {
	if direction == "income" {
		return "income"
	}
	return "expense"
}
