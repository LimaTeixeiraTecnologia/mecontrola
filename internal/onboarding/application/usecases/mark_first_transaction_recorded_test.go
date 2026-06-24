package usecases

import (
	"context"
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
)

type MarkFirstTransactionRecordedSuite struct {
	suite.Suite
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	uc          *MarkFirstTransactionRecorded
	userID      uuid.UUID
}

func TestMarkFirstTransactionRecordedSuite(t *testing.T) {
	suite.Run(t, new(MarkFirstTransactionRecordedSuite))
}

func (s *MarkFirstTransactionRecordedSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	s.uc = NewMarkFirstTransactionRecorded(&onboardingUoWStub{}, s.factory, s.obs)
}

func (s *MarkFirstTransactionRecordedSuite) TestNotFoundReturnsNotMarked() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	result, err := s.uc.Execute(context.Background(), MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Marked)
}

func (s *MarkFirstTransactionRecordedSuite) TestActiveSessionNotMarked() {
	completedAt := time.Now().UTC()
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{CompletedAt: &completedAt},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Marked)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
}

func (s *MarkFirstTransactionRecordedSuite) TestAlreadyRecordedMarkedNoUpsert() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: true},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Marked)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
}

func (s *MarkFirstTransactionRecordedSuite) TestFreshSessionMarksAndUpserts() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{FirstTxRecorded: false},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.Payload().FirstTxRecorded
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), MarkFirstTransactionRecordedInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Marked)
}
