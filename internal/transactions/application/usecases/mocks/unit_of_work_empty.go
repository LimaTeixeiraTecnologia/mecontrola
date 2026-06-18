package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkEmpty struct{}

func (_m *UnitOfWorkEmpty) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkEmpty) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkEmpty(t interface{ Cleanup(func()) }) *UnitOfWorkEmpty { return &UnitOfWorkEmpty{} }
