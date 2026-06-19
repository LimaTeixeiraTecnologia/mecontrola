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
)

type MarkFirstTransactionRecordedSuite struct {
	suite.Suite
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	uc          *usecases.MarkFirstTransactionRecorded
	userID      uuid.UUID
}

func TestMarkFirstTransactionRecordedSuite(t *testing.T) {
	suite.Run(t, new(MarkFirstTransactionRecordedSuite))
}

func (s *MarkFirstTransactionRecordedSuite) SetupTest() {
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	s.uc = usecases.NewMarkFirstTransactionRecorded(&onboardingUoWStub{}, s.factory, noop.NewProvider())
}

func (s *MarkFirstTransactionRecordedSuite) TestNotFoundReturnsNotMarked() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	result, err := s.uc.Execute(context.Background(), usecases.MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Marked)
}

func (s *MarkFirstTransactionRecordedSuite) TestActiveSessionNotMarked() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateActive,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Marked)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
}

func (s *MarkFirstTransactionRecordedSuite) TestAlreadyRecordedMarkedNoUpsert() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingFirstTransaction,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Marked)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
}

func (s *MarkFirstTransactionRecordedSuite) TestFreshSessionMarksAndUpserts() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingFirstTransaction,
		entities.OnboardingSessionPayload{FirstTxRecorded: false},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.Payload().FirstTxRecorded
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Marked)
}
