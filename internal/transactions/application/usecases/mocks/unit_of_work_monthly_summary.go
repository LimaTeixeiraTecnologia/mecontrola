package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type UnitOfWorkMonthlySummary struct {
	mock.Mock
}

func (_m *UnitOfWorkMonthlySummary) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.MonthlySummary, error), opts ...uow.Option) (output.MonthlySummary, error) {
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
