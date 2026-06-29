package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
)

type GetWorkingMemory struct {
	wm   memory.WorkingMemory
	o11y observability.Observability
}

func NewGetWorkingMemory(wm memory.WorkingMemory, o11y observability.Observability) *GetWorkingMemory {
	return &GetWorkingMemory{wm: wm, o11y: o11y}
}

func (uc *GetWorkingMemory) Execute(ctx context.Context, in input.GetWorkingMemoryInput) (string, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.get_working_memory")
	defer span.End()

	if err := in.Validate(); err != nil {
		return "", err
	}

	content, err := uc.wm.Get(ctx, in.ResourceID)
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.get_working_memory.failed",
			observability.String("resource_id", in.ResourceID),
			observability.Error(err),
		)
		return "", fmt.Errorf("platform.memory.usecase.get_working_memory: %w", err)
	}

	return content, nil
}
