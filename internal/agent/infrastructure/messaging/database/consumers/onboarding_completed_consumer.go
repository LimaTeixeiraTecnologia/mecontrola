package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agententities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type onboardingContextReader interface {
	Execute(ctx context.Context, in onbusecases.GetOnboardingContextInput) (onbusecases.GetOnboardingContextResult, error)
}

type OnboardingCompletedConsumer struct {
	contextReader onboardingContextReader
	wmRepo        agentinterfaces.WorkingMemoryRepository
	o11y          observability.Observability
	decodeFails   observability.Counter
	processTotal  observability.Counter
}

func NewOnboardingCompletedConsumer(
	contextReader onboardingContextReader,
	wmRepo agentinterfaces.WorkingMemoryRepository,
	o11y observability.Observability,
) *OnboardingCompletedConsumer {
	return &OnboardingCompletedConsumer{
		contextReader: contextReader,
		wmRepo:        wmRepo,
		o11y:          o11y,
		decodeFails: o11y.Metrics().Counter(
			"agent_onboarding_completed_consumer_decode_failed_total",
			"Total de falhas de decode do consumer onboarding_completed no agente",
			"1",
		),
		processTotal: o11y.Metrics().Counter(
			"agent_onboarding_completed_consumer_total",
			"Total de execucoes do consumer onboarding_completed no agente",
			"1",
		),
	}
}

func (c *OnboardingCompletedConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "agent.consumer.onboarding_completed.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agent.consumer.onboarding_completed: tipo de payload inesperado %T", event.GetPayload())
	}

	var p onboardingCompletedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_completed: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.onboarding_completed: parse user_id: %w", err)
	}

	snapshot, err := c.contextReader.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return fmt.Errorf("agent.consumer.onboarding_completed: read context: %w", err)
	}
	if !snapshot.Found {
		return nil
	}

	wm, found, err := c.wmRepo.Get(ctx, userID)
	if err != nil {
		c.processTotal.Add(ctx, 1, observability.String("result", "error"))
		return fmt.Errorf("agent.consumer.onboarding_completed: get wm: %w", err)
	}
	if found && wm.Content != "" {
		c.processTotal.Add(ctx, 1, observability.String("result", "skipped"))
		return nil
	}
	if !found {
		wm = agententities.NewWorkingMemory(userID)
	}
	wm.Update(buildOnboardingWM(snapshot), time.Now().UTC())
	if err := c.wmRepo.Upsert(ctx, wm); err != nil {
		c.processTotal.Add(ctx, 1, observability.String("result", "error"))
		return fmt.Errorf("agent.consumer.onboarding_completed: upsert wm: %w", err)
	}
	c.processTotal.Add(ctx, 1, observability.String("result", "success"))
	return nil
}

type onboardingCompletedPayload struct {
	UserID string `json:"user_id"`
}

func buildOnboardingWM(s onbusecases.GetOnboardingContextResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Perfil Financeiro do Usuário\n")
	fmt.Fprintf(&sb, "- **Objetivo financeiro principal**: %s\n", s.Objective)
	fmt.Fprintf(&sb, "- **Renda mensal estimada**: %s\n", money.FromCents(s.IncomeCents).Amount())
	if len(s.Cards) > 0 {
		names := make([]string, 0, len(s.Cards))
		for _, c := range s.Cards {
			names = append(names, c.Name)
		}
		fmt.Fprintf(&sb, "- **Cartões cadastrados**: %s\n", strings.Join(names, ", "))
	}
	return sb.String()
}
