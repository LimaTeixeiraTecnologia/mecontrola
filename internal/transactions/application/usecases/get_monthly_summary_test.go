package usecases

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetMonthlySummarySuite struct {
	suite.Suite
	ctx      context.Context
	userID   uuid.UUID
	factory  *mockInterfaces.RepositoryFactory
	summRepo *mockInterfaces.MonthlySummaryRepository
	uow      *uowMocks.UnitOfWorkMonthlySummary
	useCase  *GetMonthlySummary
}

func TestGetMonthlySummarySuite(t *testing.T) {
	suite.Run(t, new(GetMonthlySummarySuite))
}

func (s *GetMonthlySummarySuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.summRepo = mockInterfaces.NewMonthlySummaryRepository(s.T())
	s.factory.EXPECT().MonthlySummaryRepository(mock.Anything).Return(s.summRepo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkMonthlySummary(s.T())
	s.useCase = NewGetMonthlySummary(s.factory, s.uow, fake.NewProvider())
}

func (s *GetMonthlySummarySuite) TestExecute_ReturnsExistingSummary() {
	rm, _ := valueobjects.NewRefMonth("2026-06")
	summary := entities.NewMonthlySummary(s.userID, rm, 10000, 5000, 1, nil)
	s.summRepo.EXPECT().Get(mock.Anything, s.userID, rm).Return(&summary, nil).Once()

	out, err := s.useCase.Execute(s.ctx, "2026-06")
	s.Require().NoError(err)
	s.Equal(int64(10000), out.IncomeCents)
	s.Equal(int64(5000), out.OutcomeCents)
}

func (s *GetMonthlySummarySuite) TestExecute_ReturnsZerosWhenNoProjection() {
	rm, _ := valueobjects.NewRefMonth("2026-07")
	s.summRepo.EXPECT().Get(mock.Anything, s.userID, rm).Return(nil, nil).Once()

	out, err := s.useCase.Execute(s.ctx, "2026-07")
	s.Require().NoError(err)
	s.Equal(int64(0), out.IncomeCents)
	s.Equal(int64(0), out.OutcomeCents)
	s.Nil(out.UpdatedAt)
}

func (s *GetMonthlySummarySuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, "2026-06")
	s.Require().Error(err)
}

func (s *GetMonthlySummarySuite) TestExecute_InvalidRefMonth() {
	_, err := s.useCase.Execute(s.ctx, "not-a-month")
	s.Require().Error(err)
}
