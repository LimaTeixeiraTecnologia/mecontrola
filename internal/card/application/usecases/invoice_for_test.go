package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type InvoiceForSuite struct {
	suite.Suite
	mgr         *ucmocks.FakeManager
	factoryMock *ifacemocks.RepositoryFactory
	repoMock    *ifacemocks.CardRepository
	loc         *time.Location
}

func TestInvoiceFor(t *testing.T) {
	suite.Run(t, new(InvoiceForSuite))
}

func (s *InvoiceForSuite) SetupTest() {
	s.mgr = ucmocks.NewFakeManager()
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)
	s.loc = loc
}

func (s *InvoiceForSuite) activeCard() entities.Card {
	name, _ := valueobjects.NewCardName("Visa Gold")
	nick, _ := valueobjects.NewNickname("Visa")
	cycle, _ := valueobjects.NewBillingCycle(15, 22)
	return entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *InvoiceForSuite) TestExecute_HappyPath() {
	card := s.activeCard()
	purchase := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	in := input.InvoiceFor{CardID: card.ID, UserID: card.UserID, Purchase: purchase}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := usecases.NewInvoiceFor(s.factoryMock, s.mgr, s.loc, noop.NewProvider())
	out, err := sut.Execute(context.Background(), in)

	s.Require().NoError(err)
	s.NotEmpty(out.ClosingDate)
	s.NotEmpty(out.DueDate)
}

func (s *InvoiceForSuite) TestExecute_CardNotFound() {
	in := input.InvoiceFor{CardID: uuid.New(), UserID: uuid.New(), Purchase: time.Now()}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, in.CardID.String(), in.UserID.String()).Return(entities.Card{}, domain.ErrCardNotFound).Once()

	sut := usecases.NewInvoiceFor(s.factoryMock, s.mgr, s.loc, noop.NewProvider())
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}

func (s *InvoiceForSuite) TestExecute_SoftDeletedCardReturnsNotFound() {
	name, _ := valueobjects.NewCardName("Deleted")
	nick, _ := valueobjects.NewNickname("Del")
	cycle, _ := valueobjects.NewBillingCycle(5, 12)
	deletedAt := time.Now().UTC()
	card := entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), &deletedAt)

	in := input.InvoiceFor{CardID: card.ID, UserID: card.UserID, Purchase: time.Now()}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := usecases.NewInvoiceFor(s.factoryMock, s.mgr, s.loc, noop.NewProvider())
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}

func (s *InvoiceForSuite) TestExecute_ZeroPurchaseDate() {
	in := input.InvoiceFor{CardID: uuid.New(), UserID: uuid.New(), Purchase: time.Time{}}

	sut := usecases.NewInvoiceFor(s.factoryMock, s.mgr, s.loc, noop.NewProvider())
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidPurchaseDate)
}
