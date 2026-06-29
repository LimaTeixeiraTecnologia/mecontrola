package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
)

type RecentMessages struct {
	store memory.MessageStore
	o11y  observability.Observability
}

func NewRecentMessages(store memory.MessageStore, o11y observability.Observability) *RecentMessages {
	return &RecentMessages{store: store, o11y: o11y}
}

func (uc *RecentMessages) Execute(ctx context.Context, in input.RecentMessagesInput) ([]memory.Message, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.recent_messages")
	defer span.End()

	if err := in.Validate(); err != nil {
		return nil, err
	}

	msgs, err := uc.store.Recent(ctx, in.ThreadPK, in.Limit)
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.recent_messages.failed",
			observability.Error(err),
		)
		return nil, fmt.Errorf("platform.memory.usecase.recent_messages: %w", err)
	}

	return msgs, nil
}
