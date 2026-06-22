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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ListMonthlyEntriesSuite struct {
	suite.Suite
	ctx      context.Context
	userID   uuid.UUID
	factory  *mockInterfaces.RepositoryFactory
	summRepo *mockInterfaces.MonthlySummaryRepository
	uow      *uowMocks.UnitOfWorkMonthlyEntries
	useCase  *ListMonthlyEntries
}

func TestListMonthlyEntriesSuite(t *testing.T) {
	suite.Run(t, new(ListMonthlyEntriesSuite))
}

func (s *ListMonthlyEntriesSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.summRepo = mockInterfaces.NewMonthlySummaryRepository(s.T())
	s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summRepo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkMonthlyEntries(s.T())
	s.useCase = NewListMonthlyEntries(s.factory, s.uow, fake.NewProvider())
}

func (s *ListMonthlyEntriesSuite) TestExecute_ReturnsCombinedEntries() {
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	entries := []interfaces.MonthlyEntry{
		{Kind: "transaction", ID: uuid.New().String(), UserID: s.userID, RefMonth: "2026-06", AmountCents: 5000, Direction: "outcome", CreatedAt: now},
		{Kind: "card_invoice_item", ID: uuid.New().String(), UserID: s.userID, RefMonth: "2026-06", AmountCents: 3000, Direction: "outcome", CreatedAt: now.Add(-time.Second)},
	}
	s.summRepo.EXPECT().ListEntries(mock.Anything, s.userID, rm, interfaces.Cursor{}, 50).
		Return(entries, interfaces.Cursor{}, nil).Once()

	page, err := s.useCase.Execute(s.ctx, "2026-06", "", 0)
	s.Require().NoError(err)
	s.Len(page.Items, 2)
	s.False(page.HasMore)
}

func (s *ListMonthlyEntriesSuite) TestExecute_WithCursorPreservesOrder() {
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	entries := []interfaces.MonthlyEntry{
		{Kind: "transaction", ID: uuid.New().String(), UserID: s.userID, RefMonth: "2026-06", AmountCents: 1000, Direction: "income", CreatedAt: now},
	}
	nextCursor := interfaces.Cursor{Value: "abc123"}
	s.summRepo.EXPECT().ListEntries(mock.Anything, s.userID, rm, interfaces.Cursor{Value: "prev"}, 10).
		Return(entries, nextCursor, nil).Once()

	page, err := s.useCase.Execute(s.ctx, "2026-06", "prev", 10)
	s.Require().NoError(err)
	s.Len(page.Items, 1)
	s.Equal("abc123", page.NextCursor)
	s.True(page.HasMore)
}

func (s *ListMonthlyEntriesSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, "2026-06", "", 0)
	s.Require().Error(err)
}
