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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/pagination"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type ListCardsSuite struct {
	suite.Suite
	mgr         *ucmocks.FakeManager
	factoryMock *ifacemocks.RepositoryFactory
	repoMock    *ifacemocks.CardRepository
}

func TestListCards(t *testing.T) {
	suite.Run(t, new(ListCardsSuite))
}

func (s *ListCardsSuite) SetupTest() {
	s.mgr = ucmocks.NewFakeManager()
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
}

func (s *ListCardsSuite) makeCard(userID uuid.UUID) entities.Card {
	name, _ := valueobjects.NewCardName("Card")
	nick, _ := valueobjects.NewNickname("Alias")
	cycle, _ := valueobjects.NewBillingCycle(10, 17)
	return entities.HydrateCard(uuid.New(), userID, name, nick, cycle, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *ListCardsSuite) TestExecute_HappyPath() {
	userID := uuid.New()
	cards := []entities.Card{s.makeCard(userID), s.makeCard(userID)}
	in := input.ListCards{UserID: userID, Cursor: "", Limit: 10}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().ListByUser(mock.Anything, userID.String(), "", 10).Return(cards, "", nil).Once()

	sut := usecases.NewListCards(s.factoryMock, s.mgr, noop.NewProvider())
	out, err := sut.Execute(context.Background(), in)

	s.Require().NoError(err)
	s.Len(out.Items, 2)
	s.Empty(out.NextCursor)
}

func (s *ListCardsSuite) TestExecute_WithCursor() {
	userID := uuid.New()
	cursor, err := pagination.Encode(time.Now().UTC(), uuid.New().String())
	s.Require().NoError(err)

	cards := []entities.Card{s.makeCard(userID)}
	nextCursor, encErr := pagination.Encode(time.Now().UTC().Add(-time.Hour), uuid.New().String())
	s.Require().NoError(encErr)

	in := input.ListCards{UserID: userID, Cursor: cursor, Limit: 5}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().ListByUser(mock.Anything, userID.String(), cursor, 5).Return(cards, nextCursor, nil).Once()

	sut := usecases.NewListCards(s.factoryMock, s.mgr, noop.NewProvider())
	out, err := sut.Execute(context.Background(), in)

	s.Require().NoError(err)
	s.Len(out.Items, 1)
	s.Require().NotNil(out.NextCursor)
	s.Equal(nextCursor, *out.NextCursor)
}

func (s *ListCardsSuite) TestExecute_InvalidCursor_Base64() {
	in := input.ListCards{
		UserID: uuid.New(),
		Cursor: "not-valid-base64!@#",
		Limit:  10,
	}

	sut := usecases.NewListCards(s.factoryMock, s.mgr, noop.NewProvider())
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidCursor)
}

func (s *ListCardsSuite) TestExecute_InvalidCursor_EmptyJSON() {
	import64 := "e30="
	in := input.ListCards{
		UserID: uuid.New(),
		Cursor: import64,
		Limit:  10,
	}

	sut := usecases.NewListCards(s.factoryMock, s.mgr, noop.NewProvider())
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidCursor)
}
