package outbox_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type JobSuite struct {
	suite.Suite
	storage *outboxmocks.Storage
	cfg     configs.OutboxConfig
}

func TestJob(t *testing.T) {
	suite.Run(t, new(JobSuite))
}

func (s *JobSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
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

func (s *JobSuite) TestDispatcherJob_NameAndSchedule() {
	rng := rand.New(rand.NewSource(0))
	reg := &fakeRegistry{}
	j := outbox.NewDispatcherJob(s.storage, reg, s.cfg, noopLogger{}, rng)

	s.Equal("outbox-dispatcher", j.Name())
	s.Equal("@every 500ms", j.Schedule())
}

func (s *JobSuite) TestDispatcherJob_RunDelegatesParaRunOnce() {
	rng := rand.New(rand.NewSource(0))
	reg := &fakeRegistry{}
	j := outbox.NewDispatcherJob(s.storage, reg, s.cfg, noopLogger{}, rng)

	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 50).Return(nil, nil)

	err := j.Run(context.Background())
	s.NoError(err)
}

func (s *JobSuite) TestReaperJob_NameAndSchedule() {
	j := outbox.NewReaperJob(s.storage, s.cfg, noopLogger{})

	s.Equal("outbox-reaper", j.Name())
	s.Equal("@every 1m", j.Schedule())
}

func (s *JobSuite) TestHousekeepingJob_NameAndSchedule() {
	j := outbox.NewHousekeepingJob(s.storage, s.cfg, noopLogger{})

	s.Equal("outbox-housekeeping", j.Name())
	s.Equal("@daily", j.Schedule())
}
