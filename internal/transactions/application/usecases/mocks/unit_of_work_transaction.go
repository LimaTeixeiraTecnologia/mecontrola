package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type UnitOfWorkTransaction struct {
	mock.Mock
}

type UnitOfWorkTransaction_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkTransaction) EXPECT() *UnitOfWorkTransaction_Expecter {
	return &UnitOfWorkTransaction_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkTransaction) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.Transaction, error), opts ...uow.Option) (entities.Transaction, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkTransaction(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkTransaction {
	m := &UnitOfWorkTransaction{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
