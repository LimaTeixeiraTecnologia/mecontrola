package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
)

type GetDaySummarySuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	factory *mockInterfaces.RepositoryFactory
	dayRepo *mockInterfaces.DayEntriesRepository
	uow     *uowMocks.UnitOfWorkDaySummary
	useCase *GetDaySummary
}

func TestGetDaySummarySuite(t *testing.T) {
	suite.Run(t, new(GetDaySummarySuite))
}

func (s *GetDaySummarySuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.dayRepo = mockInterfaces.NewDayEntriesRepository(s.T())
	s.factory.EXPECT().DayEntriesRepository(mock.Anything).Return(s.dayRepo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkDaySummary(s.T())
	s.useCase = NewGetDaySummary(s.factory, s.uow, fake.NewProvider())
}

func (s *GetDaySummarySuite) TestExecute_SumsIncomeAndOutcome() {
	now := time.Now()
	entries := []interfaces.MonthlyEntry{
		{Kind: "transaction", ID: uuid.NewString(), UserID: s.userID, RefMonth: "2026-07", AmountCents: 50000, Direction: "income", Description: "freelance", CreatedAt: now},
		{Kind: "transaction", ID: uuid.NewString(), UserID: s.userID, RefMonth: "2026-07", AmountCents: 4500, Direction: "outcome", Description: "farmácia", CreatedAt: now},
		{Kind: "transaction", ID: uuid.NewString(), UserID: s.userID, RefMonth: "2026-07", AmountCents: 3000, Direction: "outcome", Description: "uber", CreatedAt: now},
	}
	s.dayRepo.EXPECT().ListDayEntries(mock.Anything, s.userID, "2026-07-28").Return(entries, nil).Once()

	out, err := s.useCase.Execute(s.ctx, "2026-07-28")
	s.Require().NoError(err)
	s.Equal("2026-07-28", out.Day)
	s.Equal(int64(50000), out.IncomeCents)
	s.Equal(int64(7500), out.OutcomeCents)
	s.Equal(int64(42500), out.TotalCents)
	s.Len(out.Entries, 3)
}

func (s *GetDaySummarySuite) TestExecute_EmptyDay() {
	s.dayRepo.EXPECT().ListDayEntries(mock.Anything, s.userID, "2026-07-28").Return([]interfaces.MonthlyEntry{}, nil).Once()

	out, err := s.useCase.Execute(s.ctx, "2026-07-28")
	s.Require().NoError(err)
	s.Equal(int64(0), out.IncomeCents)
	s.Equal(int64(0), out.OutcomeCents)
	s.Equal(int64(0), out.TotalCents)
	s.NotNil(out.Entries)
	s.Empty(out.Entries)
}

func (s *GetDaySummarySuite) TestExecute_Unauthorized() {
	_, err := s.useCase.Execute(context.Background(), "2026-07-28")
	s.Require().Error(err)
}

func (s *GetDaySummarySuite) TestExecute_InvalidDay() {
	_, err := s.useCase.Execute(s.ctx, "not-a-day")
	s.Require().Error(err)
}
