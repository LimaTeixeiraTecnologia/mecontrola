package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const (
	categoryDetailOutcomeOK       = "ok"
	categoryDetailOutcomeClarify  = "clarify"
	categoryDetailOutcomeNotFound = "not_found"
	categoryDetailEntriesLimit    = 200
)

type CategoryDetailInput struct {
	Category     string `json:"category,omitempty"`
	MonthRefKind string `json:"monthRefKind,omitempty"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
}

type CategoryDetailOutput struct {
	Outcome           string `json:"outcome"`
	Competence        string `json:"competence"`
	Message           string `json:"message"`
	ClarifyPrompt     string `json:"clarifyPrompt,omitempty"`
	OfferCreatePrompt string `json:"offerCreatePrompt,omitempty"`
}

func BuildCategoryDetailTool(planner interfaces.BudgetPlanner, ledger interfaces.TransactionsLedger, reader interfaces.CategoriesReader) tool.ToolHandle {
	in := llm.Schema{
		Name:   "category_detail_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{"type": "string", "description": "Nome da categoria ou subcategoria a detalhar (ex.: mercado, custo fixo). Deixe vazio para o resumo geral do orçamento."},
				"monthRefKind": map[string]any{
					"type":        "string",
					"enum":        []string{"current", "previous", "next", "explicit", "named_without_year", "unknown"},
					"description": "Classificação da referência de mês citada pelo usuário. Vazio assume o mês corrente.",
				},
				"year":  map[string]any{"type": "integer"},
				"month": map[string]any{"type": "integer", "minimum": 1, "maximum": 12},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "category_detail_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":           map[string]any{"type": "string"},
				"competence":        map[string]any{"type": "string"},
				"message":           map[string]any{"type": "string"},
				"clarifyPrompt":     map[string]any{"type": "string"},
				"offerCreatePrompt": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "competence", "message"},
			"additionalProperties": false,
		},
	}
	exec := buildCategoryDetailExec(planner, ledger, reader)
	return tool.NewVerbatimTool("category_detail", "Consulta o resumo do orçamento por categoria (planejado, gasto, disponível e lançamentos do período) ou o panorama geral quando nenhuma categoria é informada.", in, out, exec, extractCategoryDetailVerbatim)
}

func extractCategoryDetailVerbatim(o CategoryDetailOutput) (string, bool) {
	return o.Message, o.Outcome == categoryDetailOutcomeOK && o.Message != ""
}

func buildCategoryDetailExec(planner interfaces.BudgetPlanner, ledger interfaces.TransactionsLedger, reader interfaces.CategoriesReader) func(context.Context, CategoryDetailInput) (CategoryDetailOutput, error) {
	return func(ctx context.Context, in CategoryDetailInput) (CategoryDetailOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return CategoryDetailOutput{}, fmt.Errorf("agents.tool.category_detail: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return CategoryDetailOutput{}, fmt.Errorf("agents.tool.category_detail: userId inválido: %w", err)
		}

		competence, clarifyReason, err := resolveCompetenceReference(in.MonthRefKind, in.Year, in.Month)
		if err != nil {
			return CategoryDetailOutput{}, fmt.Errorf("agents.tool.category_detail: resolver competência: %w", err)
		}
		if clarifyReason != budgetsvo.ClarifyNone {
			return CategoryDetailOutput{
				Outcome:       categoryDetailOutcomeClarify,
				ClarifyPrompt: competenceReferenceClarifyPrompt(clarifyReason),
			}, nil
		}
		competenceStr := competence.String()
		if competenceStr == "" {
			competenceStr = currentCompetenceFallback()
		}

		summary, err := planner.GetMonthlySummary(ctx, userID, competenceStr)
		if err != nil {
			if errors.Is(err, interfaces.ErrBudgetNotFound) {
				return CategoryDetailOutput{
					Outcome:           categoryDetailOutcomeNotFound,
					Competence:        competenceStr,
					OfferCreatePrompt: budgetNotFoundOfferPrompt(competenceStr),
				}, nil
			}
			return CategoryDetailOutput{}, fmt.Errorf("agents.tool.category_detail: resumo orçamentário: %w", err)
		}

		if strings.TrimSpace(in.Category) == "" {
			nameBySlug := map[string]string{}
			if generalCategories, listErr := reader.ListCategories(ctx, userID); listErr == nil {
				nameBySlug = categoryNamesBySlug(generalCategories)
			}
			return CategoryDetailOutput{
				Outcome:    categoryDetailOutcomeOK,
				Competence: competenceStr,
				Message:    buildGeneralSummaryMessage(summary, nameBySlug),
			}, nil
		}

		categories, err := reader.ListCategories(ctx, userID)
		if err != nil {
			return CategoryDetailOutput{}, fmt.Errorf("agents.tool.category_detail: listar categorias: %w", err)
		}
		root, matched := matchCategoryRoot(categories, in.Category)
		if !matched {
			return CategoryDetailOutput{
				Outcome:       categoryDetailOutcomeClarify,
				Competence:    competenceStr,
				ClarifyPrompt: "Não encontrei essa categoria. Pode me dizer o nome exato da categoria (ex.: custo fixo, prazeres)?",
			}, nil
		}

		alloc := matchAllocation(summary.Allocations, root.Slug)
		entries, err := ledger.ListMonthlyEntries(ctx, userID, competenceStr, "", categoryDetailEntriesLimit)
		if err != nil {
			return CategoryDetailOutput{}, fmt.Errorf("agents.tool.category_detail: lançamentos mensais: %w", err)
		}

		return CategoryDetailOutput{
			Outcome:    categoryDetailOutcomeOK,
			Competence: competenceStr,
			Message:    buildCategorySummaryMessage(root, alloc, entries),
		}, nil
	}
}

func matchCategoryRoot(categories []interfaces.Category, term string) (interfaces.Category, bool) {
	normalized := normalizeCategoryTerm(term)
	for _, c := range categories {
		if normalizeCategoryTerm(c.Name) == normalized || normalizeCategoryTerm(c.Slug) == normalized {
			return c, true
		}
	}
	for _, c := range categories {
		if strings.Contains(normalizeCategoryTerm(c.Name), normalized) || strings.Contains(normalizeCategoryTerm(c.Slug), normalized) {
			return c, true
		}
		for _, sub := range c.Subcategories {
			if normalizeCategoryTerm(sub.Name) == normalized || normalizeCategoryTerm(sub.Slug) == normalized ||
				strings.Contains(normalizeCategoryTerm(sub.Name), normalized) {
				return c, true
			}
		}
	}
	return interfaces.Category{}, false
}

func normalizeCategoryTerm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func matchAllocation(allocations []interfaces.AllocationSummary, rootSlug string) interfaces.AllocationSummary {
	for _, a := range allocations {
		if a.RootSlug == rootSlug {
			return a
		}
	}
	return interfaces.AllocationSummary{RootSlug: rootSlug}
}

func buildCategorySummaryMessage(root interfaces.Category, alloc interfaces.AllocationSummary, entries []interfaces.MonthlyEntry) string {
	entryViews := make([]messages.CategoryEntryView, 0, len(entries))
	for _, e := range entries {
		if e.CategoryID != root.ID.String() {
			continue
		}
		label := e.SubcategoryNameSnapshot
		if label == "" {
			label = e.CategoryNameSnapshot
		}
		entryViews = append(entryViews, messages.CategoryEntryView{
			DateFormatted:   e.CreatedAt.Format("02/01"),
			AmountFormatted: money.FromCents(e.AmountCents).BRL(),
			Subcategory:     label,
		})
	}

	var spent int64
	if alloc.SpentCents != 0 {
		spent = alloc.SpentCents
	}
	var planned *int64
	if alloc.PlannedCents != nil {
		planned = alloc.PlannedCents
	}
	scenario := workflows.DecideCategorySummaryScenario(planned, spent)

	view := messages.CategoryView{
		Category:         root.Name,
		Entries:          entryViews,
		PlannedFormatted: moneyOrDash(planned),
		SpentFormatted:   money.FromCents(spent).BRL(),
		Scenario:         scenario,
	}
	if planned != nil {
		available := *planned - spent
		if available < 0 {
			view.OverrunFormatted = money.FromCents(-available).BRL()
			view.AvailableFormatted = money.FromCents(0).BRL()
		} else {
			view.AvailableFormatted = money.FromCents(available).BRL()
		}
	}
	return messages.CategorySummaryBlock(view)
}

func categoryNamesBySlug(categories []interfaces.Category) map[string]string {
	names := make(map[string]string, len(categories))
	for _, c := range categories {
		if c.Slug != "" && c.Name != "" {
			names[c.Slug] = c.Name
		}
	}
	return names
}

func friendlyCategoryName(nameBySlug map[string]string, slug string) string {
	if name, ok := nameBySlug[slug]; ok && name != "" {
		return name
	}
	return slug
}

func buildGeneralSummaryMessage(summary interfaces.BudgetSummary, nameBySlug map[string]string) string {
	rows := make([]messages.GeneralCategoryRowView, 0, len(summary.Allocations))
	for _, a := range summary.Allocations {
		available := int64(0)
		if a.PlannedCents != nil {
			available = *a.PlannedCents - a.SpentCents
		}
		rows = append(rows, messages.GeneralCategoryRowView{
			Category:           friendlyCategoryName(nameBySlug, a.RootSlug),
			PlannedFormatted:   moneyOrDash(a.PlannedCents),
			SpentFormatted:     money.FromCents(a.SpentCents).BRL(),
			AvailableFormatted: money.FromCents(available).BRL(),
		})
	}

	totalAvailable := int64(0)
	if summary.TotalPlannedCents != nil {
		totalAvailable = *summary.TotalPlannedCents - summary.TotalSpentCents
	}
	scenario := workflows.DecideGeneralSummaryScenario(summary.TotalPlannedCents, summary.TotalSpentCents)

	view := messages.GeneralView{
		Categories:              rows,
		TotalPlannedFormatted:   moneyOrDash(summary.TotalPlannedCents),
		TotalSpentFormatted:     money.FromCents(summary.TotalSpentCents).BRL(),
		TotalAvailableFormatted: money.FromCents(totalAvailable).BRL(),
		Scenario:                scenario,
	}
	return messages.GeneralSummaryBlock(view)
}

func moneyOrDash(cents *int64) string {
	if cents == nil {
		return "—"
	}
	return money.FromCents(*cents).BRL()
}
