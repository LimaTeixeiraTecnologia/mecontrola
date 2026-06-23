package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
)

type OnboardingSnapshotCard struct {
	Name   string
	DueDay int
}

type OnboardingSnapshotSplit struct {
	Slug    string
	Percent int
}

type OnboardingSnapshot struct {
	InProgress      bool
	State           string
	Phase           string
	IncomeCents     int64
	Objective       string
	Cards           []OnboardingSnapshotCard
	Splits          []OnboardingSnapshotSplit
	FirstTxRecorded bool
}

type OnboardingStateReader interface {
	Load(ctx context.Context, userID uuid.UUID) (OnboardingSnapshot, error)
}

type OnboardingToolResult struct {
	Reply    string
	Advance  bool
	Terminal bool
}

type OnboardingToolDispatcher interface {
	Dispatch(ctx context.Context, userID uuid.UUID, channel string, call interfaces.ToolCall) (OnboardingToolResult, error)
}

type OnboardingPhaseSetter interface {
	SetPhase(ctx context.Context, userID uuid.UUID, phase string) error
}

type RunOnboardingTurnInput struct {
	UserID  uuid.UUID
	Channel string
	Text    string
}

type RunOnboardingTurnResult struct {
	Handled bool
	Reply   string
}

type onboardingSessionReader interface {
	GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (interfaces.AgentSessionRecord, error)
	Upsert(ctx context.Context, record interfaces.AgentSessionRecord) error
}

type OnboardingV2SessionGateway interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (onboardingv2draft.Draft, bool, error)
	Save(ctx context.Context, userID uuid.UUID, channel string, draft onboardingv2draft.Draft) error
	Clear(ctx context.Context, userID uuid.UUID, channel string) error
}

type RunOnboardingTurn struct {
	interpreter IntentInterpreter
	reader      OnboardingStateReader
	dispatcher  OnboardingToolDispatcher
	phases      OnboardingPhaseSetter
	v2session   OnboardingV2SessionGateway
	maxTokens   int
	o11y        observability.Observability
	turnsTotal  observability.Counter
	turnHistory services.TurnHistory
	sessionRepo onboardingSessionReader
}

func NewRunOnboardingTurn(
	interpreter IntentInterpreter,
	reader OnboardingStateReader,
	dispatcher OnboardingToolDispatcher,
	phases OnboardingPhaseSetter,
	maxTokens int,
	o11y observability.Observability,
	sessionRepo onboardingSessionReader,
	v2session OnboardingV2SessionGateway,
) (*RunOnboardingTurn, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: interpreter is nil")
	}
	if reader == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: state reader is nil")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: tool dispatcher is nil")
	}
	if phases == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: phase setter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: observability is nil")
	}
	if v2session == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: v2session gateway is nil")
	}
	turnsTotal := o11y.Metrics().Counter(
		"agent_onboarding_turn_total",
		"Total de turnos de onboarding conversacional conduzidos pela IA por fase e outcome",
		"1",
	)
	return &RunOnboardingTurn{
		interpreter: interpreter,
		reader:      reader,
		dispatcher:  dispatcher,
		phases:      phases,
		v2session:   v2session,
		maxTokens:   maxTokens,
		o11y:        o11y,
		turnsTotal:  turnsTotal,
		sessionRepo: sessionRepo,
	}, nil
}

func (uc *RunOnboardingTurn) Execute(ctx context.Context, in RunOnboardingTurnInput) (RunOnboardingTurnResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.run_onboarding_turn")
	defer span.End()

	text := strings.TrimSpace(in.Text)
	if in.UserID == uuid.Nil || text == "" {
		return RunOnboardingTurnResult{Handled: false}, nil
	}

	snapshot, err := uc.reader.Load(ctx, in.UserID)
	if err != nil {
		span.RecordError(err)
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: load context: %w", err)
	}

	isWelcomeSignal := text == services.OnboardingWelcomeSignal

	if !snapshot.InProgress {
		return RunOnboardingTurnResult{Handled: isWelcomeSignal}, nil
	}

	if isWelcomeSignal && strings.TrimSpace(snapshot.Phase) != "" {
		return RunOnboardingTurnResult{Handled: true}, nil
	}

	draft, _, err := uc.v2session.Load(ctx, in.UserID, in.Channel)
	if err != nil {
		span.RecordError(err)
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: load v2 draft: %w", err)
	}

	switch strings.TrimSpace(snapshot.Phase) {
	case "":
		return uc.emitWelcome(ctx, in.UserID)
	case OnbPhaseObjective:
		return uc.objectivePhase(ctx, in, snapshot, draft)
	case OnbPhaseBudget:
		return uc.budgetPhase(ctx, in, snapshot, draft)
	case OnbPhaseCards:
		return uc.cardsPhase(ctx, in, snapshot, draft)
	case OnbPhaseFinancialPlan:
		return uc.financialPlanPhase(ctx, in, snapshot, draft)
	case OnbPhaseFirstTx:
		return uc.firstTxPhase(ctx, in, snapshot)
	default:
		return uc.emitWelcome(ctx, in.UserID)
	}
}

func (uc *RunOnboardingTurn) emitWelcome(ctx context.Context, userID uuid.UUID) (RunOnboardingTurnResult, error) {
	if err := uc.phases.SetPhase(ctx, userID, OnbPhaseObjective); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase objective: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", "welcome"), observability.String("outcome", "emit"))
	return RunOnboardingTurnResult{Handled: true, Reply: scriptWelcome}, nil
}

func (uc *RunOnboardingTurn) objectivePhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseObjective)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseObjective), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	updated := draft.WithStep(onboardingv2draft.StepBudget)
	if saveErr := uc.v2session.Save(ctx, in.UserID, in.Channel, updated); saveErr != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: save draft objective: %w", saveErr)
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseBudget); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase budget: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseObjective), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, scriptBudget)}, nil
}

func (uc *RunOnboardingTurn) budgetPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseBudget)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseBudget), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	refreshed, err := uc.reader.Load(ctx, in.UserID)
	if err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: reload after budget: %w", err)
	}
	autoSplits := buildAutoSplits(refreshed.IncomeCents)
	updated := draft.WithIncome(refreshed.IncomeCents).WithAutoSplits(autoSplits).WithStep(onboardingv2draft.StepCards)
	if saveErr := uc.v2session.Save(ctx, in.UserID, in.Channel, updated); saveErr != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: save draft budget: %w", saveErr)
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseCards); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase cards: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseBudget), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, buildAutoSplitPreview(autoSplits), scriptCards)}, nil
}

func effectiveSplits(draft onboardingv2draft.Draft, snapshot OnboardingSnapshot) []onboardingv2draft.SplitEntry {
	if splits := draft.Splits(); len(splits) > 0 {
		return splits
	}
	return buildAutoSplits(snapshot.IncomeCents)
}

func (uc *RunOnboardingTurn) cardsPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	splits := effectiveSplits(draft, snapshot)
	if matchesOnboardingNegation(in.Text) {
		if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFinancialPlan); err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase financial_plan: %w", err)
		}
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "negation"))
		return RunOnboardingTurnResult{Handled: true, Reply: buildFinancialPlanMessage(splits)}, nil
	}
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseCards)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "stay"))
		if strings.TrimSpace(out.Reply) == "" {
			return RunOnboardingTurnResult{Handled: true, Reply: scriptCardQuestion}, nil
		}
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFinancialPlan); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase financial_plan: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, buildFinancialPlanMessage(splits))}, nil
}

func (uc *RunOnboardingTurn) financialPlanPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, draft onboardingv2draft.Draft) (RunOnboardingTurnResult, error) {
	splits := effectiveSplits(draft, snapshot)
	if !onboardingTextHasDigit(in.Text) && shouldAdvanceScriptedPhase(in.Text) {
		confirmCall := buildAutoSplitToolCall(splits)
		out, err := uc.dispatcher.Dispatch(ctx, in.UserID, in.Channel, confirmCall)
		if err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: dispatch confirm splits: %w", err)
		}
		_ = uc.v2session.Clear(ctx, in.UserID, in.Channel)
		if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFirstTx); err != nil {
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase first_tx: %w", err)
		}
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFinancialPlan), observability.String("outcome", "confirm"))
		return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, scriptFirstTx)}, nil
	}
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseFinancialPlan)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFinancialPlan), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	_ = uc.v2session.Clear(ctx, in.UserID, in.Channel)
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseFirstTx); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase first_tx: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFinancialPlan), observability.String("outcome", "adjust"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, scriptFirstTx)}, nil
}

func (uc *RunOnboardingTurn) firstTxPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseFirstTx)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if out.Terminal {
		_ = uc.v2session.Clear(ctx, in.UserID, in.Channel)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseFirstTx), observability.String("outcome", outcomeForAdvance(out.Advance)))
	return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
}

func (uc *RunOnboardingTurn) runDataPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, phase string) (OnboardingToolResult, error) {
	toolName := onboardingPhaseTool(phase)
	tool, ok := onboardingToolByName(toolName)
	if !ok {
		return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: tool not found for phase %s", phase)
	}

	llmMessages, sessionRecord, sessionFound := uc.loadOnbHistory(ctx, in.UserID, in.Channel)

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: onboardingDataPhasePrompt(phase, snapshot),
		UserMessage:  strings.TrimSpace(in.Text),
		Messages:     llmMessages,
		Tools:        []interfaces.ToolSpec{tool},
		ToolChoice:   "auto",
		FreeText:     true,
		MaxTokens:    uc.maxTokens,
	})
	if err != nil {
		return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: interpret %s: %w", phase, err)
	}

	assistantReply := sanitizeWhatsAppText(string(resp.RawJSON))
	if assistantReply == "" && len(resp.ToolCalls) > 0 {
		assistantReply = "[tool_call]"
	}
	uc.saveOnbTurn(ctx, in.UserID, in.Channel, strings.TrimSpace(in.Text), assistantReply, sessionRecord, sessionFound)

	if len(resp.ToolCalls) == 0 {
		return OnboardingToolResult{Reply: sanitizeWhatsAppText(string(resp.RawJSON)), Advance: false}, nil
	}

	var replies []string
	var advance, terminal bool
	for _, call := range resp.ToolCalls {
		result, dispatchErr := uc.dispatcher.Dispatch(ctx, in.UserID, in.Channel, call)
		if dispatchErr != nil {
			return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: dispatch %s: %w", call.FunctionName, dispatchErr)
		}
		if reply := strings.TrimSpace(result.Reply); reply != "" {
			replies = append(replies, reply)
		}
		if result.Advance {
			advance = true
		}
		if result.Terminal {
			terminal = true
		}
	}
	return OnboardingToolResult{Reply: strings.Join(replies, "\n\n"), Advance: advance, Terminal: terminal}, nil
}

func (uc *RunOnboardingTurn) loadOnbHistory(ctx context.Context, userID uuid.UUID, channel string) ([]interfaces.ConversationMessage, interfaces.AgentSessionRecord, bool) {
	if uc.sessionRepo == nil || userID == uuid.Nil {
		return nil, interfaces.AgentSessionRecord{}, false
	}
	rec, err := uc.sessionRepo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		return nil, interfaces.AgentSessionRecord{}, false
	}
	turns, deErr := uc.turnHistory.Deserialize(rec.RecentTurns)
	if deErr != nil || len(turns) == 0 {
		return nil, rec, true
	}
	return uc.turnHistory.ToLLMMessages(turns), rec, true
}

func (uc *RunOnboardingTurn) saveOnbTurn(ctx context.Context, userID uuid.UUID, channel, userMsg, assistantReply string, sessionRecord interfaces.AgentSessionRecord, sessionFound bool) {
	if uc.sessionRepo == nil || userID == uuid.Nil {
		return
	}
	var existingTurns []entities.ConversationMessage
	if sessionFound {
		existingTurns, _ = uc.turnHistory.Deserialize(sessionRecord.RecentTurns)
	}
	updatedTurns := uc.turnHistory.Append(existingTurns, userMsg, assistantReply, time.Now().UTC(), 3)
	serialized, serErr := uc.turnHistory.Serialize(updatedTurns)
	if serErr != nil {
		return
	}
	newRecord := sessionRecord
	if !sessionFound {
		newRecord = interfaces.AgentSessionRecord{
			ID:            uuid.New(),
			UserID:        userID,
			Channel:       channel,
			PendingAction: []byte("{}"),
		}
	}
	newRecord.RecentTurns = serialized
	newRecord.UpdatedAt = time.Now().UTC()
	newRecord.ExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	_ = uc.sessionRepo.Upsert(ctx, newRecord)
}
