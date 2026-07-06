package binding

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	budgetsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
)

type budgetPlannerAdapter struct {
	createBudget           *budgetsusecases.CreateBudget
	deleteDraftBudget      *budgetsusecases.DeleteDraftBudget
	activateBudget         *budgetsusecases.ActivateBudget
	createRecurrence       *budgetsusecases.CreateRecurrence
	editCategoryPercentage *budgetsusecases.EditCategoryPercentage
	getMonthlySummary      *budgetsusecases.GetMonthlySummary
	listAlerts             *budgetsusecases.ListAlerts
	suggestAllocation      *budgetsusecases.SuggestAllocation
	o11y                   observability.Observability
}

func NewBudgetPlannerAdapter(
	createBudget *budgetsusecases.CreateBudget,
	deleteDraftBudget *budgetsusecases.DeleteDraftBudget,
	activateBudget *budgetsusecases.ActivateBudget,
	createRecurrence *budgetsusecases.CreateRecurrence,
	editCategoryPercentage *budgetsusecases.EditCategoryPercentage,
	getMonthlySummary *budgetsusecases.GetMonthlySummary,
	listAlerts *budgetsusecases.ListAlerts,
	suggestAllocation *budgetsusecases.SuggestAllocation,
	o11y observability.Observability,
) agentsifaces.BudgetPlanner {
	return &budgetPlannerAdapter{
		createBudget:           createBudget,
		deleteDraftBudget:      deleteDraftBudget,
		activateBudget:         activateBudget,
		createRecurrence:       createRecurrence,
		editCategoryPercentage: editCategoryPercentage,
		getMonthlySummary:      getMonthlySummary,
		listAlerts:             listAlerts,
		suggestAllocation:      suggestAllocation,
		o11y:                   o11y,
	}
}

func (a *budgetPlannerAdapter) CreateBudget(ctx context.Context, in agentsifaces.DraftBudget) (agentsifaces.BudgetRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.create_budget")
	defer span.End()

	allocations := make([]budgetsinput.AllocationInput, 0, len(in.Allocations))
	for _, alloc := range in.Allocations {
		allocations = append(allocations, budgetsinput.AllocationInput{
			RootSlug:    alloc.RootSlug,
			BasisPoints: alloc.BasisPoints,
		})
	}

	out, err := a.createBudget.Execute(ctx, budgetsinput.CreateBudgetInput{
		UserID:      in.UserID.String(),
		Competence:  in.Competence,
		TotalCents:  in.TotalCents,
		Allocations: allocations,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.BudgetRef{}, fmt.Errorf("agents/binding/budget_planner: criar orçamento: %w", err)
	}
	return agentsifaces.BudgetRef{
		ID:         out.ID,
		Competence: out.Competence,
		State:      out.State,
	}, nil
}

func (a *budgetPlannerAdapter) DeleteDraftBudget(ctx context.Context, userID uuid.UUID, competence string) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.delete_draft_budget")
	defer span.End()

	if err := a.deleteDraftBudget.Execute(ctx, budgetsinput.DeleteDraftInput{
		UserID:     userID.String(),
		Competence: competence,
	}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/budget_planner: excluir rascunho de orçamento: %w", err)
	}
	return nil
}

func (a *budgetPlannerAdapter) ActivateBudget(ctx context.Context, userID uuid.UUID, competence string) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.activate_budget")
	defer span.End()

	_, err := a.activateBudget.Execute(ctx, budgetsinput.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: competence,
	})
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, budgetsifaces.ErrBudgetAlreadyActive) {
			return agentsifaces.ErrBudgetAlreadyActive
		}
		return fmt.Errorf("agents/binding/budget_planner: ativar orçamento: %w", err)
	}
	return nil
}

func (a *budgetPlannerAdapter) CreateRecurrence(ctx context.Context, userID uuid.UUID, competence string, months int) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.create_recurrence")
	defer span.End()

	_, err := a.createRecurrence.Execute(ctx, budgetsinput.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: competence,
		Months:           months,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/budget_planner: criar recorrência: %w", err)
	}
	return nil
}

func (a *budgetPlannerAdapter) EditCategoryPercentage(ctx context.Context, userID uuid.UUID, competence, rootSlug string, percentage int) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.edit_category_percentage")
	defer span.End()

	_, err := a.editCategoryPercentage.Execute(ctx, budgetsinput.EditCategoryPercentageInput{
		UserID:     userID.String(),
		Competence: competence,
		RootSlug:   rootSlug,
		Percentage: percentage,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/budget_planner: editar percentual de categoria: %w", err)
	}
	return nil
}

func (a *budgetPlannerAdapter) GetMonthlySummary(ctx context.Context, userID uuid.UUID, competence string) (agentsifaces.BudgetSummary, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.get_monthly_summary")
	defer span.End()

	out, err := a.getMonthlySummary.Execute(ctx, userID.String(), competence)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, budgetsifaces.ErrBudgetNotFound) {
			return agentsifaces.BudgetSummary{}, agentsifaces.ErrBudgetNotFound
		}
		return agentsifaces.BudgetSummary{}, fmt.Errorf("agents/binding/budget_planner: resumo mensal: %w", err)
	}

	allocations := make([]agentsifaces.AllocationSummary, 0, len(out.Allocations))
	for _, a := range out.Allocations {
		alloc := a
		allocations = append(allocations, agentsifaces.AllocationSummary{
			RootSlug:        alloc.RootSlug,
			PlannedCents:    alloc.PlannedCents,
			SpentCents:      alloc.SpentCents,
			PercentageSpent: alloc.PercentageSpent,
		})
	}

	return agentsifaces.BudgetSummary{
		Competence:        out.Competence,
		TotalCents:        out.TotalCents,
		State:             out.State,
		AutoDraft:         out.AutoDraft,
		Allocations:       allocations,
		TotalSpentCents:   out.TotalSpentCents,
		TotalPlannedCents: out.TotalPlannedCents,
	}, nil
}

func (a *budgetPlannerAdapter) ListAlerts(ctx context.Context, userID uuid.UUID) ([]agentsifaces.Alert, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.list_alerts")
	defer span.End()

	out, err := a.listAlerts.Execute(ctx, budgetsinput.ListAlertsInput{
		UserID: userID.String(),
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/budget_planner: listar alertas: %w", err)
	}

	alerts := make([]agentsifaces.Alert, 0, len(out.Alerts))
	for _, alert := range out.Alerts {
		alerts = append(alerts, agentsifaces.Alert{
			ID:           alert.ID,
			Competence:   alert.Competence,
			RootSlug:     alert.RootSlug,
			Threshold:    alert.Threshold,
			State:        alert.State,
			SpentCents:   alert.SpentCents,
			PlannedCents: alert.PlannedCents,
		})
	}
	return alerts, nil
}

func (a *budgetPlannerAdapter) SuggestAllocation(ctx context.Context, totalCents int64, allocations []agentsifaces.AllocationBP) ([]agentsifaces.AllocationCents, error) {
	_, span := a.o11y.Tracer().Start(ctx, "agents.binding.budget_planner.suggest_allocation")
	defer span.End()

	bps := make([]budgetsusecases.AllocationBP, 0, len(allocations))
	for _, alloc := range allocations {
		bps = append(bps, budgetsusecases.AllocationBP{
			RootSlug:    alloc.RootSlug,
			BasisPoints: alloc.BasisPoints,
		})
	}

	result, err := a.suggestAllocation.Execute(ctx, budgetsusecases.SuggestAllocationInput{
		TotalCents:  totalCents,
		Allocations: bps,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/budget_planner: sugerir alocação: %w", err)
	}

	out := make([]agentsifaces.AllocationCents, 0, len(result.Allocations))
	for _, r := range result.Allocations {
		out = append(out, agentsifaces.AllocationCents{
			RootSlug:     r.RootSlug,
			BasisPoints:  r.BasisPoints,
			PlannedCents: r.PlannedCents,
		})
	}
	return out, nil
}
