package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type IncomeSummaryReaderAdapterSuite struct {
	suite.Suite
	ctx context.Context
}

func TestIncomeSummaryReaderAdapterSuite(t *testing.T) {
	suite.Run(t, new(IncomeSummaryReaderAdapterSuite))
}

func (s *IncomeSummaryReaderAdapterSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *IncomeSummaryReaderAdapterSuite) TestExecute() {
	type args struct {
		in tools.IncomeSummaryInput
	}
	type dependencies struct {
		listUC *fakeListTransactionsUC
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result tools.IncomeSummaryResult, err error)
	}{
		{
			name: "deve agregar apenas entradas income",
			args: args{in: tools.IncomeSummaryInput{UserID: "00000000-0000-0000-0000-000000000001", RefMonth: "2026-06"}},
			dependencies: dependencies{
				listUC: &fakeListTransactionsUC{
					out: transactionsusecases.TransactionPage{
						Transactions: []transactionsoutput.Transaction{
							{Direction: "income", AmountCents: 300000, Description: "Salário"},
							{Direction: "outcome", AmountCents: 100000, Description: "Mercado"},
							{Direction: "income", AmountCents: 50000, Description: "Freelance"},
						},
					},
				},
			},
			expect: func(result tools.IncomeSummaryResult, err error) {
				s.NoError(err)
				s.Equal(int64(350000), result.TotalCents)
				s.Len(result.Sources, 2)
				s.Equal("2026-06", result.RefMonth)
			},
		},
		{
			name: "deve retornar zero quando nao ha entradas income",
			args: args{in: tools.IncomeSummaryInput{UserID: "00000000-0000-0000-0000-000000000001", RefMonth: "2026-06"}},
			dependencies: dependencies{
				listUC: &fakeListTransactionsUC{
					out: transactionsusecases.TransactionPage{
						Transactions: []transactionsoutput.Transaction{
							{Direction: "outcome", AmountCents: 100000, Description: "Mercado"},
						},
					},
				},
			},
			expect: func(result tools.IncomeSummaryResult, err error) {
				s.NoError(err)
				s.Equal(int64(0), result.TotalCents)
				s.Empty(result.Sources)
			},
		},
		{
			name: "deve propagar erro do use case",
			args: args{in: tools.IncomeSummaryInput{UserID: "00000000-0000-0000-0000-000000000001", RefMonth: "2026-06"}},
			dependencies: dependencies{
				listUC: &fakeListTransactionsUC{err: errors.New("falha no banco")},
			},
			expect: func(result tools.IncomeSummaryResult, err error) {
				s.Error(err)
				s.Equal(tools.IncomeSummaryResult{}, result)
			},
		},
		{
			name: "deve retornar erro quando userID e invalido",
			args: args{in: tools.IncomeSummaryInput{UserID: "invalido", RefMonth: "2026-06"}},
			dependencies: dependencies{
				listUC: &fakeListTransactionsUC{},
			},
			expect: func(result tools.IncomeSummaryResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			adapter := NewIncomeSummaryReaderAdapter(scenario.dependencies.listUC)
			result, err := adapter.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}
