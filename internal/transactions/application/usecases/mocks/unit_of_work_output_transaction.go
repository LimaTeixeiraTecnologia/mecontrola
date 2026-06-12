package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type UnitOfWorkOutputTransaction struct {
	mock.Mock
}

type UnitOfWorkOutputTransaction_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkOutputTransaction) EXPECT() *UnitOfWorkOutputTransaction_Expecter {
	return &UnitOfWorkOutputTransaction_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkOutputTransaction) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.Transaction, error), opts ...uow.Option) (output.Transaction, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkOutputTransaction(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkOutputTransaction {
	m := &UnitOfWorkOutputTransaction{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
