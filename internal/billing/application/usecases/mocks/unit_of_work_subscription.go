package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
)

type UnitOfWorkSubscription struct {
	mock.Mock
}

type UnitOfWorkSubscription_Expecter struct {
	mock *mock.Mock
}

func (_m *UnitOfWorkSubscription) EXPECT() *UnitOfWorkSubscription_Expecter {
	return &UnitOfWorkSubscription_Expecter{mock: &_m.Mock}
}

func (_m *UnitOfWorkSubscription) Do(ctx context.Context, fn func(context.Context, database.DBTX) (entities.Subscription, error), opts ...uow.Option) (entities.Subscription, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkSubscription(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkSubscription {
	m := &UnitOfWorkSubscription{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
