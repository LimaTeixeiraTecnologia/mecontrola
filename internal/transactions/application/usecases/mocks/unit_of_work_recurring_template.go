package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkRecurringTemplate struct{}

func (_m *UnitOfWorkRecurringTemplate) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkRecurringTemplate) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkRecurringTemplate(t interface{ Cleanup(func()) }) *UnitOfWorkRecurringTemplate {
	return &UnitOfWorkRecurringTemplate{}
}
