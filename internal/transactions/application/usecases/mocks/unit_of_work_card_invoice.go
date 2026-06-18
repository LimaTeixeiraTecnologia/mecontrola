package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkCardInvoiceOutput struct{}

func (_m *UnitOfWorkCardInvoiceOutput) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkCardInvoiceOutput) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkCardInvoiceOutput(t interface{ Cleanup(func()) }) *UnitOfWorkCardInvoiceOutput {
	return &UnitOfWorkCardInvoiceOutput{}
}
