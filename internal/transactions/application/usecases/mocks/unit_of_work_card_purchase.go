package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkCardPurchase struct{}

func (_m *UnitOfWorkCardPurchase) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkCardPurchase) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkCardPurchase(t interface{ Cleanup(func()) }) *UnitOfWorkCardPurchase {
	return &UnitOfWorkCardPurchase{}
}
