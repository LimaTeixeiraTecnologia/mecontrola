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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeDeleteExpense struct {
	calls        int
	capturedUser string
	capturedExt  string
	err          error
}

func (f *fakeDeleteExpense) ExecuteByExternalID(_ context.Context, userID, _ string, externalTransactionID string) error {
	f.calls++
	f.capturedUser = userID
	f.capturedExt = externalTransactionID
	return f.err
}

type transactionDeletedConsumerSuite struct {
	suite.Suite
}

func TestTransactionDeletedConsumer(t *testing.T) {
	suite.Run(t, new(transactionDeletedConsumerSuite))
}

func (s *transactionDeletedConsumerSuite) buildEnvelope(aggregateID, userID string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"aggregate_id": aggregateID,
		"user_id":      userID,
		"occurred_at":  time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *transactionDeletedConsumerSuite) TestSuccess_CallsDeleteWithCorrectArgs() {
	aggregateID := uuid.New().String()
	userID := uuid.New().String()
	env := s.buildEnvelope(aggregateID, userID)

	fake := &fakeDeleteExpense{}
	consumer := consumers.NewTransactionDeletedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "transactions.transaction.deleted.v1",
		payload:   env,
	})

	s.Require().NoError(err)
	s.Equal(1, fake.calls)
	s.Equal(userID, fake.capturedUser)
	s.Equal(aggregateID, fake.capturedExt)
}

func (s *transactionDeletedConsumerSuite) TestInvalidJSON_ReturnsError_NeverCallsUseCase() {
	env := outbox.Envelope{ID: uuid.New().String(), Payload: []byte(`{not valid json`)}

	fake := &fakeDeleteExpense{}
	consumer := consumers.NewTransactionDeletedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "transactions.transaction.deleted.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *transactionDeletedConsumerSuite) TestAggregateIDNotUUID_ReturnsError_NeverCallsUseCase() {
	env := s.buildEnvelope("not-a-uuid", uuid.New().String())

	fake := &fakeDeleteExpense{}
	consumer := consumers.NewTransactionDeletedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "transactions.transaction.deleted.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *transactionDeletedConsumerSuite) TestUnexpectedPayloadType_ReturnsError() {
	fake := &fakeDeleteExpense{}
	consumer := consumers.NewTransactionDeletedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "transactions.transaction.deleted.v1",
		payload:   "not-envelope",
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *transactionDeletedConsumerSuite) TestUseCaseError_PropagatesError() {
	aggregateID := uuid.New().String()
	userID := uuid.New().String()
	env := s.buildEnvelope(aggregateID, userID)

	fake := &fakeDeleteExpense{err: errors.New("downstream failure")}
	consumer := consumers.NewTransactionDeletedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "transactions.transaction.deleted.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(1, fake.calls)
}
