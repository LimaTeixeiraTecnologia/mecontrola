package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"
)

type UnitOfWorkGeneric[T any] struct {
	mock.Mock
}

type UnitOfWorkGeneric_Expecter[T any] struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkGeneric[T]) EXPECT() *UnitOfWorkGeneric_Expecter[T] {
	return &UnitOfWorkGeneric_Expecter[T]{mock: &_m.Mock}
}

func (_m *UnitOfWorkGeneric[T]) Do(ctx context.Context, fn func(context.Context, database.DBTX) (T, error), opts ...uow.Option) (T, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkGeneric[T any](t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkGeneric[T] {
	m := &UnitOfWorkGeneric[T]{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
