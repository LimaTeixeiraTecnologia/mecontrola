package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CronInternalSuite struct {
	suite.Suite
}

func TestCronInternal(t *testing.T) {
	suite.Run(t, new(CronInternalSuite))
}

func (s *CronInternalSuite) TestStopReturnsContextErrorWhenJobIsStillRunning() {
	cron, err := NewCron(CronDeps{
		Storage:              noopStorage{},
		HousekeepingSchedule: "@daily",
		ReaperInterval:       "@every 1m",
		RetentionDays:        90,
		ReaperStuckAfter:     5 * time.Minute,
	})
	s.Require().NoError(err)

	started := make(chan struct{})
	release := make(chan struct{})
	var closeStarted sync.Once
	_, err = cron.inner.AddFunc("@every 100ms", func() {
		closeStarted.Do(func() { close(started) })
		<-release
	})
	s.Require().NoError(err)

	cron.inner.Start()
	defer close(release)

	select {
	case <-started:
	case <-time.After(time.Second):
		s.FailNow("job de teste nao iniciou")
	}

	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err = cron.Stop(stopCtx)
	s.ErrorIs(err, context.Canceled)
}

func (s *CronInternalSuite) TestStopReturnsNilWhenJobsDrainBeforeDeadline() {
	cron, err := NewCron(CronDeps{
		Storage:              noopStorage{},
		HousekeepingSchedule: "@daily",
		ReaperInterval:       "@every 1m",
		RetentionDays:        90,
		ReaperStuckAfter:     5 * time.Minute,
	})
	s.Require().NoError(err)

	s.Require().NoError(cron.Start(context.Background()))

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = cron.Stop(stopCtx)
	s.NoError(err)
	s.False(errors.Is(err, context.Canceled))
}
