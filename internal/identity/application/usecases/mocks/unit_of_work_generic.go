package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkGeneric[T any] struct{}

func (_m *UnitOfWorkGeneric[T]) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkGeneric[T]) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkGeneric[T any](t interface{ Cleanup(func()) }) *UnitOfWorkGeneric[T] {
	return &UnitOfWorkGeneric[T]{}
}
