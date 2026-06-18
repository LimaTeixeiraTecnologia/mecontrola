package producers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeExpenseCommitted = "budgets.expense.committed.v1"

const aggregateTypeExpense = "budgets.expense"

type expenseCommittedPayload struct {
	UserID             string `json:"user_id"`
	Competence         string `json:"competence"`
	SubcategoryID      string `json:"subcategory_id"`
	RootSlug           string `json:"root_slug"`
	MutationKind       string `json:"mutation_kind"`
	CommittedAt        string `json:"committed_at"`
	CutoffCompetenceBR string `json:"cutoff_competence_br"`
}

type ExpenseCommittedPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
	o11y          observability.Observability
}

func NewExpenseCommittedPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
	o11y observability.Observability,
) *ExpenseCommittedPublisher {
	return &ExpenseCommittedPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		idGen:         idGen,
		o11y:          o11y,
	}
}

func (p *ExpenseCommittedPublisher) Publish(ctx context.Context, db database.DBTX, evt events.ExpenseCommitted) error {
	payload := expenseCommittedPayload{
		UserID:             evt.UserID.String(),
		Competence:         evt.Competence.String(),
		SubcategoryID:      evt.SubcategoryID.String(),
		RootSlug:           evt.RootSlug.String(),
		MutationKind:       evt.MutationKind.String(),
		CommittedAt:        evt.CommittedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CutoffCompetenceBR: evt.CutoffCompetenceBR.String(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("budgets/producer: marshal payload: %w", err)
	}

	outboxEvt, err := outbox.NewEvent(outbox.EventInput{
		ID:              p.idGen.NewID(),
		Type:            eventTypeExpenseCommitted,
		AggregateType:   aggregateTypeExpense,
		AggregateID:     evt.ExpenseID.String(),
		AggregateUserID: evt.UserID.String(),
		Payload:         raw,
		OccurredAt:      evt.CommittedAt,
	})
	if err != nil {
		return fmt.Errorf("budgets/producer: new event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)

	if err := publisher.Publish(ctx, outboxEvt); err != nil {
		return fmt.Errorf("budgets/producer: publish: %w", err)
	}
	return nil
}
