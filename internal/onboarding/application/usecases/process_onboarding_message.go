package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type ProcessOnboardingMessageInput struct {
	UserID    uuid.UUID
	Channel   entities.OnboardingChannel
	MessageID string
	Text      string
}

type ProcessOnboardingMessageOutcome uint8

const (
	ProcessOnboardingOutcomeNoOp ProcessOnboardingMessageOutcome = iota + 1
	ProcessOnboardingOutcomeAdvanced
	ProcessOnboardingOutcomeReplyOnly
	ProcessOnboardingOutcomeCompleted
)

type ProcessOnboardingMessageResult struct {
	Outcome   ProcessOnboardingMessageOutcome
	Reply     string
	FromState valueobjects.OnboardingState
	ToState   valueobjects.OnboardingState
}

type ProcessOnboardingMessage struct {
	uow         uow.UnitOfWork
	factory     appinterfaces.RepositoryFactory
	workflow    services.OnboardingWorkflow
	publisher   outbox.Publisher
	idGen       id.Generator
	o11y        observability.Observability
	transitions observability.Counter
}

func NewProcessOnboardingMessage(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	workflow services.OnboardingWorkflow,
	publisher outbox.Publisher,
	idGen id.Generator,
	o11y observability.Observability,
) *ProcessOnboardingMessage {
	transitions := o11y.Metrics().Counter(
		"onboarding_session_transitions_total",
		"Total de transicoes da sessao de onboarding conversacional",
		"1",
	)
	return &ProcessOnboardingMessage{
		uow:         u,
		factory:     factory,
		workflow:    workflow,
		publisher:   publisher,
		idGen:       idGen,
		o11y:        o11y,
		transitions: transitions,
	}
}

func (uc *ProcessOnboardingMessage) Execute(ctx context.Context, in ProcessOnboardingMessageInput) (ProcessOnboardingMessageResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.process_message")
	defer span.End()

	if in.UserID == uuid.Nil {
		return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: user id required")
	}

	var res ProcessOnboardingMessageResult
	err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		out, txErr := uc.executeInTx(ctx, tx, in)
		res = out
		return txErr
	})
	return res, err
}

func (uc *ProcessOnboardingMessage) executeInTx(ctx context.Context, tx database.DBTX, in ProcessOnboardingMessageInput) (ProcessOnboardingMessageResult, error) {
	repo := uc.factory.OnboardingSessionRepository(tx)

	session, err := repo.Find(ctx, in.UserID)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrOnboardingSessionNotFound) {
			return ProcessOnboardingMessageResult{}, appinterfaces.ErrOnboardingSessionNotFound
		}
		return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: find session: %w", err)
	}

	now := time.Now().UTC()
	eventIDs := uc.allocateEventIDs(session.State())

	decision, err := uc.workflow.DecideNext(session, services.InboundMessage{Text: in.Text}, eventIDs, now)
	if err != nil {
		return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: decide: %w", err)
	}

	fromState := session.State()

	switch decision.Kind {
	case services.DecisionKindNoOp:
		return ProcessOnboardingMessageResult{
			Outcome:   ProcessOnboardingOutcomeNoOp,
			FromState: fromState,
			ToState:   fromState,
		}, nil

	case services.DecisionKindReplyOnly:
		return ProcessOnboardingMessageResult{
			Outcome:   ProcessOnboardingOutcomeReplyOnly,
			Reply:     decision.OutboundText,
			FromState: fromState,
			ToState:   fromState,
		}, nil

	case services.DecisionKindAdvanceState, services.DecisionKindComplete:
		updated := session.With(decision.NewState, decision.NewPayload, now)
		if err := repo.Upsert(ctx, updated); err != nil {
			return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: upsert session: %w", err)
		}
		for _, evt := range decision.DomainEvents {
			envelope, err := buildOutboxEvent(in.UserID, evt, now)
			if err != nil {
				return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: build event: %w", err)
			}
			if err := uc.publisher.Publish(ctx, envelope); err != nil {
				return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: publish event: %w", err)
			}
		}
		uc.transitions.Add(ctx, 1,
			observability.String("from", fromState.String()),
			observability.String("to", decision.NewState.String()),
			observability.String("channel", session.Channel().String()),
		)
		slog.InfoContext(ctx, "onboarding.session.transition",
			"from", fromState.String(),
			"to", decision.NewState.String(),
			"channel", session.Channel().String(),
		)
		outcome := ProcessOnboardingOutcomeAdvanced
		if decision.Kind == services.DecisionKindComplete {
			outcome = ProcessOnboardingOutcomeCompleted
		}
		return ProcessOnboardingMessageResult{
			Outcome:   outcome,
			Reply:     decision.OutboundText,
			FromState: fromState,
			ToState:   decision.NewState,
		}, nil

	default:
		return ProcessOnboardingMessageResult{}, fmt.Errorf("onboarding: process message: unsupported decision kind=%d", decision.Kind)
	}
}

func (uc *ProcessOnboardingMessage) allocateEventIDs(state valueobjects.OnboardingState) []uuid.UUID {
	switch state {
	case valueobjects.OnboardingStateAwaitingIncome,
		valueobjects.OnboardingStateAwaitingCardName,
		valueobjects.OnboardingStateAwaitingCardDueDay:
		return []uuid.UUID{uc.parseID(uc.idGen.NewID())}
	case valueobjects.OnboardingStateAwaitingSplitConfirm:
		return []uuid.UUID{uc.parseID(uc.idGen.NewID()), uc.parseID(uc.idGen.NewID())}
	default:
		return nil
	}
}

func (uc *ProcessOnboardingMessage) parseID(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.New()
	}
	return parsed
}

func buildOutboxEvent(userID uuid.UUID, evt entities.OnboardingDomainEvent, now time.Time) (outbox.Event, error) {
	payload, err := json.Marshal(evt)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("onboarding: marshal event: %w", err)
	}
	eventID := extractEventID(evt)
	return outbox.NewEvent(outbox.EventInput{
		ID:              eventID.String(),
		Type:            evt.EventType(),
		AggregateType:   "onboarding_session",
		AggregateID:     userID.String(),
		AggregateUserID: userID.String(),
		Payload:         payload,
		OccurredAt:      now,
	})
}

func extractEventID(evt entities.OnboardingDomainEvent) uuid.UUID {
	switch e := evt.(type) {
	case entities.IncomeRegistered:
		return e.EventID
	case entities.CardRegistered:
		return e.EventID
	case entities.SplitsCalculated:
		return e.EventID
	case entities.OnboardingCompleted:
		return e.EventID
	default:
		return uuid.New()
	}
}
