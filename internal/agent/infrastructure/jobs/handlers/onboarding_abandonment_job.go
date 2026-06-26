package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/robfig/cron/v3"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const onboardingWorkflowID = "onboarding"

type OnboardingAbandonmentJob struct {
	uow     uow.UnitOfWork
	factory platform.StoreFactory
	cfg     configs.OnboardingConfig
	o11y    observability.Observability
	metrics abandonmentMetrics
}

type abandonmentMetrics struct {
	abandonedTotal observability.Counter
}

func NewOnboardingAbandonmentJob(
	uow uow.UnitOfWork,
	factory platform.StoreFactory,
	cfg configs.OnboardingConfig,
	o11y observability.Observability,
) (*OnboardingAbandonmentJob, error) {
	if uow == nil {
		return nil, errors.New("onboarding.abandonment: uow is nil")
	}
	if factory == nil {
		return nil, errors.New("onboarding.abandonment: factory is nil")
	}
	if cfg.AbandonmentTTLHours < 1 {
		return nil, fmt.Errorf("onboarding.abandonment: ttl_hours must be > 0 (got %d)", cfg.AbandonmentTTLHours)
	}
	if cfg.AbandonmentJobSchedule == "" {
		return nil, errors.New("onboarding.abandonment: schedule must not be empty")
	}
	if _, err := cron.ParseStandard(cfg.AbandonmentJobSchedule); err != nil {
		return nil, fmt.Errorf("onboarding.abandonment: invalid schedule %q: %w", cfg.AbandonmentJobSchedule, err)
	}
	if cfg.AbandonmentBatchSize < 1 {
		return nil, fmt.Errorf("onboarding.abandonment: batch_size must be > 0 (got %d)", cfg.AbandonmentBatchSize)
	}
	return &OnboardingAbandonmentJob{
		uow:     uow,
		factory: factory,
		cfg:     cfg,
		o11y:    o11y,
		metrics: abandonmentMetrics{
			abandonedTotal: o11y.Metrics().Counter(
				"onboarding_step_abandoned_total",
				"Total de passos de onboarding abandonados por etapa",
				"1",
			),
		},
	}, nil
}

func (j *OnboardingAbandonmentJob) Name() string           { return "onboarding-abandonment" }
func (j *OnboardingAbandonmentJob) Schedule() string       { return j.cfg.AbandonmentJobSchedule }
func (j *OnboardingAbandonmentJob) Timeout() time.Duration { return 5 * time.Minute }

func (j *OnboardingAbandonmentJob) Run(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-time.Duration(j.cfg.AbandonmentTTLHours) * time.Hour)
	limit := j.cfg.AbandonmentBatchSize

	for {
		var reported int
		err := j.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
			store := j.factory.Store(tx)
			runs, listErr := store.ListSuspended(ctx, onboardingWorkflowID, cutoff, limit)
			if listErr != nil {
				return fmt.Errorf("onboarding.abandonment: list suspended: %w", listErr)
			}
			for _, run := range runs {
				if reportErr := j.reportIfNeeded(ctx, store, run); reportErr != nil {
					return fmt.Errorf("onboarding.abandonment: report run %s: %w", run.RunID, reportErr)
				}
			}
			reported = len(runs)
			return nil
		})
		if err != nil {
			return err
		}
		if reported == 0 {
			break
		}
	}
	return nil
}

func (j *OnboardingAbandonmentJob) reportIfNeeded(ctx context.Context, store platform.Store, run platform.Snapshot) error {
	state, decodeErr := j.decodeState(run.State)
	if decodeErr != nil {
		j.o11y.Logger().Warn(ctx, "onboarding.abandonment: decode_state_failed",
			observability.String("run_id", run.RunID.String()),
			observability.Error(decodeErr),
		)
	}
	if !state.AbandonedAt.IsZero() {
		return nil
	}

	step := state.Phase.String()
	j.metrics.abandonedTotal.Add(ctx, 1, observability.String("step", step))
	j.o11y.Logger().Info(ctx, "onboarding.abandonment: abandoned_step_reported",
		observability.String("thread_id", run.CorrelationKey),
		observability.String("run_id", run.RunID.String()),
		observability.String("workflow", run.Workflow),
		observability.String("step", step),
		observability.String("status", run.Status.String()),
	)

	state.AbandonedAt = time.Now().UTC()
	updated, encodeErr := json.Marshal(state)
	if encodeErr != nil {
		return fmt.Errorf("encode state: %w", encodeErr)
	}
	run.State = updated
	run.UpdatedAt = time.Now().UTC()
	if err := store.Save(ctx, run, run.Version); err != nil {
		if errors.Is(err, platform.ErrVersionConflict) {
			j.o11y.Logger().Warn(ctx, "onboarding.abandonment: version_conflict_skipping",
				observability.String("run_id", run.RunID.String()),
			)
			return nil
		}
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func (j *OnboardingAbandonmentJob) decodeState(state []byte) (workflow.OnboardingState, error) {
	var decoded workflow.OnboardingState
	if len(state) == 0 {
		decoded.Phase = valueobjects.PhaseWelcome
		return decoded, nil
	}
	if err := json.Unmarshal(state, &decoded); err != nil {
		decoded.Phase = valueobjects.PhaseWelcome
		return decoded, err
	}
	return decoded, nil
}
