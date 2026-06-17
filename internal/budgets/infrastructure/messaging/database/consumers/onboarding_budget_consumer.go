package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type onboardingCreateBudgetUseCase interface {
	Execute(ctx context.Context, in input.CreateBudgetInput) (output.BudgetOutput, error)
}

type onboardingActivateBudgetUseCase interface {
	Execute(ctx context.Context, in input.ActivateBudgetInput) (output.BudgetOutput, error)
}

type splitsCalculatedPayload struct {
	UserID      string                         `json:"UserID"`
	IncomeCents int64                          `json:"IncomeCents"`
	Allocations []splitsCalculatedEntryPayload `json:"Allocations"`
}

type splitsCalculatedEntryPayload struct {
	Kind    string `json:"Kind"`
	Percent int    `json:"Percent"`
}

type OnboardingBudgetConsumer struct {
	createBudget   onboardingCreateBudgetUseCase
	activateBudget onboardingActivateBudgetUseCase
	o11y           observability.Observability
	decodeFails    observability.Counter
}

func NewOnboardingBudgetConsumer(
	createBudget onboardingCreateBudgetUseCase,
	activateBudget onboardingActivateBudgetUseCase,
	o11y observability.Observability,
) *OnboardingBudgetConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_onboarding_budget_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de onboarding splits",
		"1",
	)
	return &OnboardingBudgetConsumer{
		createBudget:   createBudget,
		activateBudget: activateBudget,
		o11y:           o11y,
		decodeFails:    decodeFails,
	}
}

func (c *OnboardingBudgetConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.onboarding_budget.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.onboarding_budget: unexpected payload type %T", rawPayload)
	}

	var p splitsCalculatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.onboarding_budget: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.onboarding_budget: user_id inválido: %w", err)
	}

	competence := time.Now().UTC().Format("2006-01")

	allocations, mapErr := mapAllocations(p.Allocations)
	if mapErr != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.onboarding_budget: mapear allocations: %w", mapErr)
	}

	_, createErr := c.createBudget.Execute(ctx, input.CreateBudgetInput{
		UserID:      userID.String(),
		Competence:  competence,
		TotalCents:  p.IncomeCents,
		Allocations: allocations,
	})
	if createErr != nil {
		if errors.Is(createErr, appinterfaces.ErrBudgetConflict) {
			return nil
		}
		return fmt.Errorf("budgets.consumer.onboarding_budget: criar orçamento: %w", createErr)
	}

	if _, activateErr := c.activateBudget.Execute(ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: competence,
	}); activateErr != nil {
		return fmt.Errorf("budgets.consumer.onboarding_budget: ativar orçamento: %w", activateErr)
	}

	return nil
}

func mapAllocations(entries []splitsCalculatedEntryPayload) ([]input.AllocationInput, error) {
	out := make([]input.AllocationInput, 0, len(entries))
	for _, e := range entries {
		slug, err := mapCategoryKindToRootSlug(e.Kind)
		if err != nil {
			return nil, err
		}
		out = append(out, input.AllocationInput{
			RootSlug:    slug,
			BasisPoints: e.Percent * 100,
		})
	}
	return out, nil
}

func mapCategoryKindToRootSlug(kind string) (string, error) {
	switch kind {
	case "fixed_cost":
		return "expense.custo_fixo", nil
	case "knowledge":
		return "expense.conhecimento", nil
	case "pleasures":
		return "expense.prazeres", nil
	case "goals":
		return "expense.metas", nil
	case "financial_freedom":
		return "expense.liberdade_financeira", nil
	default:
		return "", fmt.Errorf("budgets.consumer.onboarding_budget: kind desconhecido: %q", kind)
	}
}
