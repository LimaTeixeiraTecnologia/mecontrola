package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type UnitOfWorkListCardPurchases struct {
	mock.Mock
}

func (_m *UnitOfWorkListCardPurchases) Do(ctx context.Context, fn func(context.Context, database.DBTX) (usecases.ListCardPurchasesOutput, error), opts ...uow.Option) (usecases.ListCardPurchasesOutput, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkListCardPurchases(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkListCardPurchases {
	m := &UnitOfWorkListCardPurchases{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
