package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeNotifyUC struct {
	called bool
	input  usecases.NotifyThresholdAlertInput
	result usecases.NotifyThresholdAlertResult
	err    error
}

func (f *fakeNotifyUC) Execute(_ context.Context, in usecases.NotifyThresholdAlertInput) (usecases.NotifyThresholdAlertResult, error) {
	f.called = true
	f.input = in
	return f.result, f.err
}

type stubEvent struct {
	eventType string
	payload   any
}

func (e stubEvent) GetEventType() string { return e.eventType }
func (e stubEvent) GetPayload() any      { return e.payload }

type ThresholdAlertNotifierSuite struct {
	suite.Suite
}

func TestThresholdAlertNotifier(t *testing.T) {
	suite.Run(t, new(ThresholdAlertNotifierSuite))
}

func (s *ThresholdAlertNotifierSuite) TestHandleHappyPath() {
	userID := uuid.New()
	budgetID := uuid.New()
	payload := map[string]any{
		"user_id":                userID.String(),
		"budget_id":              budgetID.String(),
		"kind":                   "category_threshold",
		"root_slug":              "alimentacao",
		"percent_used_bps":       8500,
		"amount_remaining_cents": 50000,
		"ref_day":                "2026-06-15",
	}
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}

	uc := &fakeNotifyUC{result: usecases.NotifyThresholdAlertResult{Outcome: usecases.NotifyOutcomeSent, Channel: "whatsapp"}}
	notifier := consumers.NewThresholdAlertNotifier(uc, noop.NewProvider())

	err := notifier.Handle(context.Background(), stubEvent{eventType: "budgets.threshold_alert_triggered.v1", payload: env})
	s.Require().NoError(err)
	s.True(uc.called)
	s.Equal(userID, uc.input.UserID)
	s.Equal(budgetID, uc.input.BudgetID)
	s.Equal("alimentacao", uc.input.RootSlug)
	s.Equal(int32(8500), uc.input.PercentUsedBps)
	s.Equal(int64(50000), uc.input.AmountRemainingCents)
	s.True(uc.input.RefDay.Equal(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)))
}

func (s *ThresholdAlertNotifierSuite) TestHandleRejectsUnknownEventType() {
	notifier := consumers.NewThresholdAlertNotifier(&fakeNotifyUC{}, noop.NewProvider())
	err := notifier.Handle(context.Background(), stubEvent{eventType: "other.event", payload: outbox.Envelope{}})
	s.Require().Error(err)
}

func (s *ThresholdAlertNotifierSuite) TestHandleRejectsUnexpectedPayloadType() {
	notifier := consumers.NewThresholdAlertNotifier(&fakeNotifyUC{}, noop.NewProvider())
	err := notifier.Handle(context.Background(), stubEvent{eventType: "budgets.threshold_alert_triggered.v1", payload: "not-envelope"})
	s.Require().Error(err)
}

func (s *ThresholdAlertNotifierSuite) TestHandleRejectsInvalidUUID() {
	payload := map[string]any{
		"user_id":   "not-a-uuid",
		"budget_id": uuid.New().String(),
		"kind":      "goal_achieved",
		"ref_day":   "2026-06-15",
	}
	raw, _ := json.Marshal(payload)
	notifier := consumers.NewThresholdAlertNotifier(&fakeNotifyUC{}, noop.NewProvider())
	err := notifier.Handle(context.Background(), stubEvent{eventType: "budgets.threshold_alert_triggered.v1", payload: outbox.Envelope{Payload: raw}})
	s.Require().Error(err)
}

func (s *ThresholdAlertNotifierSuite) TestHandlePropagatesUseCaseError() {
	userID := uuid.New()
	budgetID := uuid.New()
	payload := map[string]any{
		"user_id":   userID.String(),
		"budget_id": budgetID.String(),
		"kind":      "category_threshold",
		"ref_day":   "2026-06-15",
	}
	raw, _ := json.Marshal(payload)
	uc := &fakeNotifyUC{err: errors.New("downstream error")}
	notifier := consumers.NewThresholdAlertNotifier(uc, noop.NewProvider())
	err := notifier.Handle(context.Background(), stubEvent{eventType: "budgets.threshold_alert_triggered.v1", payload: outbox.Envelope{Payload: raw}})
	s.Require().Error(err)
}
