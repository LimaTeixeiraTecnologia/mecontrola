package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RecomputeMonthlySummarySuite struct {
	suite.Suite
	ctx      context.Context
	userID   uuid.UUID
	refMonth valueobjects.RefMonth
	factory  *mockInterfaces.RepositoryFactory
	txRepo   *mockInterfaces.TransactionRepository
	invRepo  *mockInterfaces.CardInvoiceRepository
	summRepo *mockInterfaces.MonthlySummaryRepository
	uow      *uowMocks.UnitOfWorkEmpty
	useCase  *RecomputeMonthlySummary
}

func TestRecomputeMonthlySummarySuite(t *testing.T) {
	suite.Run(t, new(RecomputeMonthlySummarySuite))
}

func (s *RecomputeMonthlySummarySuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = context.Background()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.refMonth = rm

	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.txRepo = mockInterfaces.NewTransactionRepository(s.T())
	s.invRepo = mockInterfaces.NewCardInvoiceRepository(s.T())
	s.summRepo = mockInterfaces.NewMonthlySummaryRepository(s.T())

	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.txRepo).Maybe()
	s.factory.EXPECT().CardInvoiceRepository(mock.Anything).Return(s.invRepo).Maybe()
	s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summRepo).Maybe()

	s.uow = uowMocks.NewUnitOfWorkEmpty(s.T())
	s.useCase = NewRecomputeMonthlySummary(s.factory, s.uow, fake.NewProvider())
}

func (s *RecomputeMonthlySummarySuite) TestExecute_Success() {
	s.txRepo.EXPECT().SumByMonthExcludingCredit(mock.Anything, s.userID, s.refMonth).Return(int64(10000), int64(5000), nil).Once()
	s.invRepo.EXPECT().SumByMonth(mock.Anything, s.userID, s.refMonth).Return(int64(3000), nil).Once()
	s.summRepo.EXPECT().Upsert(mock.Anything, s.userID, s.refMonth, int64(10000), int64(8000), mock.Anything).Return(nil).Once()

	err := s.useCase.Execute(s.ctx, RecomputeMonthlySummaryInput{
		UserID:   s.userID,
		RefMonth: s.refMonth,
	})
	s.Require().NoError(err)
}

func (s *RecomputeMonthlySummarySuite) TestExecute_SoftDeleteFiltered() {
	s.txRepo.EXPECT().SumByMonthExcludingCredit(mock.Anything, s.userID, s.refMonth).Return(int64(0), int64(0), nil).Once()
	s.invRepo.EXPECT().SumByMonth(mock.Anything, s.userID, s.refMonth).Return(int64(0), nil).Once()
	s.summRepo.EXPECT().Upsert(mock.Anything, s.userID, s.refMonth, int64(0), int64(0), mock.Anything).Return(nil).Once()

	err := s.useCase.Execute(s.ctx, RecomputeMonthlySummaryInput{
		UserID:   s.userID,
		RefMonth: s.refMonth,
	})
	s.Require().NoError(err)
}

func (s *RecomputeMonthlySummarySuite) TestExecute_SumByMonthError() {
	s.txRepo.EXPECT().SumByMonthExcludingCredit(mock.Anything, s.userID, s.refMonth).Return(int64(0), int64(0), errors.New("db error")).Once()

	err := s.useCase.Execute(s.ctx, RecomputeMonthlySummaryInput{
		UserID:   s.userID,
		RefMonth: s.refMonth,
	})
	s.Require().Error(err)
}
