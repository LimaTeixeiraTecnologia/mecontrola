package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type GetCardForUserSuite struct {
	suite.Suite
	mgr         *ucmocks.FakeManager
	factoryMock *ifacemocks.RepositoryFactory
	repoMock    *ifacemocks.CardRepository
}

func TestGetCardForUser(t *testing.T) {
	suite.Run(t, new(GetCardForUserSuite))
}

func (s *GetCardForUserSuite) SetupTest() {
	s.mgr = ucmocks.NewFakeManager()
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
}

func (s *GetCardForUserSuite) activeCard() entities.Card {
	name, _ := valueobjects.NewCardName("Nubank Gold")
	nick, _ := valueobjects.NewNickname("Nu")
	cycle, _ := valueobjects.NewBillingCycle(10, 17)
	return entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *GetCardForUserSuite) TestExecute_HappyPath() {
	card := s.activeCard()

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := usecases.NewGetCardForUser(s.factoryMock, s.mgr, noop.NewProvider())
	got, err := sut.Execute(context.Background(), card.ID, card.UserID)

	s.Require().NoError(err)
	s.Equal(card.Cycle.ClosingDay, got.ClosingDay)
	s.Equal(card.Cycle.DueDay, got.DueDay)
}

func (s *GetCardForUserSuite) TestExecute_CardNotFound() {
	cardID := uuid.New()
	userID := uuid.New()

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, cardID.String(), userID.String()).Return(entities.Card{}, domain.ErrCardNotFound).Once()

	sut := usecases.NewGetCardForUser(s.factoryMock, s.mgr, noop.NewProvider())
	_, err := sut.Execute(context.Background(), cardID, userID)

	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrCardNotFound)
}

func (s *GetCardForUserSuite) TestExecute_OwnershipMismatch_SoftDeleted() {
	name, _ := valueobjects.NewCardName("Deleted Card")
	nick, _ := valueobjects.NewNickname("Del")
	cycle, _ := valueobjects.NewBillingCycle(5, 12)
	deletedAt := time.Now().UTC()
	card := entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, time.Now().UTC(), time.Now().UTC(), &deletedAt)

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := usecases.NewGetCardForUser(s.factoryMock, s.mgr, noop.NewProvider())
	_, err := sut.Execute(context.Background(), card.ID, card.UserID)

	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrCardNotFound)
}
