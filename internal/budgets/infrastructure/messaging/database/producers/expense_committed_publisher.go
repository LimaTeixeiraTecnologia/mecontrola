package producers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
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
}

func NewExpenseCommittedPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
) *ExpenseCommittedPublisher {
	return &ExpenseCommittedPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		idGen:         idGen,
	}
}

func (p *ExpenseCommittedPublisher) Publish(ctx context.Context, db database.DBTX, env interfaces.ExpenseCommittedEnvelope) error {
	payload := expenseCommittedPayload{
		UserID:             env.UserID.String(),
		Competence:         env.Competence.String(),
		SubcategoryID:      env.SubcategoryID.String(),
		RootSlug:           env.RootSlug.String(),
		MutationKind:       env.MutationKind.String(),
		CommittedAt:        env.CommittedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CutoffCompetenceBR: env.CutoffCompetenceBR.String(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("budgets/producer: marshal payload: %w", err)
	}

	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:            p.idGen.NewID(),
		Type:          eventTypeExpenseCommitted,
		AggregateType: aggregateTypeExpense,
		AggregateID:   env.ExpenseID.String(),
		Payload:       raw,
		OccurredAt:    env.CommittedAt,
	})
	if err != nil {
		return fmt.Errorf("budgets/producer: new event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)

	if err := publisher.Publish(ctx, evt); err != nil {
		return fmt.Errorf("budgets/producer: publish: %w", err)
	}
	return nil
}
