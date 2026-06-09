package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"
)

type UnitOfWorkVoid struct {
	mock.Mock
}

type UnitOfWorkVoid_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkVoid) EXPECT() *UnitOfWorkVoid_Expecter {
	return &UnitOfWorkVoid_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkVoid) Do(ctx context.Context, fn func(context.Context, database.DBTX) (struct{}, error), opts ...uow.Option) (struct{}, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkVoid(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkVoid {
	m := &UnitOfWorkVoid{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
