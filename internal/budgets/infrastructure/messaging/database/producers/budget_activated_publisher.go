package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeBudgetActivated = "budgets.budget_activated.v1"

const aggregateTypeBudgetActivated = "budgets.budget"

type budgetActivatedAllocationPayload struct {
	RootSlug     string `json:"root_slug"`
	BasisPoints  int    `json:"basis_points"`
	PlannedCents int64  `json:"planned_cents"`
}

type budgetActivatedPayload struct {
	UserID      string                             `json:"user_id"`
	BudgetID    string                             `json:"budget_id"`
	Competence  string                             `json:"competence"`
	TotalCents  int64                              `json:"total_cents"`
	ActivatedAt string                             `json:"activated_at"`
	Allocations []budgetActivatedAllocationPayload `json:"allocations"`
}

type BudgetActivatedPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
	o11y          observability.Observability
}

func NewBudgetActivatedPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
	o11y observability.Observability,
) *BudgetActivatedPublisher {
	return &BudgetActivatedPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		idGen:         idGen,
		o11y:          o11y,
	}
}

func (p *BudgetActivatedPublisher) Publish(ctx context.Context, db database.DBTX, budget entities.Budget, occurredAt time.Time) error {
	payload := budgetActivatedPayload{
		UserID:      budget.UserID().String(),
		BudgetID:    budget.ID().String(),
		Competence:  budget.Competence().String(),
		TotalCents:  budget.TotalCents(),
		ActivatedAt: occurredAt.UTC().Format(time.RFC3339),
		Allocations: make([]budgetActivatedAllocationPayload, 0, len(budget.Allocations())),
	}
	for _, allocation := range budget.Allocations() {
		payload.Allocations = append(payload.Allocations, budgetActivatedAllocationPayload{
			RootSlug:     allocation.RootSlug().String(),
			BasisPoints:  allocation.BasisPoints(),
			PlannedCents: allocation.PlannedCents(),
		})
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("budgets/producer: marshal budget_activated payload: %w", err)
	}

	event, err := outbox.NewEvent(outbox.EventInput{
		ID:              p.idGen.NewID(),
		Type:            eventTypeBudgetActivated,
		AggregateType:   aggregateTypeBudgetActivated,
		AggregateID:     budget.ID().String(),
		AggregateUserID: budget.UserID().String(),
		Payload:         raw,
		OccurredAt:      occurredAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("budgets/producer: new budget_activated event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if err := publisher.Publish(ctx, event); err != nil {
		return fmt.Errorf("budgets/producer: publish budget_activated: %w", err)
	}
	return nil
}
