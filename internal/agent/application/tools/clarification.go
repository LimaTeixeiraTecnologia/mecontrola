package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type ClarificationResolver struct {
	recorder *Recorder
	o11y     observability.Observability
}

func NewClarificationResolver(recorder *Recorder, o11y observability.Observability) *ClarificationResolver {
	return &ClarificationResolver{recorder: recorder, o11y: o11y}
}

func (c *ClarificationResolver) ResolveCategory(ctx context.Context, userID uuid.UUID, channel string, kind intent.Kind, in intent.Intent, err error) (ToolResult, bool) {
	var ambiguous *CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		c.o11y.Logger().Warn(ctx, "agent.intent_router.category_ambiguous",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		c.recorder.Record(ctx, kind.String(), channel, OutcomeClarify)
		return ToolResult{Reply: formatCategoryAmbiguous(ambiguous.Candidates), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if needsConfirmation, ok := errors.AsType[*CategoryNeedsConfirmationError](err); ok {
		c.o11y.Logger().Warn(ctx, "agent.intent_router.category_needs_confirmation",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
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

func resolveCategoryHint(in intent.Intent) string {
	hint := strings.TrimSpace(in.CategoryHint())
	if hint == "" {
		hint = strings.TrimSpace(in.Merchant())
	}
	return hint
}
