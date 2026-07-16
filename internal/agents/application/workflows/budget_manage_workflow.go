package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	budgetsentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	BudgetManageWorkflowID  = "budget-manage"
	stepBudgetManageID      = "budget-manage"
	BudgetManageStaleAfter  = 35 * time.Minute
	BudgetManageReaperBatch = 100
)

func BudgetManageKey(resourceID, threadID string) string {
	return CorrelationKey(resourceID, threadID, BudgetManageWorkflowID)
}

func BuildBudgetManageWorkflow(a agent.Agent, planner interfaces.BudgetPlanner) workflow.Definition[BudgetManageState] {
	step := workflow.NewStepFunc(stepBudgetManageID, buildBudgetManageStep(a, planner))
	return workflow.Definition[BudgetManageState]{
		ID:          BudgetManageWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildBudgetManageReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, BudgetManageWorkflowID, BudgetManageStaleAfter, BudgetManageReaperBatch, o11y)
}

type budgetManageExecFn func(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error)

func budgetManageExecMap() map[BudgetManageOperationKind]budgetManageExecFn {
	return map[BudgetManageOperationKind]budgetManageExecFn{
		BudgetManageOpCreateRetroactive: executeBudgetManageCreateRetroactive,
		BudgetManageOpEditTotal:         executeBudgetManageEditTotal,
		BudgetManageOpEditDistribution:  executeBudgetManageEditDistribution,
	}
}

func buildBudgetManageStep(a agent.Agent, planner interfaces.BudgetPlanner) func(context.Context, BudgetManageState) (workflow.StepOutput[BudgetManageState], error) {
	execMap := budgetManageExecMap()
	return func(ctx context.Context, state BudgetManageState) (workflow.StepOutput[BudgetManageState], error) {
		if state.Awaiting == 0 {
			return budgetManageEnter(ctx, state, planner)
		}
		switch state.Awaiting {
		case BudgetManageAwaitingTotal:
			return handleBudgetManageTotalSlot(ctx, state, a)
		case BudgetManageAwaitingDistribution:
			return handleBudgetManageDistributionSlot(ctx, state, a)
		case BudgetManageAwaitingConfirm:
			return handleBudgetManageConfirmSlot(ctx, state, planner, execMap)
		default:
			return budgetManageSuspend(state, budgetTotalPrompt())
		}
	}
}

func budgetManageSuspend(state BudgetManageState, prompt string) (workflow.StepOutput[BudgetManageState], error) {
	state.ResponseText = prompt
	if state.SuspendedAt.IsZero() {
		state.SuspendedAt = time.Now().UTC()
	}
	return workflow.StepOutput[BudgetManageState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}, nil
}

func budgetManageComplete(state BudgetManageState) (workflow.StepOutput[BudgetManageState], error) {
	return workflow.StepOutput[BudgetManageState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func budgetManageFail(state BudgetManageState, err error) (workflow.StepOutput[BudgetManageState], error) {
	return workflow.StepOutput[BudgetManageState]{State: state, Status: workflow.StepStatusFailed}, err
}

func budgetManageExpireStep(state BudgetManageState) (workflow.StepOutput[BudgetManageState], error) {
	state.Status = BudgetManageExpired
	state.Expired = true
	state.ResponseText = ""
	state.ResumeText = ""
	return budgetManageComplete(state)
}

func budgetManageEnter(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error) {
	switch state.Operation {
	case BudgetManageOpCreateRetroactive:
		state.Awaiting = BudgetManageAwaitingTotal
		return budgetManageSuspend(state, budgetTotalPrompt())
	case BudgetManageOpEditTotal:
		return enterBudgetManageEditTotal(ctx, state, planner)
	case BudgetManageOpEditDistribution:
		return enterBudgetManageEditDistribution(ctx, state, planner)
	default:
		state.Status = BudgetManageCancelled
		state.ResponseText = "🚫 Não foi possível identificar a operação de orçamento solicitada."
		return budgetManageComplete(state)
	}
}

func enterBudgetManageEditTotal(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error) {
	summary, err := planner.GetMonthlySummary(ctx, state.UserID, state.Competence)
	if err != nil {
		if errors.Is(err, interfaces.ErrBudgetNotFound) {
			state.Status = BudgetManageCancelled
			state.ResponseText = "Você ainda não tem um orçamento ativo. Crie um orçamento antes de alterar o valor total."
			return budgetManageComplete(state)
		}
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.edit_total: get_monthly_summary: %w", err))
	}
	if summary.TotalCents != nil {
		state.PreviousTotalCents = *summary.TotalCents
	}
	state.Awaiting = BudgetManageAwaitingTotal
	return budgetManageSuspend(state, budgetManageEditTotalPrompt(state.PreviousTotalCents))
}

func enterBudgetManageEditDistribution(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error) {
	summary, err := planner.GetMonthlySummary(ctx, state.UserID, state.Competence)
	if err != nil {
		if errors.Is(err, interfaces.ErrBudgetNotFound) {
			state.Status = BudgetManageCancelled
			state.ResponseText = "Você ainda não tem um orçamento ativo. Crie um orçamento antes de alterar a distribuição."
			return budgetManageComplete(state)
		}
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.edit_distribution: get_monthly_summary: %w", err))
	}
	if summary.TotalCents != nil {
		state.TotalCents = *summary.TotalCents
		state.PreviousTotalCents = *summary.TotalCents
	}
	previous := make(map[string]int, len(summary.Allocations))
	for _, alloc := range summary.Allocations {
		if alloc.PlannedCents == nil || state.TotalCents <= 0 {
			continue
		}
		previous[alloc.RootSlug] = int(*alloc.PlannedCents * 10000 / state.TotalCents)
	}
	state.PreviousAllocations = previous
	state.Awaiting = BudgetManageAwaitingDistribution
	return budgetManageSuspend(state, budgetManageDistributionPrompt(state.PreviousAllocations, state.TotalCents))
}

func budgetManageEditTotalPrompt(previousCents int64) string {
	return fmt.Sprintf("Seu orçamento atual é de %s por mês. Qual é o novo valor total (em R$)? As categorias serão reescaladas proporcionalmente.", money.FromCents(previousCents).BRL())
}

func budgetManageEditTotalReprompt() string {
	return "Não consegui identificar o valor. Qual é o novo valor total (em R$) do orçamento? Por exemplo: R$ 3.500,00."
}

func budgetManageDistributionPrompt(previous map[string]int, totalCents int64) string {
	var b strings.Builder
	b.WriteString("Esta é sua distribuição atual:\n\n")
	for _, slug := range canonicalSlugs {
		bp := previous[slug]
		cents := totalCents * int64(bp) / 10000
		fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(cents).BRL(), bp/100)
	}
	b.WriteString("\nQual é a nova distribuição? Pode ser em reais (R$) ou em porcentagem (%) para cada categoria.")
	return b.String()
}

func budgetManageDistributionReprompt(reason string, previous map[string]int, totalCents int64) string {
	return "Ops, não consegui aplicar essa distribuição: " + reason + "\n\n" + budgetManageDistributionPrompt(previous, totalCents)
}

func handleBudgetManageTotalSlot(ctx context.Context, state BudgetManageState, a agent.Agent) (workflow.StepOutput[BudgetManageState], error) {
	if state.ResumeText == "" {
		if state.Operation == BudgetManageOpEditTotal {
			return budgetManageSuspend(state, budgetManageEditTotalPrompt(state.PreviousTotalCents))
		}
		return budgetManageSuspend(state, budgetTotalPrompt())
	}
	if isBudgetManageExpired(state, time.Now().UTC()) {
		return budgetManageExpireStep(state)
	}
	resumeText := state.ResumeText
	state.ResumeText = ""

	extracted, err := a.Execute(ctx, agent.Request{
		Messages: []llm.Message{
			{Role: "system", Content: "Extraia o valor total do orçamento em reais (BRL) do texto do usuario. Retorne como numero decimal."},
			{Role: "user", Content: resumeText},
		},
		Schema: &llm.Schema{Name: "monthly_budget_extract", Strict: true, Schema: monthlyBudgetSchema},
	})
	if err != nil {
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.total: parse: %w", err))
	}
	var extract monthlyBudgetExtract
	if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.total: unmarshal: %w", err))
	}

	decision := DecideBudgetManageTotal(mustCentsFromBRL(extract.AmountBRL))
	if decision.Action == BudgetManageActionRepromptTotal {
		if state.Operation == BudgetManageOpEditTotal {
			return budgetManageSuspend(state, budgetManageEditTotalReprompt())
		}
		return budgetManageSuspend(state, budgetTotalReprompt())
	}

	state.TotalCents = decision.TotalCents
	if state.Operation == BudgetManageOpEditTotal {
		state.Awaiting = BudgetManageAwaitingConfirm
		return budgetManageConfirmSuspend(state)
	}
	state.Awaiting = BudgetManageAwaitingDistribution
	return budgetManageSuspend(state, budgetDistributionPrompt(state.TotalCents))
}

func handleBudgetManageDistributionSlot(ctx context.Context, state BudgetManageState, a agent.Agent) (workflow.StepOutput[BudgetManageState], error) {
	if state.ResumeText == "" {
		if state.Operation == BudgetManageOpEditDistribution {
			return budgetManageSuspend(state, budgetManageDistributionPrompt(state.PreviousAllocations, state.TotalCents))
		}
		return budgetManageSuspend(state, budgetDistributionPrompt(state.TotalCents))
	}
	if isBudgetManageExpired(state, time.Now().UTC()) {
		return budgetManageExpireStep(state)
	}
	resumeText := state.ResumeText
	state.ResumeText = ""

	extracted, err := a.Execute(ctx, agent.Request{
		Messages: []llm.Message{
			{Role: "system", Content: allocationInputSystemPrompt},
			{Role: "user", Content: resumeText},
		},
		Schema: &llm.Schema{Name: "allocation_input", Strict: true, Schema: allocationInputSchema},
	})
	if err != nil {
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.distribution: parse: %w", err))
	}
	var input allocationInputExtract
	if err := json.Unmarshal(extracted.RawJSON, &input); err != nil {
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.distribution: unmarshal: %w", err))
	}
	kind, kindErr := ParseAllocationInputKind(input.Action)
	if kindErr != nil {
		return budgetManageRepromptDistribution(state, "não entendi sua resposta.")
	}
	values := map[string]float64{
		"expense.custo_fixo":           input.CustoFixo,
		"expense.conhecimento":         input.Conhecimento,
		"expense.prazeres":             input.Prazeres,
		"expense.metas":                input.Metas,
		"expense.liberdade_financeira": input.LiberdadeFinanceira,
	}
	kind = DecideAllocationKind(kind, values, state.TotalCents)
	balance := DecideDistributionBalance(kind, values, state.TotalCents)
	bp, decErr := DecideAllocationsBP(kind, values, state.TotalCents)
	if decErr != nil {
		reason := decErr.Error()
		if errors.Is(decErr, errAllocationOutOfTolerance) {
			reason = budgetBalanceReason(balance)
		}
		return budgetManageRepromptDistribution(state, reason)
	}

	decision := DecideBudgetManageDistribution(bp)
	if decision.Action == BudgetManageActionRepromptDistribution {
		return budgetManageRepromptDistribution(state, "a soma precisa fechar 100%.")
	}

	state.Allocations = decision.Allocations
	state.Awaiting = BudgetManageAwaitingConfirm
	return budgetManageConfirmSuspend(state)
}

func budgetManageRepromptDistribution(state BudgetManageState, reason string) (workflow.StepOutput[BudgetManageState], error) {
	if state.Operation == BudgetManageOpEditDistribution {
		return budgetManageSuspend(state, budgetManageDistributionReprompt(reason, state.PreviousAllocations, state.TotalCents))
	}
	return budgetManageSuspend(state, budgetDistributionReprompt(reason, state.TotalCents))
}

func budgetManageConfirmSuspend(state BudgetManageState) (workflow.StepOutput[BudgetManageState], error) {
	return budgetManageSuspend(state, budgetManageConfirmPrompt(state))
}

func budgetManageConfirmPrompt(state BudgetManageState) string {
	monthLabel := state.Competence
	if competence, err := budgetsvo.NewCompetence(state.Competence); err == nil {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	var b strings.Builder
	switch state.Operation {
	case BudgetManageOpEditTotal:
		fmt.Fprintf(&b, "Vamos revisar antes de confirmar o novo total de %s:\n\n", monthLabel)
		fmt.Fprintf(&b, "💵 Novo total: %s\n\n", money.FromCents(state.TotalCents).BRL())
		b.WriteString("As categorias serão reescaladas proporcionalmente à distribuição atual.\n\n")
	case BudgetManageOpEditDistribution:
		fmt.Fprintf(&b, "Vamos revisar a nova distribuição de %s:\n\n", monthLabel)
		for _, slug := range canonicalSlugs {
			bp := state.Allocations[slug]
			cents := state.TotalCents * int64(bp) / 10000
			fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(cents).BRL(), bp/100)
		}
		b.WriteString("\n")
	default:
		fmt.Fprintf(&b, "Vamos revisar antes de ativar o orçamento de %s:\n\n", monthLabel)
		fmt.Fprintf(&b, "💵 Total: %s\n\n", money.FromCents(state.TotalCents).BRL())
		b.WriteString("Distribuição:\n")
		for _, slug := range canonicalSlugs {
			bp := state.Allocations[slug]
			cents := state.TotalCents * int64(bp) / 10000
			fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(cents).BRL(), bp/100)
		}
		b.WriteString("\n")
	}
	b.WriteString("Posso confirmar? Responda \"sim\" para confirmar ou \"não\" para cancelar.")
	return b.String()
}

func handleBudgetManageConfirmSlot(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner, execMap map[BudgetManageOperationKind]budgetManageExecFn) (workflow.StepOutput[BudgetManageState], error) {
	if state.ResumeText == "" {
		return budgetManageConfirmSuspend(state)
	}

	msg := BudgetManageMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
	now := time.Now().UTC()
	decision := DecideBudgetManageConfirmation(state, msg, now)
	state.ResumeText = ""

	switch decision.Action {
	case BudgetManageActionConfirm:
		state.MessageID = state.IncomingMessageID
		fn, ok := execMap[state.Operation]
		if !ok {
			state.Status = BudgetManageCancelled
			state.ResponseText = "🚫 Não foi possível identificar a operação de orçamento solicitada."
			return budgetManageComplete(state)
		}
		return fn(ctx, state, planner)
	case BudgetManageActionCancel:
		state.MessageID = state.IncomingMessageID
		state.Status = BudgetManageCancelled
		state.ResponseText = "🚫 Operação de orçamento cancelada conforme solicitado."
		return budgetManageComplete(state)
	case BudgetManageActionExpire:
		state.Status = BudgetManageExpired
		state.Expired = true
		state.ResponseText = ""
		return budgetManageComplete(state)
	case BudgetManageActionReplay:
		state.Status = BudgetManageCompleted
		return budgetManageComplete(state)
	case BudgetManageActionRepromptConfirm:
		state.RepromptCount++
		state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar."
		return budgetManageSuspend(state, state.ResponseText)
	default:
		state.Status = BudgetManageCancelled
		state.ResponseText = "🚫 Operação de orçamento cancelada: resposta não reconhecida."
		return budgetManageComplete(state)
	}
}

func executeBudgetManageCreateRetroactive(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error) {
	allocations := make([]interfaces.AllocationDraft, 0, len(canonicalSlugs))
	for _, slug := range canonicalSlugs {
		allocations = append(allocations, interfaces.AllocationDraft{
			RootSlug:    slug,
			BasisPoints: state.Allocations[slug],
		})
	}
	draft := interfaces.DraftBudget{
		UserID:      state.UserID,
		Competence:  state.Competence,
		TotalCents:  state.TotalCents,
		Allocations: allocations,
	}

	_, createErr := planner.CreateBudget(ctx, draft)
	if createErr != nil {
		if errors.Is(createErr, interfaces.ErrBudgetConflict) {
			state.Status = BudgetManageCompleted
			state.ResponseText = budgetAlreadyExistsMessage(state.Competence)
			return budgetManageComplete(state)
		}
		state.ResponseText = "Não consegui criar o orçamento. Tente novamente em breve."
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.create_retroactive: create_budget: %w", createErr))
	}

	if activateErr := planner.ActivateBudget(ctx, state.UserID, state.Competence); activateErr != nil && !errors.Is(activateErr, interfaces.ErrBudgetAlreadyActive) {
		state.ResponseText = "Não consegui ativar o orçamento. Tente novamente em breve."
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.create_retroactive: activate_budget: %w", activateErr))
	}

	state.Status = BudgetManageCompleted
	state.ResponseText = budgetManageCreatedMessage(state)
	return budgetManageComplete(state)
}

func executeBudgetManageEditTotal(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error) {
	if err := planner.EditBudgetTotal(ctx, state.UserID, state.Competence, state.TotalCents); err != nil {
		state.ResponseText = budgetManageDomainErrorMessage(err)
		if state.ResponseText != "" {
			state.Status = BudgetManageCompleted
			return budgetManageComplete(state)
		}
		state.ResponseText = "Não consegui alterar o valor total do orçamento. Tente novamente em breve."
		return budgetManageFail(state, fmt.Errorf("agents.budget_manage.edit_total: edit_budget_total: %w", err))
	}
	state.Status = BudgetManageCompleted
	state.ResponseText = budgetManageEditTotalSuccessMessage(state)
	return budgetManageComplete(state)
}

func executeBudgetManageEditDistribution(ctx context.Context, state BudgetManageState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetManageState], error) {
	for _, slug := range canonicalSlugs {
		bp := state.Allocations[slug]
		percentage := bp / 100
		if err := planner.EditCategoryPercentage(ctx, state.UserID, state.Competence, slug, percentage); err != nil {
			state.ResponseText = budgetManageDomainErrorMessage(err)
			if state.ResponseText != "" {
				state.Status = BudgetManageCompleted
				return budgetManageComplete(state)
			}
			state.ResponseText = "Não consegui alterar a distribuição do orçamento. Tente novamente em breve."
			return budgetManageFail(state, fmt.Errorf("agents.budget_manage.edit_distribution: edit_category_percentage: %w", err))
		}
	}
	state.Status = BudgetManageCompleted
	state.ResponseText = budgetManageEditDistributionSuccessMessage(state)
	return budgetManageComplete(state)
}

func budgetManageDomainErrorMessage(err error) string {
	switch {
	case errors.Is(err, budgetsentities.ErrBudgetNotActive):
		return "❌ Você ainda não tem um orçamento ativo para essa competência."
	case errors.Is(err, budgetsentities.ErrBudgetTotalMustBePositive):
		return "❌ O valor total do orçamento precisa ser maior que zero."
	case errors.Is(err, budgetsentities.ErrBudgetAllocationSumMustBe10000):
		return "❌ A soma da distribuição precisa fechar exatamente 100%."
	default:
		return ""
	}
}

func budgetManageCreatedMessage(state BudgetManageState) string {
	monthLabel := state.Competence
	if competence, err := budgetsvo.NewCompetence(state.Competence); err == nil {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	seed := messages.NewMotivationSeed(state.MessageID)
	return fmt.Sprintf("🎉 Orçamento de %s criado e ativado com sucesso!\n\n%s", monthLabel, messages.BudgetManageMotivation(seed))
}

func budgetManageEditTotalSuccessMessage(state BudgetManageState) string {
	seed := messages.NewMotivationSeed(state.MessageID)
	return fmt.Sprintf("✅ Valor total do orçamento atualizado para %s! As categorias foram reescaladas proporcionalmente.\n\n%s", money.FromCents(state.TotalCents).BRL(), messages.BudgetManageMotivation(seed))
}

func budgetManageEditDistributionSuccessMessage(state BudgetManageState) string {
	seed := messages.NewMotivationSeed(state.MessageID)
	return fmt.Sprintf("✅ Distribuição do orçamento atualizada com sucesso!\n\n%s", messages.BudgetManageMotivation(seed))
}

func ContinueBudgetManage(
	ctx context.Context,
	engine workflow.Engine[BudgetManageState],
	def workflow.Definition[BudgetManageState],
	key string,
	userMessage string,
) (bool, string, error) {
	resumeBytes, err := json.Marshal(map[string]string{"resumeText": userMessage})
	if err != nil {
		return false, "", fmt.Errorf("workflows.budget_manage: marshal resume: %w", err)
	}

	result, resumeErr := engine.Resume(ctx, def, key, resumeBytes)
	if result.Status == 0 && resumeErr == nil {
		return false, "", nil
	}

	if resumeErr != nil {
		return true, result.State.ResponseText, fmt.Errorf("workflows.budget_manage: resume: %w", resumeErr)
	}

	if result.State.Expired {
		return false, "", nil
	}

	return true, result.State.ResponseText, nil
}
