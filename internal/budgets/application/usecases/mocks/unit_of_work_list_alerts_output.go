package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkListAlertsOutput struct{}

func (_m *UnitOfWorkListAlertsOutput) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkListAlertsOutput) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkListAlertsOutput(t interface{ Cleanup(func()) }) *UnitOfWorkListAlertsOutput {
	return &UnitOfWorkListAlertsOutput{}
}
