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

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type LoadOnboardingTurnsSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	userID      uuid.UUID
}

func TestLoadOnboardingTurnsSuite(t *testing.T) {
	suite.Run(t, new(LoadOnboardingTurnsSuite))
}

func (s *LoadOnboardingTurnsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.userID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
}

func (s *LoadOnboardingTurnsSuite) TestLoadTurns() {
	type args struct {
		userID uuid.UUID
	}
	type dependencies struct {
		sessionRepo *mocks.OnboardingSessionRepository
	}

	now := time.Now().UTC()
	turns := []entities.OnboardingTurn{
		{Role: "user", Text: "olá", OccurredAt: now},
		{Role: "assistant", Text: "oi", OccurredAt: now},
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result []entities.OnboardingTurn, err error)
	}{
		{
			name: "deve retornar os turnos recentes",
			args: args{userID: s.userID},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{RecentTurns: turns},
						now,
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result []entities.OnboardingTurn, err error) {
				s.NoError(err)
				s.Len(result, 2)
				s.Equal("user", result[0].Role)
				s.Equal("olá", result[0].Text)
			},
		},
		{
			name: "deve retornar slice vazio quando sem turnos",
			args: args{userID: s.userID},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{},
						now,
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result []entities.OnboardingTurn, err error) {
				s.NoError(err)
				s.NotNil(result)
				s.Len(result, 0)
			},
		},
		{
			name: "deve retornar erro quando sessão não encontrada",
			args: args{userID: s.userID},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
						Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result []entities.OnboardingTurn, err error) {
				s.Error(err)
				s.ErrorIs(err, appinterfaces.ErrOnboardingSessionNotFound)
				s.Nil(result)
			},
		},
		{
			name:         "deve retornar erro quando user id é nulo",
			args:         args{userID: uuid.Nil},
			dependencies: dependencies{sessionRepo: s.sessionRepo},
			expect: func(result []entities.OnboardingTurn, err error) {
				s.Error(err)
				s.Nil(result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewLoadOnboardingTurns(scenario.dependencies.sessionRepo, s.obs)
			result, err := uc.Execute(s.ctx, scenario.args.userID)
			scenario.expect(result, err)
		})
	}
}
