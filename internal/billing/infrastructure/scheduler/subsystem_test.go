package scheduler_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/scheduler"
)

type BillingSchedulerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestBillingScheduler(t *testing.T) {
	suite.Run(t, new(BillingSchedulerSuite))
}

func (s *BillingSchedulerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *BillingSchedulerSuite) TestStartAgenda2JobsSemErro() {
	sched := scheduler.NewBillingScheduler(scheduler.Deps{
		ReconcileUseCase:  nil,
		AnonymizeUseCase:  nil,
		ReconcileSchedule: "@every 1h",
		AnonymizeSchedule: "@daily",
		Logger:            slog.Default(),
	})

	err := sched.Start(s.ctx)
	s.NoError(err)

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	err = sched.Stop(ctx)
	s.NoError(err)
}

func (s *BillingSchedulerSuite) TestScheduleInvalidoRetornaErro() {
	sched := scheduler.NewBillingScheduler(scheduler.Deps{
		ReconcileUseCase:  nil,
		AnonymizeUseCase:  nil,
		ReconcileSchedule: "invalid-schedule",
		AnonymizeSchedule: "@daily",
		Logger:            slog.Default(),
	})

	err := sched.Start(s.ctx)
	s.Error(err)
}

func (s *BillingSchedulerSuite) TestStopSemStartNaoRetornaErro() {
	sched := scheduler.NewBillingScheduler(scheduler.Deps{
		ReconcileUseCase:  nil,
		AnonymizeUseCase:  nil,
		ReconcileSchedule: "@every 1h",
		AnonymizeSchedule: "@daily",
		Logger:            slog.Default(),
	})

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	err := sched.Stop(ctx)
	s.NoError(err, "Stop sem Start não deve retornar erro")
}
