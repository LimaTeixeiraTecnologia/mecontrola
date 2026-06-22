package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type GetCardForUserSuite struct {
	suite.Suite
	obs      observability.Observability
	repoMock *ifacemocks.CardRepository
}

func TestGetCardForUser(t *testing.T) {
	suite.Run(t, new(GetCardForUserSuite))
}

func (s *GetCardForUserSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.repoMock = ifacemocks.NewCardRepository(s.T())
}

func (s *GetCardForUserSuite) activeCard() entities.Card {
	name, _ := valueobjects.NewCardName("Nubank Gold")
	nick, _ := valueobjects.NewNickname("Nu")
	cycle, _ := valueobjects.NewBillingCycle(10, 17)
	return entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *GetCardForUserSuite) TestExecute_HappyPath() {
	card := s.activeCard()

	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := NewGetCardForUser(s.repoMock, s.obs)
	got, err := sut.Execute(context.Background(), card.ID, card.UserID)

	s.Require().NoError(err)
	s.Equal(card.Cycle.ClosingDay, got.ClosingDay)
	s.Equal(card.Cycle.DueDay, got.DueDay)
}

func (s *GetCardForUserSuite) TestExecute_CardNotFound() {
	cardID := uuid.New()
	userID := uuid.New()

	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, cardID.String(), userID.String()).Return(entities.Card{}, domain.ErrCardNotFound).Once()

	sut := NewGetCardForUser(s.repoMock, s.obs)
	_, err := sut.Execute(context.Background(), cardID, userID)

	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrCardNotFound)
}

func (s *GetCardForUserSuite) TestExecute_OwnershipMismatch_SoftDeleted() {
	name, _ := valueobjects.NewCardName("Deleted Card")
	nick, _ := valueobjects.NewNickname("Del")
	cycle, _ := valueobjects.NewBillingCycle(5, 12)
	deletedAt := time.Now().UTC()
	card := entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), &deletedAt)

	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := NewGetCardForUser(s.repoMock, s.obs)
	_, err := sut.Execute(context.Background(), card.ID, card.UserID)

	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrCardNotFound)
}
