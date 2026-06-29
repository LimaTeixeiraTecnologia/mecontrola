package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
)

type AppendMessage struct {
	store memory.MessageStore
	o11y  observability.Observability
}

func NewAppendMessage(store memory.MessageStore, o11y observability.Observability) *AppendMessage {
	return &AppendMessage{store: store, o11y: o11y}
}

func (uc *AppendMessage) Execute(ctx context.Context, in input.AppendMessageInput) (memory.Message, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "platform.memory.usecase.append_message")
	defer span.End()

	if err := in.Validate(); err != nil {
		return memory.Message{}, err
	}

	role, _ := memory.ParseMessageRole(in.Role)

	msg := memory.Message{
		ID:         uuid.New(),
		ThreadPK:   in.ThreadPK,
		ResourceID: in.ResourceID,
		Role:       role,
		Content:    in.Content,
		Parts:      in.Parts,
		CreatedAt:  time.Now().UTC(),
	}

	if err := uc.store.Append(ctx, in.ThreadPK, msg); err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Error(ctx, "platform.memory.usecase.append_message.failed",
			observability.String("resource_id", in.ResourceID),
			observability.Error(err),
		)
		return memory.Message{}, fmt.Errorf("platform.memory.usecase.append_message: %w", err)
	}

	return msg, nil
}
