package services

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type OnboardingAgent struct {
	onboardingRunner OnboardingTurnRunner
	onboarding       OnboardingContinuation
	o11y             observability.Observability
	routedTotal      observability.Counter
}

func newOnboardingAgent(o11y observability.Observability, routedTotal observability.Counter, deps IntentRouterDeps) *OnboardingAgent {
	return &OnboardingAgent{
		onboardingRunner: deps.OnboardingRunner,
		onboarding:       deps.Onboarding,
		o11y:             o11y,
		routedTotal:      routedTotal,
	}
}

func (a *OnboardingAgent) Handle(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (RouteResult, bool) {
	if a.onboardingRunner != nil {
		turn, err := a.onboardingRunner.Run(ctx, userID, channel, text)
		if err != nil {
			if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
				span.RecordError(err)
			}
			a.o11y.Logger().Warn(ctx, "agent.intent_router.onboarding_llm_failed",
				observability.String("channel", channel),
				observability.Error(err),
			)
			if reply, degraded := a.degradeOnboarding(ctx, userID, channel, peer, text, messageID); degraded {
				a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
				return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}, true
			}
			return RouteResult{}, false
		}
		if turn.Handled {
			a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
			return RouteResult{Reply: turn.Reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}, true
		}
		return RouteResult{}, false
	}
	if a.onboarding != nil {
		conversation, err := a.onboarding.Continue(ctx, userID, channel, peer, text, messageID)
		if err == nil && conversation.Handled {
			a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
			return RouteResult{Reply: conversation.Reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}, true
		}
	}
	return RouteResult{}, false
}

func (a *OnboardingAgent) degradeOnboarding(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (string, bool) {
	if a.onboarding == nil {
		return "", false
	}
	conversation, err := a.onboarding.Continue(ctx, userID, channel, peer, text, messageID)
	if err != nil || !conversation.Handled {
		return "", false
	}
	return conversation.Reply, true
}

func (a *OnboardingAgent) record(ctx context.Context, kind, channel string, outcome tools.ToolOutcome) {
	a.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}
