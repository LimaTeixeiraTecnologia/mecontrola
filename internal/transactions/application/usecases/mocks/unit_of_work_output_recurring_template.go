package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type UnitOfWorkOutputRecurringTemplate struct {
	mock.Mock
}

type UnitOfWorkOutputRecurringTemplate_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkOutputRecurringTemplate) EXPECT() *UnitOfWorkOutputRecurringTemplate_Expecter {
	return &UnitOfWorkOutputRecurringTemplate_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkOutputRecurringTemplate) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.RecurringTemplate, error), opts ...uow.Option) (output.RecurringTemplate, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkOutputRecurringTemplate(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkOutputRecurringTemplate {
	m := &UnitOfWorkOutputRecurringTemplate{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
