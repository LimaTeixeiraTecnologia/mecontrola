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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type unitOfWorkStartBudget struct{}

func (u *unitOfWorkStartBudget) DBTX() database.DBTX { return nil }

func (u *unitOfWorkStartBudget) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type StartBudgetConfigurationSuite struct {
	suite.Suite
	obs         observability.Observability
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	uc          *StartBudgetConfiguration
	userID      uuid.UUID
}

func TestStartBudgetConfigurationSuite(t *testing.T) {
	suite.Run(t, new(StartBudgetConfigurationSuite))
}

func (s *StartBudgetConfigurationSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	s.uc = NewStartBudgetConfiguration(
		&unitOfWorkStartBudget{},
		s.factory,
		s.obs,
	)
}

func (s *StartBudgetConfigurationSuite) TestUserIDRequired() {
	_, err := s.uc.Execute(context.Background(), StartBudgetConfigurationInput{
		Channel: entities.OnboardingChannelTelegram,
	})
	require.ErrorIs(s.T(), err, ErrStartBudgetUserIDRequired)
}

func (s *StartBudgetConfigurationSuite) TestSessionNotFoundCreatesAwaitingIncome() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.UserID() == s.userID &&
			sess.Channel() == entities.OnboardingChannelTelegram &&
			sess.State() == valueobjects.OnboardingStateAwaitingIncome
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), StartBudgetConfigurationInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelTelegram,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), StartBudgetOutcomeStarted, result.Outcome)
	require.Equal(s.T(), valueobjects.OnboardingStateAwaitingIncome, result.NewState)
	require.Contains(s.T(), result.Reply, "renda mensal")
}

func (s *StartBudgetConfigurationSuite) TestActiveSessionResetsToAwaitingIncome() {
	existing := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelTelegram,
		valueobjects.OnboardingStateActive,
		entities.OnboardingSessionPayload{IncomeCents: 500000},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(existing, nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.State() == valueobjects.OnboardingStateAwaitingIncome &&
			sess.Payload().IncomeCents == 0
	})).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), StartBudgetConfigurationInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelTelegram,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), StartBudgetOutcomeReset, result.Outcome)
	require.Equal(s.T(), valueobjects.OnboardingStateAwaitingIncome, result.NewState)
}

func (s *StartBudgetConfigurationSuite) TestNonTerminalSessionReturnsResume() {
	existing := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelTelegram,
		valueobjects.OnboardingStateAwaitingCardDecision,
		entities.OnboardingSessionPayload{IncomeCents: 500000},
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(existing, nil).Once()

	result, err := s.uc.Execute(context.Background(), StartBudgetConfigurationInput{
		UserID:  s.userID,
		Channel: entities.OnboardingChannelTelegram,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), StartBudgetOutcomeResume, result.Outcome)
	require.Equal(s.T(), valueobjects.OnboardingStateAwaitingCardDecision, result.NewState)
	require.Contains(s.T(), result.Reply, "configurando seu orçamento")
	require.Contains(s.T(), result.Reply, "cartao")
}
