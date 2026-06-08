package interfaces

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

type MetaMessageRepository interface {
	InsertIfAbsent(ctx context.Context, wamid string) (inserted bool, err error)
}

type MetaMessageRepositoryFactory interface {
	MetaMessageRepository(db database.DBTX) MetaMessageRepository
}
