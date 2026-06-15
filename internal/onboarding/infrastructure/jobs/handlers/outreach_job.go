package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/job"
)

const outreachJobSchedule = "5 * * * *"

type OutreachJob struct {
	useCase *usecases.SendOutreach
	enabled bool
}

func NewOutreachJob(useCase *usecases.SendOutreach, enabled bool) *job.Adapter {
	h := &OutreachJob{
		useCase: useCase,
		enabled: enabled,
	}
	return job.NewAdapterWithTimeout("onboarding.outreach_job", outreachJobSchedule, h.run, 2*time.Minute)
}

func (j *OutreachJob) run(ctx context.Context) error {
	if !j.enabled {
		slog.InfoContext(ctx, "onboarding.outreach_job.skipped", "reason", "outreach_disabled")
		return nil
	}

	if err := j.useCase.Execute(ctx); err != nil {
		return fmt.Errorf("onboarding: outreach job: %w", err)
	}

	return nil
}
