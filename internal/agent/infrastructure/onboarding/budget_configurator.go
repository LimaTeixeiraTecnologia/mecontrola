package onboarding

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type budgetConfiguratorAdapter struct {
	uc *usecases.StartBudgetConfiguration
}

func NewBudgetConfiguratorAdapter(uc *usecases.StartBudgetConfiguration) tools.BudgetConfigurator {
	if uc == nil {
		return nil
	}
	return &budgetConfiguratorAdapter{uc: uc}
}

func (a *budgetConfiguratorAdapter) Start(ctx context.Context, userID uuid.UUID, channel string) (string, error) {
	parsed := mapAgentChannelToOnboarding(channel)
	if _, err := a.uc.Execute(ctx, usecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: parsed,
	}); err != nil {
		return "", err
	}
	return "", nil
}

func mapAgentChannelToOnboarding(channel string) entities.OnboardingChannel {
	switch channel {
	case appservices.ChannelTelegram:
		return entities.OnboardingChannelTelegram
	default:
		return entities.OnboardingChannelWhatsApp
	}
}
