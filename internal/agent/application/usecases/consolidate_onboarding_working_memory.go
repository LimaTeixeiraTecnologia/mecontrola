package usecases

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
)

type onboardingContextReader interface {
	Execute(ctx context.Context, in onbusecases.GetOnboardingContextInput) (onbusecases.GetOnboardingContextResult, error)
}

type ConsolidateOnboardingWorkingMemoryInput struct {
	UserID     uuid.UUID
	EventID    uuid.UUID
	EventType  string
	OccurredAt time.Time
}

type ConsolidateOnboardingWorkingMemory struct {
	uow              uow.UnitOfWork
	contextReader    onboardingContextReader
	wmRepoFactory    agentinterfaces.WorkingMemoryRepositoryFactory
	processedFactory agentinterfaces.ProcessedEventRepositoryFactory
	o11y             observability.Observability
}

func NewConsolidateOnboardingWorkingMemory(
	u uow.UnitOfWork,
	contextReader onboardingContextReader,
	wmRepoFactory agentinterfaces.WorkingMemoryRepositoryFactory,
	processedFactory agentinterfaces.ProcessedEventRepositoryFactory,
	o11y observability.Observability,
) *ConsolidateOnboardingWorkingMemory {
	return &ConsolidateOnboardingWorkingMemory{
		uow:              u,
		contextReader:    contextReader,
		wmRepoFactory:    wmRepoFactory,
		processedFactory: processedFactory,
		o11y:             o11y,
	}
}

func (uc *ConsolidateOnboardingWorkingMemory) Execute(ctx context.Context, in ConsolidateOnboardingWorkingMemoryInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.consolidate_onboarding_working_memory")
	defer span.End()

	if in.UserID == uuid.Nil {
		return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: user id required")
	}
	if in.EventID == uuid.Nil {
		return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: event id required")
	}

	snapshot, err := uc.contextReader.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: in.UserID})
	if err != nil {
		return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: read context: %w", err)
	}
	if !snapshot.Found {
		return nil
	}

	return uc.uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		processedRepo := uc.processedFactory.ProcessedEventRepository(db)
		alreadyProcessed, checkErr := processedRepo.IsProcessed(ctx, in.EventID)
		if checkErr != nil {
			return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: check processed: %w", checkErr)
		}
		if alreadyProcessed {
			return nil
		}

		wmRepo := uc.wmRepoFactory.WorkingMemoryRepository(db)
		wm, found, getErr := wmRepo.Get(ctx, in.UserID)
		if getErr != nil {
			return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: get wm: %w", getErr)
		}
		if !found {
			wm = entities.NewWorkingMemory(in.UserID)
		}
		if wm.Content == "" {
			wm.Update(buildOnboardingWorkingMemory(snapshot), in.OccurredAt)
			if upsertErr := wmRepo.Upsert(ctx, wm); upsertErr != nil {
				return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: upsert wm: %w", upsertErr)
			}
		}

		if markErr := processedRepo.MarkProcessed(ctx, in.EventID, in.EventType, in.UserID, in.OccurredAt); markErr != nil {
			if errors.Is(markErr, agentinterfaces.ErrProcessedEventAlreadyExists) {
				return nil
			}
			return fmt.Errorf("agent.usecase.consolidate_onboarding_wm: mark processed: %w", markErr)
		}
		return nil
	})
}

func buildOnboardingWorkingMemory(s onbusecases.GetOnboardingContextResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Perfil Financeiro do Usuário\n")
	fmt.Fprintf(&sb, "- **Objetivo financeiro principal**: %s\n", s.Objective)
	fmt.Fprintf(&sb, "- **Renda mensal estimada**: R$ %s\n", money.FromCents(s.IncomeCents).Amount())
	if len(s.Cards) > 0 {
		names := make([]string, 0, len(s.Cards))
		for _, c := range s.Cards {
			names = append(names, c.Name)
		}
		fmt.Fprintf(&sb, "- **Cartões cadastrados**: %s\n", strings.Join(names, ", "))
	}
	if len(s.CustomSplit) > 0 {
		fmt.Fprintf(&sb, "- **Distribuição planejada**:\n")
		for _, a := range s.CustomSplit {
			percent := float64(a.BasisPoints) / 100.0
			fmt.Fprintf(&sb, "  - %s: %s%%\n", allocationKindLabel(a.Kind), formatPercent(percent))
		}
	}
	return sb.String()
}

func formatPercent(v float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(v, 'f', 2, 64), ".", ",")
}

func allocationKindLabel(kind string) string {
	switch kind {
	case "fixed_cost":
		return "💰 Custo Fixo"
	case "knowledge":
		return "🎓 Conhecimento"
	case "pleasures":
		return "🎉 Prazeres"
	case "goals":
		return "🎯 Metas"
	case "financial_freedom":
		return "🏦 Liberdade Financeira"
	default:
		return kind
	}
}
