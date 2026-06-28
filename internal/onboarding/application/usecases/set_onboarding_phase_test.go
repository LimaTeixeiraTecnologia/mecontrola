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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type SetOnboardingPhaseSuite struct {
	suite.Suite
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	uc          *SetOnboardingPhase
	userID      uuid.UUID
}

func TestSetOnboardingPhaseSuite(t *testing.T) {
	suite.Run(t, new(SetOnboardingPhaseSuite))
}

func (s *SetOnboardingPhaseSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	s.uc = NewSetOnboardingPhase(&onboardingUoWStub{}, s.factory, s.obs)
}

func (s *SetOnboardingPhaseSuite) baseSession() entities.OnboardingSession {
	return entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
}

func (s *SetOnboardingPhaseSuite) TestNilUserIDRejected() {
	_, err := s.uc.Execute(context.Background(), SetOnboardingPhaseInput{
		UserID: uuid.Nil,
		Phase:  valueobjects.PhaseObjective,
	})
	require.Error(s.T(), err)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SetOnboardingPhaseSuite) TestInvalidPhaseRejected() {
	_, err := s.uc.Execute(context.Background(), SetOnboardingPhaseInput{
		UserID: s.userID,
		Phase:  valueobjects.OnboardingPhase(0),
	})
	require.ErrorIs(s.T(), err, valueobjects.ErrOnboardingPhaseInvalid)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SetOnboardingPhaseSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), SetOnboardingPhaseInput{
		UserID: s.userID,
		Phase:  valueobjects.PhaseObjective,
	})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}

func (s *SetOnboardingPhaseSuite) TestHappyPath() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(s.baseSession(), nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.Payload().Phase == valueobjects.PhaseObjective
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), SetOnboardingPhaseInput{
		UserID: s.userID,
		Phase:  valueobjects.PhaseObjective,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), valueobjects.PhaseObjective, result.Phase)
}
