package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ReconcileMonthlySummarySuite struct {
	suite.Suite
	ctx         context.Context
	factory     *mockInterfaces.RepositoryFactory
	txRepo      *mockInterfaces.TransactionRepository
	invoiceRepo *mockInterfaces.CardInvoiceRepository
	summaryRepo *mockInterfaces.MonthlySummaryRepository
}

func TestReconcileMonthlySummarySuite(t *testing.T) {
	suite.Run(t, new(ReconcileMonthlySummarySuite))
}

func (s *ReconcileMonthlySummarySuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.txRepo = mockInterfaces.NewTransactionRepository(s.T())
	s.invoiceRepo = mockInterfaces.NewCardInvoiceRepository(s.T())
	s.summaryRepo = mockInterfaces.NewMonthlySummaryRepository(s.T())

	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.txRepo).Maybe()
	s.factory.EXPECT().CardInvoiceRepository(mock.Anything).Return(s.invoiceRepo).Maybe()
	s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summaryRepo).Maybe()
}

func (s *ReconcileMonthlySummarySuite) TestExecute_NoActiveSummaries() {
	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return(nil, interfaces.Cursor{}, nil).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *ReconcileMonthlySummarySuite) TestExecute_NoDrift_NoUpdate() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	key := interfaces.MonthlySummaryKey{UserID: userID, RefMonth: "2026-06"}

	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return([]interfaces.MonthlySummaryKey{key}, interfaces.Cursor{}, nil).Once()

	s.txRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(50000), int64(20000), nil).Once()

	s.invoiceRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(10000), nil).Once()

	summary := entities.NewMonthlySummary(userID, rm, 50000, 30000, 1, &time.Time{})
	s.summaryRepo.EXPECT().
		Get(mock.Anything, userID, rm).
		Return(&summary, nil).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *ReconcileMonthlySummarySuite) TestExecute_WithDrift_Corrects() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	key := interfaces.MonthlySummaryKey{UserID: userID, RefMonth: "2026-06"}

	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return([]interfaces.MonthlySummaryKey{key}, interfaces.Cursor{}, nil).Once()

	s.txRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(60000), int64(20000), nil).Once()

	s.invoiceRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(10000), nil).Once()

	summary := entities.NewMonthlySummary(userID, rm, 50000, 30000, 1, &time.Time{})
	s.summaryRepo.EXPECT().
		Get(mock.Anything, userID, rm).
		Return(&summary, nil).Once()

	s.summaryRepo.EXPECT().
		Upsert(mock.Anything, userID, rm, int64(60000), int64(30000), mock.Anything).
		Return(nil).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *ReconcileMonthlySummarySuite) TestExecute_SummaryAbsent_Upserts() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	key := interfaces.MonthlySummaryKey{UserID: userID, RefMonth: "2026-06"}

	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return([]interfaces.MonthlySummaryKey{key}, interfaces.Cursor{}, nil).Once()

	s.txRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(30000), int64(10000), nil).Once()

	s.invoiceRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(5000), nil).Once()

	s.summaryRepo.EXPECT().
		Get(mock.Anything, userID, rm).
		Return(nil, nil).Once()

	s.summaryRepo.EXPECT().
		Upsert(mock.Anything, userID, rm, int64(30000), int64(15000), mock.Anything).
		Return(nil).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *ReconcileMonthlySummarySuite) TestExecute_InvalidRefMonth_Skips() {
	userID := uuid.New()
	key := interfaces.MonthlySummaryKey{UserID: userID, RefMonth: "invalid-month"}

	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return([]interfaces.MonthlySummaryKey{key}, interfaces.Cursor{}, nil).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().Error(err)
}

func (s *ReconcileMonthlySummarySuite) TestExecute_TxRepoError_ReturnsError() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	key := interfaces.MonthlySummaryKey{UserID: userID, RefMonth: "2026-06"}

	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return([]interfaces.MonthlySummaryKey{key}, interfaces.Cursor{}, nil).Once()

	s.txRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(0), int64(0), errors.New("db error")).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().Error(err)
}

func (s *ReconcileMonthlySummarySuite) TestExecute_CardInvoiceRepoError_ReturnsError() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	key := interfaces.MonthlySummaryKey{UserID: userID, RefMonth: "2026-06"}

	s.summaryRepo.EXPECT().
		ListActiveSince(mock.Anything, mock.Anything, interfaces.Cursor{}, 200).
		Return([]interfaces.MonthlySummaryKey{key}, interfaces.Cursor{}, nil).Once()

	s.txRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(50000), int64(10000), nil).Once()

	s.invoiceRepo.EXPECT().
		SumByMonth(mock.Anything, userID, rm).
		Return(int64(0), errors.New("invoice db error")).Once()

	uc := usecases.NewReconcileMonthlySummary(nil, s.factory, 48, noop.NewProvider())
	err := uc.Execute(s.ctx)
	s.Require().Error(err)
}
