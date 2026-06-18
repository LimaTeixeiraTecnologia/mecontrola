package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkExpense struct{}

func (_m *UnitOfWorkExpense) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkExpense) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkExpense(t interface{ Cleanup(func()) }) *UnitOfWorkExpense {
	return &UnitOfWorkExpense{}
}
