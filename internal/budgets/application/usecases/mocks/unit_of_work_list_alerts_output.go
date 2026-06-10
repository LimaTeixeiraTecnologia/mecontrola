// Code generated manually — mirrors billing pattern for UoW test double.
package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type UnitOfWorkListAlertsOutput struct {
	mock.Mock
}

type UnitOfWorkListAlertsOutput_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkListAlertsOutput) EXPECT() *UnitOfWorkListAlertsOutput_Expecter {
	return &UnitOfWorkListAlertsOutput_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkListAlertsOutput) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.ListAlertsOutput, error), opts ...uow.Option) (output.ListAlertsOutput, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkListAlertsOutput(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkListAlertsOutput {
	m := &UnitOfWorkListAlertsOutput{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
