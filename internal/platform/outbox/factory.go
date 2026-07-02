package outbox

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type repositoryFactory struct {
	claimDeferredTotal observability.Counter
}

func NewRepositoryFactory(o11y observability.Observability) OutboxRepositoryFactory {
	f := &repositoryFactory{}
	if o11y != nil {
		f.claimDeferredTotal = o11y.Metrics().Counter(
			"outbox_claim_deferred_total",
			"Total de claim batches adiados por violação de índice único (23505)",
			"1",
		)
	}
	return f
}

func (f *repositoryFactory) OutboxRepository(db database.DBTX) OutboxRepository {
	return NewObservablePostgresStorage(db, f.claimDeferredTotal)
}
