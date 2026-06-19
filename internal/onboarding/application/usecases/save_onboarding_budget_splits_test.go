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

type SaveOnboardingBudgetSplitsSuite struct {
	suite.Suite
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	uc          *usecases.SaveOnboardingBudgetSplits
	userID      uuid.UUID
}

func TestSaveOnboardingBudgetSplitsSuite(t *testing.T) {
	suite.Run(t, new(SaveOnboardingBudgetSplitsSuite))
}

func (s *SaveOnboardingBudgetSplitsSuite) SetupTest() {
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	s.uc = usecases.NewSaveOnboardingBudgetSplits(
		&onboardingUoWStub{},
		s.factory,
		s.publisher,
		&onboardingFixedIDGen{id: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"},
		noop.NewProvider(),
	)
}

func (s *SaveOnboardingBudgetSplitsSuite) seedSession() entities.OnboardingSession {
	return entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingSplitConfirm,
		entities.OnboardingSessionPayload{IncomeCents: 500000},
		time.Now().UTC(),
	)
}

func (s *SaveOnboardingBudgetSplitsSuite) TestHappyPath() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(s.seedSession(), nil).Once()
	s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
		return sess.State() == valueobjects.OnboardingStateAwaitingFirstTransaction &&
			len(sess.Payload().CustomSplit) == 5
	})).Return(nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.SaveOnboardingBudgetSplitsInput{
		UserID: s.userID,
		Allocations: []usecases.BudgetSplitItem{
			{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 200000},
			{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 50000},
			{Kind: valueobjects.CategoryKindPleasures, AmountCents: 75000},
			{Kind: valueobjects.CategoryKindGoals, AmountCents: 100000},
			{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 75000},
		},
	})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Applied)
	require.Equal(s.T(), int64(500000), result.SumCents)
	require.Equal(s.T(), int64(500000), result.TotalCents)
	require.Len(s.T(), result.Allocations, 5)

	percents := map[valueobjects.CategoryKind]int{}
	for _, a := range result.Allocations {
		percents[a.Kind] = a.Percent
	}
	require.Equal(s.T(), 40, percents[valueobjects.CategoryKindFixedCost])
	require.Equal(s.T(), 10, percents[valueobjects.CategoryKindKnowledge])
	require.Equal(s.T(), 15, percents[valueobjects.CategoryKindPleasures])
	require.Equal(s.T(), 20, percents[valueobjects.CategoryKindGoals])
	require.Equal(s.T(), 15, percents[valueobjects.CategoryKindFinancialFreedom])
}

func (s *SaveOnboardingBudgetSplitsSuite) TestSumMismatchNotApplied() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(s.seedSession(), nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.SaveOnboardingBudgetSplitsInput{
		UserID: s.userID,
		Allocations: []usecases.BudgetSplitItem{
			{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 300000},
			{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 50000},
			{Kind: valueobjects.CategoryKindPleasures, AmountCents: 75000},
			{Kind: valueobjects.CategoryKindGoals, AmountCents: 100000},
			{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 75000},
		},
	})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Applied)
	require.Equal(s.T(), int64(600000), result.SumCents)
	require.Equal(s.T(), int64(500000), result.TotalCents)
	s.sessionRepo.AssertNotCalled(s.T(), "Upsert", mock.Anything, mock.Anything)
	s.publisher.AssertNotCalled(s.T(), "Publish", mock.Anything, mock.Anything)
}

func (s *SaveOnboardingBudgetSplitsSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	_, err := s.uc.Execute(context.Background(), usecases.SaveOnboardingBudgetSplitsInput{
		UserID: s.userID,
		Allocations: []usecases.BudgetSplitItem{
			{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 200000},
			{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 50000},
			{Kind: valueobjects.CategoryKindPleasures, AmountCents: 75000},
			{Kind: valueobjects.CategoryKindGoals, AmountCents: 100000},
			{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 75000},
		},
	})
	require.ErrorIs(s.T(), err, appinterfaces.ErrOnboardingSessionNotFound)
}
