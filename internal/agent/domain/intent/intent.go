package intent

import (
	"errors"
	"fmt"
	"strings"
)

type Kind int

const (
	KindUnknown Kind = iota + 1
	KindLogExpense
	KindQueryCategory
	KindQueryGoal
	KindQueryCard
	KindMonthlySummary
	KindHowAmIDoing
	KindConfigureBudget
)

func (k Kind) String() string {
	switch k {
	case KindLogExpense:
		return "log_expense"
	case KindQueryCategory:
		return "query_category"
	case KindQueryGoal:
		return "query_goal"
	case KindQueryCard:
		return "query_card"
	case KindMonthlySummary:
		return "monthly_summary"
	case KindHowAmIDoing:
		return "how_am_i_doing"
	case KindConfigureBudget:
		return "configure_budget"
	case KindUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

func ParseKind(raw string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "log_expense":
		return KindLogExpense, nil
	case "query_category":
		return KindQueryCategory, nil
	case "query_goal":
		return KindQueryGoal, nil
	case "query_card":
		return KindQueryCard, nil
	case "monthly_summary":
		return KindMonthlySummary, nil
	case "how_am_i_doing":
		return KindHowAmIDoing, nil
	case "configure_budget":
		return KindConfigureBudget, nil
	case "unknown", "":
		return KindUnknown, nil
	default:
		return 0, fmt.Errorf("agent.intent: %q: %w", raw, ErrKindUnknown)
	}
}

var (
	ErrKindUnknown          = errors.New("agent.intent: kind not allowed")
	ErrAmountNonPositive    = errors.New("agent.intent: amount_cents must be positive")
	ErrCategoryNameEmpty    = errors.New("agent.intent: category_name is empty")
	ErrCategoryNameTooLong  = errors.New("agent.intent: category_name exceeds maximum length")
	ErrGoalNameEmpty        = errors.New("agent.intent: goal_name is empty")
	ErrGoalNameTooLong      = errors.New("agent.intent: goal_name exceeds maximum length")
	ErrCardNameEmpty        = errors.New("agent.intent: card_name is empty")
	ErrCardNameTooLong      = errors.New("agent.intent: card_name exceeds maximum length")
	ErrRawTextEmpty         = errors.New("agent.intent: raw_text is empty")
	ErrRefMonthInvalid      = errors.New("agent.intent: ref_month must be in YYYY-MM format")
	ErrMerchantTooLong      = errors.New("agent.intent: merchant exceeds maximum length")
	ErrCategoryHintTooLong  = errors.New("agent.intent: category_hint exceeds maximum length")
	ErrPaymentMethodInvalid = errors.New("agent.intent: payment_method not allowed")
	ErrCardHintTooLong      = errors.New("agent.intent: card_hint exceeds maximum length")
)

const (
	maxMerchantLength     = 120
	maxCategoryHintLength = 80
	maxCardHintLength     = 80
	maxCategoryNameLength = 120
	maxGoalNameLength     = 120
	maxCardNameLength     = 120
	maxRawTextLength      = 4096
)

const (
	paymentMethodPix        = "pix"
	paymentMethodCredit     = "credit"
	paymentMethodDebit      = "debit"
	paymentMethodCash       = "cash"
	paymentMethodTransfer   = "transfer"
	paymentMethodBoleto     = "boleto"
	paymentMethodUnknownTag = "unknown"
)

type Intent struct {
	kind          Kind
	amountCents   int64
	merchant      string
	categoryHint  string
	paymentMethod string
	cardHint      string
	categoryName  string
	goalName      string
	cardName      string
	refMonth      string
	rawText       string
}

func (i Intent) Kind() Kind            { return i.kind }
func (i Intent) AmountCents() int64    { return i.amountCents }
func (i Intent) Merchant() string      { return i.merchant }
func (i Intent) CategoryHint() string  { return i.categoryHint }
func (i Intent) PaymentMethod() string { return i.paymentMethod }
func (i Intent) CardHint() string      { return i.cardHint }
func (i Intent) CategoryName() string  { return i.categoryName }
func (i Intent) GoalName() string      { return i.goalName }
func (i Intent) CardName() string      { return i.cardName }
func (i Intent) RefMonth() string      { return i.refMonth }
func (i Intent) RawText() string       { return i.rawText }
func (i Intent) IsZero() bool          { return i.kind == 0 }

type LogExpenseFields struct {
	AmountCents   int64
	Merchant      string
	CategoryHint  string
	PaymentMethod string
	CardHint      string
}

func NewLogExpense(f LogExpenseFields) (Intent, error) {
	if f.AmountCents <= 0 {
		return Intent{}, ErrAmountNonPositive
	}
	merchant := strings.TrimSpace(f.Merchant)
	if len([]rune(merchant)) > maxMerchantLength {
		return Intent{}, ErrMerchantTooLong
	}
	categoryHint := strings.TrimSpace(f.CategoryHint)
	if len([]rune(categoryHint)) > maxCategoryHintLength {
		return Intent{}, ErrCategoryHintTooLong
	}
	cardHint := strings.TrimSpace(f.CardHint)
	if len([]rune(cardHint)) > maxCardHintLength {
		return Intent{}, ErrCardHintTooLong
	}
	paymentMethod, err := normalizePaymentMethod(f.PaymentMethod)
	if err != nil {
		return Intent{}, err
	}
	return Intent{
		kind:          KindLogExpense,
		amountCents:   f.AmountCents,
		merchant:      merchant,
		categoryHint:  categoryHint,
		paymentMethod: paymentMethod,
		cardHint:      cardHint,
	}, nil
}

func NewQueryCategory(categoryName string) (Intent, error) {
	trimmed := strings.TrimSpace(categoryName)
	if trimmed == "" {
		return Intent{}, ErrCategoryNameEmpty
	}
	if len([]rune(trimmed)) > maxCategoryNameLength {
		return Intent{}, ErrCategoryNameTooLong
	}
	return Intent{kind: KindQueryCategory, categoryName: trimmed}, nil
}

func NewQueryGoal(goalName string) (Intent, error) {
	trimmed := strings.TrimSpace(goalName)
	if trimmed == "" {
		return Intent{}, ErrGoalNameEmpty
	}
	if len([]rune(trimmed)) > maxGoalNameLength {
		return Intent{}, ErrGoalNameTooLong
	}
	return Intent{kind: KindQueryGoal, goalName: trimmed}, nil
}

func NewQueryCard(cardName string) (Intent, error) {
	trimmed := strings.TrimSpace(cardName)
	if trimmed == "" {
		return Intent{}, ErrCardNameEmpty
	}
	if len([]rune(trimmed)) > maxCardNameLength {
		return Intent{}, ErrCardNameTooLong
	}
	return Intent{kind: KindQueryCard, cardName: trimmed}, nil
}

func NewMonthlySummary(refMonth string) (Intent, error) {
	trimmed := strings.TrimSpace(refMonth)
	if trimmed == "" {
		return Intent{kind: KindMonthlySummary}, nil
	}
	if !isYearMonth(trimmed) {
		return Intent{}, ErrRefMonthInvalid
	}
	return Intent{kind: KindMonthlySummary, refMonth: trimmed}, nil
}

func NewHowAmIDoing() Intent {
	return Intent{kind: KindHowAmIDoing}
}

func NewConfigureBudget() Intent {
	return Intent{kind: KindConfigureBudget}
}

func NewUnknown(rawText string) (Intent, error) {
	trimmed := strings.TrimSpace(rawText)
	if trimmed == "" {
		return Intent{}, ErrRawTextEmpty
	}
	if len([]rune(trimmed)) > maxRawTextLength {
		trimmed = string([]rune(trimmed)[:maxRawTextLength])
	}
	return Intent{kind: KindUnknown, rawText: trimmed}, nil
}

func normalizePaymentMethod(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	switch trimmed {
	case paymentMethodPix,
		paymentMethodCredit,
		paymentMethodDebit,
		paymentMethodCash,
		paymentMethodTransfer,
		paymentMethodBoleto,
		paymentMethodUnknownTag:
		return trimmed, nil
	default:
		return "", fmt.Errorf("agent.intent: %q: %w", raw, ErrPaymentMethodInvalid)
	}
}

func isYearMonth(s string) bool {
	if len(s) != 7 {
		return false
	}
	if s[4] != '-' {
		return false
	}
	for idx, r := range s {
		if idx == 4 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	month := (int(s[5]-'0') * 10) + int(s[6]-'0')
	if month < 1 || month > 12 {
		return false
	}
	return true
}
