package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type SupportSignalRepository interface {
	Insert(ctx context.Context, signal entities.SupportSignal) error
}

type SupportSignalRepositoryFactory interface {
	SupportSignalRepository(db database.DBTX) SupportSignalRepository
}
