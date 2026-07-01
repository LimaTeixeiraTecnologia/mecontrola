package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type OnboardingResult struct {
	Handled bool
	Message string
	Done    bool
}

type resolveOnboardingWorkflowStore interface {
	Load(ctx context.Context, workflowID, key string) (workflow.Snapshot, bool, error)
}

type ResolveOnboardingOrAgent struct {
	engine     workflow.Engine[workflows.OnboardingState]
	store      resolveOnboardingWorkflowStore
	workingMem memory.WorkingMemory
	def        workflow.Definition[workflows.OnboardingState]
	o11y       observability.Observability
	total      observability.Counter
}

func NewResolveOnboardingOrAgent(
	engine workflow.Engine[workflows.OnboardingState],
	store resolveOnboardingWorkflowStore,
	workingMem memory.WorkingMemory,
	def workflow.Definition[workflows.OnboardingState],
	o11y observability.Observability,
) *ResolveOnboardingOrAgent {
	total := o11y.Metrics().Counter(
		"onboarding_workflow_total",
		"Total de execucoes do workflow de onboarding",
		"1",
	)
	return &ResolveOnboardingOrAgent{
		engine:     engine,
		store:      store,
		workingMem: workingMem,
		def:        def,
		o11y:       o11y,
		total:      total,
	}
}

func (uc *ResolveOnboardingOrAgent) Execute(ctx context.Context, userID, message string) (OnboardingResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.resolve_onboarding_or_agent")
	defer span.End()

	snap, found, err := uc.store.Load(ctx, uc.def.ID, userID)
	if err != nil {
		span.RecordError(err)
		return OnboardingResult{}, fmt.Errorf("agents.usecase.resolve_onboarding_or_agent: load: %w", err)
	}

	if found && (snap.Status == workflow.RunStatusSuspended || snap.Status == workflow.RunStatusRunning) {
		resumePayload, _ := json.Marshal(map[string]string{"resumeText": message})
		result, err := uc.engine.Resume(ctx, uc.def, userID, resumePayload)
		if err != nil {
			span.RecordError(err)
			return OnboardingResult{}, fmt.Errorf("agents.usecase.resolve_onboarding_or_agent: resume: %w", err)
		}
		if result.Status == workflow.RunStatusSucceeded {
			uc.total.Add(ctx, 1, observability.String("outcome", "completed"))
			return OnboardingResult{Handled: true, Done: true, Message: result.State.FinalMessage}, nil
		}
		msg := ""
		if result.Suspend != nil {
			msg = result.Suspend.Prompt
		}
		uc.total.Add(ctx, 1, observability.String("outcome", "resumed"))
		return OnboardingResult{Handled: true, Message: msg}, nil
	}

	wm, err := uc.workingMem.Get(ctx, userID)
	if err != nil && !errors.Is(err, memory.ErrWorkingMemoryNotFound) {
		span.RecordError(err)
		return OnboardingResult{}, fmt.Errorf("agents.usecase.resolve_onboarding_or_agent: get_wm: %w", err)
	}

	if strings.Contains(wm, "## Objetivo Financeiro") {
		uc.total.Add(ctx, 1, observability.String("outcome", "already_onboarded"))
		return OnboardingResult{Handled: false}, nil
	}

	initial := workflows.OnboardingState{Phase: workflows.PhaseWelcome, UserID: userID}
	result, err := uc.engine.Start(ctx, uc.def, userID, initial)
	if err != nil {
		span.RecordError(err)
		return OnboardingResult{}, fmt.Errorf("agents.usecase.resolve_onboarding_or_agent: start: %w", err)
	}
	msg := ""
	if result.Suspend != nil {
		msg = result.Suspend.Prompt
	}
	uc.total.Add(ctx, 1, observability.String("outcome", "started"))
	return OnboardingResult{Handled: true, Message: msg}, nil
}
