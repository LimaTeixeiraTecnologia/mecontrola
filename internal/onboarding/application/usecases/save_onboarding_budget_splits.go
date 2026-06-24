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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type BudgetSplitItem struct {
	Kind        valueobjects.CategoryKind
	AmountCents int64
}

type OnboardingSplitView struct {
	Kind        valueobjects.CategoryKind
	Percent     int
	AmountCents int64
}

type SaveOnboardingBudgetSplitsInput struct {
	UserID      uuid.UUID
	Allocations []BudgetSplitItem
}

type SaveOnboardingBudgetSplitsResult struct {
	Applied     bool
	SumCents    int64
	TotalCents  int64
	Allocations []OnboardingSplitView
}

type SaveOnboardingBudgetSplits struct {
	uow       uow.UnitOfWork
	factory   appinterfaces.RepositoryFactory
	publisher outbox.Publisher
	idGen     id.Generator
	o11y      observability.Observability
}

func NewSaveOnboardingBudgetSplits(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	publisher outbox.Publisher,
	idGen id.Generator,
	o11y observability.Observability,
) *SaveOnboardingBudgetSplits {
	return &SaveOnboardingBudgetSplits{uow: u, factory: factory, publisher: publisher, idGen: idGen, o11y: o11y}
}

func (uc *SaveOnboardingBudgetSplits) Execute(ctx context.Context, in SaveOnboardingBudgetSplitsInput) (SaveOnboardingBudgetSplitsResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.save_budget_splits")
	defer span.End()

	if in.UserID == uuid.Nil {
		return SaveOnboardingBudgetSplitsResult{}, fmt.Errorf("onboarding: save splits: user id required")
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (SaveOnboardingBudgetSplitsResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return SaveOnboardingBudgetSplitsResult{}, findErr
			}
			return SaveOnboardingBudgetSplitsResult{}, fmt.Errorf("onboarding: save splits: find session: %w", findErr)
		}

		totalCents := session.Payload().IncomeCents
		items := make([]valueobjects.CategoryAmount, 0, len(in.Allocations))
		var sum int64
		for _, a := range in.Allocations {
			items = append(items, valueobjects.CategoryAmount{Kind: a.Kind, AmountCents: a.AmountCents})
			sum += a.AmountCents
		}

		allocation, allocErr := valueobjects.NewBudgetAllocationFromAmounts(items, totalCents)
		if allocErr != nil {
			return SaveOnboardingBudgetSplitsResult{
				Applied:    false,
				SumCents:   sum,
				TotalCents: totalCents,
			}, nil
		}

		now := time.Now().UTC()
		withSplit := session.WithCustomSplit(allocation, now)
		if upsertErr := repo.Upsert(ctx, withSplit); upsertErr != nil {
			return SaveOnboardingBudgetSplitsResult{}, fmt.Errorf("onboarding: save splits: upsert session: %w", upsertErr)
		}

		event := domainservices.BuildSplitsCalculatedFromAllocation(
			in.UserID,
			session.Channel().String(),
			totalCents,
			allocation,
			newEventID(uc.idGen),
			now,
		)
		envelope, buildErr := buildOutboxEvent(in.UserID, event, now)
		if buildErr != nil {
			return SaveOnboardingBudgetSplitsResult{}, fmt.Errorf("onboarding: save splits: build event: %w", buildErr)
		}
		if pubErr := uc.publisher.Publish(ctx, envelope); pubErr != nil {
			return SaveOnboardingBudgetSplitsResult{}, fmt.Errorf("onboarding: save splits: publish event: %w", pubErr)
		}

		views := make([]OnboardingSplitView, 0, len(in.Allocations))
		for _, a := range in.Allocations {
			views = append(views, OnboardingSplitView{
				Kind:        a.Kind,
				Percent:     allocation.Percent(a.Kind),
				AmountCents: a.AmountCents,
			})
		}
		return SaveOnboardingBudgetSplitsResult{
			Applied:     true,
			SumCents:    sum,
			TotalCents:  totalCents,
			Allocations: views,
		}, nil
	})
}
