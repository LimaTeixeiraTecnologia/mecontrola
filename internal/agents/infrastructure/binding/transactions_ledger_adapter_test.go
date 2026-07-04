package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	txifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	txifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type TransactionsLedgerAdapterSuite struct {
	suite.Suite
	userID   uuid.UUID
	ctx      context.Context
	factory  *txifacemocks.RepositoryFactory
	summMock *txifacemocks.MonthlySummaryRepository
}

func TestTransactionsLedgerAdapterSuite(t *testing.T) {
	suite.Run(t, new(TransactionsLedgerAdapterSuite))
}

func (s *TransactionsLedgerAdapterSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = txifacemocks.NewRepositoryFactory(s.T())
	s.summMock = txifacemocks.NewMonthlySummaryRepository(s.T())
}

func (s *TransactionsLedgerAdapterSuite) buildAdapter() agentsifaces.TransactionsLedger {
	o11y := fake.NewProvider()
	summUoW := uowMocks.NewUnitOfWorkMonthlySummary(s.T())
	entriesUoW := uowMocks.NewUnitOfWorkMonthlyEntries(s.T())
	getMSUC := txusecases.NewGetMonthlySummary(s.factory, summUoW, o11y)
	listMEUC := txusecases.NewListMonthlyEntries(s.factory, entriesUoW, o11y)
	return NewTransactionsLedgerAdapter(nil, nil, nil, listMEUC, getMSUC, nil, nil, nil, o11y)
}

func (s *TransactionsLedgerAdapterSuite) TestGetMonthlySummary_Success() {
	type args struct {
		refMonth string
	}
	type dependencies struct {
		summMock *txifacemocks.MonthlySummaryRepository
	}

	rm, _ := valueobjects.NewRefMonth("2026-06")

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result agentsifaces.MonthlySummary, err error)
	}{
		{
			name: "deve retornar resumo mensal com sucesso",
			args: args{refMonth: "2026-06"},
			dependencies: dependencies{
				summMock: func() *txifacemocks.MonthlySummaryRepository {
					s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summMock).Once()
					summary := entities.NewMonthlySummary(s.userID, rm, 10000, 5000, 1, nil)
					s.summMock.EXPECT().Get(mock.Anything, s.userID, rm).Return(&summary, nil).Once()
					return s.summMock
				}(),
			},
			expect: func(result agentsifaces.MonthlySummary, err error) {
				s.NoError(err)
				s.Equal("2026-06", result.RefMonth)
				s.Equal(int64(10000), result.IncomeCents)
				s.Equal(int64(5000), result.OutcomeCents)
			},
		},
		{
			name: "deve retornar erro quando repositório falha",
			args: args{refMonth: "2026-06"},
			dependencies: dependencies{
				summMock: func() *txifacemocks.MonthlySummaryRepository {
					s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summMock).Once()
					s.summMock.EXPECT().Get(mock.Anything, s.userID, rm).Return(nil, errors.New("db error")).Once()
					return s.summMock
				}(),
			},
			expect: func(result agentsifaces.MonthlySummary, err error) {
				s.Error(err)
				s.Empty(result.RefMonth)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			adapter := s.buildAdapter()
			result, err := adapter.GetMonthlySummary(s.ctx, s.userID, scenario.args.refMonth)
			scenario.expect(result, err)
		})
	}
}

func (s *TransactionsLedgerAdapterSuite) TestGetMonthlySummary_InvalidRefMonth() {
	adapter := s.buildAdapter()
	_, err := adapter.GetMonthlySummary(s.ctx, s.userID, "not-a-month")
	s.Error(err)
}

func (s *TransactionsLedgerAdapterSuite) TestListMonthlyEntries_InvalidRefMonth() {
	adapter := s.buildAdapter()
	_, err := adapter.ListMonthlyEntries(s.ctx, s.userID, "not-a-month", "", 10)
	s.Error(err)
}

func (s *TransactionsLedgerAdapterSuite) TestListMonthlyEntries_RepoError() {
	type dependencies struct {
		summMock *txifacemocks.MonthlySummaryRepository
	}

	rm, _ := valueobjects.NewRefMonth("2026-06")

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(entries []agentsifaces.MonthlyEntry, err error)
	}{
		{
			name: "deve retornar erro quando repositório falha",
			dependencies: dependencies{
				summMock: func() *txifacemocks.MonthlySummaryRepository {
					s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summMock).Once()
					s.summMock.EXPECT().
						ListEntries(mock.Anything, s.userID, rm, mock.Anything, mock.Anything).
						Return(nil, txifaces.Cursor{}, errors.New("db error")).
						Once()
					return s.summMock
				}(),
			},
			expect: func(entries []agentsifaces.MonthlyEntry, err error) {
				s.Error(err)
				s.Nil(entries)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			adapter := s.buildAdapter()
			entries, err := adapter.ListMonthlyEntries(s.ctx, s.userID, "2026-06", "", 10)
			scenario.expect(entries, err)
		})
	}
}
