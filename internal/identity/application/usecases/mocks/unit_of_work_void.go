package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkVoid struct{}

func (_m *UnitOfWorkVoid) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkVoid) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkVoid(t interface{ Cleanup(func()) }) *UnitOfWorkVoid { return &UnitOfWorkVoid{} }
