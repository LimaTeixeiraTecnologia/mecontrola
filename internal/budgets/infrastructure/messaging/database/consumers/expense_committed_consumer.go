package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeExpenseCommitted = "budgets.expense.committed.v1"

type evaluateAlertUseCase interface {
	Execute(ctx context.Context, in usecases.EvaluateAlertInput) error
}

type expenseCommittedPayload struct {
	UserID             string `json:"user_id"`
	Competence         string `json:"competence"`
	SubcategoryID      string `json:"subcategory_id"`
	RootSlug           string `json:"root_slug"`
	MutationKind       string `json:"mutation_kind"`
	CommittedAt        string `json:"committed_at"`
	CutoffCompetenceBR string `json:"cutoff_competence_br"`
}

type ExpenseCommittedConsumer struct {
	evaluateAlert evaluateAlertUseCase
	o11y          observability.Observability
	decodeFails   observability.Counter
}

func NewExpenseCommittedConsumer(
	evaluateAlert evaluateAlertUseCase,
	o11y observability.Observability,
) *ExpenseCommittedConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_expense_committed_consumer_decode_failed_total",
		"Total de falhas de decode do consumer expense_committed",
		"1",
	)
	return &ExpenseCommittedConsumer{
		evaluateAlert: evaluateAlert,
		o11y:          o11y,
		decodeFails:   decodeFails,
	}
}

func (c *ExpenseCommittedConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.expense_committed.handle")
	defer span.End()

	payload := event.GetPayload()
	env, ok := payload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.expense_committed: unexpected payload type %T", payload)
	}

	if event.GetEventType() != eventTypeExpenseCommitted {
		return fmt.Errorf("budgets.consumer.expense_committed: unhandled event type %q", event.GetEventType())
	}

	var p expenseCommittedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.expense_committed: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.expense_committed: user_id inválido: %w", err)
	}

	competence, err := valueobjects.NewCompetence(p.Competence)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.expense_committed: competence inválida: %w", err)
	}

	rootSlug, err := valueobjects.ParseRootSlug(p.RootSlug)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.expense_committed: root_slug inválido: %w", err)
	}

	committedAt, err := time.Parse(time.RFC3339, p.CommittedAt)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.expense_committed: committed_at inválido: %w", err)
	}

	cutoff, err := valueobjects.NewCompetence(p.CutoffCompetenceBR)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.expense_committed: cutoff_competence_br inválido: %w", err)
	}

	in := usecases.EvaluateAlertInput{
		EventID:            env.ID,
		Payload:            env.Payload,
		CommittedAt:        committedAt.UTC(),
		CutoffCompetenceBR: cutoff,
		UserID:             userID,
		Competence:         competence,
		RootSlug:           rootSlug,
	}

	if err := c.evaluateAlert.Execute(ctx, in); err != nil {
		return fmt.Errorf("budgets.consumer.expense_committed: avaliar alerta: %w", err)
	}

	return nil
}
