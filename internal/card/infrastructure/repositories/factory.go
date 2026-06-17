package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) CardRepository(db database.DBTX) interfaces.CardRepository {
	return postgres.NewCardRepository(f.o11y, db)
}

func (f *repositoryFactory) InvoiceDueAlertSentRepository(db database.DBTX) interfaces.InvoiceDueAlertSentRepository {
	return postgres.NewInvoiceDueAlertSentRepository(f.o11y, db)
}
