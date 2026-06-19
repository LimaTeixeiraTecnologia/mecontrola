package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
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
	Terminal bool
}

type OnboardingToolDispatcher interface {
	Dispatch(ctx context.Context, userID uuid.UUID, channel string, call interfaces.ToolCall) (OnboardingToolResult, error)
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
	maxTokens   int
	o11y        observability.Observability
	turnsTotal  observability.Counter
}

func NewRunOnboardingTurn(
	interpreter IntentInterpreter,
	reader OnboardingStateReader,
	dispatcher OnboardingToolDispatcher,
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
	if o11y == nil {
		return nil, fmt.Errorf("agent.usecase.run_onboarding_turn: observability is nil")
	}
	turnsTotal := o11y.Metrics().Counter(
		"agent_onboarding_turn_total",
		"Total de turnos de onboarding conversacional conduzidos pela IA por outcome",
		"1",
	)
	return &RunOnboardingTurn{
		interpreter: interpreter,
		reader:      reader,
		dispatcher:  dispatcher,
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

	system, err := prompting.RenderOnboardingSystem(buildOnboardingSystemData(snapshot))
	if err != nil {
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: render system: %w", err)
	}

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: system,
		UserMessage:  text,
		Tools:        OnboardingToolCatalog(),
		ToolChoice:   "auto",
		FreeText:     true,
		MaxTokens:    uc.maxTokens,
	})
	if err != nil {
		span.RecordError(err)
		return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: interpret: %w", err)
	}

	if len(resp.ToolCalls) == 0 {
		uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "text"))
		return RunOnboardingTurnResult{Handled: true, Reply: strings.TrimSpace(string(resp.RawJSON))}, nil
	}

	replies := make([]string, 0, len(resp.ToolCalls))
	for _, call := range resp.ToolCalls {
		out, dispatchErr := uc.dispatcher.Dispatch(ctx, in.UserID, in.Channel, call)
		if dispatchErr != nil {
			span.RecordError(dispatchErr)
			return RunOnboardingTurnResult{}, fmt.Errorf("agent.usecase.run_onboarding_turn: dispatch %s: %w", call.FunctionName, dispatchErr)
		}
		if reply := strings.TrimSpace(out.Reply); reply != "" {
			replies = append(replies, reply)
		}
	}
	uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "tool"))
	return RunOnboardingTurnResult{Handled: true, Reply: strings.Join(replies, "\n\n")}, nil
}

func buildOnboardingSystemData(s OnboardingSnapshot) prompting.OnboardingSystemData {
	cards := make([]prompting.OnboardingPromptCard, 0, len(s.Cards))
	for _, c := range s.Cards {
		cards = append(cards, prompting.OnboardingPromptCard{Name: c.Name, DueDay: c.DueDay})
	}
	splits := make([]prompting.OnboardingPromptSplit, 0, len(s.Splits))
	for _, sp := range s.Splits {
		splits = append(splits, prompting.OnboardingPromptSplit{Slug: sp.Slug, Percent: sp.Percent})
	}
	return prompting.OnboardingSystemData{
		State:           s.State,
		IncomeCents:     s.IncomeCents,
		Objective:       s.Objective,
		Cards:           cards,
		Splits:          splits,
		FirstTxRecorded: s.FirstTxRecorded,
	}
}
