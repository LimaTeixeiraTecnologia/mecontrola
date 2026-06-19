package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
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

type RunOnboardingTurn struct {
	interpreter IntentInterpreter
	reader      OnboardingStateReader
	dispatcher  OnboardingToolDispatcher
	phases      OnboardingPhaseSetter
	maxTokens   int
	o11y        observability.Observability
	turnsTotal  observability.Counter
}

func NewRunOnboardingTurn(
	interpreter IntentInterpreter,
	reader OnboardingStateReader,
	dispatcher OnboardingToolDispatcher,
	phases OnboardingPhaseSetter,
	maxTokens int,
	o11y observability.Observability,
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
		maxTokens:   maxTokens,
		o11y:        o11y,
		turnsTotal:  turnsTotal,
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
	if !snapshot.InProgress {
		return RunOnboardingTurnResult{Handled: false}, nil
	}

	phase := strings.TrimSpace(snapshot.Phase)
	if phase == "" {
		return uc.emit(ctx, in.UserID, OnbPhaseWelcome, scriptWelcome, "welcome")
	}

	if transition, ok := onboardingScriptedTransitions[phase]; ok {
		return uc.advanceOnAffirmation(ctx, in.UserID, text, transition.next, transition.nextScript, transition.stayScript)
	}

	switch phase {
	case OnbPhaseObjective:
		return uc.dataPhase(ctx, in, snapshot, OnbPhaseObjective, OnbPhaseIncome, scriptIncome)
	case OnbPhaseIncome:
		return uc.dataPhase(ctx, in, snapshot, OnbPhaseIncome, OnbPhaseCards, scriptCards)
	case OnbPhaseCards:
		return uc.cardsPhase(ctx, in, snapshot)
	case OnbPhaseSplits:
		return uc.splitsPhase(ctx, in, snapshot)
	case OnbPhaseSummary:
		return uc.summaryPhase(ctx, in.UserID, text)
	case OnbPhaseFirstTx:
		return uc.firstTransactionPhase(ctx, in, snapshot)
	default:
		return uc.emit(ctx, in.UserID, OnbPhaseWelcome, scriptWelcome, "welcome_reset")
	}
}

type onboardingScriptedTransition struct {
	next       string
	nextScript string
	stayScript string
}

var onboardingScriptedTransitions = map[string]onboardingScriptedTransition{
	OnbPhaseWelcome:      {OnbPhaseMethodology1, scriptMethodology1, scriptWelcome},
	OnbPhaseMethodology1: {OnbPhaseMethodology2, scriptMethodology2, scriptMethodology1},
	OnbPhaseMethodology2: {OnbPhaseMethodology3, scriptMethodology3, scriptMethodology2},
	OnbPhaseMethodology3: {OnbPhaseMethodology4, scriptMethodology4, scriptMethodology3},
	OnbPhaseMethodology4: {OnbPhaseMethodology5, scriptMethodology5, scriptMethodology4},
	OnbPhaseMethodology5: {OnbPhaseObjective, scriptObjective, scriptMethodology5},
}

func (uc *RunOnboardingTurn) emit(ctx context.Context, userID uuid.UUID, phase, reply, outcome string) (RunOnboardingTurnResult, error) {
	if err := uc.phases.SetPhase(ctx, userID, phase); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase %s: %w", phase, err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", phase), observability.String("outcome", outcome))
	return RunOnboardingTurnResult{Handled: true, Reply: reply}, nil
}

func (uc *RunOnboardingTurn) advanceOnAffirmation(ctx context.Context, userID uuid.UUID, text, nextPhase, nextReply, stayReply string) (RunOnboardingTurnResult, error) {
	if shouldAdvanceScriptedPhase(text) {
		return uc.emit(ctx, userID, nextPhase, nextReply, "advance")
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", "script"), observability.String("outcome", "reask"))
	return RunOnboardingTurnResult{Handled: true, Reply: stayReply}, nil
}

func (uc *RunOnboardingTurn) dataPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot, phase, nextPhase, nextReply string) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, phase)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", phase), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, nextPhase); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase %s: %w", nextPhase, err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", phase), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, nextReply)}, nil
}

func (uc *RunOnboardingTurn) cardsPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot) (RunOnboardingTurnResult, error) {
	if matchesOnboardingNegation(in.Text) {
		return uc.emit(ctx, in.UserID, OnbPhaseSplits, scriptSplits, "cards_done")
	}
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseCards)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseCards), observability.String("outcome", "stay"))
	if strings.TrimSpace(out.Reply) == "" {
		return RunOnboardingTurnResult{Handled: true, Reply: scriptCardQuestion}, nil
	}
	return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
}

func (uc *RunOnboardingTurn) splitsPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseSplits)
	if err != nil {
		return RunOnboardingTurnResult{}, err
	}
	if !out.Advance {
		uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseSplits), observability.String("outcome", "stay"))
		return RunOnboardingTurnResult{Handled: true, Reply: out.Reply}, nil
	}
	refreshed, err := uc.reader.Load(ctx, in.UserID)
	if err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: reload after splits: %w", err)
	}
	if err := uc.phases.SetPhase(ctx, in.UserID, OnbPhaseSummary); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase summary: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseSplits), observability.String("outcome", "advance"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies(out.Reply, onboardingSummary(refreshed))}, nil
}

func (uc *RunOnboardingTurn) summaryPhase(ctx context.Context, userID uuid.UUID, text string) (RunOnboardingTurnResult, error) {
	if shouldAdvanceScriptedPhase(text) {
		return uc.emit(ctx, userID, OnbPhaseFirstTx, scriptTransition, "advance")
	}
	if err := uc.phases.SetPhase(ctx, userID, OnbPhaseSplits); err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: set phase splits: %w", err)
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("phase", OnbPhaseSummary), observability.String("outcome", "adjust"))
	return RunOnboardingTurnResult{Handled: true, Reply: joinReplies("Sem problema! Vamos ajustar a distribuição.", scriptSplits)}, nil
}

func (uc *RunOnboardingTurn) firstTransactionPhase(ctx context.Context, in RunOnboardingTurnInput, snapshot OnboardingSnapshot) (RunOnboardingTurnResult, error) {
	out, err := uc.runDataPhase(ctx, in, snapshot, OnbPhaseFirstTx)
	if err != nil {
		return RunOnboardingTurnResult{}, err
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

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: onboardingDataPhasePrompt(phase, snapshot),
		UserMessage:  strings.TrimSpace(in.Text),
		Tools:        []interfaces.ToolSpec{tool},
		ToolChoice:   "auto",
		FreeText:     true,
		MaxTokens:    uc.maxTokens,
	})
	if err != nil {
		return OnboardingToolResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: interpret %s: %w", phase, err)
	}

	if len(resp.ToolCalls) == 0 {
		return OnboardingToolResult{Reply: strings.TrimSpace(string(resp.RawJSON)), Advance: false}, nil
	}

	var replies []string
	advance := false
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
	}
	return OnboardingToolResult{Reply: strings.Join(replies, "\n\n"), Advance: advance}, nil
}

func joinReplies(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return strings.Join(out, "\n\n")
}

func outcomeForAdvance(advance bool) string {
	if advance {
		return "advance"
	}
	return "stay"
}
