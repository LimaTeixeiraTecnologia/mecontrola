package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkRecurringTemplatePage struct{}

func (_m *UnitOfWorkRecurringTemplatePage) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkRecurringTemplatePage) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkRecurringTemplatePage(t interface{ Cleanup(func()) }) *UnitOfWorkRecurringTemplatePage {
	return &UnitOfWorkRecurringTemplatePage{}
}
