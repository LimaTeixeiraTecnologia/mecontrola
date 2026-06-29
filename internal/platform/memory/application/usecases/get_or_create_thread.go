package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
)

type GetOrCreateThread struct {
	gateway memory.ThreadGateway
	o11y    observability.Observability
}

func NewGetOrCreateThread(gateway memory.ThreadGateway, o11y observability.Observability) *GetOrCreateThread {
	return &GetOrCreateThread{gateway: gateway, o11y: o11y}
}

func (uc *GetOrCreateThread) Execute(ctx context.Context, in input.GetOrCreateThreadInput) (memory.Thread, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.get_or_create_thread")
	defer span.End()

	if err := in.Validate(); err != nil {
		return memory.Thread{}, err
	}

	thread, err := uc.gateway.GetOrCreate(ctx, in.ResourceID, in.ThreadID)
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.get_or_create_thread.failed",
			observability.String("resource_id", in.ResourceID),
			observability.String("thread_id", in.ThreadID),
			observability.Error(err),
		)
		return memory.Thread{}, fmt.Errorf("platform.memory.usecase.get_or_create_thread: %w", err)
	}

	return thread, nil
}
