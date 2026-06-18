package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkCardPurchaseOutput struct{}

func (_m *UnitOfWorkCardPurchaseOutput) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkCardPurchaseOutput) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkCardPurchaseOutput(t interface{ Cleanup(func()) }) *UnitOfWorkCardPurchaseOutput {
	return &UnitOfWorkCardPurchaseOutput{}
}
