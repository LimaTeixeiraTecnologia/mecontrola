package services

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

const OnboardingWelcomeSignal = "__onboarding_welcome__"

const onboardingRetryReply = "Tive um probleminha aqui 😅 Pode me mandar de novo?"

type OnboardingAgent struct {
	onboardingRunner OnboardingTurnRunner
	o11y             observability.Observability
	routedTotal      observability.Counter
}

func newOnboardingAgent(o11y observability.Observability, routedTotal observability.Counter, deps IntentRouterDeps) *OnboardingAgent {
	return &OnboardingAgent{
		onboardingRunner: deps.OnboardingRunner,
		o11y:             o11y,
		routedTotal:      routedTotal,
	}
}

func (a *OnboardingAgent) Handle(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (RouteResult, bool) {
	if a.onboardingRunner == nil {
		return RouteResult{}, false
	}
	turn, err := a.onboardingRunner.Run(ctx, userID, channel, text)
	if err != nil {
		if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
			span.RecordError(err)
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.onboarding_llm_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: onboardingRetryReply, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindConfigureBudget}, true
	}
	if turn.Handled {
		a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeRouted)
		return RouteResult{Reply: turn.Reply, Outcome: tools.OutcomeRouted, Kind: intent.KindConfigureBudget}, true
	}
	return RouteResult{}, false
}

func (a *OnboardingAgent) record(ctx context.Context, kind, channel string, outcome tools.ToolOutcome) {
	a.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}
