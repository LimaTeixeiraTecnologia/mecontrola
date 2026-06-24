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

type AppendOnboardingTurnSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	userID      uuid.UUID
}

func TestAppendOnboardingTurnSuite(t *testing.T) {
	suite.Run(t, new(AppendOnboardingTurnSuite))
}

func (s *AppendOnboardingTurnSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
}

func (s *AppendOnboardingTurnSuite) TestAppendTurnSuccessful() {
	type args struct {
		input AppendOnboardingTurnInput
	}
	type dependencies struct {
		sessionRepo *mocks.OnboardingSessionRepository
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve appendar um par de turnos com sucesso",
			args: args{input: AppendOnboardingTurnInput{
				UserID:         s.userID,
				UserMessage:    "oi",
				AssistantReply: "olá",
			}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{},
						time.Now().UTC(),
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
						return len(sess.Payload().RecentTurns) == 2
					})).Return(nil).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando sessão não encontrada",
			args: args{input: AppendOnboardingTurnInput{
				UserID:         s.userID,
				UserMessage:    "oi",
				AssistantReply: "olá",
			}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
						Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, appinterfaces.ErrOnboardingSessionNotFound)
			},
		},
		{
			name: "deve retornar erro quando user id é nulo",
			args: args{input: AppendOnboardingTurnInput{
				UserID: uuid.Nil,
			}},
			dependencies: dependencies{sessionRepo: s.sessionRepo},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve respeitar o bound de 3 pares ao appendar",
			args: args{input: AppendOnboardingTurnInput{
				UserID:         s.userID,
				UserMessage:    "msg4",
				AssistantReply: "reply4",
			}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					existingTurns := []entities.OnboardingTurn{
						{Role: "user", Text: "msg1"},
						{Role: "assistant", Text: "reply1"},
						{Role: "user", Text: "msg2"},
						{Role: "assistant", Text: "reply2"},
						{Role: "user", Text: "msg3"},
						{Role: "assistant", Text: "reply3"},
					}
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{RecentTurns: existingTurns},
						time.Now().UTC(),
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
						return len(sess.Payload().RecentTurns) == 6
					})).Return(nil).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro de infraestrutura em upsert",
			args: args{input: AppendOnboardingTurnInput{
				UserID:         s.userID,
				UserMessage:    "oi",
				AssistantReply: "olá",
			}},
			dependencies: dependencies{
				sessionRepo: func() *mocks.OnboardingSessionRepository {
					session := entities.HydrateOnboardingSession(
						s.userID,
						entities.OnboardingChannelWhatsApp,
						entities.OnboardingSessionPayload{},
						time.Now().UTC(),
					)
					s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
					s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.AnythingOfType("entities.OnboardingSession")).
						Return(errors.New("db error")).Once()
					return s.sessionRepo
				}(),
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewAppendOnboardingTurn(&onboardingUoWStub{}, s.factory, s.obs)
			err := uc.Execute(s.ctx, scenario.args.input)
			scenario.expect(err)
		})
	}
}
