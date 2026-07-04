package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeUpsertExpense struct {
	calls         int
	capturedInput input.UpsertExpenseInput
	allInputs     []input.UpsertExpenseInput
	err           error
}

func (f *fakeUpsertExpense) Execute(_ context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
	f.calls++
	f.capturedInput = in
	f.allInputs = append(f.allInputs, in)
	return output.ExpenseOutput{}, f.err
}

type transactionCreatedConsumerSuite struct {
	suite.Suite
}

func TestTransactionCreatedConsumer(t *testing.T) {
	suite.Run(t, new(transactionCreatedConsumerSuite))
}

func (s *transactionCreatedConsumerSuite) buildEnvelope(direction int, aggregateID, userID, subcategoryID, refMonth string, amountCents int64) outbox.Envelope {
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

func (s *transactionCreatedConsumerSuite) TestOutcome_CreatesExpense() {
	aggregateID := uuid.New().String()
	userID := uuid.New().String()
	subcategoryID := uuid.New().String()
	env := s.buildEnvelope(2, aggregateID, userID, subcategoryID, "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().NoError(err)

	s.Equal(1, upsert.calls)
	s.Equal(userID, upsert.capturedInput.UserID)
	s.Equal("transactions", upsert.capturedInput.Source)
	s.Equal(aggregateID, upsert.capturedInput.ExternalTransactionID)
	s.Equal(subcategoryID, upsert.capturedInput.SubcategoryID)
	s.Equal("2026-06", upsert.capturedInput.Competence)
	s.Equal(int64(5000), upsert.capturedInput.AmountCents)
}

func (s *transactionCreatedConsumerSuite) TestCreditCard_SpreadsInstallmentsPerMonth() {
	userID := uuid.New().String()
	subcategoryID := uuid.New().String()
	item1 := uuid.New().String()
	item2 := uuid.New().String()
	item3 := uuid.New().String()
	raw, _ := json.Marshal(map[string]any{
		"aggregate_id":   uuid.New().String(),
		"user_id":        userID,
		"occurred_at":    time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		"direction":      2,
		"amount_cents":   9000,
		"ref_month":      "2026-06",
		"subcategory_id": subcategoryID,
		"installments": []map[string]any{
			{"item_id": item1, "ref_month": "2026-06", "amount_cents": 3000, "index": 1},
			{"item_id": item2, "ref_month": "2026-07", "amount_cents": 3000, "index": 2},
			{"item_id": item3, "ref_month": "2026-08", "amount_cents": 3000, "index": 3},
		},
	})
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().NoError(err)

	s.Require().Equal(3, upsert.calls)
	for _, in := range upsert.allInputs {
		s.Equal("transactions_card", in.Source)
		s.Equal(userID, in.UserID)
		s.Equal(subcategoryID, in.SubcategoryID)
		s.Equal(int64(3000), in.AmountCents)
	}
	s.Equal(item1, upsert.allInputs[0].ExternalTransactionID)
	s.Equal("2026-06", upsert.allInputs[0].Competence)
	s.Equal(item2, upsert.allInputs[1].ExternalTransactionID)
	s.Equal("2026-07", upsert.allInputs[1].Competence)
	s.Equal(item3, upsert.allInputs[2].ExternalTransactionID)
	s.Equal("2026-08", upsert.allInputs[2].Competence)
}

func (s *transactionCreatedConsumerSuite) TestIncome_NoOp() {
	env := s.buildEnvelope(1, uuid.New().String(), uuid.New().String(), uuid.New().String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(0, upsert.calls)
}

func (s *transactionCreatedConsumerSuite) TestOutcome_MissingSubcategory_NoOp() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), uuid.Nil.String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(0, upsert.calls)
}

func (s *transactionCreatedConsumerSuite) TestOutcome_EmptySubcategory_NoOp() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), "", "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(0, upsert.calls)
}

func (s *transactionCreatedConsumerSuite) TestRedelivery_IsIdempotent() {
	aggregateID := uuid.New().String()
	userID := uuid.New().String()
	subcategoryID := uuid.New().String()
	env := s.buildEnvelope(2, aggregateID, userID, subcategoryID, "2026-06", 5000)

	upsert := &fakeUpsertExpense{}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())
	evt := stubEvent{eventType: "transactions.transaction.created.v1", payload: env}

	s.Require().NoError(consumer.Handle(context.Background(), evt))
	s.Require().NoError(consumer.Handle(context.Background(), evt))

	s.Equal(2, upsert.calls)
	s.Equal(aggregateID, upsert.capturedInput.ExternalTransactionID)
}

func (s *transactionCreatedConsumerSuite) TestTombstoneConflict_NoError() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), uuid.New().String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{err: appinterfaces.ErrExpenseTombstoneConflict}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().NoError(err)
	s.Equal(1, upsert.calls)
}

func (s *transactionCreatedConsumerSuite) TestInvalidPayloadType_ReturnsError() {
	consumer := consumers.NewTransactionCreatedConsumer(&fakeUpsertExpense{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: "not-envelope"})
	s.Require().Error(err)
}

func (s *transactionCreatedConsumerSuite) TestUpsertError_Propagated() {
	env := s.buildEnvelope(2, uuid.New().String(), uuid.New().String(), uuid.New().String(), "2026-06", 5000)

	upsert := &fakeUpsertExpense{err: appinterfaces.ErrExpenseConflict}
	consumer := consumers.NewTransactionCreatedConsumer(upsert, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "transactions.transaction.created.v1", payload: env})
	s.Require().Error(err)
}
