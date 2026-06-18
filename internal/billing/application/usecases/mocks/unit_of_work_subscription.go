package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkSubscription struct{}

func (_m *UnitOfWorkSubscription) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkSubscription) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkSubscription(t interface{ Cleanup(func()) }) *UnitOfWorkSubscription {
	return &UnitOfWorkSubscription{}
}
