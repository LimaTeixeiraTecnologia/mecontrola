package usecases

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	ifmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetCardInvoiceSuite struct {
	suite.Suite
	uc       *GetCardInvoice
	factory  *ifmocks.RepositoryFactory
	invoices *ifmocks.CardInvoiceRepository
}

func TestGetCardInvoiceSuite(t *testing.T) {
	suite.Run(t, new(GetCardInvoiceSuite))
}

func (s *GetCardInvoiceSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.invoices = ifmocks.NewCardInvoiceRepository(s.T())
	s.factory.On("CardInvoiceRepository", mock.Anything).Return(s.invoices).Maybe()
	uow := ucmocks.NewUnitOfWorkCardInvoiceOutput(s.T())
	s.uc = NewGetCardInvoice(s.factory, uow, fake.NewProvider())
}

func (s *GetCardInvoiceSuite) TestExecute_Unauthorized() {
	_, err := s.uc.Execute(context.Background(), uuid.New(), "2024-01")
	s.ErrorIs(err, ErrUsecaseUnauthorized)
}

func (s *GetCardInvoiceSuite) TestExecute_InvalidRefMonth() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	_, err := s.uc.Execute(ctx, uuid.New(), "invalid")
	s.Error(err)
}

func (s *GetCardInvoiceSuite) TestExecute_NotFound_ReturnsError() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	principal, _ := auth.FromContext(ctx)
	rm, _ := valueobjects.NewRefMonth("2024-01")
	s.invoices.On("GetByMonth", mock.Anything, principal.UserID, mock.Anything, rm).
		Return((*entities.CardInvoice)(nil), ([]*entities.CardInvoiceItem)(nil), nil)
	_, err := s.uc.Execute(ctx, uuid.New(), "2024-01")
	s.ErrorIs(err, ErrCardInvoiceNotFound)
}
