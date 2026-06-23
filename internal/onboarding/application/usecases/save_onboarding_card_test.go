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

type SaveOnboardingCardSuite struct {
	suite.Suite
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	uc          *SaveOnboardingCard
	userID      uuid.UUID
}

func TestSaveOnboardingCardSuite(t *testing.T) {
	suite.Run(t, new(SaveOnboardingCardSuite))
}

func (s *SaveOnboardingCardSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	s.uc = NewSaveOnboardingCard(
		&onboardingUoWStub{},
		s.factory,
		s.publisher,
		&onboardingFixedIDGen{id: "99999999-9999-9999-9999-999999999999"},
		s.obs,
		nil,
	)
}

func (s *SaveOnboardingCardSuite) TestHappyPath() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingCardDecision,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return len(sess.Payload().Cards) == 1 && sess.Payload().Cards[0].Name == "nubank"
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), SaveOnboardingCardInput{
		UserID:   s.userID,
		Nickname: "nubank",
		DueDay:   17,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), "nubank", result.Name)
	require.Equal(s.T(), 17, result.DueDay)
	require.Equal(s.T(), 1, result.CardCount)
}

func (s *SaveOnboardingCardSuite) TestEmptyNicknameRejectedBeforeTx() {
	_, err := s.uc.Execute(context.Background(), SaveOnboardingCardInput{
		UserID:   s.userID,
		Nickname: "   ",
		DueDay:   10,
	})
	require.ErrorIs(s.T(), err, entities.ErrOnboardingCardNicknameRequired)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SaveOnboardingCardSuite) TestInvalidDueDayRejectedBeforeTx() {
	_, err := s.uc.Execute(context.Background(), SaveOnboardingCardInput{
		UserID:   s.userID,
		Nickname: "nubank",
		DueDay:   40,
	})
	require.ErrorIs(s.T(), err, valueobjects.ErrCardDueDayOutOfRange)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SaveOnboardingCardSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), SaveOnboardingCardInput{
		UserID:   s.userID,
		Nickname: "nubank",
		DueDay:   17,
	})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}
