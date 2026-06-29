package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
)

type Recall struct {
	recall memory.SemanticRecall
	o11y   observability.Observability
}

func NewRecall(recall memory.SemanticRecall, o11y observability.Observability) *Recall {
	return &Recall{recall: recall, o11y: o11y}
}

func (uc *Recall) Execute(ctx context.Context, in input.RecallInput) ([]memory.RecallHit, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.recall")
	defer span.End()

	if err := in.Validate(); err != nil {
		return nil, err
	}

	hits, err := uc.recall.Recall(ctx, in.ResourceID, in.Query, in.Embedding, in.K)
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.recall.failed",
			observability.String("resource_id", in.ResourceID),
			observability.Error(err),
		)
		return nil, fmt.Errorf("platform.memory.usecase.recall: %w", err)
	}

	return hits, nil
}
