package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkTransaction struct{}

func (_m *UnitOfWorkTransaction) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkTransaction) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkTransaction(t interface{ Cleanup(func()) }) *UnitOfWorkTransaction {
	return &UnitOfWorkTransaction{}
}
