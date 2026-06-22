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
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type SaveOnboardingIncomeSuite struct {
	suite.Suite
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	uc          *SaveOnboardingIncome
	userID      uuid.UUID
}

func TestSaveOnboardingIncomeSuite(t *testing.T) {
	suite.Run(t, new(SaveOnboardingIncomeSuite))
}

func (s *SaveOnboardingIncomeSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	s.uc = NewSaveOnboardingIncome(
		&onboardingUoWStub{},
		s.factory,
		s.publisher,
		&onboardingFixedIDGen{id: "77777777-7777-7777-7777-777777777777"},
		s.obs,
	)
}

func (s *SaveOnboardingIncomeSuite) TestHappyPath() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingIncome,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.Payload().IncomeCents == 500000
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), SaveOnboardingIncomeInput{
		UserID:      s.userID,
		IncomeCents: 500000,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(500000), result.IncomeCents)
}

func (s *SaveOnboardingIncomeSuite) TestBelowMinimumRejectedBeforeTx() {
	_, err := s.uc.Execute(context.Background(), SaveOnboardingIncomeInput{
		UserID:      s.userID,
		IncomeCents: 100,
	})
	require.ErrorIs(s.T(), err, valueobjects.ErrMonthlyIncomeBelowMinimum)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SaveOnboardingIncomeSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), SaveOnboardingIncomeInput{
		UserID:      s.userID,
		IncomeCents: 500000,
	})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}
