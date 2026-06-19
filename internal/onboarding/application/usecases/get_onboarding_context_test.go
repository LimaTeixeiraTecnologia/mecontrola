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

type GetOnboardingContextSuite struct {
	suite.Suite
	sessionRepo *mocks.OnboardingSessionRepository
	uc          *usecases.GetOnboardingContext
	userID      uuid.UUID
}

func TestGetOnboardingContextSuite(t *testing.T) {
	suite.Run(t, new(GetOnboardingContextSuite))
}

func (s *GetOnboardingContextSuite) SetupTest() {
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.userID = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	s.uc = usecases.NewGetOnboardingContext(s.sessionRepo, noop.NewProvider())
}

func (s *GetOnboardingContextSuite) TestFoundMapsAllFields() {
	payload := entities.OnboardingSessionPayload{
		IncomeCents:     500000,
		Objective:       "quitar dividas",
		FirstTxRecorded: true,
		Cards: []entities.OnboardingCardDraft{
			{Name: "nubank", DueDay: 17},
		},
		CustomSplit: []entities.OnboardingBudgetAllocationEntry{
			{Kind: "fixed_cost", BasisPoints: 4000},
			{Kind: "knowledge", BasisPoints: 1000},
			{Kind: "pleasures", BasisPoints: 1500},
			{Kind: "goals", BasisPoints: 2000},
			{Kind: "financial_freedom", BasisPoints: 1500},
		},
	}
	session := entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingFirstTransaction,
		payload,
		time.Now().UTC(),
	)
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(session, nil).Once()

	result, err := s.uc.Execute(context.Background(), usecases.GetOnboardingContextInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Found)
	require.Equal(s.T(), valueobjects.OnboardingStateAwaitingFirstTransaction, result.State)
	require.Equal(s.T(), int64(500000), result.IncomeCents)
	require.Equal(s.T(), "quitar dividas", result.Objective)
	require.True(s.T(), result.FirstTxRecorded)
	require.Len(s.T(), result.Cards, 1)
	require.Equal(s.T(), "nubank", result.Cards[0].Name)
	require.Equal(s.T(), 17, result.Cards[0].DueDay)
	require.Len(s.T(), result.CustomSplit, 5)
	require.Equal(s.T(), "fixed_cost", result.CustomSplit[0].Kind)
	require.Equal(s.T(), 4000, result.CustomSplit[0].BasisPoints)
}

func (s *GetOnboardingContextSuite) TestNotFoundReturnsFoundFalse() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	result, err := s.uc.Execute(context.Background(), usecases.GetOnboardingContextInput{UserID: s.userID})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Found)
}

func (s *GetOnboardingContextSuite) TestNilUserIDRejected() {
	_, err := s.uc.Execute(context.Background(), usecases.GetOnboardingContextInput{UserID: uuid.Nil})
	require.Error(s.T(), err)
}
