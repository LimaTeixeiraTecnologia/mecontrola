package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type CardPurchaseCreator struct {
	mock.Mock
}

type CardPurchaseCreator_Expecter struct {
	mock *mock.Mock
}

func (_m *CardPurchaseCreator) EXPECT() *CardPurchaseCreator_Expecter {
	return &CardPurchaseCreator_Expecter{mock: &_m.Mock}
}

func (_m *CardPurchaseCreator) Execute(ctx context.Context, raw input.RawCreateCardPurchase) (output.CardPurchase, error) {
	ret := _m.Called(ctx, raw)
	return ret.Get(0).(output.CardPurchase), ret.Error(1)
}

func NewCardPurchaseCreator(t interface {
	mock.TestingT
	Cleanup(func())
}) *CardPurchaseCreator {
	m := &CardPurchaseCreator{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
