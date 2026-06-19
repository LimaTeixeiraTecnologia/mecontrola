package onboarding

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

const recordTransactionTool = "record_transaction"

const onboardingCompletedReply = "🎉 **Onboarding concluído!** Agora é só me chamar: registrar gastos, ver fatura do cartão, acompanhar metas e pedir o resumo do mês. Conta comigo! ✅"

type onboardingToolDispatcher struct {
	saveObjective    *onbusecases.SaveOnboardingObjective
	saveIncome       *onbusecases.SaveOnboardingIncome
	saveCard         *onbusecases.SaveOnboardingCard
	saveBudgetSplits *onbusecases.SaveOnboardingBudgetSplits
	markFirstTx      *onbusecases.MarkFirstTransactionRecorded
	complete         *onbusecases.CompleteOnboardingSession
	expenseLogger    appservices.ExpenseLogger
}

func NewOnboardingToolDispatcher(
	saveObjective *onbusecases.SaveOnboardingObjective,
	saveIncome *onbusecases.SaveOnboardingIncome,
	saveCard *onbusecases.SaveOnboardingCard,
	saveBudgetSplits *onbusecases.SaveOnboardingBudgetSplits,
	markFirstTx *onbusecases.MarkFirstTransactionRecorded,
	complete *onbusecases.CompleteOnboardingSession,
	expenseLogger appservices.ExpenseLogger,
) appusecases.OnboardingToolDispatcher {
	return &onboardingToolDispatcher{
		saveObjective:    saveObjective,
		saveIncome:       saveIncome,
		saveCard:         saveCard,
		saveBudgetSplits: saveBudgetSplits,
		markFirstTx:      markFirstTx,
		complete:         complete,
		expenseLogger:    expenseLogger,
	}
}

func (d *onboardingToolDispatcher) Dispatch(ctx context.Context, userID uuid.UUID, channel string, call agentinterfaces.ToolCall) (appusecases.OnboardingToolResult, error) {
	switch call.FunctionName {
	case appusecases.ToolSaveOnboardingObjective:
		return d.dispatchObjective(ctx, userID, call)
	case appusecases.ToolSaveOnboardingIncome:
		return d.dispatchIncome(ctx, userID, call)
	case appusecases.ToolSaveOnboardingCard:
		return d.dispatchCard(ctx, userID, call)
	case appusecases.ToolSaveOnboardingBudgetSplits:
		return d.dispatchSplits(ctx, userID, call)
	case appusecases.ToolCompleteOnboardingSession:
		return d.dispatchComplete(ctx, userID)
	case recordTransactionTool:
		return d.dispatchRecordTransaction(ctx, userID, call)
	default:
		return appusecases.OnboardingToolResult{}, fmt.Errorf("agent.onboarding.dispatcher: tool nao suportada: %s", call.FunctionName)
	}
}

func (d *onboardingToolDispatcher) dispatchObjective(ctx context.Context, userID uuid.UUID, call agentinterfaces.ToolCall) (appusecases.OnboardingToolResult, error) {
	out, err := d.saveObjective.Execute(ctx, onbusecases.SaveOnboardingObjectiveInput{
		UserID:    userID,
		Objective: stringArg(call.ArgumentsJSON, "objective"),
	})
	if err != nil {
		if errors.Is(err, onbvalueobjects.ErrFinancialObjectiveEmpty) || errors.Is(err, onbvalueobjects.ErrFinancialObjectiveTooLong) {
			return appusecases.OnboardingToolResult{Reply: "Não consegui entender seu objetivo. Pode me contar de novo, com poucas palavras? 😊"}, nil
		}
		return appusecases.OnboardingToolResult{}, err
	}
	return appusecases.OnboardingToolResult{
		Reply: fmt.Sprintf("🎯 Anotado: seu foco é **%s**. Vou usar isso pra te manter motivado!", out.Objective),
	}, nil
}

func (d *onboardingToolDispatcher) dispatchIncome(ctx context.Context, userID uuid.UUID, call agentinterfaces.ToolCall) (appusecases.OnboardingToolResult, error) {
	out, err := d.saveIncome.Execute(ctx, onbusecases.SaveOnboardingIncomeInput{
		UserID:      userID,
		IncomeCents: intArg(call.ArgumentsJSON, "income_cents"),
	})
	if err != nil {
		if errors.Is(err, onbvalueobjects.ErrMonthlyIncomeBelowMinimum) || errors.Is(err, onbvalueobjects.ErrMonthlyIncomeAboveMaximum) {
			return appusecases.OnboardingToolResult{Reply: "Esse valor de orçamento não parece certo. Pode me dizer de novo o quanto você tem por mês? 💰"}, nil
		}
		return appusecases.OnboardingToolResult{}, err
	}
	return appusecases.OnboardingToolResult{
		Reply: fmt.Sprintf("✅ Orçamento de **R$ %s** registrado!", formatReaisCents(out.IncomeCents)),
	}, nil
}

func (d *onboardingToolDispatcher) dispatchCard(ctx context.Context, userID uuid.UUID, call agentinterfaces.ToolCall) (appusecases.OnboardingToolResult, error) {
	out, err := d.saveCard.Execute(ctx, onbusecases.SaveOnboardingCardInput{
		UserID:   userID,
		Nickname: stringArg(call.ArgumentsJSON, "nickname"),
		DueDay:   int(intArg(call.ArgumentsJSON, "due_day")),
	})
	if err != nil {
		if errors.Is(err, onbentities.ErrOnboardingCardNicknameRequired) || errors.Is(err, onbvalueobjects.ErrCardDueDayOutOfRange) {
			return appusecases.OnboardingToolResult{Reply: "Pra cadastrar o cartão preciso do apelido e do dia de vencimento (1 a 31). Pode me passar? 💳"}, nil
		}
		return appusecases.OnboardingToolResult{}, err
	}
	return appusecases.OnboardingToolResult{
		Reply: fmt.Sprintf("💳 Cartão **%s** salvo (vence dia %d 📅). Quer adicionar outro? Se não usa cartão, é só dizer.", out.Name, out.DueDay),
	}, nil
}

func (d *onboardingToolDispatcher) dispatchSplits(ctx context.Context, userID uuid.UUID, call agentinterfaces.ToolCall) (appusecases.OnboardingToolResult, error) {
	items, ok := parseAllocations(call.ArgumentsJSON)
	if !ok {
		return appusecases.OnboardingToolResult{Reply: "Não entendi a distribuição. Me diz quanto, em reais, você quer pra cada categoria. 📊"}, nil
	}
	out, err := d.saveBudgetSplits.Execute(ctx, onbusecases.SaveOnboardingBudgetSplitsInput{
		UserID:      userID,
		Allocations: items,
	})
	if err != nil {
		return appusecases.OnboardingToolResult{}, err
	}
	if !out.Applied {
		return appusecases.OnboardingToolResult{Reply: splitsMismatchReply(out.SumCents, out.TotalCents)}, nil
	}
	return appusecases.OnboardingToolResult{Reply: splitsSuccessReply(out.Allocations)}, nil
}

func (d *onboardingToolDispatcher) dispatchRecordTransaction(ctx context.Context, userID uuid.UUID, call agentinterfaces.ToolCall) (appusecases.OnboardingToolResult, error) {
	parsed, err := appusecases.ToolCallToIntent(call, "")
	if err != nil {
		return appusecases.OnboardingToolResult{Reply: "Não entendi o lançamento. Me manda algo como 'gastei 35 no mercado'. 😊"}, nil
	}
	result, err := d.expenseLogger.Execute(ctx, appservices.ExpenseLoggerInput{UserID: userID.String(), Intent: parsed})
	if err != nil {
		return appusecases.OnboardingToolResult{}, err
	}
	if !result.Persisted {
		return appusecases.OnboardingToolResult{Reply: "Não consegui registrar esse lançamento. Pode tentar de novo? 🙏"}, nil
	}
	if _, markErr := d.markFirstTx.Execute(ctx, onbusecases.MarkFirstTransactionRecordedInput{UserID: userID}); markErr != nil {
		return appusecases.OnboardingToolResult{}, markErr
	}
	reply := fmt.Sprintf("🏆 Boa! Registrei **R$ %s", formatReaisCents(result.AmountCents))
	if path := strings.TrimSpace(result.CategoryPath); path != "" {
		reply += " em " + path
	}
	reply += "**. Esse é o primeiro passo pro seu controle financeiro!"

	completion, completeErr := d.complete.Execute(ctx, onbusecases.CompleteOnboardingSessionInput{UserID: userID})
	if completeErr != nil {
		if errors.Is(completeErr, onbusecases.ErrOnboardingFirstTransactionRequired) {
			return appusecases.OnboardingToolResult{Reply: reply}, nil
		}
		return appusecases.OnboardingToolResult{}, completeErr
	}
	if completion.Completed {
		return appusecases.OnboardingToolResult{Reply: reply + "\n\n" + onboardingCompletedReply, Terminal: true}, nil
	}
	return appusecases.OnboardingToolResult{Reply: reply, Terminal: completion.AlreadyActive}, nil
}

func (d *onboardingToolDispatcher) dispatchComplete(ctx context.Context, userID uuid.UUID) (appusecases.OnboardingToolResult, error) {
	out, err := d.complete.Execute(ctx, onbusecases.CompleteOnboardingSessionInput{UserID: userID})
	if err != nil {
		if errors.Is(err, onbusecases.ErrOnboardingFirstTransactionRequired) {
			return appusecases.OnboardingToolResult{Reply: "Quase lá! Pra concluir, faça seu primeiro lançamento — me manda algo como 'gastei 35 no mercado'. 😊"}, nil
		}
		return appusecases.OnboardingToolResult{}, err
	}
	if out.AlreadyActive {
		return appusecases.OnboardingToolResult{Terminal: true}, nil
	}
	return appusecases.OnboardingToolResult{
		Reply:    onboardingCompletedReply,
		Terminal: true,
	}, nil
}

func parseAllocations(args map[string]any) ([]onbusecases.BudgetSplitItem, bool) {
	raw, ok := args["allocations"].([]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	items := make([]onbusecases.BudgetSplitItem, 0, len(raw))
	for _, entry := range raw {
		obj, ok := entry.(map[string]any)
		if !ok {
			return nil, false
		}
		slug, _ := obj["root_slug"].(string)
		kind, known := slugToCategoryKind[strings.TrimSpace(slug)]
		if !known {
			return nil, false
		}
		items = append(items, onbusecases.BudgetSplitItem{Kind: kind, AmountCents: numberToInt64(obj["amount_cents"])})
	}
	return items, true
}

func splitsMismatchReply(sumCents, totalCents int64) string {
	diff := sumCents - totalCents
	if diff > 0 {
		return fmt.Sprintf("⚠️ Quase! Você distribuiu **R$ %s**, mas seu orçamento é **R$ %s** — passou **R$ %s**. Quer ajustar pra fechar certinho?",
			formatReais(sumCents), formatReais(totalCents), formatReais(diff))
	}
	return fmt.Sprintf("⚠️ Quase! Você distribuiu **R$ %s**, mas seu orçamento é **R$ %s** — faltam **R$ %s**. Quer ajustar pra fechar certinho?",
		formatReais(sumCents), formatReais(totalCents), formatReais(-diff))
}

func splitsSuccessReply(views []onbusecases.OnboardingSplitView) string {
	sorted := make([]onbusecases.OnboardingSplitView, len(views))
	copy(sorted, views)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Kind < sorted[j].Kind })
	parts := make([]string, 0, len(sorted))
	for _, v := range sorted {
		parts = append(parts, fmt.Sprintf("%s%d%% (R$%s)", categoryEmoji(v.Kind), v.Percent, formatReais(v.AmountCents)))
	}
	return "✅ Distribuição salva! " + strings.Join(parts, " · ") + "."
}

var slugToCategoryKind = map[string]onbvalueobjects.CategoryKind{
	"expense.custo_fixo":           onbvalueobjects.CategoryKindFixedCost,
	"expense.conhecimento":         onbvalueobjects.CategoryKindKnowledge,
	"expense.prazeres":             onbvalueobjects.CategoryKindPleasures,
	"expense.metas":                onbvalueobjects.CategoryKindGoals,
	"expense.liberdade_financeira": onbvalueobjects.CategoryKindFinancialFreedom,
}

var categoryKindStringToSlug = map[string]string{
	"fixed_cost":        "expense.custo_fixo",
	"knowledge":         "expense.conhecimento",
	"pleasures":         "expense.prazeres",
	"goals":             "expense.metas",
	"financial_freedom": "expense.liberdade_financeira",
}

func categoryEmoji(kind onbvalueobjects.CategoryKind) string {
	switch kind {
	case onbvalueobjects.CategoryKindFixedCost:
		return "💰"
	case onbvalueobjects.CategoryKindKnowledge:
		return "🎓"
	case onbvalueobjects.CategoryKindPleasures:
		return "🎉"
	case onbvalueobjects.CategoryKindGoals:
		return "🎯"
	case onbvalueobjects.CategoryKindFinancialFreedom:
		return "🏦"
	default:
		return ""
	}
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func intArg(args map[string]any, key string) int64 {
	if args == nil {
		return 0
	}
	return numberToInt64(args[key])
}

func numberToInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func formatReaisCents(cents int64) string {
	if cents < 0 {
		cents = -cents
	}
	return fmt.Sprintf("%s,%02d", groupThousands(cents/100), cents%100)
}

func formatReais(cents int64) string {
	if cents < 0 {
		cents = -cents
	}
	return groupThousands(cents / 100)
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	rem := len(s) % 3
	var out []byte
	if rem > 0 {
		out = append(out, s[:rem]...)
		out = append(out, '.')
	}
	for i := rem; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, '.')
		}
	}
	return string(out)
}
