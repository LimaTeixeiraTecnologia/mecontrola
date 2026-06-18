package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkMonthlySummary struct{}

func (_m *UnitOfWorkMonthlySummary) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkMonthlySummary) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkMonthlySummary(t interface{ Cleanup(func()) }) *UnitOfWorkMonthlySummary {
	return &UnitOfWorkMonthlySummary{}
}
