package interfaces

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

type RepositoryFactory interface {
	MagicTokenRepository(db database.DBTX) MagicTokenRepository
	SupportSignalRepository(db database.DBTX) SupportSignalRepository
	OnboardingCleanupRepository(db database.DBTX) OnboardingCleanupRepository
}
