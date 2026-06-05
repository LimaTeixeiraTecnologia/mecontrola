package outbox_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type fakeUoWVoid struct{}

func (f *fakeUoWVoid) Do(ctx context.Context, fn func(context.Context, database.DBTX) (struct{}, error), _ ...uow.Option) (struct{}, error) {
	return fn(ctx, nil)
}

type JobSuite struct {
	suite.Suite
	storage *outboxmocks.Storage
	factory *outboxmocks.OutboxRepositoryFactory
	cfg     configs.OutboxConfig
}

func TestJob(t *testing.T) {
	suite.Run(t, new(JobSuite))
}

func (s *JobSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.factory = outboxmocks.NewOutboxRepositoryFactory(s.T())
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
	fakeUoW := &fakeUoWRows{}
	j := outbox.NewDispatcherJob(fakeUoW, s.factory, reg, s.cfg, noopLogger{}, rng)

	s.Equal("outbox-dispatcher", j.Name())
	s.Equal("@every 500ms", j.Schedule())
}

func (s *JobSuite) TestDispatcherJob_RunDelegatesParaRunOnce() {
	rng := rand.New(rand.NewSource(0))
	reg := &fakeRegistry{}
	fakeUoW := &fakeUoWRows{}
	j := outbox.NewDispatcherJob(fakeUoW, s.factory, reg, s.cfg, noopLogger{}, rng)

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 50).Return(nil, nil)

	err := j.Run(context.Background())
	s.NoError(err)
}

func (s *JobSuite) TestReaperJob_NameAndSchedule() {
	j := outbox.NewReaperJob(&fakeUoWVoid{}, s.factory, s.cfg, noopLogger{})

	s.Equal("outbox-reaper", j.Name())
	s.Equal("@every 1m", j.Schedule())
}

func (s *JobSuite) TestHousekeepingJob_NameAndSchedule() {
	j := outbox.NewHousekeepingJob(&fakeUoWVoid{}, s.factory, s.cfg, noopLogger{})

	s.Equal("outbox-housekeeping", j.Name())
	s.Equal("@daily", j.Schedule())
}
