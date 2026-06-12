package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type UnitOfWorkCardPurchase struct {
	mock.Mock
}

func (_m *UnitOfWorkCardPurchase) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.CardPurchase, error), opts ...uow.Option) (entities.CardPurchase, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkCardPurchase(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkCardPurchase {
	m := &UnitOfWorkCardPurchase{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
