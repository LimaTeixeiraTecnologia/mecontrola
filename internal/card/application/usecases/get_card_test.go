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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type GetCardSuite struct {
	suite.Suite
	obs      observability.Observability
	repoMock *ifacemocks.CardRepository
}

func TestGetCard(t *testing.T) {
	suite.Run(t, new(GetCardSuite))
}

func (s *GetCardSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.repoMock = ifacemocks.NewCardRepository(s.T())
}

func (s *GetCardSuite) activeCard() entities.Card {
	name, _ := valueobjects.NewCardName("Test Card")
	nick, _ := valueobjects.NewNickname("TestNick")
	cycle, _ := valueobjects.NewBillingCycle(15, 22)
	return entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *GetCardSuite) TestExecute_HappyPath() {
	card := s.activeCard()
	in := input.GetCard{ID: card.ID, UserID: card.UserID}

	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := NewGetCard(s.repoMock, s.obs)
	out, err := sut.Execute(context.Background(), in)

	s.Require().NoError(err)
	s.Equal(card.ID.String(), out.ID)
}

func (s *GetCardSuite) TestExecute_NotFound() {
	in := input.GetCard{ID: uuid.New(), UserID: uuid.New()}

	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, in.ID.String(), in.UserID.String()).Return(entities.Card{}, domain.ErrCardNotFound).Once()

	sut := NewGetCard(s.repoMock, s.obs)
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}

func (s *GetCardSuite) TestExecute_SoftDeletedReturnsNotFound() {
	name, _ := valueobjects.NewCardName("Deleted Card")
	nick, _ := valueobjects.NewNickname("Del")
	cycle, _ := valueobjects.NewBillingCycle(5, 12)
	deletedAt := time.Now().UTC()
	card := entities.HydrateCard(uuid.New(), uuid.New(), name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), &deletedAt)
	in := input.GetCard{ID: card.ID, UserID: card.UserID}

	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, card.ID.String(), card.UserID.String()).Return(card, nil).Once()

	sut := NewGetCard(s.repoMock, s.obs)
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}
