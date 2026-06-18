package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkOutputTransaction struct{}

func (_m *UnitOfWorkOutputTransaction) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkOutputTransaction) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkOutputTransaction(t interface{ Cleanup(func()) }) *UnitOfWorkOutputTransaction {
	return &UnitOfWorkOutputTransaction{}
}
