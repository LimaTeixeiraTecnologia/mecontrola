// Code generated manually — mirrors billing pattern for UoW test double.
package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type UnitOfWorkMonthlySummary struct {
	mock.Mock
}

type UnitOfWorkMonthlySummary_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkMonthlySummary) EXPECT() *UnitOfWorkMonthlySummary_Expecter {
	return &UnitOfWorkMonthlySummary_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkMonthlySummary) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.MonthlySummaryOutput, error), opts ...uow.Option) (output.MonthlySummaryOutput, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkMonthlySummary(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkMonthlySummary {
	m := &UnitOfWorkMonthlySummary{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
