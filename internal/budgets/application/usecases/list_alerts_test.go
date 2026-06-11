package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ListAlertsSuite struct {
	suite.Suite
	ctx     context.Context
	factory *mockInterfaces.RepositoryFactory
	alerts  *mockInterfaces.AlertRepository
	uow     *uowMocks.UnitOfWorkListAlertsOutput
	useCase *usecases.ListAlerts
}

func TestListAlertsSuite(t *testing.T) {
	suite.Run(t, new(ListAlertsSuite))
}

func (s *ListAlertsSuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.alerts = mockInterfaces.NewAlertRepository(s.T())
	s.factory.EXPECT().AlertRepository(mock.Anything).Return(s.alerts).Maybe()
	s.uow = uowMocks.NewUnitOfWorkListAlertsOutput(s.T())
	s.useCase = usecases.NewListAlerts(s.factory, s.uow, noop.NewProvider())
}

func buildAlertEntity(userID uuid.UUID) entities.Alert {
	comp, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo
	return entities.HydrateAlert(
		uuid.New(),
		userID,
		comp,
		slug,
		valueobjects.Threshold80,
		entities.AlertStateDelivered,
		time.Now().UTC(),
		8000,
		10000,
		time.Now().UTC(),
	)
}

func (s *ListAlertsSuite) TestExecute_InvalidUserID_ReturnsError() {
	_, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: "not-a-uuid",
		Limit:  10,
	})

	s.ErrorIs(err, usecases.ErrListAlertsInvalidUserID)
}

func (s *ListAlertsSuite) TestExecute_DefaultLimit_Applied() {
	userID := uuid.New()

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Limit == 50
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Limit:  0,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
	s.Empty(result.NextCursor)
}

func (s *ListAlertsSuite) TestExecute_NegativeLimit_AppliesDefault() {
	userID := uuid.New()

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Limit == 50
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Limit:  -5,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
}

func (s *ListAlertsSuite) TestExecute_ExceedsMaxLimit_CappedAt200() {
	userID := uuid.New()

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Limit == 200
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Limit:  500,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
}

func (s *ListAlertsSuite) TestExecute_ReturnsAlerts_WithNextCursor() {
	userID := uuid.New()
	alert := buildAlertEntity(userID)
	nextCursor := "cursor-abc"

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Limit == 10
		})).
		Return([]entities.Alert{alert}, nextCursor, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Limit:  10,
	})

	s.NoError(err)
	s.Len(result.Alerts, 1)
	s.Equal(nextCursor, result.NextCursor)
	s.Equal(alert.ID().String(), result.Alerts[0].ID)
	s.Equal(userID.String(), result.Alerts[0].UserID)
	s.Equal("delivered", result.Alerts[0].State)
	s.Equal(80, result.Alerts[0].Threshold)
}

func (s *ListAlertsSuite) TestExecute_WithCursorPagination() {
	userID := uuid.New()
	cursor := "cursor-page2"

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Cursor == cursor && q.Limit == 20
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Cursor: cursor,
		Limit:  20,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
	s.Empty(result.NextCursor)
}

func (s *ListAlertsSuite) TestExecute_WithCompetenceFilter() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-05")

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Competence != nil && q.Competence.String() == "2026-05"
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID:     userID.String(),
		Competence: &comp,
		Limit:      10,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
}

func (s *ListAlertsSuite) TestExecute_WithRootSlugFilter() {
	userID := uuid.New()
	slug := valueobjects.RootSlugCustoFixo

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.RootSlug != nil && *q.RootSlug == valueobjects.RootSlugCustoFixo
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID:   userID.String(),
		RootSlug: &slug,
		Limit:    10,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
}

func (s *ListAlertsSuite) TestExecute_WithThresholdFilter() {
	userID := uuid.New()
	threshold := valueobjects.Threshold100

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.MatchedBy(func(q input.AlertQuery) bool {
			return q.Threshold != nil && *q.Threshold == valueobjects.Threshold100
		})).
		Return([]entities.Alert{}, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID:    userID.String(),
		Threshold: &threshold,
		Limit:     10,
	})

	s.NoError(err)
	s.Empty(result.Alerts)
}

func (s *ListAlertsSuite) TestExecute_RepositoryError_PropagatesError() {
	userID := uuid.New()

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.Anything).
		Return(nil, "", errors.New("db error")).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Limit:  10,
	})

	s.Error(err)
}

func (s *ListAlertsSuite) TestExecute_UserIsolation_DifferentUserDifferentResults() {
	userID1 := uuid.New()
	userID2 := uuid.New()
	alert1 := buildAlertEntity(userID1)

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID1, mock.Anything).
		Return([]entities.Alert{alert1}, "", nil).
		Once()

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID2, mock.Anything).
		Return([]entities.Alert{}, "", nil).
		Once()

	result1, err1 := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID1.String(),
		Limit:  10,
	})
	result2, err2 := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID2.String(),
		Limit:  10,
	})

	s.NoError(err1)
	s.NoError(err2)
	s.Len(result1.Alerts, 1)
	s.Empty(result2.Alerts)
	s.Equal(userID1.String(), result1.Alerts[0].UserID)
}

func (s *ListAlertsSuite) TestExecute_AlertStateMapping() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	stateAlerts := []entities.Alert{
		entities.HydrateAlert(uuid.New(), userID, comp, slug, valueobjects.Threshold80, entities.AlertStatePendingDelivery, time.Now().UTC(), 500, 10000, time.Now().UTC()),
		entities.HydrateAlert(uuid.New(), userID, comp, slug, valueobjects.Threshold80, entities.AlertStateDelivered, time.Now().UTC(), 8000, 10000, time.Now().UTC()),
		entities.HydrateAlert(uuid.New(), userID, comp, slug, valueobjects.Threshold80, entities.AlertStateSuppressedStale, time.Now().UTC(), 100, 10000, time.Now().UTC()),
		entities.HydrateAlert(uuid.New(), userID, comp, slug, valueobjects.Threshold80, entities.AlertStateSuppressedRetroactive, time.Now().UTC(), 8000, 10000, time.Now().UTC()),
		entities.HydrateAlert(uuid.New(), userID, comp, slug, valueobjects.Threshold80, entities.AlertStateRateLimited, time.Now().UTC(), 8000, 10000, time.Now().UTC()),
	}

	s.alerts.EXPECT().
		ListForUser(s.ctx, userID, mock.Anything).
		Return(stateAlerts, "", nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ListAlertsInput{
		UserID: userID.String(),
		Limit:  10,
	})

	s.NoError(err)
	s.Len(result.Alerts, 5)
	s.Equal("pending_delivery", result.Alerts[0].State)
	s.Equal("delivered", result.Alerts[1].State)
	s.Equal("suppressed_stale", result.Alerts[2].State)
	s.Equal("suppressed_retroactive", result.Alerts[3].State)
	s.Equal("rate_limited", result.Alerts[4].State)
}
