package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type Summarize struct {
	summarizer memory.Summarizer
	o11y       observability.Observability
}

func NewSummarize(summarizer memory.Summarizer, o11y observability.Observability) *Summarize {
	return &Summarize{summarizer: summarizer, o11y: o11y}
}

func (uc *Summarize) Execute(ctx context.Context, messages []memory.Message) (string, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.summarize")
	defer span.End()

	if len(messages) == 0 {
		return "", nil
	}

	summary, err := uc.summarizer.Summarize(ctx, messages)
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.summarize.failed",
			observability.Error(err),
		)
		return "", fmt.Errorf("platform.memory.usecase.summarize: %w", err)
	}

	return summary, nil
}
