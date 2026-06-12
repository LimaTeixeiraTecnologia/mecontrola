package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"
)

type UnitOfWorkEmpty struct {
	mock.Mock
}

func (_m *UnitOfWorkEmpty) Do(ctx context.Context, fn func(context.Context, database.DBTX) (struct{}, error), opts ...uow.Option) (struct{}, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkEmpty(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkEmpty {
	m := &UnitOfWorkEmpty{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
