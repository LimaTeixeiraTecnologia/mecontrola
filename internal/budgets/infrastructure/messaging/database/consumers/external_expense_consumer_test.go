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

type fakeIngestExternalExpense struct {
	calls int
	input usecases.IngestExternalExpenseInput
	err   error
}

func (f *fakeIngestExternalExpense) Execute(_ context.Context, in usecases.IngestExternalExpenseInput) error {
	f.calls++
	f.input = in
	return f.err
}

type ExternalExpenseConsumerSuite struct {
	suite.Suite
}

func TestExternalExpenseConsumerSuite(t *testing.T) {
	suite.Run(t, new(ExternalExpenseConsumerSuite))
}

func (s *ExternalExpenseConsumerSuite) TestHandle_HappyPath() {
	payload := map[string]any{
		"event_id":                uuid.New().String(),
		"source":                  "kiwify",
		"external_transaction_id": uuid.New().String(),
		"occurred_at":             time.Date(2026, 6, 17, 15, 0, 0, 0, time.UTC),
		"user_id":                 uuid.New().String(),
		"operation":               "create",
		"version":                 1,
		"subcategory_id":          uuid.New().String(),
		"competence":              "2026-06",
		"amount_cents":            int64(5800),
	}
	raw, _ := json.Marshal(payload)

	ingest := &fakeIngestExternalExpense{}
	consumer := consumers.NewExternalExpenseConsumer(ingest, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "external.expense.v1",
		payload:   outbox.Envelope{Payload: raw},
	})

	s.Require().NoError(err)
	s.Equal(1, ingest.calls)
	s.Equal("kiwify", ingest.input.Source)
	s.Equal("create", ingest.input.Operation)
	s.Equal("2026-06", ingest.input.Competence)
	s.Equal(int64(5800), ingest.input.AmountCents)
}

func (s *ExternalExpenseConsumerSuite) TestHandle_InvalidPayloadType() {
	consumer := consumers.NewExternalExpenseConsumer(&fakeIngestExternalExpense{}, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "external.expense.v1",
		payload:   "not-envelope",
	})

	s.Error(err)
}

func (s *ExternalExpenseConsumerSuite) TestHandle_UseCaseError() {
	payload := map[string]any{
		"event_id":                uuid.New().String(),
		"source":                  "kiwify",
		"external_transaction_id": uuid.New().String(),
		"occurred_at":             time.Date(2026, 6, 17, 15, 0, 0, 0, time.UTC),
		"user_id":                 uuid.New().String(),
		"operation":               "delete",
		"version":                 2,
		"subcategory_id":          uuid.New().String(),
		"competence":              "2026-06",
		"amount_cents":            int64(5800),
	}
	raw, _ := json.Marshal(payload)

	ingest := &fakeIngestExternalExpense{err: errors.New("downstream failed")}
	consumer := consumers.NewExternalExpenseConsumer(ingest, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "external.expense.v1",
		payload:   outbox.Envelope{Payload: raw},
	})

	s.Error(err)
	s.Equal(1, ingest.calls)
}
