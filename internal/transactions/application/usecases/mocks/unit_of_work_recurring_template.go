package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type UnitOfWorkRecurringTemplate struct {
	mock.Mock
}

type UnitOfWorkRecurringTemplate_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkRecurringTemplate) EXPECT() *UnitOfWorkRecurringTemplate_Expecter {
	return &UnitOfWorkRecurringTemplate_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkRecurringTemplate) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.RecurringTemplate, error), opts ...uow.Option) (entities.RecurringTemplate, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkRecurringTemplate(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkRecurringTemplate {
	m := &UnitOfWorkRecurringTemplate{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
