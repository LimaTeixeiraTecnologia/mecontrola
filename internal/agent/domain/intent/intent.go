package intent

import (
	"errors"
	"fmt"
	"strings"
)

type Kind int

const (
	KindUnknown Kind = iota + 1
	KindRecordExpense
	KindQueryCategory
	KindQueryGoal
	KindQueryCard
	KindMonthlySummary
	KindHowAmIDoing
	KindConfigureBudget
	KindRecordIncome
	KindRecordCardPurchase
	KindListTransactions
	KindDeleteLastTransaction
	KindEditLastTransaction
	KindCreateRecurring
	KindListRecurring
	KindListCards
	KindCreateCard
	KindCountCards
)

func (k Kind) String() string { //nolint:revive // dispatch exaustivo por intent kind
	switch k {
	case KindRecordExpense:
		return "record_expense"
	case KindRecordIncome:
		return "record_income"
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
	case KindRecordCardPurchase:
		return "record_card_purchase"
	case KindListTransactions:
		return "list_transactions"
	case KindDeleteLastTransaction:
		return "delete_last_transaction"
	case KindEditLastTransaction:
		return "edit_last_transaction"
	case KindCreateRecurring:
		return "create_recurring"
	case KindListRecurring:
		return "list_recurring"
	case KindListCards:
		return "list_cards"
	case KindCreateCard:
		return "create_card"
	case KindCountCards:
		return "count_cards"
	case KindUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

func (k Kind) IsWrite() bool {
	switch k {
	case KindRecordExpense,
		KindRecordIncome,
		KindRecordCardPurchase,
		KindCreateCard,
		KindConfigureBudget:
		return true
	default:
		return false
	}
}

func ParseKind(raw string) (Kind, error) { //nolint:revive // dispatch exaustivo por intent kind
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "record_expense":
		return KindRecordExpense, nil
	case "record_income":
		return KindRecordIncome, nil
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
	case "record_card_purchase":
		return KindRecordCardPurchase, nil
	case "list_transactions":
		return KindListTransactions, nil
	case "delete_last_transaction":
		return KindDeleteLastTransaction, nil
	case "edit_last_transaction":
		return KindEditLastTransaction, nil
	case "create_recurring":
		return KindCreateRecurring, nil
	case "list_recurring":
		return KindListRecurring, nil
	case "list_cards":
		return KindListCards, nil
	case "create_card":
		return KindCreateCard, nil
	case "count_cards":
		return KindCountCards, nil
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
	ErrInstallmentsTooFew   = errors.New("agent.intent: installments must be at least 2")
	ErrInstallmentsTooMany  = errors.New("agent.intent: installments exceed maximum allowed")
	ErrDirectionInvalid     = errors.New("agent.intent: direction must be income or outcome")
	ErrFrequencyInvalid     = errors.New("agent.intent: frequency must be monthly or yearly")
	ErrDayOfMonthInvalid    = errors.New("agent.intent: day_of_month must be between 1 and 31")
	ErrCardNicknameEmpty    = errors.New("agent.intent: card nickname is empty")
	ErrCardNicknameTooLong  = errors.New("agent.intent: card nickname exceeds maximum length")
)

const (
	maxMerchantLength     = 120
	maxCategoryHintLength = 80
	maxCardHintLength     = 80
	maxCategoryNameLength = 120
	maxGoalNameLength     = 120
	maxCardNameLength     = 120
	maxRawTextLength      = 4096
	minInstallments       = 2
	maxInstallments       = 24
	minDayOfMonth         = 1
	maxDayOfMonth         = 31
)

const (
	frequencyMonthly = "monthly"
	frequencyYearly  = "yearly"
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

const (
	directionIncomeTag  = "income"
	directionOutcomeTag = "outcome"
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
	cardNickname  string
	refMonth      string
	rawText       string
	installments  int
	direction     string
	frequency     string
	dayOfMonth    int
	closingDay    int
	dueDay        int
	limitCents    int64
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
func (i Intent) CardNickname() string  { return i.cardNickname }
func (i Intent) ClosingDay() int       { return i.closingDay }
func (i Intent) DueDay() int           { return i.dueDay }
func (i Intent) LimitCents() int64     { return i.limitCents }
func (i Intent) RefMonth() string      { return i.refMonth }
func (i Intent) RawText() string       { return i.rawText }
func (i Intent) Installments() int     { return i.installments }
func (i Intent) Direction() string     { return i.direction }
func (i Intent) Frequency() string     { return i.frequency }
func (i Intent) DayOfMonth() int       { return i.dayOfMonth }
func (i Intent) IsZero() bool          { return i.kind == 0 }

type RecordExpenseFields struct {
	AmountCents   int64
	Merchant      string
	CategoryHint  string
	PaymentMethod string
	CardHint      string
}

func NewRecordExpense(f RecordExpenseFields) (Intent, error) {
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
		kind:          KindRecordExpense,
		amountCents:   f.AmountCents,
		merchant:      merchant,
		categoryHint:  categoryHint,
		paymentMethod: paymentMethod,
		cardHint:      cardHint,
	}, nil
}

type RecordIncomeFields struct {
	AmountCents   int64
	Source        string
	CategoryHint  string
	PaymentMethod string
}

func NewRecordIncome(f RecordIncomeFields) (Intent, error) {
	if f.AmountCents <= 0 {
		return Intent{}, ErrAmountNonPositive
	}
	source := strings.TrimSpace(f.Source)
	if len([]rune(source)) > maxMerchantLength {
		return Intent{}, ErrMerchantTooLong
	}
	categoryHint := strings.TrimSpace(f.CategoryHint)
	if len([]rune(categoryHint)) > maxCategoryHintLength {
		return Intent{}, ErrCategoryHintTooLong
	}
	paymentMethod, err := normalizePaymentMethod(f.PaymentMethod)
	if err != nil {
		return Intent{}, err
	}
	return Intent{
		kind:          KindRecordIncome,
		amountCents:   f.AmountCents,
		merchant:      source,
		categoryHint:  categoryHint,
		paymentMethod: paymentMethod,
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
	if len([]rune(trimmed)) > maxGoalNameLength {
		return Intent{}, ErrGoalNameTooLong
	}
	return Intent{kind: KindQueryGoal, goalName: trimmed}, nil
}

func NewQueryCard(cardName string) (Intent, error) {
	trimmed := strings.TrimSpace(cardName)
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

type RecordCardPurchaseFields struct {
	AmountCents  int64
	Merchant     string
	CategoryHint string
	CardHint     string
	Installments int
}

func NewRecordCardPurchase(f RecordCardPurchaseFields) (Intent, error) {
	if f.AmountCents <= 0 {
		return Intent{}, ErrAmountNonPositive
	}
	if f.Installments < minInstallments {
		return Intent{}, ErrInstallmentsTooFew
	}
	if f.Installments > maxInstallments {
		return Intent{}, ErrInstallmentsTooMany
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
	return Intent{
		kind:         KindRecordCardPurchase,
		amountCents:  f.AmountCents,
		merchant:     merchant,
		categoryHint: categoryHint,
		cardHint:     cardHint,
		installments: f.Installments,
	}, nil
}

func NewListTransactions(refMonth string) (Intent, error) {
	trimmed := strings.TrimSpace(refMonth)
	if trimmed == "" {
		return Intent{kind: KindListTransactions}, nil
	}
	if !isYearMonth(trimmed) {
		return Intent{}, ErrRefMonthInvalid
	}
	return Intent{kind: KindListTransactions, refMonth: trimmed}, nil
}

func NewDeleteLastTransaction() Intent {
	return Intent{kind: KindDeleteLastTransaction}
}

func NewEditLastTransaction(amountCents int64) (Intent, error) {
	if amountCents <= 0 {
		return Intent{}, ErrAmountNonPositive
	}
	return Intent{kind: KindEditLastTransaction, amountCents: amountCents}, nil
}

type CreateRecurringFields struct {
	AmountCents  int64
	Merchant     string
	CategoryHint string
	Direction    string
	Frequency    string
	DayOfMonth   int
}

func NewCreateRecurring(f CreateRecurringFields) (Intent, error) {
	if f.AmountCents <= 0 {
		return Intent{}, ErrAmountNonPositive
	}
	direction, err := normalizeDirection(f.Direction)
	if err != nil {
		return Intent{}, err
	}
	frequency, err := normalizeFrequency(f.Frequency)
	if err != nil {
		return Intent{}, err
	}
	if f.DayOfMonth < minDayOfMonth || f.DayOfMonth > maxDayOfMonth {
		return Intent{}, ErrDayOfMonthInvalid
	}
	merchant := strings.TrimSpace(f.Merchant)
	if len([]rune(merchant)) > maxMerchantLength {
		return Intent{}, ErrMerchantTooLong
	}
	categoryHint := strings.TrimSpace(f.CategoryHint)
	if len([]rune(categoryHint)) > maxCategoryHintLength {
		return Intent{}, ErrCategoryHintTooLong
	}
	return Intent{
		kind:         KindCreateRecurring,
		amountCents:  f.AmountCents,
		merchant:     merchant,
		categoryHint: categoryHint,
		direction:    direction,
		frequency:    frequency,
		dayOfMonth:   f.DayOfMonth,
	}, nil
}

func NewListRecurring() Intent {
	return Intent{kind: KindListRecurring}
}

func NewListCards() Intent {
	return Intent{kind: KindListCards}
}

type CreateCardFields struct {
	Nickname   string
	Name       string
	ClosingDay int
	DueDay     int
	LimitCents int64
}

func NewCreateCard(f CreateCardFields) (Intent, error) {
	nickname := strings.TrimSpace(f.Nickname)
	if nickname == "" {
		return Intent{}, ErrCardNicknameEmpty
	}
	if len([]rune(nickname)) > maxCardNameLength {
		return Intent{}, ErrCardNicknameTooLong
	}
	name := strings.TrimSpace(f.Name)
	if len([]rune(name)) > maxCardNameLength {
		return Intent{}, ErrCardNameTooLong
	}
	return Intent{
		kind:         KindCreateCard,
		cardNickname: nickname,
		cardName:     name,
		closingDay:   f.ClosingDay,
		dueDay:       f.DueDay,
		limitCents:   f.LimitCents,
	}, nil
}

func NewCountCards() Intent {
	return Intent{kind: KindCountCards}
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

func normalizeDirection(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case directionIncomeTag:
		return directionIncomeTag, nil
	case directionOutcomeTag, "expense":
		return directionOutcomeTag, nil
	default:
		return "", fmt.Errorf("agent.intent: %q: %w", raw, ErrDirectionInvalid)
	}
}

func normalizeFrequency(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return frequencyMonthly, nil
	}
	switch trimmed {
	case frequencyMonthly, frequencyYearly:
		return trimmed, nil
	default:
		return "", fmt.Errorf("agent.intent: %q: %w", raw, ErrFrequencyInvalid)
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
