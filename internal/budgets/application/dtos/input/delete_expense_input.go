package input

type DeleteExpenseInput struct {
	UserID                string
	Source                string
	ExternalTransactionID string
	ExpectedVersion       int64
}
