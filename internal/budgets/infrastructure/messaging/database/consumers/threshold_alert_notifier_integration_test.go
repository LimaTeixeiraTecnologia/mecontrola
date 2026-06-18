//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	budgetpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type noopChannelResolver struct{}

func (r *noopChannelResolver) ResolvePreferred(_ context.Context, _ uuid.UUID) (appinterfaces.UserChannelPreference, bool, error) {
	return appinterfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "fake-ext-id"}, true, nil
}

type noopChannelGateway struct{}

func (g *noopChannelGateway) SendText(_ context.Context, _, _, _ string) error {
	return nil
}

func (g *noopChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

type ThresholdAlertNotifierIntegrationSuite struct {
	suite.Suite
}

func TestThresholdAlertNotifierIntegration(t *testing.T) {
	suite.Run(t, new(ThresholdAlertNotifierIntegrationSuite))
}

func (s *ThresholdAlertNotifierIntegrationSuite) buildNotifier(sentRepo appinterfaces.ThresholdAlertSentRepository) *consumers.ThresholdAlertNotifier {
	o11y := noop.NewProvider()
	resolver := &noopChannelResolver{}
	gateway := &noopChannelGateway{}
	uc := usecases.NewNotifyThresholdAlert(sentRepo, resolver, gateway, o11y)
	return consumers.NewThresholdAlertNotifier(uc, o11y)
}

func (s *ThresholdAlertNotifierIntegrationSuite) buildEnvelope(userID, budgetID uuid.UUID, refDay string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"user_id":                userID.String(),
		"budget_id":              budgetID.String(),
		"kind":                   "category_threshold",
		"root_slug":              "expense.prazeres",
		"percent_used_bps":       int32(8500),
		"amount_remaining_cents": int64(15000),
		"ref_day":                refDay,
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *ThresholdAlertNotifierIntegrationSuite) TestMarksBudgetAlertSentAfterHandle() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()

	sentRepo := budgetpostgres.NewThresholdAlertSentRepository(o11y, db)

	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	s.Require().NoError(sentRepo.InsertSent(ctx, appinterfaces.ThresholdAlertSentRecord{
		UserID:   userID,
		BudgetID: budgetID,
		Kind:     services.ThresholdAlertCategory,
		RefDay:   refDay,
		SentAt:   time.Now().UTC(),
	}))

	notifier := s.buildNotifier(sentRepo)
	env := s.buildEnvelope(userID, budgetID, "2026-06-17")
	evt := stubEvent{eventType: "budgets.threshold_alert_triggered.v1", payload: env}
	s.Require().NoError(notifier.Handle(ctx, platformevents.Event(evt)))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE user_id = $1 AND budget_id = $2 AND notified_at IS NOT NULL`,
		userID, budgetID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}

func (s *ThresholdAlertNotifierIntegrationSuite) TestIdempotency() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()

	sentRepo := budgetpostgres.NewThresholdAlertSentRepository(o11y, db)

	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	s.Require().NoError(sentRepo.InsertSent(ctx, appinterfaces.ThresholdAlertSentRecord{
		UserID:   userID,
		BudgetID: budgetID,
		Kind:     services.ThresholdAlertCategory,
		RefDay:   refDay,
		SentAt:   time.Now().UTC(),
	}))

	notifier := s.buildNotifier(sentRepo)
	env := s.buildEnvelope(userID, budgetID, "2026-06-17")
	evt := stubEvent{eventType: "budgets.threshold_alert_triggered.v1", payload: env}

	s.Require().NoError(notifier.Handle(ctx, platformevents.Event(evt)))
	s.Require().NoError(notifier.Handle(ctx, platformevents.Event(evt)))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE user_id = $1 AND budget_id = $2`,
		userID, budgetID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}
