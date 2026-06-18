package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

const (
	startBudgetReplyIncomePrompt         = "Beleza! Qual a sua renda mensal? Pode me dizer o valor."
	startBudgetReplyResumeOnboarding     = "Você já está configurando seu orçamento. Continue de onde parou."
	startBudgetReplyResumeWithLastPrompt = "Você já está configurando seu orçamento. Continue de onde parou: %s"
)

var ErrStartBudgetUserIDRequired = errors.New("onboarding: start budget configuration: user id required")

type StartBudgetConfigurationInput struct {
	UserID  uuid.UUID
	Channel entities.OnboardingChannel
}

type StartBudgetConfigurationOutcome uint8

const (
	StartBudgetOutcomeStarted StartBudgetConfigurationOutcome = iota + 1
	StartBudgetOutcomeReset
	StartBudgetOutcomeResume
)

type StartBudgetConfigurationResult struct {
	Outcome  StartBudgetConfigurationOutcome
	Reply    string
	NewState valueobjects.OnboardingState
}

type StartBudgetConfiguration struct {
	uow        uow.UnitOfWork
	factory    appinterfaces.RepositoryFactory
	o11y       observability.Observability
	startTotal observability.Counter
}

func NewStartBudgetConfiguration(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *StartBudgetConfiguration {
	startTotal := o11y.Metrics().Counter(
		"onboarding_budget_configuration_started_total",
		"Total de aberturas de configuracao de orcamento via agente conversacional",
		"1",
	)
	return &StartBudgetConfiguration{
		uow:        u,
		factory:    factory,
		o11y:       o11y,
		startTotal: startTotal,
	}
}

func (uc *StartBudgetConfiguration) Execute(ctx context.Context, in StartBudgetConfigurationInput) (StartBudgetConfigurationResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.start_budget_configuration")
	defer span.End()

	if in.UserID == uuid.Nil {
		return StartBudgetConfigurationResult{}, ErrStartBudgetUserIDRequired
	}
	channel := in.Channel
	if channel != entities.OnboardingChannelWhatsApp && channel != entities.OnboardingChannelTelegram {
		channel = entities.OnboardingChannelWhatsApp
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (StartBudgetConfigurationResult, error) {
		return uc.executeInTx(ctx, tx, in.UserID, channel)
	})
}

func (uc *StartBudgetConfiguration) executeInTx(
	ctx context.Context,
	tx database.DBTX,
	userID uuid.UUID,
	channel entities.OnboardingChannel,
) (StartBudgetConfigurationResult, error) {
	repo := uc.factory.OnboardingSessionRepository(tx)
	now := time.Now().UTC()

	session, err := repo.Find(ctx, userID)
	if err != nil {
		if !errors.Is(err, appinterfaces.ErrOnboardingSessionNotFound) {
			return StartBudgetConfigurationResult{}, fmt.Errorf("onboarding: start budget: find session: %w", err)
		}
		newSession, buildErr := entities.NewOnboardingSession(userID, channel, valueobjects.OnboardingStateAwaitingIncome, now)
		if buildErr != nil {
			return StartBudgetConfigurationResult{}, fmt.Errorf("onboarding: start budget: new session: %w", buildErr)
		}
		if upsertErr := repo.Upsert(ctx, newSession); upsertErr != nil {
			return StartBudgetConfigurationResult{}, fmt.Errorf("onboarding: start budget: upsert session: %w", upsertErr)
		}
		uc.recordOutcome(ctx, channel, "started")
		return StartBudgetConfigurationResult{
			Outcome:  StartBudgetOutcomeStarted,
			Reply:    startBudgetReplyIncomePrompt,
			NewState: valueobjects.OnboardingStateAwaitingIncome,
		}, nil
	}

	state := session.State()
	switch state {
	case valueobjects.OnboardingStateActive:
		reset := entities.HydrateOnboardingSession(
			userID,
			channel,
			valueobjects.OnboardingStateAwaitingIncome,
			entities.OnboardingSessionPayload{},
			now,
		)
		if upsertErr := repo.Upsert(ctx, reset); upsertErr != nil {
			return StartBudgetConfigurationResult{}, fmt.Errorf("onboarding: start budget: upsert session: %w", upsertErr)
		}
		uc.recordOutcome(ctx, channel, "reset")
		return StartBudgetConfigurationResult{
			Outcome:  StartBudgetOutcomeReset,
			Reply:    startBudgetReplyIncomePrompt,
			NewState: valueobjects.OnboardingStateAwaitingIncome,
		}, nil
	default:
		hint := resumePromptForState(state)
		reply := startBudgetReplyResumeOnboarding
		if hint != "" {
			reply = fmt.Sprintf(startBudgetReplyResumeWithLastPrompt, hint)
		}
		uc.recordOutcome(ctx, channel, "resume")
		return StartBudgetConfigurationResult{
			Outcome:  StartBudgetOutcomeResume,
			Reply:    reply,
			NewState: state,
		}, nil
	}
}

func (uc *StartBudgetConfiguration) recordOutcome(ctx context.Context, channel entities.OnboardingChannel, outcome string) {
	uc.startTotal.Add(ctx, 1,
		observability.String("channel", channel.String()),
		observability.String("outcome", outcome),
	)
}

func resumePromptForState(state valueobjects.OnboardingState) string {
	switch state {
	case valueobjects.OnboardingStateAwaitingToken:
		return "envie ATIVAR <seu codigo> para ativar."
	case valueobjects.OnboardingStateAwaitingIncome:
		return "qual a sua renda mensal?"
	case valueobjects.OnboardingStateAwaitingCardDecision:
		return "quer cadastrar um cartao de credito agora? sim ou nao."
	case valueobjects.OnboardingStateAwaitingCardName:
		return "qual o nome do cartao?"
	case valueobjects.OnboardingStateAwaitingCardLimit:
		return "qual o limite total do cartao?"
	case valueobjects.OnboardingStateAwaitingCardClosingDay:
		return "qual o dia de fechamento da fatura (1 a 31)?"
	case valueobjects.OnboardingStateAwaitingCardDueDay:
		return "qual o dia de vencimento (1 a 31)?"
	case valueobjects.OnboardingStateAwaitingMoreCards:
		return "quer cadastrar outro cartao? sim ou nao."
	case valueobjects.OnboardingStateAwaitingSplitConfirm:
		return "confirme com sim para usar a sugestao das categorias."
	default:
		return ""
	}
}
