package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/commands"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type llmRequester interface {
	Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error)
}

type HandleInboundMessage struct {
	loader        interfaces.PromptContextLoader
	requester     llmRequester
	dispatcher    interfaces.IntentDispatcher
	eventPub      interfaces.IntentEventPublisher
	promptBuilder services.PromptBuilder
	validator     services.IntentValidator
	workflow      domainservices.IntentWorkflow
	o11y          observability.Observability
	outcomeTotal  observability.Counter
}

func NewHandleInboundMessage(
	loader interfaces.PromptContextLoader,
	requester llmRequester,
	dispatcher interfaces.IntentDispatcher,
	eventPub interfaces.IntentEventPublisher,
	promptBuilder services.PromptBuilder,
	validator services.IntentValidator,
	workflow domainservices.IntentWorkflow,
	o11y observability.Observability,
) *HandleInboundMessage {
	outcomeTotal := o11y.Metrics().Counter(
		"agent_llm_outcome_total",
		"Total de outcomes do agent LLM por tipo",
		"1",
	)
	return &HandleInboundMessage{
		loader:        loader,
		requester:     requester,
		dispatcher:    dispatcher,
		eventPub:      eventPub,
		promptBuilder: promptBuilder,
		validator:     validator,
		workflow:      workflow,
		o11y:          o11y,
		outcomeTotal:  outcomeTotal,
	}
}

type HandleInboundResult struct {
	ReplyText string
	Outcome   domainservices.IntentOutcome
}

func (uc *HandleInboundMessage) Execute(ctx context.Context, raw commands.RawInterpretMessage) (HandleInboundResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.llm.usecase.handle_inbound_message")
	defer span.End()

	cmd, err := commands.NewInterpretMessage(raw)
	if err != nil {
		span.RecordError(err)
		return HandleInboundResult{}, fmt.Errorf("agent.llm.usecase.handle_inbound_message: command: %w", err)
	}

	seed, err := uc.loader.Load(ctx, cmd.UserID, cmd.Channel)
	if err != nil {
		span.RecordError(err)
		return HandleInboundResult{}, fmt.Errorf("agent.llm.usecase.handle_inbound_message: load context: %w", err)
	}

	now := time.Now().UTC()
	startedAt := now
	eventID := uuid.New()
	systemPrompt := uc.buildSystemPrompt(cmd, seed, now)

	llmResp, err := uc.requester.Interpret(ctx, interfaces.LLMRequest{SystemPrompt: systemPrompt, UserMessage: cmd.Text})
	if err != nil {
		return uc.handleInterpretError(ctx, span, cmd, llmResp, err, eventID, now, startedAt)
	}

	intent, err := uc.validator.Validate(llmResp.RawJSON)
	if err != nil {
		span.RecordError(err)
		return HandleInboundResult{}, fmt.Errorf("agent.llm.usecase.handle_inbound_message: validate: %w", err)
	}

	outcome := uc.workflow.DecideRoute(intent, llmResp.Provider, eventID, now)
	uc.outcomeTotal.Add(ctx, 1, observability.String("outcome", outcome.Kind.String()))

	if outcome.Kind != domainservices.IntentOutcomeRouted {
		uc.publishEvent(ctx, cmd, outcome, llmResp, startedAt, false)
		return HandleInboundResult{ReplyText: outcome.ResponseHint, Outcome: outcome}, nil
	}

	return uc.dispatchIntent(ctx, span, cmd, outcome, llmResp, startedAt)
}

func (uc *HandleInboundMessage) buildSystemPrompt(
	cmd commands.InterpretMessage,
	seed interfaces.PromptSeed,
	now time.Time,
) string {
	promptCtx := services.PromptContext{
		UserID:      cmd.UserID.String(),
		Channel:     cmd.Channel,
		Permissions: seed.Permissions,
		Categories:  toServiceCategorySeeds(seed.Categories),
		Cards:       toServiceCardSeeds(seed.Cards),
		CurrentDate: now,
	}
	return uc.promptBuilder.BuildSystemPrompt(promptCtx)
}

func (uc *HandleInboundMessage) handleInterpretError(
	ctx context.Context,
	span observability.Span,
	cmd commands.InterpretMessage,
	llmResp interfaces.LLMResponse,
	err error,
	eventID uuid.UUID,
	now time.Time,
	startedAt time.Time,
) (HandleInboundResult, error) {
	span.RecordError(err)
	if errors.Is(err, services.ErrFallbackChainExhausted) {
		outcome := uc.workflow.DecideExhausted("provider_exhausted", eventID, now)
		uc.outcomeTotal.Add(ctx, 1, observability.String("outcome", outcome.Kind.String()))
		uc.publishEvent(ctx, cmd, outcome, llmResp, startedAt, false)
		return HandleInboundResult{ReplyText: outcome.ResponseHint, Outcome: outcome}, nil
	}
	return HandleInboundResult{}, fmt.Errorf("agent.llm.usecase.handle_inbound_message: provider: %w", err)
}

func (uc *HandleInboundMessage) dispatchIntent(
	ctx context.Context,
	span observability.Span,
	cmd commands.InterpretMessage,
	outcome domainservices.IntentOutcome,
	llmResp interfaces.LLMResponse,
	startedAt time.Time,
) (HandleInboundResult, error) {
	source, srcErr := auth.SourceFromChannel(cmd.Channel)
	if srcErr != nil {
		uc.o11y.Logger().Warn(ctx, "agent.llm.handle_inbound_message.unknown_channel",
			observability.String("channel", cmd.Channel),
			observability.Error(srcErr),
		)
	}
	dispatchCtx := auth.WithPrincipal(ctx, auth.Principal{
		UserID: cmd.UserID,
		Source: source,
	})

	dispatch, err := uc.dispatcher.Dispatch(dispatchCtx, cmd.UserID, outcome)
	if err != nil {
		span.RecordError(err)
		uc.publishEvent(ctx, cmd, outcome, llmResp, startedAt, false)
		return HandleInboundResult{}, fmt.Errorf("agent.llm.usecase.handle_inbound_message: dispatch: %w", err)
	}
	reply := dispatch.ReplyText
	if reply == "" {
		reply = outcome.ResponseHint
	}
	uc.publishEvent(ctx, cmd, outcome, llmResp, startedAt, dispatch.WasApplied)
	return HandleInboundResult{ReplyText: reply, Outcome: outcome}, nil
}

func (uc *HandleInboundMessage) publishEvent(
	ctx context.Context,
	cmd commands.InterpretMessage,
	outcome domainservices.IntentOutcome,
	llmResp interfaces.LLMResponse,
	startedAt time.Time,
	wasApplied bool,
) {
	if uc.eventPub == nil {
		return
	}
	ev := interfaces.IntentEvent{
		EventID:          outcome.EventID,
		UserID:           cmd.UserID,
		Channel:          cmd.Channel,
		Outcome:          outcome.Kind.String(),
		ProviderUsed:     llmResp.Provider,
		Reason:           outcome.Reason,
		ResponseHint:     outcome.ResponseHint,
		LatencyMS:        time.Since(startedAt).Milliseconds(),
		PromptTokens:     llmResp.PromptTokens,
		CompletionTokens: llmResp.CompletionTokens,
		OccurredAt:       outcome.OccurredAt,
	}
	if !outcome.Intent.IsError() && !outcome.Intent.Module().IsZero() {
		ev.Module = outcome.Intent.Module().String()
		ev.Action = outcome.Intent.Action().String()
	}
	var pubErr error
	if wasApplied && outcome.Kind == domainservices.IntentOutcomeRouted {
		pubErr = uc.eventPub.PublishExecuted(ctx, ev)
	} else {
		pubErr = uc.eventPub.PublishRejected(ctx, ev)
	}
	if pubErr != nil {
		uc.o11y.Logger().Warn(ctx, "agent.llm.handle_inbound_message.publish_failed",
			observability.String("event_id", ev.EventID.String()),
			observability.Error(pubErr),
		)
	}
}

func toServiceCategorySeeds(in []interfaces.CategorySeed) []services.CategorySeed {
	out := make([]services.CategorySeed, 0, len(in))
	for _, c := range in {
		out = append(out, services.CategorySeed{ID: c.ID, Name: c.Name})
	}
	return out
}

func toServiceCardSeeds(in []interfaces.CardSeed) []services.CardSeed {
	out := make([]services.CardSeed, 0, len(in))
	for _, c := range in {
		out = append(out, services.CardSeed{ID: c.ID, Nickname: c.Nickname, Brand: c.Brand, LastFour: c.LastFour})
	}
	return out
}
