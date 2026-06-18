//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	transactionCreatedType       = "transactions.transaction.created.v1"
	transactionUpdatedType       = "transactions.transaction.updated.v1"
	transactionDeletedType       = "transactions.transaction.deleted.v1"
	cardPurchaseCreatedType      = "transactions.card_purchase.created.v1"
	recurringTemplateCreatedType = "transactions.recurring_template.created.v1"
	agentIntentExecutedType      = "agent.intent.executed.v1"
	agentIntentRejectedType      = "agent.intent.rejected.v1"
)

type envelopeEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *envelopeEvent) GetEventType() string { return e.eventType }
func (e *envelopeEvent) GetPayload() any      { return e.envelope }

func (e *agentE2ECtx) thenOutboxContainsEventForUser(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		total         int
		aggregateType string
	)
	err := e.db.QueryRowContext(
		ctx,
		`SELECT count(*), COALESCE(max(aggregate_type), '')
		   FROM mecontrola.outbox_events
		  WHERE event_type = $1 AND aggregate_user_id = $2`,
		eventType, e.userID,
	).Scan(&total, &aggregateType)
	if err != nil {
		return fmt.Errorf("consultar outbox: %w", err)
	}
	if total < 1 {
		return fmt.Errorf("evento %q ausente no outbox do usuario %s", eventType, e.userID)
	}
	expectedAggregate := expectedAggregateType(eventType)
	if expectedAggregate != "" && aggregateType != expectedAggregate {
		return fmt.Errorf("aggregate_type inesperado %q para evento %q (esperado %q)", aggregateType, eventType, expectedAggregate)
	}
	return nil
}

func expectedAggregateType(eventType string) string {
	switch eventType {
	case transactionCreatedType, transactionUpdatedType, transactionDeletedType:
		return "transactions.transaction"
	case cardPurchaseCreatedType:
		return "transactions.card_purchase"
	case recurringTemplateCreatedType:
		return "transactions.recurring_template"
	case agentIntentExecutedType, agentIntentRejectedType:
		return "agent_session"
	default:
		return ""
	}
}

func (e *agentE2ECtx) whenOutboxIsDrained() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storage := outbox.NewPostgresStorage(e.db)
	rows, err := storage.ClaimBatch(ctx, "agent-e2e-outbox-drain", 200)
	if err != nil {
		return fmt.Errorf("claim outbox: %w", err)
	}

	e.budgetsConsumerHits = 0
	e.budgetsDeletedHits = 0
	e.budgetsCardHits = 0
	for _, row := range rows {
		envelope := outbox.Pack(row)
		event := &envelopeEvent{eventType: row.Type, envelope: envelope}

		switch row.Type {
		case transactionCreatedType, transactionUpdatedType, transactionDeletedType, cardPurchaseCreatedType:
			if handleErr := e.recomputeConsumer.Handle(ctx, platformevents.Event(event)); handleErr != nil {
				return fmt.Errorf("recompute consumer (%s): %w", row.Type, handleErr)
			}
		}

		switch row.Type {
		case transactionCreatedType:
			before := e.budgetsExpenseCount(ctx, row.AggregateID)
			if budgetErr := e.budgetsConsumer.Handle(ctx, platformevents.Event(event)); budgetErr != nil {
				return fmt.Errorf("budgets created consumer: %w", budgetErr)
			}
			if e.budgetsExpenseCount(ctx, row.AggregateID) > before {
				e.budgetsConsumerHits++
			}
		case transactionDeletedType:
			before := e.budgetsExpenseCount(ctx, row.AggregateID)
			if budgetErr := e.budgetsDelConsumer.Handle(ctx, platformevents.Event(event)); budgetErr != nil {
				return fmt.Errorf("budgets deleted consumer: %w", budgetErr)
			}
			if e.budgetsExpenseCount(ctx, row.AggregateID) < before {
				e.budgetsDeletedHits++
			}
		case cardPurchaseCreatedType:
			before := e.budgetsCardExpenseCount(ctx)
			if budgetErr := e.budgetsCardConsumer.Handle(ctx, platformevents.Event(event)); budgetErr != nil {
				return fmt.Errorf("budgets card purchase consumer: %w", budgetErr)
			}
			e.budgetsCardHits += e.budgetsCardExpenseCount(ctx) - before
		}
	}

	e.recomputeConsumer.Stop(ctx)
	return nil
}

func (e *agentE2ECtx) budgetsExpenseCount(ctx context.Context, externalTxID string) int {
	var total int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT count(*) FROM mecontrola.budgets_expenses
		  WHERE user_id = $1 AND external_transaction_id = $2 AND deleted_at IS NULL`,
		e.userID, externalTxID,
	).Scan(&total)
	if err != nil {
		return 0
	}
	return total
}

func (e *agentE2ECtx) budgetsCardExpenseCount(ctx context.Context) int {
	var total int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT count(*) FROM mecontrola.budgets_expenses
		  WHERE user_id = $1 AND source = $2 AND deleted_at IS NULL`,
		e.userID, "transactions_card",
	).Scan(&total)
	if err != nil {
		return 0
	}
	return total
}

func (e *agentE2ECtx) thenBudgetsRecordedInstallments(expected int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		total int
		sum   int64
	)
	err := e.db.QueryRowContext(
		ctx,
		`SELECT count(*), COALESCE(SUM(amount_cents), 0)
		   FROM mecontrola.budgets_expenses
		  WHERE user_id = $1 AND source = $2 AND deleted_at IS NULL`,
		e.userID, "transactions_card",
	).Scan(&total, &sum)
	if err != nil {
		return fmt.Errorf("consultar parcelas no orcamento: %w", err)
	}
	if total != expected {
		return fmt.Errorf("orcamento registrou %d parcelas de cartao (esperado %d); budgetsCardHits=%d", total, expected, e.budgetsCardHits)
	}
	if e.budgetsCardHits != expected {
		return fmt.Errorf("consumer de cartao gerou %d novas parcelas no drain (esperado %d): subcategoria ausente ou filtro aplicado", e.budgetsCardHits, expected)
	}
	totalAmount, _, cpErr := e.latestCardPurchase(e.userID)
	if cpErr != nil {
		return cpErr
	}
	if sum != totalAmount {
		return fmt.Errorf("soma das parcelas no orcamento %d difere do total da compra %d", sum, totalAmount)
	}
	return nil
}

func (e *agentE2ECtx) thenBudgetsExpenseWasRecorded() error {
	if e.budgetsConsumerHits < 1 {
		return fmt.Errorf("budgets consumer nao registrou despesa (hits=%d): subcategory ausente ou filtro aplicado", e.budgetsConsumerHits)
	}
	return nil
}

func (e *agentE2ECtx) thenMonthlySummaryReflectsExpense() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var outcomeCents int64
	err := e.db.QueryRowContext(
		ctx,
		`SELECT outcome_cents FROM mecontrola.transactions_monthly_summary
		  WHERE user_id = $1 AND ref_month = $2`,
		e.userID, e.lastRefMonth,
	).Scan(&outcomeCents)
	if err != nil {
		return fmt.Errorf("consultar resumo mensal (user=%s ref_month=%s): %w", e.userID, e.lastRefMonth, err)
	}
	if outcomeCents < 5000 {
		return fmt.Errorf("resumo mensal nao reflete a despesa: outcome_cents=%d (esperado >= 5000)", outcomeCents)
	}
	return nil
}
