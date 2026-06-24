package usecases

import (
	"context"
	"errors"
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

type MarkWelcomeSentSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	userID      uuid.UUID
}

func TestMarkWelcomeSentSuite(t *testing.T) {
	suite.Run(t, new(MarkWelcomeSentSuite))
}

func (s *MarkWelcomeSentSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
}

func (s *MarkWelcomeSentSuite) TestMarkWelcomeSent() {
	type args struct {
		input MarkWelcomeSentInput
	}
	type dependencies struct {
		sessionRepo *mocks.OnboardingSessionRepository
	}

	now := time.Now().UTC()
	welcomeAt := now.Add(-1 * time.Minute)

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result MarkWelcomeSentResult, err error)
	}{
		{
			name: "deve marcar welcome enviado com sucesso",
			args: args{input: MarkWelcomeSentInput{UserID: s.userID}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{},
						now,
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
						return sess.Payload().WelcomeSentAt != nil
					})).Return(nil).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result MarkWelcomeSentResult, err error) {
				s.NoError(err)
				s.False(result.AlreadySent)
			},
		},
		{
			name: "deve retornar alreadySent=true quando welcome já foi enviado (idempotente)",
			args: args{input: MarkWelcomeSentInput{UserID: s.userID}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{WelcomeSentAt: &welcomeAt},
						now,
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result MarkWelcomeSentResult, err error) {
				s.NoError(err)
				s.True(result.AlreadySent)
			},
		},
		{
			name:         "deve retornar erro quando user id é nulo",
			args:         args{input: MarkWelcomeSentInput{UserID: uuid.Nil}},
			dependencies: dependencies{sessionRepo: s.sessionRepo},
			expect: func(result MarkWelcomeSentResult, err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro quando sessão não encontrada",
			args: args{input: MarkWelcomeSentInput{UserID: s.userID}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
						Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result MarkWelcomeSentResult, err error) {
				s.Error(err)
				s.ErrorIs(err, appinterfaces.ErrOnboardingSessionNotFound)
			},
		},
		{
			name: "deve retornar erro de infraestrutura em upsert",
			args: args{input: MarkWelcomeSentInput{UserID: s.userID}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{},
						now,
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.AnythingOfType("entities.OnboardingSession")).
						Return(errors.New("db error")).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(result MarkWelcomeSentResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewMarkWelcomeSent(&onboardingUoWStub{}, s.factory, s.obs)
			result, err := uc.Execute(s.ctx, scenario.args.input)
			scenario.expect(result, err)
		})
	}
}
