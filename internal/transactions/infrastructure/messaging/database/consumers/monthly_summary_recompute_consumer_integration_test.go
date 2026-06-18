//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	consumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers"
)

type MonthlySummaryRecomputeConsumerSuite struct {
	suite.Suite
}

func TestMonthlySummaryRecomputeConsumerSuite(t *testing.T) {
	suite.Run(t, new(MonthlySummaryRecomputeConsumerSuite))
}

type fakeRecompute struct {
	calls int64
}

func (f *fakeRecompute) Execute(_ context.Context, _ usecases.RecomputeMonthlySummaryInput) error {
	atomic.AddInt64(&f.calls, 1)
	return nil
}

func makeEnvelope(userID uuid.UUID, refMonths []string) outbox.Envelope {
	payload := map[string]any{
		"user_id":             userID.String(),
		"ref_months_affected": refMonths,
	}
	b, _ := json.Marshal(payload)
	return outbox.Envelope{
		ID:        uuid.New().String(),
		EventType: "transactions.transaction.created.v1",
		Payload:   b,
	}
}

func makeSingleEnvelope(userID uuid.UUID, refMonth string) outbox.Envelope {
	payload := map[string]any{
		"user_id":   userID.String(),
		"ref_month": refMonth,
	}
	b, _ := json.Marshal(payload)
	return outbox.Envelope{
		ID:        uuid.New().String(),
		EventType: "transactions.transaction.created.v1",
		Payload:   b,
	}
}

func (s *MonthlySummaryRecomputeConsumerSuite) TestCoalesces_10EventsSameKey_To1Recompute() {
	fake := &fakeRecompute{}
	window := 200 * time.Millisecond
	c := consumers.NewMonthlySummaryRecomputeConsumer(fake, window, noop.NewProvider())

	userID := uuid.New()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		env := makeEnvelope(userID, []string{"2026-06"})
		event := mocks.NewEvent(s.T())
		event.EXPECT().GetPayload().Return(env)
		s.Require().NoError(c.Handle(ctx, event))
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(window + 100*time.Millisecond)
	s.Equal(int64(1), atomic.LoadInt64(&fake.calls))
}

func (s *MonthlySummaryRecomputeConsumerSuite) TestDistinctKeys_DoNotCoalesce() {
	fake := &fakeRecompute{}
	window := 200 * time.Millisecond
	c := consumers.NewMonthlySummaryRecomputeConsumer(fake, window, noop.NewProvider())

	ctx := context.Background()
	months := []string{"2026-01", "2026-02", "2026-03"}

	for _, m := range months {
		userID := uuid.New()
		env := makeSingleEnvelope(userID, m)
		event := mocks.NewEvent(s.T())
		event.EXPECT().GetPayload().Return(env)
		s.Require().NoError(c.Handle(ctx, event))
	}

	time.Sleep(window + 100*time.Millisecond)
	s.Equal(int64(3), atomic.LoadInt64(&fake.calls))
}

func (s *MonthlySummaryRecomputeConsumerSuite) TestIdempotency_SameEventIDTwice_CallsUseCaseOnce() {
	fake := &fakeRecompute{}
	window := 200 * time.Millisecond
	c := consumers.NewMonthlySummaryRecomputeConsumer(fake, window, noop.NewProvider())

	userID := uuid.New()
	ctx := context.Background()

	eventID := uuid.New().String()
	payload := map[string]any{
		"user_id":   userID.String(),
		"ref_month": "2026-06",
	}
	b, _ := json.Marshal(payload)
	env := outbox.Envelope{
		ID:        eventID,
		EventType: "transactions.transaction.created.v1",
		Payload:   b,
	}

	for range 2 {
		event := mocks.NewEvent(s.T())
		event.EXPECT().GetPayload().Return(env)
		s.Require().NoError(c.Handle(ctx, event))
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(window + 100*time.Millisecond)
	s.Equal(int64(1), atomic.LoadInt64(&fake.calls), "mesmo event_id processado duas vezes deve resultar em apenas uma chamada ao use case")
}

func (s *MonthlySummaryRecomputeConsumerSuite) TestStop_DrainsPendingTimers() {
	fake := &fakeRecompute{}
	window := 2 * time.Second
	c := consumers.NewMonthlySummaryRecomputeConsumer(fake, window, noop.NewProvider())

	userID := uuid.New()
	ctx := context.Background()

	env := makeSingleEnvelope(userID, "2026-06")
	event := mocks.NewEvent(s.T())
	event.EXPECT().GetPayload().Return(env)
	s.Require().NoError(c.Handle(ctx, event))

	c.Stop(ctx)

	s.Equal(int64(1), atomic.LoadInt64(&fake.calls))
}
