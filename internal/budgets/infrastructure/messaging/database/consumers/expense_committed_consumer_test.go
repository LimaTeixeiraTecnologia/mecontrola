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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeEvaluateAlert struct {
	calls    int
	captured usecases.EvaluateAlertInput
	err      error
}

func (f *fakeEvaluateAlert) Execute(_ context.Context, in usecases.EvaluateAlertInput) error {
	f.calls++
	f.captured = in
	return f.err
}

type expenseCommittedConsumerSuite struct {
	suite.Suite
}

func TestExpenseCommittedConsumer(t *testing.T) {
	suite.Run(t, new(expenseCommittedConsumerSuite))
}

func (s *expenseCommittedConsumerSuite) buildEnvelope(userID, competence, rootSlug, mutationKind, committedAt, cutoff string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"user_id":              userID,
		"competence":           competence,
		"root_slug":            rootSlug,
		"mutation_kind":        mutationKind,
		"committed_at":         committedAt,
		"cutoff_competence_br": cutoff,
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *expenseCommittedConsumerSuite) TestSuccess_CallsEvaluateAlertWithCorrectInput() {
	userID := uuid.New()
	committedAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	env := s.buildEnvelope(
		userID.String(),
		"2026-01",
		"expense.prazeres",
		"debit",
		committedAt.Format(time.RFC3339),
		"2026-01",
	)

	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   env,
	})

	s.Require().NoError(err)
	s.Equal(1, fake.calls)
	s.Equal(userID, fake.captured.UserID)
	s.Equal("2026-01", fake.captured.Competence.String())
	s.Equal(valueobjects.RootSlugPrazeres, fake.captured.RootSlug)
	s.Equal(committedAt.UTC(), fake.captured.CommittedAt)
	s.Equal("2026-01", fake.captured.CutoffCompetenceBR.String())
	s.Equal(env.ID, fake.captured.EventID)
}

func (s *expenseCommittedConsumerSuite) TestWrongEventType_ReturnsError_NeverCallsUseCase() {
	env := s.buildEnvelope(
		uuid.New().String(),
		"2026-01",
		"expense.prazeres",
		"debit",
		time.Now().UTC().Format(time.RFC3339),
		"2026-01",
	)

	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.other.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *expenseCommittedConsumerSuite) TestPayloadNotEnvelope_ReturnsError() {
	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   "not-envelope",
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *expenseCommittedConsumerSuite) TestInvalidUserID_ReturnsError() {
	env := s.buildEnvelope(
		"not-a-uuid",
		"2026-01",
		"expense.prazeres",
		"debit",
		time.Now().UTC().Format(time.RFC3339),
		"2026-01",
	)

	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *expenseCommittedConsumerSuite) TestInvalidCompetence_ReturnsError() {
	env := s.buildEnvelope(
		uuid.New().String(),
		"invalid-competence",
		"expense.prazeres",
		"debit",
		time.Now().UTC().Format(time.RFC3339),
		"2026-01",
	)

	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *expenseCommittedConsumerSuite) TestInvalidRootSlug_ReturnsError() {
	env := s.buildEnvelope(
		uuid.New().String(),
		"2026-01",
		"expense.unknown_slug",
		"debit",
		time.Now().UTC().Format(time.RFC3339),
		"2026-01",
	)

	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *expenseCommittedConsumerSuite) TestInvalidCommittedAt_ReturnsError() {
	env := s.buildEnvelope(
		uuid.New().String(),
		"2026-01",
		"expense.prazeres",
		"debit",
		"not-a-date",
		"2026-01",
	)

	fake := &fakeEvaluateAlert{}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(0, fake.calls)
}

func (s *expenseCommittedConsumerSuite) TestUseCaseError_PropagatesError() {
	env := s.buildEnvelope(
		uuid.New().String(),
		"2026-01",
		"expense.prazeres",
		"debit",
		time.Now().UTC().Format(time.RFC3339),
		"2026-01",
	)

	fake := &fakeEvaluateAlert{err: errors.New("evaluate failed")}
	consumer := consumers.NewExpenseCommittedConsumer(fake, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{
		eventType: "budgets.expense.committed.v1",
		payload:   env,
	})

	s.Require().Error(err)
	s.Equal(1, fake.calls)
}
