package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type CompleteOnboardingSessionSuite struct {
	suite.Suite
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	uc          *usecases.CompleteOnboardingSession
	userID      uuid.UUID
}

func TestCompleteOnboardingSessionSuite(t *testing.T) {
	suite.Run(t, new(CompleteOnboardingSessionSuite))
}

func (s *CompleteOnboardingSessionSuite) SetupTest() {
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	s.uc = usecases.NewCompleteOnboardingSession(
		&onboardingUoWStub{},
		s.factory,
		s.publisher,
		&onboardingFixedIDGen{id: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"},
		noop.NewProvider(),
	)
}

func (s *CompleteOnboardingSessionSuite) TestActiveSessionAlreadyActive() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateActive,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.CompleteOnboardingSessionInput{UserID: s.userID})
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
		valueobjects.OnboardingStateAwaitingFirstTransaction,
		entities.OnboardingSessionPayload{FirstTxRecorded: false},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	_, err := s.uc.Execute(context.Background(), usecases.CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorIs(s.T(), err, usecases.ErrOnboardingFirstTransactionRequired)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
}

func (s *CompleteOnboardingSessionSuite) TestHappyPath() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingFirstTransaction,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.State() == valueobjects.OnboardingStateActive
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.CompleteOnboardingSessionInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Completed)
	require.False(s.T(), result.AlreadyActive)
}

func (s *CompleteOnboardingSessionSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), usecases.CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}
