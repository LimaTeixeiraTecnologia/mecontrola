package handlers_test

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type mockCreateCard struct{ mock.Mock }

func (m *mockCreateCard) Execute(ctx context.Context, in input.CreateCard) (output.Card, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Card), args.Error(1)
}

type mockListCards struct{ mock.Mock }

func (m *mockListCards) Execute(ctx context.Context, in input.ListCards) (output.CardList, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.CardList), args.Error(1)
}

type mockGetCard struct{ mock.Mock }

func (m *mockGetCard) Execute(ctx context.Context, in input.GetCard) (output.Card, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Card), args.Error(1)
}

type mockUpdateCard struct{ mock.Mock }

func (m *mockUpdateCard) Execute(ctx context.Context, in input.UpdateCard) (output.Card, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Card), args.Error(1)
}

type mockSoftDeleteCard struct{ mock.Mock }

func (m *mockSoftDeleteCard) Execute(ctx context.Context, in input.SoftDeleteCard) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockInvoiceFor struct{ mock.Mock }

func (m *mockInvoiceFor) Execute(ctx context.Context, in input.InvoiceFor) (output.Invoice, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Invoice), args.Error(1)
}

type mockBestPurchaseDay struct{ mock.Mock }

func (m *mockBestPurchaseDay) Execute(ctx context.Context, in input.BestPurchaseDay) (output.BestPurchaseDay, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.BestPurchaseDay), args.Error(1)
}
