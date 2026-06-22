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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/pagination"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type ListCardsSuite struct {
	suite.Suite
	obs      observability.Observability
	repoMock *ifacemocks.CardRepository
}

func TestListCards(t *testing.T) {
	suite.Run(t, new(ListCardsSuite))
}

func (s *ListCardsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.repoMock = ifacemocks.NewCardRepository(s.T())
}

func (s *ListCardsSuite) makeCard(userID uuid.UUID) entities.Card {
	name, _ := valueobjects.NewCardName("Card")
	nick, _ := valueobjects.NewNickname("Alias")
	cycle, _ := valueobjects.NewBillingCycle(10, 17)
	return entities.HydrateCard(uuid.New(), userID, name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *ListCardsSuite) TestExecute_HappyPath() {
	userID := uuid.New()
	cards := []entities.Card{s.makeCard(userID), s.makeCard(userID)}
	in := input.ListCards{UserID: userID, Cursor: "", Limit: 10}

	s.repoMock.EXPECT().ListByUser(mock.Anything, userID.String(), "", 10).Return(cards, "", nil).Once()

	sut := NewListCards(s.repoMock, s.obs)
	out, err := sut.Execute(context.Background(), in)

	s.Require().NoError(err)
	s.Len(out.Items, 2)
	s.Empty(out.NextCursor)
}

func (s *ListCardsSuite) TestExecute_WithCursor() {
	userID := uuid.New()
	cursor, err := pagination.C.Encode(time.Now().UTC(), uuid.New().String())
	s.Require().NoError(err)

	cards := []entities.Card{s.makeCard(userID)}
	nextCursor, encErr := pagination.C.Encode(time.Now().UTC().Add(-time.Hour), uuid.New().String())
	s.Require().NoError(encErr)

	in := input.ListCards{UserID: userID, Cursor: cursor, Limit: 5}

	s.repoMock.EXPECT().ListByUser(mock.Anything, userID.String(), cursor, 5).Return(cards, nextCursor, nil).Once()

	sut := NewListCards(s.repoMock, s.obs)
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

	sut := NewListCards(s.repoMock, s.obs)
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

	sut := NewListCards(s.repoMock, s.obs)
	_, err := sut.Execute(context.Background(), in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidCursor)
}
