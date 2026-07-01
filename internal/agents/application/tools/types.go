package tools

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
)

type idempotentWriter interface {
	Execute(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error)
}
