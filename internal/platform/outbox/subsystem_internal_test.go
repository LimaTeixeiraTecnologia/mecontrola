package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

type SubsystemInternalSuite struct {
	suite.Suite
}

func TestSubsystemInternal(t *testing.T) {
	suite.Run(t, new(SubsystemInternalSuite))
}

func (s *SubsystemInternalSuite) TestNewSubsystemConnectsMetricsAndTracerToDispatcher() {
	obs := fake.NewProvider()
	metrics, err := NewOutboxMetrics(obs)
	s.Require().NoError(err)

	subsystem, err := NewSubsystem(SubsystemDeps{
		Config: configs.OutboxConfig{
			DispatcherEnabled:         true,
			DispatcherTickInterval:    10 * time.Millisecond,
			DispatcherBatchSize:       10,
			DispatcherHandlerTimeout:  100 * time.Millisecond,
			RetryMaxAttempts:          5,
			RetryBaseBackoff:          time.Second,
			RetryMaxBackoff:           10 * time.Second,
			HousekeepingRetentionDays: 30,
			HousekeepingSchedule:      "@daily",
			ReaperInterval:            "@every 1m",
			ReaperStuckAfter:          5 * time.Minute,
		},
		Storage:  noopStorage{},
		Registry: NewRegistry(),
		Metrics:  metrics,
	})
	s.Require().NoError(err)
	s.IsType(&outboxMetricsAdapter{}, subsystem.dispatcher.metrics)
	s.NotNil(subsystem.dispatcher.tracer)
}

type noopStorage struct{}

func (noopStorage) InsertEvent(context.Context, database.DBTX, Event) error { return nil }

func (noopStorage) InsertDeliveries(context.Context, database.DBTX, events.EventID, []SubscriptionName) error {
	return nil
}

func (noopStorage) ClaimReady(context.Context, int, string) ([]Claim, error) { return nil, nil }

func (noopStorage) MarkProcessed(context.Context, ClaimID, time.Time) error { return nil }

func (noopStorage) MarkFailed(context.Context, ClaimID, string, Attempt, time.Time) error {
	return nil
}

func (noopStorage) MarkDLQ(context.Context, ClaimID, string, time.Time) error { return nil }

func (noopStorage) ReleaseStuck(context.Context, time.Time) (int64, error) { return 0, nil }

func (noopStorage) PurgeOlderThan(context.Context, time.Time) (int64, error) { return 0, nil }

func (noopStorage) Stats(context.Context) (Stats, error) { return Stats{}, nil }
