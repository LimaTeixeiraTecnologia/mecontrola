package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
)

type BestPurchaseDaySuite struct {
	suite.Suite
	obs            observability.Observability
	ctx            context.Context
	factoryMock    *ifacemocks.RepositoryFactory
	bankReaderMock *ifacemocks.BankDaysReader
}

func TestBestPurchaseDay(t *testing.T) {
	suite.Run(t, new(BestPurchaseDaySuite))
}

func (s *BestPurchaseDaySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.bankReaderMock = ifacemocks.NewBankDaysReader(s.T())
}

func (s *BestPurchaseDaySuite) TestExecute_HappyPath() {
	type args struct {
		in input.BestPurchaseDay
	}
	type deps struct {
		factory    *ifacemocks.RepositoryFactory
		bankReader *ifacemocks.BankDaysReader
	}

	scenarios := []struct {
		name   string
		args   args
		deps   deps
		expect func(closingDay, bestDay int, err error)
	}{
		{
			name: "due_day=22 e 7 dias antes retorna closing_day=15 best=16",
			args: args{in: input.BestPurchaseDay{Bank: "nubank", DueDay: 22}},
			deps: deps{
				factory: func() *ifacemocks.RepositoryFactory {
					s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
					return s.factoryMock
				}(),
				bankReader: func() *ifacemocks.BankDaysReader {
					s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
					return s.bankReaderMock
				}(),
			},
			expect: func(closingDay, bestDay int, err error) {
				s.Require().NoError(err)
				s.Greater(closingDay, 0)
				s.Greater(bestDay, 0)
			},
		},
		{
			name: "due_day=31 wrap: best_purchase_day=1",
			args: args{in: input.BestPurchaseDay{Bank: "itau", DueDay: 31}},
			deps: deps{
				factory: func() *ifacemocks.RepositoryFactory {
					s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
					return s.factoryMock
				}(),
				bankReader: func() *ifacemocks.BankDaysReader {
					s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
					return s.bankReaderMock
				}(),
			},
			expect: func(closingDay, bestDay int, err error) {
				s.Require().NoError(err)
				s.Equal(24, closingDay)
				s.Equal(25, bestDay)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewBestPurchaseDay(scenario.deps.factory, nil, s.obs)
			out, err := sut.Execute(s.ctx, scenario.args.in)
			scenario.expect(out.ClosingDay, out.BestPurchaseDay, err)
		})
	}
}

func (s *BestPurchaseDaySuite) TestExecute_ValidationError() {
	type args struct {
		in input.BestPurchaseDay
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name: "banco vazio retorna erro de validacao",
			args: args{in: input.BestPurchaseDay{Bank: "", DueDay: 22}},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, input.ErrCardBankRequired)
			},
		},
		{
			name: "due_day=0 retorna erro de validacao",
			args: args{in: input.BestPurchaseDay{Bank: "nubank", DueDay: 0}},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, input.ErrCardDueDayInvalid)
			},
		},
		{
			name: "due_day=32 retorna erro de validacao",
			args: args{in: input.BestPurchaseDay{Bank: "nubank", DueDay: 32}},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, input.ErrCardDueDayInvalid)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewBestPurchaseDay(s.factoryMock, nil, s.obs)
			_, err := sut.Execute(s.ctx, scenario.args.in)
			scenario.expect(err)
		})
	}
}

func (s *BestPurchaseDaySuite) TestExecute_BankReaderError() {
	bankErr := errors.New("bank reader unavailable")

	s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
	s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(0, bankErr).Once()

	sut := NewBestPurchaseDay(s.factoryMock, nil, s.obs)
	_, err := sut.Execute(s.ctx, input.BestPurchaseDay{Bank: "nubank", DueDay: 22})

	s.Require().Error(err)
	s.Contains(err.Error(), "bank reader unavailable")
}
