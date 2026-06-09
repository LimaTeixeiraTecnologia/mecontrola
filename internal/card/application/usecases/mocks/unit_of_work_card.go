package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
)

type UnitOfWorkCard struct {
	mock.Mock
}

type UnitOfWorkCard_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkCard) EXPECT() *UnitOfWorkCard_Expecter {
	return &UnitOfWorkCard_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkCard) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.Card, error), opts ...uow.Option) (entities.Card, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkCard(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkCard {
	m := &UnitOfWorkCard{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
