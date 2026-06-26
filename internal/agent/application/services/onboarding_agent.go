package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const onboardingRetryReply = "Tive um probleminha aqui 😅 Pode me mandar de novo?"

type OnboardingStateChecker interface {
	Check(ctx context.Context, userID uuid.UUID) (inProgress bool, phase onbvalueobjects.OnboardingPhase, err error)
}

type OnboardingAgent struct {
	engine         platform.Engine[workflow.OnboardingState]
	def            platform.Definition[workflow.OnboardingState]
	store          platform.Store
	stateChecker   OnboardingStateChecker
	historyGateway workflow.HistoryGateway
	o11y           observability.Observability
	routedTotal    observability.Counter
	completedTotal observability.Counter
	runDuration    observability.Histogram
	codec          platform.Codec[workflow.OnboardingState]
}

func NewOnboardingAgent(
	o11y observability.Observability,
	routedTotal observability.Counter,
	engine platform.Engine[workflow.OnboardingState],
	def platform.Definition[workflow.OnboardingState],
	store platform.Store,
	stateChecker OnboardingStateChecker,
	historyGateway workflow.HistoryGateway,
) *OnboardingAgent {
	return &OnboardingAgent{
		engine:         engine,
		def:            def,
		store:          store,
		stateChecker:   stateChecker,
		historyGateway: historyGateway,
		o11y:           o11y,
		routedTotal:    routedTotal,
		completedTotal: o11y.Metrics().Counter("onboarding_completed_total", "Total de onboardings concluidos", "1"),
		runDuration:    o11y.Metrics().Histogram("onboarding_run_duration_seconds", "Duracao dos runs de onboarding", "s"),
		codec:          platform.NewCodec[workflow.OnboardingState](),
	}
}

func (a *OnboardingAgent) Handle(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (RouteResult, bool) {
	runStart := time.Now().UTC()
	correlationKey, ok, phase := a.checkInProgress(ctx, userID, channel)
	if !ok {
		return RouteResult{}, false
	}

	isReplay, currentState := a.isReplay(ctx, correlationKey, messageID)
	if isReplay {
		return a.replyReplay(ctx, channel), true
	}

	resumeBytes, err := a.encodeResume(text, messageID, currentState.ProcessedMessageIDs)
	if err != nil {
		return a.replyError(ctx, channel, "agent.onboarding.resume_encode_failed", err), true
	}

	result, err := a.engine.Resume(ctx, a.def, correlationKey, resumeBytes)
	if err != nil {
		return a.handleResumeError(ctx, channel, err)
	}

	result, ok, err = a.startIfNeeded(ctx, correlationKey, userID, phase, messageID, resumeBytes, result)
	if !ok {
		return a.replyError(ctx, channel, "agent.onboarding.start_failed", err), true
	}

	res, handled := a.toRouteResult(ctx, channel, correlationKey, runStart, result)
	if handled && a.historyGateway != nil && res.Reply != "" {
		if appendErr := a.historyGateway.AppendTurn(ctx, userID, text, res.Reply); appendErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.onboarding.append_turn_failed",
				observability.String("user_id", userID.String()),
				observability.Error(appendErr),
			)
		}
	}
	return res, handled
}

func (a *OnboardingAgent) checkInProgress(ctx context.Context, userID uuid.UUID, channel string) (string, bool, onbvalueobjects.OnboardingPhase) {
	if a.engine == nil || a.def.Root == nil || a.store == nil || a.stateChecker == nil {
		return "", false, onbvalueobjects.PhaseWelcome
	}

	inProgress, phase, err := a.stateChecker.Check(ctx, userID)
	if err != nil {
		if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
			span.RecordError(err)
		}
		a.o11y.Logger().Warn(ctx, "agent.onboarding.check_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return "", false, onbvalueobjects.PhaseWelcome
	}
	if !inProgress {
		return "", false, onbvalueobjects.PhaseWelcome
	}

	return userID.String(), true, phase
}

func (a *OnboardingAgent) handleResumeError(ctx context.Context, channel string, err error) (RouteResult, bool) {
	if errors.Is(err, platform.ErrRunConflict) {
		return a.replyReplay(ctx, channel), true
	}
	return a.replyError(ctx, channel, "agent.onboarding.resume_failed", err), true
}

func (a *OnboardingAgent) startIfNeeded(
	ctx context.Context,
	correlationKey string,
	userID uuid.UUID,
	phase onbvalueobjects.OnboardingPhase,
	messageID string,
	resumeBytes []byte,
	result platform.RunResult[workflow.OnboardingState],
) (platform.RunResult[workflow.OnboardingState], bool, error) {
	if result.RunID != uuid.Nil {
		return result, true, nil
	}

	started, err := a.start(ctx, correlationKey, userID, phase, messageID)
	if err == nil {
		return started, true, nil
	}

	if !errors.Is(err, platform.ErrRunAlreadyExists) {
		return platform.RunResult[workflow.OnboardingState]{}, false, err
	}

	resumed, err := a.engine.Resume(ctx, a.def, correlationKey, resumeBytes)
	if err != nil {
		return platform.RunResult[workflow.OnboardingState]{}, false, err
	}
	return resumed, true, nil
}

func (a *OnboardingAgent) start(ctx context.Context, correlationKey string, userID uuid.UUID, phase onbvalueobjects.OnboardingPhase, messageID string) (platform.RunResult[workflow.OnboardingState], error) {
	initial := workflow.OnboardingState{
		UserID:              userID,
		Phase:               phase,
		MessageID:           messageID,
		ProcessedMessageIDs: []string{messageID},
	}
	return a.engine.Start(ctx, a.def, correlationKey, initial)
}

func (a *OnboardingAgent) encodeResume(text, messageID string, processed []string) ([]byte, error) {
	processed = append(processed, messageID)
	resume := map[string]any{
		"inbound":               text,
		"message_id":            messageID,
		"processed_message_ids": processed,
	}
	return json.Marshal(resume)
}

func (a *OnboardingAgent) isReplay(ctx context.Context, correlationKey, messageID string) (bool, workflow.OnboardingState) {
	var state workflow.OnboardingState
	if messageID == "" {
		return false, state
	}
	snap, found, err := a.store.Load(ctx, a.def.ID, correlationKey)
	if err != nil || !found {
		return false, state
	}
	state, err = a.codec.Decode(snap.State)
	if err != nil {
		return false, workflow.OnboardingState{}
	}
	for _, id := range state.ProcessedMessageIDs {
		if id == messageID {
			return true, state
		}
	}
	return false, state
}

func (a *OnboardingAgent) toRouteResult(ctx context.Context, channel, correlationKey string, runStart time.Time, result platform.RunResult[workflow.OnboardingState]) (RouteResult, bool) {
	durationMs := time.Since(runStart).Milliseconds()
	step := result.State.Phase.String()
	status := result.Status.String()

	a.o11y.Logger().Info(ctx, "agent.onboarding.run",
		observability.String("thread_id", correlationKey),
		observability.String("run_id", result.RunID.String()),
		observability.String("workflow", a.def.ID),
		observability.String("step", step),
		observability.String("status", status),
		observability.Int64("duration_ms", durationMs),
	)

	switch result.Status {
	case platform.RunStatusSuspended:
		if result.Suspend != nil {
			a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeRouted)
			return RouteResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeRouted, Kind: intent.KindConfigureBudget}, true
		}
	case platform.RunStatusSucceeded:
		reply := ""
		if result.Suspend != nil {
			reply = result.Suspend.Prompt
		}
		a.completedTotal.Increment(ctx)
		a.runDuration.Record(ctx, time.Since(runStart).Seconds())
		a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeRouted)
		return RouteResult{Reply: reply, Outcome: tools.OutcomeRouted, Kind: intent.KindConfigureBudget}, true
	case platform.RunStatusFailed:
		a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: onboardingRetryReply, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindConfigureBudget}, true
	}

	return RouteResult{}, false
}

func (a *OnboardingAgent) record(ctx context.Context, kind, channel string, outcome tools.ToolOutcome) {
	a.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}

func (a *OnboardingAgent) replyReplay(ctx context.Context, channel string) RouteResult {
	a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeReplay)
	return RouteResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: intent.KindConfigureBudget}
}

func (a *OnboardingAgent) replyError(ctx context.Context, channel, logKey string, err error) RouteResult {
	if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
		span.RecordError(err)
	}
	a.o11y.Logger().Warn(ctx, logKey,
		observability.String("channel", channel),
		observability.Error(err),
	)
	a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeUsecaseError)
	return RouteResult{Reply: onboardingRetryReply, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
}
