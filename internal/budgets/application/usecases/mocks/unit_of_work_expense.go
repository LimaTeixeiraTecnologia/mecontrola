// Code generated manually — mirrors billing pattern for UoW test double.
package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type UnitOfWorkExpense struct {
	mock.Mock
}

type UnitOfWorkExpense_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkExpense) EXPECT() *UnitOfWorkExpense_Expecter {
	return &UnitOfWorkExpense_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkExpense) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.Expense, error), opts ...uow.Option) (entities.Expense, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkExpense(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkExpense {
	m := &UnitOfWorkExpense{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
