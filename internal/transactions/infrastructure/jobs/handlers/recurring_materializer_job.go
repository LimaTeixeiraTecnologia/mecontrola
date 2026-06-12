package handlers

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type materializeRecurringForDayUseCase interface {
	Execute(ctx context.Context, today time.Time) error
}

type RecurringMaterializerJob struct {
	usecase   materializeRecurringForDayUseCase
	brazilLoc *time.Location
	schedule  string
}

func NewRecurringMaterializerJob(
	uc materializeRecurringForDayUseCase,
	brazilLoc *time.Location,
	cfg configs.TransactionsConfig,
) *RecurringMaterializerJob {
	schedule := cfg.RecurringMaterializerCron
	if schedule == "" {
		schedule = "@daily"
	}
	return &RecurringMaterializerJob{
		usecase:   uc,
		brazilLoc: brazilLoc,
		schedule:  schedule,
	}
}

func (j *RecurringMaterializerJob) Name() string     { return "transactions-recurring-materializer" }
func (j *RecurringMaterializerJob) Schedule() string { return j.schedule }

func (j *RecurringMaterializerJob) Run(ctx context.Context) error {
	today := time.Now().In(j.brazilLoc)
	return j.usecase.Execute(ctx, today)
}
