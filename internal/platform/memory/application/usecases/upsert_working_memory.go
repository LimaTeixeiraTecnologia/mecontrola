package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
)

type UpsertWorkingMemory struct {
	wm   memory.WorkingMemory
	o11y observability.Observability
}

func NewUpsertWorkingMemory(wm memory.WorkingMemory, o11y observability.Observability) *UpsertWorkingMemory {
	return &UpsertWorkingMemory{wm: wm, o11y: o11y}
}

func (uc *UpsertWorkingMemory) Execute(ctx context.Context, in input.UpsertWorkingMemoryInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.upsert_working_memory")
	defer span.End()

	if err := in.Validate(); err != nil {
		return err
	}

	if err := uc.wm.Upsert(ctx, in.ResourceID, in.Content); err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.upsert_working_memory.failed",
			observability.String("resource_id", in.ResourceID),
			observability.Error(err),
		)
		return fmt.Errorf("platform.memory.usecase.upsert_working_memory: %w", err)
	}

	return nil
}
