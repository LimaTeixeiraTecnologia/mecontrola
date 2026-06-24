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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type CompleteOnboardingSessionSuite struct {
	suite.Suite
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	uc          *CompleteOnboardingSession
	userID      uuid.UUID
}

func TestCompleteOnboardingSessionSuite(t *testing.T) {
	suite.Run(t, new(CompleteOnboardingSessionSuite))
}

func (s *CompleteOnboardingSessionSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	s.uc = NewCompleteOnboardingSession(
		&onboardingUoWStub{},
		s.factory,
		s.publisher,
		&onboardingFixedIDGen{id: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"},
		s.obs,
	)
}

func (s *CompleteOnboardingSessionSuite) TestActiveSessionAlreadyActive() {
	completedAt := time.Now().UTC()
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: true, CompletedAt: &completedAt},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.AlreadyActive)
	require.False(s.T(), result.Completed)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
	s.publisher.AssertNotCalled(s.T(), "Publish", mock.Anything, mock.Anything)
}

func (s *CompleteOnboardingSessionSuite) TestMissingFirstTransactionRejected() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: false},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorIs(s.T(), err, ErrOnboardingFirstTransactionRequired)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
}

func (s *CompleteOnboardingSessionSuite) TestHappyPath() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.IsActive()
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Completed)
	require.False(s.T(), result.AlreadyActive)
}

func (s *CompleteOnboardingSessionSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}

func (s *CompleteOnboardingSessionSuite) TestUpsertFailure() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorContains(s.T(), err, "db error")
	s.publisher.AssertNotCalled(s.T(), "Publish", mock.Anything, mock.Anything)
}

func (s *CompleteOnboardingSessionSuite) TestPublishFailure() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("publish error")).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorContains(s.T(), err, "publish error")
}

func (s *CompleteOnboardingSessionSuite) TestHappyPath_CompletedAtSetAndTurnsCleared() {
	now := time.Now().UTC()
	turns := []entities.OnboardingTurn{
		{Role: "user", Text: "msg1", OccurredAt: now},
		{Role: "assistant", Text: "reply1", OccurredAt: now},
	}
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: true, RecentTurns: turns},
		now,
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		p := sess.Payload()
		return sess.IsActive() &&
			p.CompletedAt != nil &&
			len(p.RecentTurns) == 0
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Completed)
}
