package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) appinterfaces.RepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) MagicTokenRepository(db database.DBTX) appinterfaces.MagicTokenRepository {
	return postgres.NewMagicTokenRepository(f.o11y, db)
}

func (f *repositoryFactory) SupportSignalRepository(db database.DBTX) appinterfaces.SupportSignalRepository {
	return postgres.NewSupportSignalRepository(f.o11y, db)
}

func (f *repositoryFactory) OnboardingCleanupRepository(db database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return postgres.NewOnboardingCleanupRepository(f.o11y, db)
}
