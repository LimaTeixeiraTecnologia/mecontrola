package usecases

import (
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

const (
	toolRecordTransaction = "record_transaction"
	toolMonthlySummary    = "monthly_summary"
	toolListCards         = "list_cards"
	toolConfigureBudget   = "configure_budget"
	toolCreateCard        = "create_card"
	toolCountCards        = "count_cards"
	toolUpdateCard        = "update_card"
	toolDeleteCard        = "delete_card"
	toolEditCategoryPct   = "edit_category_percentage"
)

var ErrToolUnsupported = fmt.Errorf("agent.usecase.tool_catalog: tool nao suportada")

func ToolCallToIntent(call interfaces.ToolCall, fallbackText string) (intent.Intent, error) {
	switch call.FunctionName {
	case toolRecordTransaction:
		return recordTransactionIntent(call.ArgumentsJSON)
	case toolMonthlySummary:
		return intent.NewMonthlySummary(stringArg(call.ArgumentsJSON, "ref_month"))
	case toolListCards:
		return intent.NewListCards(), nil
	case toolConfigureBudget:
		return intent.NewConfigureBudget(intent.ConfigureBudgetFields{})
	case toolCreateCard:
		return intent.NewCreateCard(intent.CreateCardFields{
			Nickname:   stringArg(call.ArgumentsJSON, "nickname"),
			Name:       stringArg(call.ArgumentsJSON, "name"),
			ClosingDay: int(intArg(call.ArgumentsJSON, "closing_day")),
			DueDay:     int(intArg(call.ArgumentsJSON, "due_day")),
			LimitCents: intArg(call.ArgumentsJSON, "limit_cents"),
		})
	case toolCountCards:
		return intent.NewCountCards(), nil
	case toolUpdateCard:
		return intent.NewUpdateCard(intent.UpdateCardFields{
			CardName:   stringArg(call.ArgumentsJSON, "card_name"),
			Nickname:   stringPtrArg(call.ArgumentsJSON, "new_nickname"),
			Name:       stringPtrArg(call.ArgumentsJSON, "new_name"),
			ClosingDay: intPtrArg(call.ArgumentsJSON, "new_closing_day"),
			DueDay:     intPtrArg(call.ArgumentsJSON, "new_due_day"),
		})
	case toolDeleteCard:
		return intent.NewDeleteCard(stringArg(call.ArgumentsJSON, "card_name"))
	case toolEditCategoryPct:
		return intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
			CategoryName: stringArg(call.ArgumentsJSON, "category_name"),
			Percentage:   int(intArg(call.ArgumentsJSON, "percentage")),
		})
	default:
		return intent.Intent{}, fmt.Errorf("%w: %s", ErrToolUnsupported, call.FunctionName)
	}
}

func recordTransactionIntent(args map[string]any) (intent.Intent, error) {
	direction := strings.ToLower(strings.TrimSpace(stringArg(args, "direction")))
	dto := rawIntentDTO{
		AmountCents:   intArg(args, "amount_cents"),
		Merchant:      stringArg(args, "merchant"),
		CategoryHint:  stringArg(args, "category_hint"),
		PaymentMethod: stringArg(args, "payment_method"),
	}
	if direction == directionIncome {
		return intent.NewRecordIncome(intent.RecordIncomeFields{
			AmountCents:   dto.AmountCents,
			Source:        dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
		})
	}
	return intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:   dto.AmountCents,
		Merchant:      dto.Merchant,
		CategoryHint:  dto.CategoryHint,
		PaymentMethod: dto.PaymentMethod,
	})
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

func stringPtrArg(args map[string]any, key string) *string {
	if args == nil {
		return nil
	}
	v, ok := args[key].(string)
	if !ok {
		return nil
	}
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func intPtrArg(args map[string]any, key string) *int {
	if args == nil {
		return nil
	}
	if _, ok := args[key]; !ok {
		return nil
	}
	value := int(intArg(args, key))
	return &value
}

func intArg(args map[string]any, key string) int64 {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}
