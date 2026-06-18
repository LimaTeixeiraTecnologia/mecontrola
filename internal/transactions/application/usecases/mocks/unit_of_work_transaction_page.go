package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkTransactionPage struct{}

func (_m *UnitOfWorkTransactionPage) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkTransactionPage) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkTransactionPage(t interface{ Cleanup(func()) }) *UnitOfWorkTransactionPage {
	return &UnitOfWorkTransactionPage{}
}
