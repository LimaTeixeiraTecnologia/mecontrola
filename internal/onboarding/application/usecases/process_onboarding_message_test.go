package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type unitOfWorkProcess struct{}

func (u *unitOfWorkProcess) DBTX() database.DBTX { return nil }

func (u *unitOfWorkProcess) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type fixedIDGen struct {
	id string
}

func (f *fixedIDGen) NewID() string { return f.id }

type ProcessOnboardingMessageSuite struct {
	suite.Suite
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	uc          *usecases.ProcessOnboardingMessage
	userID      uuid.UUID
}

func TestProcessOnboardingMessageSuite(t *testing.T) {
	suite.Run(t, new(ProcessOnboardingMessageSuite))
}

func (s *ProcessOnboardingMessageSuite) SetupTest() {
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	s.uc = usecases.NewProcessOnboardingMessage(
		&unitOfWorkProcess{},
		s.factory,
		services.NewOnboardingWorkflow(),
		s.publisher,
		&fixedIDGen{id: "22222222-2222-2222-2222-222222222222"},
		noop.NewProvider(),
	)
}

func (s *ProcessOnboardingMessageSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), usecases.ProcessOnboardingMessageInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelWhatsApp,
		Text:    "oi",
	})
	require.Error(s.T(), err)
	require.True(s.T(), errors.Is(err, appinterfaces.ErrOnboardingSessionNotFound))
}

func (s *ProcessOnboardingMessageSuite) TestAwaitingTokenRepliesOnly() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingToken,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.ProcessOnboardingMessageInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelWhatsApp,
		Text:    "ola",
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), usecases.ProcessOnboardingOutcomeReplyOnly, result.Outcome)
	require.NotEmpty(s.T(), result.Reply)
}

func (s *ProcessOnboardingMessageSuite) TestAwaitingIncomeAdvancesAndPublishes() {
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingIncome,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.State() == valueobjects.OnboardingStateAwaitingCardDecision && sess.Payload().IncomeCents == 350000
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
		return evt.Type == "onboarding.income_registered"
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.ProcessOnboardingMessageInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelWhatsApp,
		Text:    "R$ 3500",
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), usecases.ProcessOnboardingOutcomeAdvanced, result.Outcome)
	require.Equal(s.T(), valueobjects.OnboardingStateAwaitingCardDecision, result.ToState)
}

func (s *ProcessOnboardingMessageSuite) TestSplitConfirmCompletes() {
	split := []entities.OnboardingCardSplitEntry{
		{Kind: "fixed_cost", Percent: 40},
		{Kind: "knowledge", Percent: 10},
		{Kind: "pleasures", Percent: 15},
		{Kind: "goals", Percent: 20},
		{Kind: "financial_freedom", Percent: 15},
	}
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingSplitConfirm,
		entities.OnboardingSessionPayload{Split: split},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.State() == valueobjects.OnboardingStateActive
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Twice()

	result, err := s.uc.Execute(context.Background(), usecases.ProcessOnboardingMessageInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelWhatsApp,
		Text:    "sim",
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), usecases.ProcessOnboardingOutcomeCompleted, result.Outcome)
}

func (s *ProcessOnboardingMessageSuite) TestNilUserIDRejected() {
	_, err := s.uc.Execute(context.Background(), usecases.ProcessOnboardingMessageInput{
		UserID: uuid.Nil,
		Text:   "x",
	})
	require.Error(s.T(), err)
}
