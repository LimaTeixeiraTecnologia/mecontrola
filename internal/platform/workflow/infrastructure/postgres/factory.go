package postgres

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type StoreFactory interface {
	Store(db database.DBTX) workflow.Store
}

type storeFactory struct {
	o11y observability.Observability
}

func NewStoreFactory(o11y observability.Observability) StoreFactory {
	return &storeFactory{o11y: o11y}
}

func (f *storeFactory) Store(db database.DBTX) workflow.Store {
	return NewPostgresStore(f.o11y, db)
}
