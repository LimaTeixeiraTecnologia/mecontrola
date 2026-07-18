package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type ResolveCardByNicknameSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	repoMock *mocks.CardRepository
	userID   uuid.UUID
}

func TestResolveCardByNicknameSuite(t *testing.T) {
	suite.Run(t, new(ResolveCardByNicknameSuite))
}

func (s *ResolveCardByNicknameSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repoMock = mocks.NewCardRepository(s.T())
	s.userID = uuid.New()
}

func (s *ResolveCardByNicknameSuite) buildCard(nickname string) entities.Card {
	nick, err := valueobjects.NewNickname(nickname)
	s.Require().NoError(err)
	bank, err := valueobjects.NewBankCode(nickname)
	s.Require().NoError(err)
	cycle, err := valueobjects.NewBillingCycle(3, 10)
	s.Require().NoError(err)
	return entities.NewCard(entities.NewCardInput{UserID: s.userID, Nickname: nick, Bank: bank, Cycle: cycle})
}

func (s *ResolveCardByNicknameSuite) TestExecute() {
	type args struct {
		nickname string
	}
	type dependencies struct {
		repoMock *mocks.CardRepository
	}

	xpCard := s.buildCard("XP Investimentos")
	nubankCard := s.buildCard("Roxinho")

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out output.Card, err error)
	}{
		{
			name: "deve resolver por match exato",
			args: args{nickname: "XP Investimentos"},
			dependencies: dependencies{
				repoMock: func() *mocks.CardRepository {
					s.repoMock.EXPECT().
						ListByUser(mock.Anything, s.userID.String(), "", mock.AnythingOfType("int")).
						Return([]entities.Card{nubankCard, xpCard}, "", nil).
						Once()
					return s.repoMock
				}(),
			},
			expect: func(out output.Card, err error) {
				s.NoError(err)
				s.Equal(xpCard.ID.String(), out.ID)
			},
		},
		{
			name: "deve resolver por prefixo unico quando nao ha exato",
			args: args{nickname: "XP"},
			dependencies: dependencies{
				repoMock: func() *mocks.CardRepository {
					s.repoMock.EXPECT().
						ListByUser(mock.Anything, s.userID.String(), "", mock.AnythingOfType("int")).
						Return([]entities.Card{nubankCard, xpCard}, "", nil).
						Once()
					return s.repoMock
				}(),
			},
			expect: func(out output.Card, err error) {
				s.NoError(err)
				s.Equal(xpCard.ID.String(), out.ID)
			},
		},
		{
			name: "deve retornar not found quando match parcial e ambiguo",
			args: args{nickname: "in"},
			dependencies: dependencies{
				repoMock: func() *mocks.CardRepository {
					s.repoMock.EXPECT().
						ListByUser(mock.Anything, s.userID.String(), "", mock.AnythingOfType("int")).
						Return([]entities.Card{s.buildCard("Inter"), s.buildCard("XP Investimentos")}, "", nil).
						Once()
					return s.repoMock
				}(),
			},
			expect: func(out output.Card, err error) {
				s.ErrorIs(err, domain.ErrCardNotFound)
			},
		},
		{
			name: "nao aplica match parcial para termo de um caractere",
			args: args{nickname: "X"},
			dependencies: dependencies{
				repoMock: func() *mocks.CardRepository {
					s.repoMock.EXPECT().
						ListByUser(mock.Anything, s.userID.String(), "", mock.AnythingOfType("int")).
						Return([]entities.Card{xpCard}, "", nil).
						Once()
					return s.repoMock
				}(),
			},
			expect: func(out output.Card, err error) {
				s.ErrorIs(err, domain.ErrCardNotFound)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewResolveCardByNickname(scenario.dependencies.repoMock, s.obs)
			out, err := uc.Execute(s.ctx, input.ResolveCardByNickname{UserID: s.userID, Nickname: scenario.args.nickname})
			scenario.expect(out, err)
		})
	}
}
