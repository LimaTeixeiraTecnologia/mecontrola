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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type transactionUpdatedConsumerSuite struct {
	suite.Suite
}

func TestTransactionUpdatedConsumer(t *testing.T) {
	suite.Run(t, new(transactionUpdatedConsumerSuite))
}

func (s *transactionUpdatedConsumerSuite) buildEnvelope(direction int, aggregateID, userID, subcategoryID, refMonth string, amountCents int64) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"aggregate_id":   aggregateID,
		"user_id":        userID,
		"occurred_at":    time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		"direction":      direction,
		"amount_cents":   amountCents,
		"ref_month":      refMonth,
		"subcategory_id": subcategoryID,
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *transactionUpdatedConsumerSuite) TestOutcome_ReconcilesExpense() {
	aggregateID := uuid.New().String()
	userID := uuid.New().String()
	subcategoryID := uuid.New().String()
	env := s.buildEnvelope(2, aggregateID, userID, subcategoryID, "2026-06", 7000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionUpdatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: env})
	s.Require().NoError(err)

	s.Equal(1, upsert.calls)
	s.Equal(userID, upsert.capturedInput.UserID)
	s.Equal("transactions", upsert.capturedInput.Source)
	s.Equal(aggregateID, upsert.capturedInput.ExternalTransactionID)
	s.Equal(subcategoryID, upsert.capturedInput.SubcategoryID)
	s.Equal("2026-06", upsert.capturedInput.Competence)
	s.Equal(int64(7000), upsert.capturedInput.AmountCents)
	s.True(upsert.capturedInput.Reconcile)
}

func (s *transactionUpdatedConsumerSuite) TestIncome_NoOp() {
	env := s.buildEnvelope(1, uuid.New().String(), uuid.New().String(), uuid.New().String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionUpdatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(0, upsert.calls)
}

func (s *transactionUpdatedConsumerSuite) TestOutcome_MissingSubcategory_NoOp() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), uuid.Nil.String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionUpdatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(0, upsert.calls)
}

func (s *transactionUpdatedConsumerSuite) TestOutcome_EmptySubcategory_NoOp() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), "", "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionUpdatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(0, upsert.calls)
}

func (s *transactionUpdatedConsumerSuite) TestTombstoneConflict_NoError() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), uuid.New().String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{err: appinterfaces.ErrExpenseTombstoneConflict}
	consumer := consumers.NewTransactionUpdatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(1, upsert.calls)
}

func (s *transactionUpdatedConsumerSuite) TestInvalidPayloadType_ReturnsError() {
	consumer := consumers.NewTransactionUpdatedConsumer(&fakeUpsertExpense{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: "not-envelope"})
	s.Require().Error(err)
}

func (s *transactionUpdatedConsumerSuite) TestUpsertError_Propagated() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), uuid.New().String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{err: appinterfaces.ErrExpenseConflict}
	consumer := consumers.NewTransactionUpdatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.updated.v1", payload: env})
	s.Require().Error(err)
}
