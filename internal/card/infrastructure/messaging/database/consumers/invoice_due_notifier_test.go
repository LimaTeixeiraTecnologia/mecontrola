package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeNotifyInvoiceDueUC struct {
	called bool
	input  usecases.NotifyInvoiceDueInput
	result usecases.NotifyInvoiceDueResult
	err    error
}

func (f *fakeNotifyInvoiceDueUC) Execute(_ context.Context, in usecases.NotifyInvoiceDueInput) (usecases.NotifyInvoiceDueResult, error) {
	f.called = true
	f.input = in
	return f.result, f.err
}

type InvoiceDueNotifierSuite struct {
	suite.Suite
}

func TestInvoiceDueNotifier(t *testing.T) {
	suite.Run(t, new(InvoiceDueNotifierSuite))
}

func (s *InvoiceDueNotifierSuite) TestHandleHappyPath() {
	userID := uuid.New()
	cardID := uuid.New()
	payload := map[string]any{
		"user_id":     userID.String(),
		"card_id":     cardID.String(),
		"card_name":   "Nubank",
		"limit_cents": 500000,
		"due_date":    "2026-07-10",
		"days_until":  3,
	}
	raw, _ := json.Marshal(payload)
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}

	uc := &fakeNotifyInvoiceDueUC{result: usecases.NotifyInvoiceDueResult{Outcome: usecases.NotifyInvoiceDueOutcomeSent, Channel: "whatsapp"}}
	notifier := consumers.NewInvoiceDueNotifier(uc, noop.NewProvider())

	err := notifier.Handle(context.Background(), stubEvent{eventType: "card.invoice_due.v1", payload: env})
	s.Require().NoError(err)
	s.True(uc.called)
	s.Equal(userID, uc.input.UserID)
	s.Equal(cardID, uc.input.CardID)
	s.Equal("Nubank", uc.input.CardName)
	s.Equal(int64(500000), uc.input.LimitCents)
	s.Equal(3, uc.input.DaysUntil)
	s.True(uc.input.DueDate.Equal(time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)))
}

func (s *InvoiceDueNotifierSuite) TestHandleUnknownEventType() {
	uc := &fakeNotifyInvoiceDueUC{}
	notifier := consumers.NewInvoiceDueNotifier(uc, noop.NewProvider())

	err := notifier.Handle(context.Background(), stubEvent{eventType: "other.event", payload: outbox.Envelope{}})
	s.Require().Error(err)
	s.False(uc.called)
}
