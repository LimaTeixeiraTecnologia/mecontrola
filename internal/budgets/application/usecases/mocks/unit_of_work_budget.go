// Code generated manually — mirrors billing pattern for UoW test double.
package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type UnitOfWorkBudget struct {
	mock.Mock
}

type UnitOfWorkBudget_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkBudget) EXPECT() *UnitOfWorkBudget_Expecter {
	return &UnitOfWorkBudget_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkBudget) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.Budget, error), opts ...uow.Option) (entities.Budget, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkBudget(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkBudget {
	m := &UnitOfWorkBudget{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
