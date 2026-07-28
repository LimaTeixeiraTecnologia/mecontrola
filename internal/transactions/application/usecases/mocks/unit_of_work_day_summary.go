package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkDaySummary struct{}

func (_m *UnitOfWorkDaySummary) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkDaySummary) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkDaySummary(t interface{ Cleanup(func()) }) *UnitOfWorkDaySummary {
	return &UnitOfWorkDaySummary{}
}
