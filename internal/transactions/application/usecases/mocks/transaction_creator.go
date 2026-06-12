package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type TransactionCreator struct {
	mock.Mock
}

type TransactionCreator_Expecter struct {
	mock *mock.Mock
}

func (_m *TransactionCreator) EXPECT() *TransactionCreator_Expecter {
	return &TransactionCreator_Expecter{mock: &_m.Mock}
}

func (_m *TransactionCreator) Execute(ctx context.Context, raw input.RawCreateTransaction) (output.Transaction, error) {
	ret := _m.Called(ctx, raw)
	return ret.Get(0).(output.Transaction), ret.Error(1)
}

func NewTransactionCreator(t interface {
	mock.TestingT
	Cleanup(func())
}) *TransactionCreator {
	m := &TransactionCreator{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
