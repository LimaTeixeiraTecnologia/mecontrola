package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkListCardPurchases struct{}

func (_m *UnitOfWorkListCardPurchases) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkListCardPurchases) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkListCardPurchases(t interface{ Cleanup(func()) }) *UnitOfWorkListCardPurchases {
	return &UnitOfWorkListCardPurchases{}
}
