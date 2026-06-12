package mocks

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type UnitOfWorkCardPurchaseOutput struct {
	mock.Mock
}

func (_m *UnitOfWorkCardPurchaseOutput) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.CardPurchase, error), opts ...uow.Option) (output.CardPurchase, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkCardPurchaseOutput(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkCardPurchaseOutput {
	m := &UnitOfWorkCardPurchaseOutput{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

type UnitOfWorkCardInvoiceOutput struct {
	mock.Mock
}

func (_m *UnitOfWorkCardInvoiceOutput) Do(ctx context.Context, fn func(context.Context, database.DBTX) (output.CardInvoice, error), opts ...uow.Option) (output.CardInvoice, error) {
	return fn(ctx, nil)
}

func NewUnitOfWorkCardInvoiceOutput(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnitOfWorkCardInvoiceOutput {
	m := &UnitOfWorkCardInvoiceOutput{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
