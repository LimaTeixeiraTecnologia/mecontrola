package tools

import (
	"context"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type Conversational struct {
	recorder *Recorder
	fallback Fallback
	o11y     observability.Observability
}

func NewConversational(recorder *Recorder, fallback Fallback, o11y observability.Observability) *Conversational {
	return &Conversational{recorder: recorder, fallback: fallback, o11y: o11y}
}

func (t *Conversational) Name() string { return "conversational" }

func (t *Conversational) Descriptor() ToolSpec {
	return ToolSpec{Name: "conversational", IntentKind: intent.KindUnknown, Description: "conversational", SchemaVersion: "v1", Timeout: 0, AuthzMode: AuthzPublic}
}

func (t *Conversational) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	reply := t.Reply(ctx, in.UserID, in.Channel, in.Intent.RawText())
	t.recorder.Record(ctx, intent.KindUnknown.String(), in.Channel, OutcomeFallback)
	return ToolResult{Reply: reply, Outcome: OutcomeFallback, Kind: intent.KindUnknown}, nil
}

func (t *Conversational) Reply(ctx context.Context, userID uuid.UUID, channel, text string) string {
	reply, err := t.fallback.Reply(ctx, userID, channel, text)
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.fallback_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return FallbackParseError
	}
	if strings.TrimSpace(reply) == "" {
		return FallbackParseError
	}
	return reply
}
