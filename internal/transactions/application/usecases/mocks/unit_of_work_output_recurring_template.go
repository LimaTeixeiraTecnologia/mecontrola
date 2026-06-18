package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkOutputRecurringTemplate struct{}

func (_m *UnitOfWorkOutputRecurringTemplate) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkOutputRecurringTemplate) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkOutputRecurringTemplate(t interface{ Cleanup(func()) }) *UnitOfWorkOutputRecurringTemplate {
	return &UnitOfWorkOutputRecurringTemplate{}
}
