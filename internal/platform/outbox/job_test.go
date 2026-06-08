package outbox_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type JobSuite struct {
	suite.Suite
	cfg configs.OutboxConfig
}

func TestJob(t *testing.T) {
	suite.Run(t, new(JobSuite))
}

func (s *JobSuite) SetupTest() {
	s.cfg = configs.OutboxConfig{
		DispatcherTickInterval:    500 * time.Millisecond,
		DispatcherBatchSize:       50,
		DispatcherHandlerTimeout:  10 * time.Second,
		RetryMaxAttempts:          15,
		RetryBaseBackoff:          2 * time.Second,
		RetryMaxBackoff:           5 * time.Minute,
		ReaperInterval:            "@every 1m",
		ReaperStuckAfter:          5 * time.Minute,
		HousekeepingSchedule:      "@daily",
		HousekeepingRetentionDays: 90,
	}
}

func (s *JobSuite) TestJobs() {
	type args struct {
		ctx context.Context
	}

	scenarios := []struct {
		name  string
		args  args
		setup func() (interface {
			Name() string
			Schedule() string
		}, func() error)
		expect func(interface {
			Name() string
			Schedule() string
		}, error)
	}{
		{
			name: "deve expor nome e schedule do dispatcher",
			args: args{ctx: context.Background()},
			setup: func() (interface {
				Name() string
				Schedule() string
			}, func() error) {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())
				jobInstance := outbox.NewDispatcherJob(&unitOfWorkRows{}, factory, registry, s.cfg, noopLogger{}, rand.New(rand.NewSource(0)))
				return jobInstance, func() error { return nil }
			},
			expect: func(jobInstance interface {
				Name() string
				Schedule() string
			}, err error) {
				s.NoError(err)
				s.Equal("outbox-dispatcher", jobInstance.Name())
				s.Equal("@every 500ms", jobInstance.Schedule())
			},
		},
		{
			name: "deve delegar run do dispatcher para run once",
			args: args{ctx: context.Background()},
			setup: func() (interface {
				Name() string
				Schedule() string
			}, func() error) {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 50).Return(nil, nil).Once()
				registry.EXPECT().HandlersOf(mock.Anything).Maybe().Return([]events.Handler{})
				jobInstance := outbox.NewDispatcherJob(&unitOfWorkRows{}, factory, registry, s.cfg, noopLogger{}, rand.New(rand.NewSource(0)))
				return jobInstance, func() error { return jobInstance.Run(context.Background()) }
			},
			expect: func(_ interface {
				Name() string
				Schedule() string
			}, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve expor nome e schedule do reaper",
			args: args{ctx: context.Background()},
			setup: func() (interface {
				Name() string
				Schedule() string
			}, func() error) {
				jobInstance := outbox.NewReaperJob(&unitOfWorkVoid{}, outboxmocks.NewOutboxRepositoryFactory(s.T()), s.cfg, noopLogger{})
				return jobInstance, func() error { return nil }
			},
			expect: func(jobInstance interface {
				Name() string
				Schedule() string
			}, err error) {
				s.NoError(err)
				s.Equal("outbox-reaper", jobInstance.Name())
				s.Equal("@every 1m", jobInstance.Schedule())
			},
		},
		{
			name: "deve expor nome e schedule do housekeeping",
			args: args{ctx: context.Background()},
			setup: func() (interface {
				Name() string
				Schedule() string
			}, func() error) {
				jobInstance := outbox.NewHousekeepingJob(&unitOfWorkVoid{}, outboxmocks.NewOutboxRepositoryFactory(s.T()), s.cfg, noopLogger{})
				return jobInstance, func() error { return nil }
			},
			expect: func(jobInstance interface {
				Name() string
				Schedule() string
			}, err error) {
				s.NoError(err)
				s.Equal("outbox-housekeeping", jobInstance.Name())
				s.Equal("@daily", jobInstance.Schedule())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			jobInstance, act := scenario.setup()
			err := act()
			scenario.expect(jobInstance, err)
		})
	}
}
