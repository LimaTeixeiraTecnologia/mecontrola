package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ListBudgetsAlertsUseCase interface {
	Execute(ctx context.Context, in budgetsinput.ListAlertsInput) (budgetsoutput.ListAlertsOutput, error)
}

type GetMonthlySummaryUseCase interface {
	Execute(ctx context.Context, userID string, competence string) (budgetsoutput.MonthlySummaryOutput, error)
}

type CreateBudgetUseCase interface {
	Execute(ctx context.Context, in budgetsinput.CreateBudgetInput) (budgetsoutput.BudgetOutput, error)
}

type ActivateBudgetUseCase interface {
	Execute(ctx context.Context, in budgetsinput.ActivateBudgetInput) (budgetsoutput.BudgetOutput, error)
}

type CreateRecurrenceUseCase interface {
	Execute(ctx context.Context, in budgetsinput.CreateRecurrenceInput) (budgetsoutput.RecurrenceResultOutput, error)
}

type UpsertExpenseUseCase interface {
	Execute(ctx context.Context, in budgetsinput.UpsertExpenseInput) (budgetsoutput.ExpenseOutput, error)
}

type DeleteDraftBudgetUseCase interface {
	Execute(ctx context.Context, in budgetsinput.DeleteDraftInput) error
}

type DeleteExpenseUseCase interface {
	Execute(ctx context.Context, in budgetsinput.DeleteExpenseInput) error
}

type BudgetsAdapter struct {
	listUseCase             ListBudgetsAlertsUseCase
	getSummaryUseCase       GetMonthlySummaryUseCase
	createBudgetUseCase     CreateBudgetUseCase
	activateBudgetUseCase   ActivateBudgetUseCase
	createRecurrenceUseCase CreateRecurrenceUseCase
	upsertExpenseUseCase    UpsertExpenseUseCase
	deleteDraftUseCase      DeleteDraftBudgetUseCase
	deleteExpenseUseCase    DeleteExpenseUseCase
}

func NewBudgetsAdapter(listUseCase ListBudgetsAlertsUseCase) *BudgetsAdapter {
	return &BudgetsAdapter{listUseCase: listUseCase}
}

func NewBudgetsAdapterFull(
	listUseCase ListBudgetsAlertsUseCase,
	getSummaryUseCase GetMonthlySummaryUseCase,
	createBudgetUseCase CreateBudgetUseCase,
	activateBudgetUseCase ActivateBudgetUseCase,
	createRecurrenceUseCase CreateRecurrenceUseCase,
	upsertExpenseUseCase UpsertExpenseUseCase,
	deleteDraftUseCase DeleteDraftBudgetUseCase,
	deleteExpenseUseCase DeleteExpenseUseCase,
) *BudgetsAdapter {
	return &BudgetsAdapter{
		listUseCase:             listUseCase,
		getSummaryUseCase:       getSummaryUseCase,
		createBudgetUseCase:     createBudgetUseCase,
		activateBudgetUseCase:   activateBudgetUseCase,
		createRecurrenceUseCase: createRecurrenceUseCase,
		upsertExpenseUseCase:    upsertExpenseUseCase,
		deleteDraftUseCase:      deleteDraftUseCase,
		deleteExpenseUseCase:    deleteExpenseUseCase,
	}
}

type budgetsListFilters struct {
	Operation string `json:"operation"`
	Month     string `json:"month"`
	RootSlug  string `json:"root_slug"`
	Threshold int    `json:"threshold"`
}

func (a *BudgetsAdapter) List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	if a.listUseCase == nil {
		return "", fmt.Errorf("agent.llm.dispatcher.budgets.list: %w", ErrIntentUnsupported)
	}
	competence := defaultBudgetsCompetence()
	var parsed budgetsListFilters
	if len(rawFilters) > 0 {
		if err := json.Unmarshal(rawFilters, &parsed); err == nil && strings.TrimSpace(parsed.Month) != "" {
			competence = strings.TrimSpace(parsed.Month)
		}
	}
	if strings.EqualFold(strings.TrimSpace(parsed.Operation), "summary") {
		return a.Get(ctx, userID, rawFilters)
	}

	competenceVO, err := valueobjects.NewCompetence(competence)
	if err != nil {
		return "", fmt.Errorf("budgets.list: invalid competence %q: %w", competence, err)
	}

	in := budgetsinput.ListAlertsInput{
		UserID:     userID.String(),
		Competence: &competenceVO,
		Limit:      10,
	}

	out, err := a.listUseCase.Execute(ctx, in)
	if err != nil {
		return "", fmt.Errorf("budgets.list: %w", err)
	}
	if len(out.Alerts) == 0 {
		return fmt.Sprintf("Nenhum alerta de orcamento em %s.", competence), nil
	}

	parts := make([]string, 0, len(out.Alerts))
	for _, a := range out.Alerts {
		state := strings.ToLower(a.State)
		parts = append(parts, fmt.Sprintf("%s %d%% (%s)", a.RootSlug, a.Threshold, state))
	}
	return fmt.Sprintf("Em %s voce tem %d alerta(s) de orcamento: %s.",
		competence, len(out.Alerts), strings.Join(parts, "; ")), nil
}

func (a *BudgetsAdapter) Get(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	if a.getSummaryUseCase == nil {
		return "", fmt.Errorf("budgets.get: %w", ErrIntentUnsupported)
	}
	competence := defaultBudgetsCompetence()
	if len(rawFilters) > 0 {
		var filters budgetsListFilters
		if err := json.Unmarshal(rawFilters, &filters); err == nil && strings.TrimSpace(filters.Month) != "" {
			competence = strings.TrimSpace(filters.Month)
		}
	}
	out, err := a.getSummaryUseCase.Execute(ctx, userID.String(), competence)
	if err != nil {
		return "", fmt.Errorf("budgets.get: %w", err)
	}
	return formatBudgetSummary(out), nil
}

type budgetsCreatePayload struct {
	Operation        string                   `json:"operation"`
	Competence       string                   `json:"competence"`
	TotalCents       int64                    `json:"total_cents"`
	Allocations      []budgetsAllocationEntry `json:"allocations"`
	SourceCompetence string                   `json:"source_competence"`
	Months           int                      `json:"months"`
	Source           string                   `json:"source"`
	ExternalID       string                   `json:"external_transaction_id"`
	SubcategoryID    string                   `json:"subcategory_id"`
	AmountCents      int64                    `json:"amount_cents"`
	OccurredAt       string                   `json:"occurred_at"`
}

type budgetsAllocationEntry struct {
	RootSlug    string `json:"root_slug"`
	BasisPoints int    `json:"basis_points"`
}

func (a *BudgetsAdapter) Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	var payload budgetsCreatePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return "", fmt.Errorf("budgets.create: payload invalido")
	}
	switch strings.ToLower(strings.TrimSpace(payload.Operation)) {
	case "budget":
		if a.createBudgetUseCase == nil {
			return "", fmt.Errorf("budgets.create_budget: %w", ErrIntentUnsupported)
		}
		allocations := make([]budgetsinput.AllocationInput, 0, len(payload.Allocations))
		for _, item := range payload.Allocations {
			allocations = append(allocations, budgetsinput.AllocationInput{
				RootSlug:    strings.TrimSpace(item.RootSlug),
				BasisPoints: item.BasisPoints,
			})
		}
		out, err := a.createBudgetUseCase.Execute(ctx, budgetsinput.CreateBudgetInput{
			UserID:      userID.String(),
			Competence:  normalizeBudgetCompetence(payload.Competence),
			TotalCents:  payload.TotalCents,
			Allocations: allocations,
		})
		if err != nil {
			return "", fmt.Errorf("budgets.create_budget: %w", err)
		}
		return fmt.Sprintf("Orcamento criado para %s com total de R$ %s.", out.Competence, formatCents(out.TotalCents)), nil
	case "recurrence":
		if a.createRecurrenceUseCase == nil {
			return "", fmt.Errorf("budgets.create_recurrence: %w", ErrIntentUnsupported)
		}
		out, err := a.createRecurrenceUseCase.Execute(ctx, budgetsinput.CreateRecurrenceInput{
			UserID:           userID.String(),
			SourceCompetence: strings.TrimSpace(payload.SourceCompetence),
			Months:           payload.Months,
		})
		if err != nil {
			return "", fmt.Errorf("budgets.create_recurrence: %w", err)
		}
		return fmt.Sprintf("Recorrencia criada a partir de %s para %d mes(es).", out.SourceCompetence, len(out.Results)), nil
	case "expense":
		if a.upsertExpenseUseCase == nil {
			return "", fmt.Errorf("budgets.create_expense: %w", ErrIntentUnsupported)
		}
		occurredAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.OccurredAt))
		if err != nil {
			occurredAt = time.Now().UTC()
		}
		out, err := a.upsertExpenseUseCase.Execute(ctx, budgetsinput.UpsertExpenseInput{
			UserID:                userID.String(),
			Source:                fallbackString(payload.Source, "agent"),
			ExternalTransactionID: fallbackString(payload.ExternalID, uuid.NewString()),
			SubcategoryID:         strings.TrimSpace(payload.SubcategoryID),
			Competence:            normalizeBudgetCompetence(payload.Competence),
			AmountCents:           payload.AmountCents,
			OccurredAt:            occurredAt,
		})
		if err != nil {
			return "", fmt.Errorf("budgets.create_expense: %w", err)
		}
		return fmt.Sprintf("Despesa registrada em budgets: R$ %s na competencia %s.", formatCents(out.AmountCents), out.Competence), nil
	default:
		return "", fmt.Errorf("budgets.create: operation nao suportada")
	}
}

type budgetsUpdatePayload struct {
	Operation   string `json:"operation"`
	Competence  string `json:"competence"`
	Source      string `json:"source"`
	ExternalID  string `json:"external_transaction_id"`
	AmountCents int64  `json:"amount_cents"`
	OccurredAt  string `json:"occurred_at"`
}

func (a *BudgetsAdapter) Update(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	var payload budgetsUpdatePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return "", fmt.Errorf("budgets.update: payload invalido")
	}
	switch strings.ToLower(strings.TrimSpace(payload.Operation)) {
	case "activate_budget":
		if a.activateBudgetUseCase == nil {
			return "", fmt.Errorf("budgets.activate_budget: %w", ErrIntentUnsupported)
		}
		out, err := a.activateBudgetUseCase.Execute(ctx, budgetsinput.ActivateBudgetInput{
			UserID:     userID.String(),
			Competence: normalizeBudgetCompetence(payload.Competence),
		})
		if err != nil {
			return "", fmt.Errorf("budgets.activate_budget: %w", err)
		}
		return fmt.Sprintf("Orcamento %s ativado com total de R$ %s.", out.Competence, formatCents(out.TotalCents)), nil
	default:
		return "", fmt.Errorf("budgets.update: operation nao suportada")
	}
}

type budgetsDeletePayload struct {
	Operation  string `json:"operation"`
	Competence string `json:"competence"`
	Source     string `json:"source"`
	ExternalID string `json:"external_transaction_id"`
	Version    int64  `json:"version"`
}

func (a *BudgetsAdapter) Delete(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	var payload budgetsDeletePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return "", fmt.Errorf("budgets.delete: payload invalido")
	}
	switch strings.ToLower(strings.TrimSpace(payload.Operation)) {
	case "draft_budget":
		if a.deleteDraftUseCase == nil {
			return "", fmt.Errorf("budgets.delete_draft: %w", ErrIntentUnsupported)
		}
		if err := a.deleteDraftUseCase.Execute(ctx, budgetsinput.DeleteDraftInput{
			UserID:     userID.String(),
			Competence: normalizeBudgetCompetence(payload.Competence),
		}); err != nil {
			return "", fmt.Errorf("budgets.delete_draft: %w", err)
		}
		return "Rascunho de orcamento removido.", nil
	case "expense":
		if a.deleteExpenseUseCase == nil {
			return "", fmt.Errorf("budgets.delete_expense: %w", ErrIntentUnsupported)
		}
		if err := a.deleteExpenseUseCase.Execute(ctx, budgetsinput.DeleteExpenseInput{
			UserID:                userID.String(),
			Source:                fallbackString(payload.Source, "agent"),
			ExternalTransactionID: strings.TrimSpace(payload.ExternalID),
			ExpectedVersion:       payload.Version,
		}); err != nil {
			return "", fmt.Errorf("budgets.delete_expense: %w", err)
		}
		return "Despesa removida de budgets.", nil
	default:
		return "", fmt.Errorf("budgets.delete: operation nao suportada")
	}
}

func defaultBudgetsCompetence() string {
	return time.Now().UTC().Format("2006-01")
}

func normalizeBudgetCompetence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultBudgetsCompetence()
	}
	return trimmed
}

func formatBudgetSummary(out budgetsoutput.MonthlySummaryOutput) string {
	totalPlanned := "sem planejamento"
	if out.TotalPlannedCents != nil {
		totalPlanned = formatCents(*out.TotalPlannedCents)
	}
	return fmt.Sprintf("Resumo de %s: gasto total R$ %s, planejado R$ %s, %d categoria(s) acompanhada(s).",
		out.Competence,
		formatCents(out.TotalSpentCents),
		totalPlanned,
		len(out.Allocations),
	)
}

func fallbackString(raw string, fallback string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
