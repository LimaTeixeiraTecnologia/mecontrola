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
	nick, err := valueobjects.NewNickname("OldNick")
	s.Require().NoError(err)
	bank, err := valueobjects.NewBankCode("Nubank")
	s.Require().NoError(err)
	cycle, err := valueobjects.NewBillingCycle(13, 20)
	s.Require().NoError(err)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	s.existing = entities.HydrateCard(id, userID, nick, bank, cycle, createdAt, updatedAt, nil)
}

func (s *UpdateCardDeciderSuite) ptrStr(v string) *string { return &v }

func (s *UpdateCardDeciderSuite) ptrCycle(closing, due int) *valueobjects.BillingCycle {
	c, err := valueobjects.NewBillingCycle(closing, due)
	s.Require().NoError(err)
	return &c
}

func (s *UpdateCardDeciderSuite) ptrBank(v string) *valueobjects.BankCode {
	b, err := valueobjects.NewBankCode(v)
	s.Require().NoError(err)
	return &b
}

func (s *UpdateCardDeciderSuite) TestDecide_EmptyCommandKeepsFieldsButBumpsUpdatedAt() {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{}, now)
	s.Require().NoError(err)
	s.Equal(s.existing.ID, got.ID)
	s.Equal(s.existing.UserID, got.UserID)
	s.Equal(s.existing.Nickname, got.Nickname)
	s.Equal(s.existing.Bank, got.Bank)
	s.Equal(s.existing.Cycle, got.Cycle)
	s.True(s.existing.CreatedAt.Equal(got.CreatedAt))
	s.True(now.Equal(got.UpdatedAt))
	s.Equal(time.UTC, got.UpdatedAt.Location())
	s.Equal(s.existing.DeletedAt, got.DeletedAt)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_NicknameOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Nickname: s.ptrStr("NewNick"),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal("NewNick", got.Nickname.String())
	s.Equal(s.existing.Bank, got.Bank)
	s.Equal(s.existing.Cycle, got.Cycle)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_BankOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Bank: s.ptrBank("Itaú"),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal("Itaú", got.Bank.String())
	s.Equal("itau", got.Bank.LookupKey())
	s.Equal(s.existing.Nickname, got.Nickname)
	s.Equal(s.existing.Cycle, got.Cycle)
}

func (s *UpdateCardDeciderSuite) TestDecide_PartialUpdate_CycleOnly() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Cycle: s.ptrCycle(2, 10),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal(2, got.Cycle.ClosingDay)
	s.Equal(10, got.Cycle.DueDay)
	s.Equal(s.existing.Nickname, got.Nickname)
	s.Equal(s.existing.Bank, got.Bank)
}

func (s *UpdateCardDeciderSuite) TestDecide_FullUpdate() {
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Nickname: s.ptrStr("FullNick"),
		Bank:     s.ptrBank("Bradesco"),
		Cycle:    s.ptrCycle(3, 10),
	}, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal("FullNick", got.Nickname.String())
	s.Equal("Bradesco", got.Bank.String())
	s.Equal(3, got.Cycle.ClosingDay)
	s.Equal(10, got.Cycle.DueDay)
}

func (s *UpdateCardDeciderSuite) TestDecide_InvalidNicknameReturnsSentinel() {
	_, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Nickname: s.ptrStr(""),
	}, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, domain.ErrInvalidNickname))
}

func (s *UpdateCardDeciderSuite) TestDecide_PreservesImmutableFieldsAndBumpsUpdatedAt() {
	now := time.Date(2026, 6, 15, 9, 30, 0, 0, time.UTC)
	got, err := s.decider.Decide(s.existing, services.UpdateCardCommand{
		Nickname: s.ptrStr("Diff"),
	}, now)
	s.Require().NoError(err)
	s.Equal(s.existing.ID, got.ID)
	s.Equal(s.existing.UserID, got.UserID)
	s.True(s.existing.CreatedAt.Equal(got.CreatedAt))
	s.True(now.Equal(got.UpdatedAt))
	s.Equal(s.existing.DeletedAt, got.DeletedAt)
}
