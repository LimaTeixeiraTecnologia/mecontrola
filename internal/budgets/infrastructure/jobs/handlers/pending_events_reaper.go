package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type runPendingEventsUseCase interface {
	Execute(ctx context.Context) error
}

type PendingEventsReaper struct {
	usecase runPendingEventsUseCase
	cfg     configs.BudgetsConfig
}

func NewPendingEventsReaper(
	usecase runPendingEventsUseCase,
	cfg configs.BudgetsConfig,
) *PendingEventsReaper {
	return &PendingEventsReaper{usecase: usecase, cfg: cfg}
}

func (j *PendingEventsReaper) Name() string           { return "budgets-pending-events-reaper" }
func (j *PendingEventsReaper) Schedule() string       { return j.cfg.PendingReaperInterval }
func (j *PendingEventsReaper) Timeout() time.Duration { return 2 * time.Minute }

func (j *PendingEventsReaper) Run(ctx context.Context) error {
	return j.usecase.Execute(ctx)
}
