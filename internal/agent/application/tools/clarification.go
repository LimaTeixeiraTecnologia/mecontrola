package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
)

const (
	directionOutcomeConst = "outcome"
	directionIncomeConst  = "income"
)

type ClarificationResolver struct {
	pending  PendingExpenseConfirmationGateway
	recorder *Recorder
	o11y     observability.Observability
}

func NewClarificationResolver(pending PendingExpenseConfirmationGateway, recorder *Recorder, o11y observability.Observability) *ClarificationResolver {
	return &ClarificationResolver{pending: pending, recorder: recorder, o11y: o11y}
}

func (c *ClarificationResolver) ResolveCategory(ctx context.Context, userID uuid.UUID, channel string, kind intent.Kind, in intent.Intent, err error) (ToolResult, bool) {
	var ambiguous *CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		c.o11y.Logger().Warn(ctx, "agent.intent_router.category_ambiguous",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		if c.pending != nil && len(ambiguous.Candidates) > 0 {
			draft := c.buildPendingDraft(in, kind, ambiguous.Candidates, pendingexpense.AwaitingCategoryChoice)
			c.savePendingDraft(ctx, userID, channel, draft)
		}
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: formatCategoryAmbiguous(ambiguous.Candidates), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if needsConfirmation, ok := errors.AsType[*CategoryNeedsConfirmationError](err); ok {
		c.o11y.Logger().Warn(ctx, "agent.intent_router.category_needs_confirmation",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		if c.pending != nil && len(needsConfirmation.Candidates) > 0 {
			draft := c.buildPendingDraft(in, kind, needsConfirmation.Candidates, pendingexpense.AwaitingCategoryConfirm)
			c.savePendingDraft(ctx, userID, channel, draft)
		}
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: formatCategoryNeedsConfirmation(needsConfirmation.Candidates), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if errors.Is(err, ErrCategoryNotFound) {
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: formatCategoryNotFound(resolveCategoryHint(in)), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if errors.Is(err, ErrCategoryHintMissing) {
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: categoryNoHintText, Outcome: OutcomeClarify, Kind: kind}, true
	}
	return ToolResult{}, false
}

func (c *ClarificationResolver) ResolveCard(ctx context.Context, channel string, kind intent.Kind, cardName string, err error) (ToolResult, bool) {
	if errors.Is(err, ErrAgentCardAmbiguous) {
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: formatCardAmbiguous(cardName), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if errors.Is(err, ErrAgentCardNotFound) {
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: formatCardNotFound(cardName), Outcome: OutcomeClarify, Kind: kind}, true
	}
	return ToolResult{}, false
}

func (c *ClarificationResolver) savePendingDraft(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) {
	if saveErr := c.pending.Save(ctx, userID, channel, draft); saveErr != nil {
		c.o11y.Logger().Warn(ctx, "agent.intent_router.pending_draft_save_failed",
			observability.String("channel", channel),
			observability.Error(saveErr),
		)
	}
}

func (c *ClarificationResolver) buildPendingDraft(in intent.Intent, kind intent.Kind, candidates []string, awaitingKind pendingexpense.AwaitingKind) pendingexpense.Draft {
	txnKind := pendingexpense.TransactionKindExpense
	direction := directionOutcomeConst
	if kind == intent.KindRecordIncome { //nolint:staticcheck // switch proibido por R-AGENT-WF-001: gate rejeita case intent.Kind > 1
		txnKind = pendingexpense.TransactionKindIncome
		direction = directionIncomeConst
	} else if kind == intent.KindRecordCardPurchase {
		txnKind = pendingexpense.TransactionKindCardPurchase
	}
	categoryPath := ""
	if len(candidates) > 0 {
		categoryPath = candidates[0]
	}
	return pendingexpense.Draft{
		AmountCents:     in.AmountCents(),
		Merchant:        in.Merchant(),
		PaymentMethod:   in.PaymentMethod(),
		Direction:       direction,
		OccurredAt:      "",
		CategoryID:      categoryPath,
		CategoryPath:    categoryPath,
		Candidates:      candidates,
		AwaitingKind:    awaitingKind,
		TransactionKind: txnKind,
		Installments:    in.Installments(),
		CardHint:        in.CardHint(),
	}
}

func resolveCategoryHint(in intent.Intent) string {
	hint := strings.TrimSpace(in.CategoryHint())
	if hint == "" {
		hint = strings.TrimSpace(in.Merchant())
	}
	return hint
}
