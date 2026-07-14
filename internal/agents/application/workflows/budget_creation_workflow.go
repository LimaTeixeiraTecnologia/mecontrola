package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	BudgetCreationWorkflowID  = "budget-creation"
	stepBudgetCreationID      = "budget-creation"
	BudgetCreationStaleAfter  = 35 * time.Minute
	BudgetCreationReaperBatch = 100
)

func BudgetCreationKey(resourceID string) string {
	return resourceID + ":" + BudgetCreationWorkflowID
}

func BuildBudgetCreationWorkflow(a agent.Agent, planner interfaces.BudgetPlanner) workflow.Definition[BudgetCreationState] {
	step := workflow.NewStepFunc(stepBudgetCreationID, buildBudgetCreationStep(a, planner))
	return workflow.Definition[BudgetCreationState]{
		ID:          BudgetCreationWorkflowID,
		Root:        step,
		Durable:     true,
		MaxAttempts: 1,
	}
}

func BuildBudgetCreationReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, BudgetCreationWorkflowID, BudgetCreationStaleAfter, BudgetCreationReaperBatch, o11y)
}

func buildBudgetCreationStep(a agent.Agent, planner interfaces.BudgetPlanner) func(context.Context, BudgetCreationState) (workflow.StepOutput[BudgetCreationState], error) {
	return func(ctx context.Context, state BudgetCreationState) (workflow.StepOutput[BudgetCreationState], error) {
		switch state.Awaiting {
		case AwaitingBudgetTotal:
			return handleBudgetTotalSlot(ctx, state, a)
		case AwaitingBudgetDistribution:
			return handleBudgetDistributionSlot(ctx, state, a)
		case AwaitingBudgetConfirm:
			return handleBudgetConfirmSlot(ctx, state, planner)
		default:
			return budgetSuspend(state, budgetTotalPrompt())
		}
	}
}

func budgetSuspend(state BudgetCreationState, prompt string) (workflow.StepOutput[BudgetCreationState], error) {
	state.ResponseText = prompt
	if state.SuspendedAt.IsZero() {
		state.SuspendedAt = time.Now().UTC()
	}
	return workflow.StepOutput[BudgetCreationState]{
		State:  state,
		Status: workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{
			Reason: workflow.SuspendAwaitingInput,
			Prompt: prompt,
		},
	}, nil
}

func budgetComplete(state BudgetCreationState) (workflow.StepOutput[BudgetCreationState], error) {
	return workflow.StepOutput[BudgetCreationState]{State: state, Status: workflow.StepStatusCompleted}, nil
}

func budgetFail(state BudgetCreationState, err error) (workflow.StepOutput[BudgetCreationState], error) {
	return workflow.StepOutput[BudgetCreationState]{State: state, Status: workflow.StepStatusFailed}, err
}

func budgetExpire(state BudgetCreationState) (workflow.StepOutput[BudgetCreationState], error) {
	state.Status = BudgetCreationExpired
	state.Expired = true
	state.ResponseText = ""
	state.ResumeText = ""
	return budgetComplete(state)
}

func budgetTotalPrompt() string {
	return "Vamos criar seu orçamento. Qual é o valor total (em R$) que você quer planejar para esse mês? (por exemplo: R$ 3.500,00)"
}

func budgetTotalReprompt() string {
	return "Não consegui identificar o valor. Qual é o valor total (em R$) do orçamento? Por exemplo: R$ 3.500,00."
}

func handleBudgetTotalSlot(ctx context.Context, state BudgetCreationState, a agent.Agent) (workflow.StepOutput[BudgetCreationState], error) {
	if state.ResumeText == "" {
		return budgetSuspend(state, budgetTotalPrompt())
	}
	if isBudgetExpired(state, time.Now().UTC()) {
		return budgetExpire(state)
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
		return budgetFail(state, fmt.Errorf("agents.budget_creation.total: parse: %w", err))
	}
	var extract monthlyBudgetExtract
	if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
		return budgetFail(state, fmt.Errorf("agents.budget_creation.total: unmarshal: %w", err))
	}

	decision := DecideBudgetTotal(mustCentsFromBRL(extract.AmountBRL))
	if decision.Action == BudgetActionRepromptTotal {
		return budgetSuspend(state, budgetTotalReprompt())
	}

	state.TotalCents = decision.TotalCents
	state.Awaiting = AwaitingBudgetDistribution
	return handleBudgetDistributionSlot(ctx, state, a)
}

func mustCentsFromBRL(amountBRL float64) int64 {
	cents, err := DecideMonthlyBudgetCents(amountBRL)
	if err != nil {
		return 0
	}
	return cents
}

func budgetDistributionPrompt(totalCents int64) string {
	var b strings.Builder
	b.WriteString("Agora vamos distribuir os ")
	b.WriteString(money.FromCents(totalCents).BRL())
	b.WriteString(" entre as 5 categorias. Esta é a sugestão padrão:\n\n")
	for _, slug := range canonicalSlugs {
		bp := defaultDistributionBP[slug]
		cents := totalCents * int64(bp) / 10000
		fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(cents).BRL(), bp/100)
	}
	b.WriteString("\nAceita esta sugestão? Responda \"sim\" para confirmar, ou envie novos valores para cada categoria — pode ser em reais (R$) ou em porcentagem (%).")
	return b.String()
}

func budgetDistributionReprompt(reason string, totalCents int64) string {
	return "Ops, não consegui aplicar essa distribuição: " + reason + "\n\n" + budgetDistributionPrompt(totalCents)
}

func budgetBalanceReason(balance DistributionBalance) string {
	var unitLabel string
	var deltaLabel string
	if balance.Unit == allocationInputReais {
		unitLabel = "o orçamento mensal"
		deltaLabel = money.FromCents(int64(math.Round(balance.DeltaAbs * 100))).BRL()
	} else {
		unitLabel = "100%"
		deltaLabel = fmt.Sprintf("%.1f%%", balance.DeltaAbs)
	}
	switch balance.Status {
	case distributionOver:
		return fmt.Sprintf("passou %s de %s. Envie novamente a distribuição para fechar exatamente %s.", deltaLabel, unitLabel, unitLabel)
	case distributionUnder:
		return fmt.Sprintf("faltou %s para %s. Envie novamente a distribuição para fechar exatamente %s.", deltaLabel, unitLabel, unitLabel)
	default:
		return "a soma precisa fechar exatamente o total."
	}
}

func handleBudgetDistributionSlot(ctx context.Context, state BudgetCreationState, a agent.Agent) (workflow.StepOutput[BudgetCreationState], error) {
	if state.ResumeText == "" {
		return budgetSuspend(state, budgetDistributionPrompt(state.TotalCents))
	}
	if isBudgetExpired(state, time.Now().UTC()) {
		return budgetExpire(state)
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
		return budgetFail(state, fmt.Errorf("agents.budget_creation.distribution: parse: %w", err))
	}
	var input allocationInputExtract
	if err := json.Unmarshal(extracted.RawJSON, &input); err != nil {
		return budgetFail(state, fmt.Errorf("agents.budget_creation.distribution: unmarshal: %w", err))
	}
	kind, kindErr := ParseAllocationInputKind(input.Action)
	if kindErr != nil {
		return budgetSuspend(state, budgetDistributionReprompt("não entendi sua resposta.", state.TotalCents))
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
		return budgetSuspend(state, budgetDistributionReprompt(reason, state.TotalCents))
	}

	decision := DecideBudgetDistribution(bp)
	if decision.Action == BudgetActionRepromptDistribution {
		return budgetSuspend(state, budgetDistributionReprompt("a soma precisa fechar 100%.", state.TotalCents))
	}

	state.Allocations = decision.Allocations
	state.Awaiting = AwaitingBudgetConfirm
	return budgetConfirmSuspend(state)
}

func budgetConfirmSuspend(state BudgetCreationState) (workflow.StepOutput[BudgetCreationState], error) {
	competence, err := budgetsvo.NewCompetence(state.Competence)
	prompt := budgetConfirmPrompt(state, competence, err == nil)
	return budgetSuspend(state, prompt)
}

func budgetConfirmPrompt(state BudgetCreationState, competence budgetsvo.Competence, competenceOK bool) string {
	monthLabel := state.Competence
	if competenceOK {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Vamos revisar antes de ativar o orçamento de %s:\n\n", monthLabel)
	fmt.Fprintf(&b, "💵 Total: %s\n\n", money.FromCents(state.TotalCents).BRL())
	b.WriteString("Distribuição:\n")
	for _, slug := range canonicalSlugs {
		bp := state.Allocations[slug]
		cents := state.TotalCents * int64(bp) / 10000
		fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(cents).BRL(), bp/100)
	}
	b.WriteString("\nPosso ativar seu orçamento com esses dados? Responda \"sim\" para confirmar ou \"não\" para cancelar.")
	return b.String()
}

func handleBudgetConfirmSlot(ctx context.Context, state BudgetCreationState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetCreationState], error) {
	if state.ResumeText == "" {
		return budgetConfirmSuspend(state)
	}

	msg := BudgetCreationMessage{Text: state.ResumeText, MessageID: state.IncomingMessageID}
	now := time.Now().UTC()
	decision := DecideBudgetConfirmation(state, msg, now)
	state.ResumeText = ""

	switch decision.Action {
	case BudgetActionConfirm:
		state.MessageID = state.IncomingMessageID
		return executeBudgetCreation(ctx, state, planner)
	case BudgetActionCancel:
		state.MessageID = state.IncomingMessageID
		state.Status = BudgetCreationCancelled
		state.ResponseText = "🚫 Criação de orçamento cancelada conforme solicitado."
		return budgetComplete(state)
	case BudgetActionExpire:
		state.Status = BudgetCreationExpired
		state.Expired = true
		state.ResponseText = ""
		return budgetComplete(state)
	case BudgetActionReplay:
		state.Status = BudgetCreationCompleted
		return budgetComplete(state)
	case BudgetActionRepromptConfirm:
		state.RepromptCount++
		state.ResponseText = "Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar a criação do orçamento."
		return budgetSuspend(state, state.ResponseText)
	default:
		state.Status = BudgetCreationCancelled
		state.ResponseText = "🚫 Criação de orçamento cancelada: resposta não reconhecida."
		return budgetComplete(state)
	}
}

func executeBudgetCreation(ctx context.Context, state BudgetCreationState, planner interfaces.BudgetPlanner) (workflow.StepOutput[BudgetCreationState], error) {
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
			state.Status = BudgetCreationCompleted
			state.ResponseText = budgetAlreadyExistsMessage(state.Competence)
			return budgetComplete(state)
		}
		state.ResponseText = "Não consegui criar o orçamento. Tente novamente em breve."
		return workflow.StepOutput[BudgetCreationState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.budget_creation.confirm: create_budget: %w", createErr)
	}

	if activateErr := planner.ActivateBudget(ctx, state.UserID, state.Competence); activateErr != nil && !errors.Is(activateErr, interfaces.ErrBudgetAlreadyActive) {
		state.ResponseText = "Não consegui ativar o orçamento. Tente novamente em breve."
		return workflow.StepOutput[BudgetCreationState]{State: state, Status: workflow.StepStatusFailed}, fmt.Errorf("agents.budget_creation.confirm: activate_budget: %w", activateErr)
	}

	state.Status = BudgetCreationCompleted
	state.ResponseText = budgetActivatedMessage(state)
	return budgetComplete(state)
}

func budgetAlreadyExistsMessage(rawCompetence string) string {
	monthLabel := rawCompetence
	if competence, err := budgetsvo.NewCompetence(rawCompetence); err == nil {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	return fmt.Sprintf("Já existe um orçamento para %s. Não é possível criar outro.", monthLabel)
}

func budgetActivatedMessage(state BudgetCreationState) string {
	monthLabel := state.Competence
	if competence, err := budgetsvo.NewCompetence(state.Competence); err == nil {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	return fmt.Sprintf("🎉 Orçamento de %s criado e ativado com sucesso!", monthLabel)
}
