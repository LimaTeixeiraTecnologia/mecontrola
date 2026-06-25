package pendingexpense

import "encoding/json"

type AwaitingKind string

const (
	AwaitingCategoryConfirm AwaitingKind = "category_confirm"
	AwaitingCategoryChoice  AwaitingKind = "category_choice"
)

type TransactionKind string

const (
	TransactionKindExpense      TransactionKind = "expense"
	TransactionKindIncome       TransactionKind = "income"
	TransactionKindCardPurchase TransactionKind = "card_purchase"
)

type CandidateRef struct {
	RootCategoryID string
	SubcategoryID  string
}

type Draft struct {
	AmountCents     int64
	Merchant        string
	PaymentMethod   string
	Direction       string
	OccurredAt      string
	CategoryID      string
	SubcategoryID   string
	CategoryPath    string
	Candidates      []string
	CandidateRefs   []CandidateRef
	AwaitingKind    AwaitingKind
	TransactionKind TransactionKind
	Installments    int
	CardHint        string
}

func Encode(d Draft) ([]byte, error) {
	return json.Marshal(d)
}

func Decode(raw []byte) (Draft, error) {
	var d Draft
	if err := json.Unmarshal(raw, &d); err != nil {
		return Draft{}, err
	}
	return d, nil
}
