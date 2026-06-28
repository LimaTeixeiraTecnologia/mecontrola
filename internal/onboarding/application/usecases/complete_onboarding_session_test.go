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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
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
		entities.OnboardingSessionPayload{CompletedAt: &completedAt},
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

func (s *CompleteOnboardingSessionSuite) TestNotReadyToComplete_RejectsIncompleteSession() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorIs(s.T(), err, application.ErrOnboardingNotReadyToComplete)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
	s.publisher.AssertNotCalled(s.T(), "Publish", mock.Anything, mock.Anything)
}

func (s *CompleteOnboardingSessionSuite) TestHappyPath_WithoutFirstTransaction() {
	session := newReadyToCompleteSession(s.userID)
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
	session := newReadyToCompleteSession(s.userID)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorContains(s.T(), err, "db error")
	s.publisher.AssertNotCalled(s.T(), "Publish", mock.Anything, mock.Anything)
}

func (s *CompleteOnboardingSessionSuite) TestPublishFailure() {
	session := newReadyToCompleteSession(s.userID)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("publish error")).Once()

	_, err := s.uc.Execute(context.Background(), CompleteOnboardingSessionInput{UserID: s.userID})
	require.ErrorContains(s.T(), err, "publish error")
}

func (s *CompleteOnboardingSessionSuite) TestHappyPath_CompletedAtSetAndTurnsCleared() {
	now := time.Now().UTC()
	session := newReadyToCompleteSession(s.userID).
		WithAppendedTurn("user", "msg1", now).
		WithAppendedTurn("assistant", "reply1", now)
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

func newReadyToCompleteSession(userID uuid.UUID) entities.OnboardingSession {
	objective, _ := valueobjects.NewFinancialObjective("quitar dívidas")
	income, _ := valueobjects.NewMonthlyIncome(500000)
	allocation, _ := valueobjects.NewBudgetAllocationFromAmounts([]valueobjects.CategoryAmount{
		{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 200000},
		{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 50000},
		{Kind: valueobjects.CategoryKindPleasures, AmountCents: 75000},
		{Kind: valueobjects.CategoryKindGoals, AmountCents: 100000},
		{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 75000},
	}, 500000)
	now := time.Now().UTC()
	session, _ := entities.NewOnboardingSession(userID, entities.OnboardingChannelWhatsApp, now)
	return session.
		WithObjective(objective, now).
		WithIncome(income, now).
		WithCustomSplit(allocation, now)
}
