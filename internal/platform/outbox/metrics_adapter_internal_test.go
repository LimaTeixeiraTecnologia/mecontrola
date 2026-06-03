package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

type MetricsAdapterSuite struct {
	suite.Suite
	ctx         context.Context
	fakeMetrics *fake.FakeMetrics
	adapter     Metrics
}

func TestMetricsAdapter(t *testing.T) {
	suite.Run(t, new(MetricsAdapterSuite))
}

func (s *MetricsAdapterSuite) SetupTest() {
	obs := fake.NewProvider()
	metrics, err := NewOutboxMetrics(obs)
	s.Require().NoError(err)

	s.ctx = context.Background()
	s.fakeMetrics = obs.Metrics().(*fake.FakeMetrics)
	s.adapter = newMetricsAdapter(metrics)
}

func (s *MetricsAdapterSuite) TestDeliverySuccessRecordsProcessedAndLatency() {
	recorder := s.adapter.(deliveryMetricsRecorder)
	recorder.recordDeliverySuccess(s.ctx, "test-subscription", 123)

	counter := s.fakeMetrics.GetCounter("outbox.deliveries.processed.total")
	s.Require().NotNil(counter)
	s.Require().Len(counter.GetValues(), 1)
	s.Equal(int64(1), counter.GetValues()[0].Value)

	histogram := s.fakeMetrics.GetHistogram("outbox.delivery.latency_ms")
	s.Require().NotNil(histogram)
	s.Require().Len(histogram.GetValues(), 1)
	s.Equal(float64(123), histogram.GetValues()[0].Value)
}

func (s *MetricsAdapterSuite) TestFailureDLQPollAndPendingAreForwarded() {
	deliveryRecorder := s.adapter.(deliveryMetricsRecorder)
	pollRecorder := s.adapter.(pollMetricsRecorder)
	pendingRecorder := s.adapter.(pendingMetricsRecorder)

	deliveryRecorder.recordDeliveryFailure(s.ctx, "test-subscription", errors.New("db timeout"))
	deliveryRecorder.recordDeliveryDLQ(s.ctx, "test-subscription")
	pollRecorder.recordPoll(s.ctx, 17*time.Millisecond, 3)
	pendingRecorder.setPendingDelta(s.ctx, "test-subscription", 4)

	failed := s.fakeMetrics.GetCounter("outbox.deliveries.failed.total")
	s.Require().NotNil(failed)
	s.Require().Len(failed.GetValues(), 1)
	s.Equal(int64(1), failed.GetValues()[0].Value)

	dlq := s.fakeMetrics.GetCounter("outbox.deliveries.dlq.total")
	s.Require().NotNil(dlq)
	s.Require().Len(dlq.GetValues(), 1)
	s.Equal(int64(1), dlq.GetValues()[0].Value)

	pollDuration := s.fakeMetrics.GetHistogram("outbox.poll.duration_ms")
	s.Require().NotNil(pollDuration)
	s.Require().Len(pollDuration.GetValues(), 1)
	s.Equal(float64(17), pollDuration.GetValues()[0].Value)

	pollBatchSize := s.fakeMetrics.GetHistogram("outbox.poll.batch_size")
	s.Require().NotNil(pollBatchSize)
	s.Require().Len(pollBatchSize.GetValues(), 1)
	s.Equal(float64(3), pollBatchSize.GetValues()[0].Value)

	pending := s.fakeMetrics.GetUpDownCounter("outbox.deliveries.pending")
	s.Require().NotNil(pending)
	s.Require().Len(pending.GetValues(), 1)
	s.Equal(int64(4), pending.GetValues()[0].Value)
}

func (s *MetricsAdapterSuite) TestDispatcherCreatesDeliveryAndHandlerSpansFromMetricsAdapter() {
	adapter := s.adapter
	d := &Dispatcher{tracer: adapter.(tracerMetricsProvider).tracer()}

	evtID, err := events.NewEventID("01JTEST000000000000000003")
	s.Require().NoError(err)
	evtType, err := events.NewEventName("order.placed")
	s.Require().NoError(err)
	subName, err := NewSubscriptionName("test-subscription")
	s.Require().NoError(err)
	evt, err := NewEvent(NewEventParams{
		ID:            evtID,
		EventType:     evtType,
		AggregateType: "order",
		AggregateID:   "123",
		Payload:       json.RawMessage(`{"id":"123"}`),
	})
	s.Require().NoError(err)
	claim := Claim{
		ID:               ClaimID(1),
		Event:            evt,
		SubscriptionName: subName,
		Attempt:          NewAttempt(1),
		ClaimedAt:        time.Now().UTC(),
	}

	ctx, deliverSpan := d.startDeliverySpan(s.ctx, claim)
	_, handlerSpan := d.startHandlerSpan(ctx, claim)
	d.endDeliverySpan(handlerSpan, nil)
	d.endDeliverySpan(deliverSpan, nil)

	spans := s.fakeTracer().GetSpans()
	s.Require().Len(spans, 2)

	byName := make(map[string]*fake.FakeSpan, len(spans))
	for _, span := range spans {
		byName[span.Name] = span
	}

	s.Require().NotNil(byName["outbox.deliver"])
	s.Require().NotNil(byName["outbox.handle.test-subscription"])
	s.NotNil(byName["outbox.deliver"].EndTime)
	s.NotNil(byName["outbox.handle.test-subscription"].EndTime)
	s.Equal(observability.StatusCodeOK, byName["outbox.deliver"].Status)
	s.Equal(observability.StatusCodeOK, byName["outbox.handle.test-subscription"].Status)
}

func (s *MetricsAdapterSuite) fakeTracer() *fake.FakeTracer {
	return s.adapter.(tracerMetricsProvider).tracer().(*fake.FakeTracer)
}
