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

type SaveOnboardingObjectiveSuite struct {
	suite.Suite
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	uc          *usecases.SaveOnboardingObjective
	userID      uuid.UUID
}

func TestSaveOnboardingObjectiveSuite(t *testing.T) {
	suite.Run(t, new(SaveOnboardingObjectiveSuite))
}

func (s *SaveOnboardingObjectiveSuite) SetupTest() {
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	s.uc = usecases.NewSaveOnboardingObjective(&onboardingUoWStub{}, s.factory, noop.NewProvider())
}

func (s *SaveOnboardingObjectiveSuite) TestHappyPath() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingIncome,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.Payload().Objective == "comprar uma casa"
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.SaveOnboardingObjectiveInput{
		UserID:    s.userID,
		Objective: "  comprar   uma casa ",
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), "comprar uma casa", result.Objective)
}

func (s *SaveOnboardingObjectiveSuite) TestInvalidObjectiveRejectedBeforeTx() {
	_, err := s.uc.Execute(context.Background(), usecases.SaveOnboardingObjectiveInput{
		UserID:    s.userID,
		Objective: "",
	})
	require.ErrorIs(s.T(), err, valueobjects.ErrFinancialObjectiveEmpty)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SaveOnboardingObjectiveSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), usecases.SaveOnboardingObjectiveInput{
		UserID:    s.userID,
		Objective: "meta valida",
	})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}
