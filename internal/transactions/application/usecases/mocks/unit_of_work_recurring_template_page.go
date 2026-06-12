package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type UnitOfWorkRecurringTemplatePage struct {
	mock.Mock
}

type UnitOfWorkRecurringTemplatePage_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkRecurringTemplatePage) EXPECT() *UnitOfWorkRecurringTemplatePage_Expecter {
	return &UnitOfWorkRecurringTemplatePage_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkRecurringTemplatePage) Do(ctx context.Context, fn func(context.Context, database.DBTX) (usecases.RecurringTemplatePage, error), opts ...uow.Option) (usecases.RecurringTemplatePage, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkRecurringTemplatePage(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkRecurringTemplatePage {
	m := &UnitOfWorkRecurringTemplatePage{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
