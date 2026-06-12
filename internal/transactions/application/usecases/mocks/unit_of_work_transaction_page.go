package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type UnitOfWorkTransactionPage struct {
	mock.Mock
}

type UnitOfWorkTransactionPage_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkTransactionPage) EXPECT() *UnitOfWorkTransactionPage_Expecter {
	return &UnitOfWorkTransactionPage_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkTransactionPage) Do(ctx context.Context, fn func(context.Context, database.DBTX) (usecases.TransactionPage, error), opts ...uow.Option) (usecases.TransactionPage, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkTransactionPage(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkTransactionPage {
	m := &UnitOfWorkTransactionPage{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
