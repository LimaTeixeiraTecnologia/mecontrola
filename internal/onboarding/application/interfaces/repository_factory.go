package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

type RepositoryFactory interface {
	MagicTokenRepository(db database.DBTX) MagicTokenRepository
	SupportSignalRepository(db database.DBTX) SupportSignalRepository
	MetaMessageRepository(db database.DBTX) MetaMessageRepository
	OnboardingCleanupRepository(db database.DBTX) OnboardingCleanupRepository
}
