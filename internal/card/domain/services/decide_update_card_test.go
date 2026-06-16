package services_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type UpdateCardDeciderSuite struct {
	suite.Suite
	decider  services.UpdateCardDecider
	existing entities.Card
}

func TestUpdateCardDecider(t *testing.T) {
	suite.Run(t, new(UpdateCardDeciderSuite))
}

func (s *UpdateCardDeciderSuite) SetupTest() {
	s.decider = services.NewUpdateCardDecider()
	name, err := valueobjects.NewCardName("OldName")
	s.Require().NoError(err)
	nick, err := valueobjects.NewNickname("OldNick")
	s.Require().NoError(err)
	cycle, err := valueobjects.NewBillingCycle(10, 17)
	s.Require().NoError(err)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	s.existing = entities.HydrateCard(id, userID, name, nick, cycle, 0, createdAt, updatedAt, nil)
}

func (s *UpdateCardDeciderSuite) ptrStr(v string) *string { return &v }
func (s *UpdateCardDeciderSuite) ptrInt(v int) *int       { return &v }

func (s *UpdateCardDeciderSuite) TestDecide_EmptyCommandKeepsFieldsButBumpsUpdatedAt() {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{}, now)
	s.Require().NoError(err)
	s.Equal(s.existing.ID, got.ID)
	s.Equal(s.existing.UserID, got.UserID)
	s.Equal(s.existing.Name, got.Name)
	s.Equal(s.existing.Nickname, got.Nickname)
	s.Equal(s.existing.Cycle, got.Cycle)
	s.True(s.existing.CreatedAt.Equal(got.CreatedAt))
	s.True(now.Equal(got.UpdatedAt))
	s.Equal(time.UTC, got.UpdatedAt.Location())
	s.Equal(s.existing.DeletedAt, got.DeletedAt)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_NameOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Name: s.ptrStr("NewName"),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal("NewName", got.Name.String())
	s.Equal(s.existing.Nickname, got.Nickname)
	s.Equal(s.existing.Cycle, got.Cycle)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_NicknameOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Nickname: s.ptrStr("NewNick"),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal("NewNick", got.Nickname.String())
	s.Equal(s.existing.Name, got.Name)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_ClosingDayOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		ClosingDay: s.ptrInt(5),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal(5, got.Cycle.ClosingDay)
	s.Equal(s.existing.Cycle.DueDay, got.Cycle.DueDay)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_DueDayOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		DueDay: s.ptrInt(28),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal(s.existing.Cycle.ClosingDay, got.Cycle.ClosingDay)
	s.Equal(28, got.Cycle.DueDay)
}

func (s *UpdateCardDeciderSuite) TestDecide_FullUpdate() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Name:       s.ptrStr("Full"),
		Nickname:   s.ptrStr("FullNick"),
		ClosingDay: s.ptrInt(3),
		DueDay:     s.ptrInt(10),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal("Full", got.Name.String())
	s.Equal("FullNick", got.Nickname.String())
	s.Equal(3, got.Cycle.ClosingDay)
	s.Equal(10, got.Cycle.DueDay)
}

func (s *UpdateCardDeciderSuite) TestDecide_InvalidNameReturnsSentinel() {
	_, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Name: s.ptrStr(""),
	}, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrInvalidCardName))
}

func (s *UpdateCardDeciderSuite) TestDecide_InvalidNicknameReturnsSentinel() {
	_, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Nickname: s.ptrStr(""),
	}, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrInvalidNickname))
}

func (s *UpdateCardDeciderSuite) TestDecide_InvalidClosingDayReturnsSentinel() {
	_, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		ClosingDay: s.ptrInt(0),
	}, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrInvalidClosingDay))
}

func (s *UpdateCardDeciderSuite) TestDecide_InvalidDueDayReturnsSentinel() {
	_, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		DueDay: s.ptrInt(32),
	}, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrInvalidDueDay))
}

func (s *UpdateCardDeciderSuite) TestDecide_NameValidatedBeforeNickname() {
	_, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Name:     s.ptrStr(""),
		Nickname: s.ptrStr(""),
	}, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrInvalidCardName))
	s.False(errors.Is(err, domain.ErrInvalidNickname))
}

func (s *UpdateCardDeciderSuite) TestDecide_PreservesImmutableFieldsAndBumpsUpdatedAt() {
	now := time.Date(2026, 6, 15, 9, 30, 0, 0, time.UTC)
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Name: s.ptrStr("Diff"),
	}, now)
	s.Require().NoError(err)
	s.Equal(s.existing.ID, got.ID)
	s.Equal(s.existing.UserID, got.UserID)
	s.True(s.existing.CreatedAt.Equal(got.CreatedAt))
	s.True(now.Equal(got.UpdatedAt))
	s.Equal(s.existing.DeletedAt, got.DeletedAt)
}
