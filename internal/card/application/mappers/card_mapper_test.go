package mappers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type CardMapperSuite struct {
	suite.Suite
}

func TestCardMapper(t *testing.T) {
	suite.Run(t, new(CardMapperSuite))
}

func (s *CardMapperSuite) makeCard(deleted *time.Time) entities.Card {
	name, err := valueobjects.NewCardName("Itaú Platinum")
	s.Require().NoError(err)
	nick, err := valueobjects.NewNickname("itau-pt")
	s.Require().NoError(err)
	cycle, err := valueobjects.NewBillingCycle(10, 17)
	s.Require().NoError(err)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 3, 3, 4, 5, 0, time.UTC)
	return entities.HydrateCard(id, userID, name, nick, cycle, createdAt, updatedAt, deleted)
}

func (s *CardMapperSuite) TestToCardOutput_PreservesAllFields() {
	card := s.makeCard(nil)
	out := mappers.ToCardOutput(card)

	s.Equal(card.ID.String(), out.ID)
	s.Equal(card.UserID.String(), out.UserID)
	s.Equal("Itaú Platinum", out.Name)
	s.Equal("itau-pt", out.Nickname)
	s.Equal(10, out.ClosingDay)
	s.Equal(17, out.DueDay)
	s.Equal(card.CreatedAt, out.CreatedAt)
	s.Equal(card.UpdatedAt, out.UpdatedAt)
	s.Nil(out.DeletedAt)
}

func (s *CardMapperSuite) TestToCardOutput_PropagatesDeletedAt() {
	deletedAt := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	card := s.makeCard(&deletedAt)
	out := mappers.ToCardOutput(card)

	s.Require().NotNil(out.DeletedAt)
	s.Equal(deletedAt, *out.DeletedAt)
}

func (s *CardMapperSuite) TestToCardListOutput_EmptySliceProducesEmptyItemsAndNilCursor() {
	out := mappers.ToCardListOutput(nil, "")
	s.NotNil(out.Items)
	s.Empty(out.Items)
	s.Nil(out.NextCursor)
}

func (s *CardMapperSuite) TestToCardListOutput_PopulatesItemsAndCursor() {
	cards := []entities.Card{s.makeCard(nil), s.makeCard(nil)}
	out := mappers.ToCardListOutput(cards, "next-token")

	s.Len(out.Items, 2)
	s.Require().NotNil(out.NextCursor)
	s.Equal("next-token", *out.NextCursor)
}

func (s *CardMapperSuite) TestToInvoiceOutput_FormatsInGivenTimezone() {
	tz, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)
	invoice := services.Invoice{
		ClosingDate: time.Date(2026, 1, 10, 3, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, 1, 17, 3, 0, 0, 0, time.UTC),
	}

	out := mappers.ToInvoiceOutput(invoice, tz)

	s.Equal("2026-01-10", out.ClosingDate)
	s.Equal("2026-01-17", out.DueDate)
}

func (s *CardMapperSuite) TestToInvoiceOutput_TimezoneAffectsLocalDay() {
	tz, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)
	invoice := services.Invoice{
		ClosingDate: time.Date(2026, 1, 10, 1, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, 1, 17, 1, 0, 0, 0, time.UTC),
	}

	out := mappers.ToInvoiceOutput(invoice, tz)

	s.Equal("2026-01-09", out.ClosingDate)
	s.Equal("2026-01-16", out.DueDate)
}
