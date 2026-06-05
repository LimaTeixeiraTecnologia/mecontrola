package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type UnitOfWorkUser struct {
	mock.Mock
}

type UnitOfWorkUser_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkUser) EXPECT() *UnitOfWorkUser_Expecter {
	return &UnitOfWorkUser_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkUser) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.User, error), opts ...uow.Option) (entities.User, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkUser(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkUser {
	m := &UnitOfWorkUser{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
