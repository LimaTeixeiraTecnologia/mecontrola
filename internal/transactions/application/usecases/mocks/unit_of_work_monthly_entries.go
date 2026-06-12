package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type UnitOfWorkMonthlyEntries struct {
	mock.Mock
}

func (_m *UnitOfWorkMonthlyEntries) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.MonthlyEntriesPage, error), opts ...uow.Option) (output.MonthlyEntriesPage, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkMonthlyEntries(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkMonthlyEntries {
	m := &UnitOfWorkMonthlyEntries{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
