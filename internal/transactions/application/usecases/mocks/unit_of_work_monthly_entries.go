package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkMonthlyEntries struct{}

func (_m *UnitOfWorkMonthlyEntries) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkMonthlyEntries) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkMonthlyEntries(t interface{ Cleanup(func()) }) *UnitOfWorkMonthlyEntries {
	return &UnitOfWorkMonthlyEntries{}
}
