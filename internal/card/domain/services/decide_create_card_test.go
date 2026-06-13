package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type CreateCardDeciderSuite struct {
	suite.Suite
	decider services.CreateCardDecider
}

func TestCreateCardDecider(t *testing.T) {
	suite.Run(t, new(CreateCardDeciderSuite))
}

func (s *CreateCardDeciderSuite) SetupTest() {
	s.decider = services.NewCreateCardDecider()
}

func (s *CreateCardDeciderSuite) buildCommand() (services.CreateCardCommand, uuid.UUID) {
	name, err := valueobjects.NewCardName("Visa Black")
	s.Require().NoError(err)
	nick, err := valueobjects.NewNickname("Visa")
	s.Require().NoError(err)
	cycle, err := valueobjects.NewBillingCycle(10, 17)
	s.Require().NoError(err)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	return services.CreateCardCommand{
		UserID:   userID,
		Name:     name,
		Nickname: nick,
		Cycle:    cycle,
	}, userID
}

func (s *CreateCardDeciderSuite) TestDecide_AssemblesCardWithExplicitIDAndTimestamps() {
	cmd, userID := s.buildCommand()
	cardID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

	got := s.decider.Decide(cmd, cardID, now)

	s.Equal(cardID, got.ID)
	s.Equal(userID, got.UserID)
	s.Equal(cmd.Name, got.Name)
	s.Equal(cmd.Nickname, got.Nickname)
	s.Equal(cmd.Cycle, got.Cycle)
	s.Equal(now, got.CreatedAt)
	s.Equal(now, got.UpdatedAt)
	s.Nil(got.DeletedAt)
}

func (s *CreateCardDeciderSuite) TestDecide_NormalizesNowToUTC() {
	cmd, _ := s.buildCommand()
	cardID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)
	nowSP := time.Date(2026, 6, 12, 9, 0, 0, 0, loc)

	got := s.decider.Decide(cmd, cardID, nowSP)

	s.Equal(time.UTC, got.CreatedAt.Location())
	s.Equal(time.UTC, got.UpdatedAt.Location())
	s.True(got.CreatedAt.Equal(nowSP))
}

func (s *CreateCardDeciderSuite) TestDecide_IsDeterministic() {
	cmd, _ := s.buildCommand()
	cardID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

	a := s.decider.Decide(cmd, cardID, now)
	b := s.decider.Decide(cmd, cardID, now)

	s.Equal(a, b)
}
