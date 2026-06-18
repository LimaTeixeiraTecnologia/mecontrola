package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkBudget struct{}

func (_m *UnitOfWorkBudget) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkBudget) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkBudget(t interface{ Cleanup(func()) }) *UnitOfWorkBudget {
	return &UnitOfWorkBudget{}
}
