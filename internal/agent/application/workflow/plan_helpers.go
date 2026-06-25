package workflow

import (
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func serializePlanStep(in intent.Intent, confidence float64, index int) PlanStepSerialized {
	s := PlanStepSerialized{
		Index:            index,
		IntentKind:       in.Kind().String(),
		AmountCents:      in.AmountCents(),
		Merchant:         in.Merchant(),
		CategoryHint:     in.CategoryHint(),
		PaymentMethod:    in.PaymentMethod(),
		CardHint:         in.CardHint(),
		CategoryName:     in.CategoryName(),
		GoalName:         in.GoalName(),
		RefMonth:         in.RefMonth(),
		RawText:          in.RawText(),
		Installments:     in.Installments(),
		Direction:        in.Direction(),
		Frequency:        in.Frequency(),
		DayOfMonth:       in.DayOfMonth(),
		ClosingDay:       in.ClosingDay(),
		DueDay:           in.DueDay(),
		LimitCents:       in.LimitCents(),
		Percentage:       in.Percentage(),
		CardName:         in.CardName(),
		Nickname:         in.CardNickname(),
		Confidence:       confidence,
		Months:           in.Months(),
		SourceCompetence: in.SourceCompetence(),
	}
	if in.NicknamePtr() != nil {
		s.NewNickname = *in.NicknamePtr()
	}
	if in.NamePtr() != nil {
		s.NewName = *in.NamePtr()
	}
	if in.ClosingDayPtr() != nil {
		s.NewClosingDay = *in.ClosingDayPtr()
	}
	if in.DueDayPtr() != nil {
		s.NewDueDay = *in.DueDayPtr()
	}
	return s
}

func deserializePlanStep(s PlanStepSerialized) (intent.Intent, error) { //nolint:cyclop,revive // dispatch exaustivo por intent kind
	k, err := intent.ParseKind(s.IntentKind)
	if err != nil {
		return intent.Intent{}, fmt.Errorf("agent.workflow.plan_helpers: parse kind %q: %w", s.IntentKind, err)
	}
	switch k {
	case intent.KindRecordExpense:
		return intent.NewRecordExpense(intent.RecordExpenseFields{
			AmountCents:   s.AmountCents,
			Merchant:      s.Merchant,
			CategoryHint:  s.CategoryHint,
			PaymentMethod: s.PaymentMethod,
			CardHint:      s.CardHint,
		})
	case intent.KindRecordIncome:
		return intent.NewRecordIncome(intent.RecordIncomeFields{
			AmountCents:   s.AmountCents,
			Source:        s.Merchant,
			CategoryHint:  s.CategoryHint,
			PaymentMethod: s.PaymentMethod,
		})
	case intent.KindRecordCardPurchase:
		return intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{
			AmountCents:  s.AmountCents,
			Merchant:     s.Merchant,
			CategoryHint: s.CategoryHint,
			CardHint:     s.CardHint,
			Installments: s.Installments,
		})
	case intent.KindCreateRecurring:
		return intent.NewCreateRecurring(intent.CreateRecurringFields{
			AmountCents:  s.AmountCents,
			Merchant:     s.Merchant,
			CategoryHint: s.CategoryHint,
			Direction:    s.Direction,
			Frequency:    s.Frequency,
			DayOfMonth:   s.DayOfMonth,
		})
	case intent.KindCreateCard:
		return intent.NewCreateCard(intent.CreateCardFields{
			Nickname:   s.Nickname,
			Name:       s.CardName,
			ClosingDay: s.ClosingDay,
			DueDay:     s.DueDay,
			LimitCents: s.LimitCents,
		})
	case intent.KindUpdateCard:
		var nicknamePtr *string
		if s.NewNickname != "" {
			v := s.NewNickname
			nicknamePtr = &v
		}
		var namePtr *string
		if s.NewName != "" {
			v := s.NewName
			namePtr = &v
		}
		var closingDayPtr *int
		if s.NewClosingDay != 0 {
			v := s.NewClosingDay
			closingDayPtr = &v
		}
		var dueDayPtr *int
		if s.NewDueDay != 0 {
			v := s.NewDueDay
			dueDayPtr = &v
		}
		return intent.NewUpdateCard(intent.UpdateCardFields{
			CardName:   s.CardName,
			Nickname:   nicknamePtr,
			Name:       namePtr,
			ClosingDay: closingDayPtr,
			DueDay:     dueDayPtr,
		})
	case intent.KindDeleteCard:
		return intent.NewDeleteCard(s.CardName)
	case intent.KindDeleteLastTransaction:
		return intent.NewDeleteLastTransaction(), nil
	case intent.KindEditLastTransaction:
		return intent.NewEditLastTransaction(s.AmountCents)
	case intent.KindEditCategoryPercentage:
		return intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
			CategoryName: s.CategoryName,
			Percentage:   s.Percentage,
		})
	case intent.KindMonthlySummary:
		return intent.NewMonthlySummary(s.RefMonth)
	case intent.KindListTransactions:
		return intent.NewListTransactions(s.RefMonth)
	case intent.KindQueryIncomeSummary:
		return intent.NewQueryIncomeSummary(s.RefMonth)
	case intent.KindQueryCategory:
		return intent.NewQueryCategory(s.CategoryName)
	case intent.KindQueryGoal:
		return intent.NewQueryGoal(s.GoalName)
	case intent.KindQueryCard:
		return intent.NewQueryCard(s.CardName)
	case intent.KindHowAmIDoing:
		return intent.NewHowAmIDoing(), nil
	case intent.KindConfigureBudget:
		return intent.NewConfigureBudget(intent.ConfigureBudgetFields{})
	case intent.KindListRecurring:
		return intent.NewListRecurring(), nil
	case intent.KindListCards:
		return intent.NewListCards(), nil
	case intent.KindCountCards:
		return intent.NewCountCards(), nil
	case intent.KindBudgetRecurrence:
		return intent.NewBudgetRecurrence(intent.BudgetRecurrenceFields{
			SourceCompetence: s.SourceCompetence,
			Months:           s.Months,
		})
	default:
		return intent.NewUnknown(s.RawText)
	}
}

func joinReplies(replies []string) string {
	var parts []string
	for _, r := range replies {
		if strings.TrimSpace(r) != "" {
			parts = append(parts, r)
		}
	}
	return strings.Join(parts, "\n\n")
}
