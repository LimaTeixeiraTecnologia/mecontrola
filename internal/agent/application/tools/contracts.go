package tools

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type MonthlySummaryReader interface {
	Execute(ctx context.Context, userID string, competence string) (budgetsoutput.MonthlySummaryOutput, error)
}

type CardLister interface {
	Execute(ctx context.Context, in cardinput.ListCards) (cardoutput.CardList, error)
}

type CardInvoiceReader interface {
	Execute(ctx context.Context, in cardinput.InvoiceFor) (cardoutput.Invoice, error)
}

type CardCreator interface {
	Execute(ctx context.Context, userID uuid.UUID, in intent.Intent) (CardCreatorResult, error)
}

type CardCreatorResult struct {
	Nickname   string
	Name       string
	ClosingDay int
	DueDay     int
	LimitCents int64
}

type CardCounter interface {
	Execute(ctx context.Context, userID uuid.UUID) (int64, error)
}

type CardUpdater interface {
	Execute(ctx context.Context, userID uuid.UUID, in intent.Intent) (CardUpdaterResult, error)
}

type CardUpdaterResult struct {
	Nickname   string
	Name       string
	ClosingDay int
	DueDay     int
	LimitCents int64
}

type CardDeleter interface {
	Execute(ctx context.Context, userID uuid.UUID, cardName string) (CardDeleterResult, error)
}

type CardDeleterResult struct {
	Name string
}

type CategoryPercentageEditorInput struct {
	UserID       uuid.UUID
	Competence   string
	CategoryName string
	Percentage   int
}

type CategoryPercentageEditorResult struct {
	Competence string
	RootSlug   string
	Percentage int
}

type CategoryPercentageEditor interface {
	Execute(ctx context.Context, in CategoryPercentageEditorInput) (CategoryPercentageEditorResult, error)
}

type Fallback interface {
	Reply(ctx context.Context, userID uuid.UUID, channel, text string) (string, error)
}

type BudgetConfigurator interface {
	Start(ctx context.Context, userID uuid.UUID, channel string) (string, error)
}

type BudgetConversationResult struct {
	Draft    budgetdraft.Draft
	Complete bool
	Reply    string
}

type BudgetConversation interface {
	Configure(ctx context.Context, text string, draft budgetdraft.Draft) (BudgetConversationResult, error)
}

type BudgetConfigCommitter interface {
	Commit(ctx context.Context, userID uuid.UUID, draft budgetdraft.Draft) (string, error)
}

type BudgetSessionGateway interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (budgetdraft.Draft, bool, error)
	Save(ctx context.Context, userID uuid.UUID, channel string, draft budgetdraft.Draft) error
	Clear(ctx context.Context, userID uuid.UUID, channel string) error
}

type ExpenseRecorder interface {
	Execute(ctx context.Context, in ExpenseRecorderInput) (ExpenseRecorderResult, error)
}

type ExpenseRecorderInput struct {
	UserID        string
	Intent        intent.Intent
	ForceCategory *string
	AmountCents   int64
	Merchant      string
	PaymentMethod string
	Direction     string
	OccurredAt    string
}

type ExpenseRecorderResult struct {
	Persisted      bool
	SubcategoryID  string
	RootCategoryID string
	AmountCents    int64
	Competence     string
	CategoryPath   string
	OccurredAt     time.Time
}

type CardPurchaseLogger interface {
	Execute(ctx context.Context, in CardPurchaseLoggerInput) (CardPurchaseLoggerResult, error)
}

type CardPurchaseLoggerInput struct {
	UserID        string
	Intent        intent.Intent
	ForceCategory *string
	AmountCents   int64
	Merchant      string
	PaymentMethod string
	CardHint      string
	Installments  int
}

type CardPurchaseLoggerResult struct {
	Persisted    bool
	CardFound    bool
	CardName     string
	AmountCents  int64
	Installments int
	CategoryPath string
}

type TransactionView struct {
	ID          string
	Direction   string
	AmountCents int64
	Description string
	OccurredAt  time.Time
	CreatedAt   time.Time
	Version     int64
}

type TransactionLister interface {
	Execute(ctx context.Context, in TransactionListInput) (TransactionListResult, error)
}

type TransactionListInput struct {
	UserID   string
	RefMonth string
}

type TransactionListResult struct {
	RefMonth     string
	Transactions []TransactionView
}

type LastTransactionDeleter interface {
	Execute(ctx context.Context, userID, txID string, version int64) error
}

type LastTransactionEditor interface {
	Execute(ctx context.Context, in EditTransactionInput) (EditTransactionResult, error)
}

type EditTransactionInput struct {
	UserID    string
	Current   TransactionView
	NewAmount int64
}

type EditTransactionResult struct {
	Persisted   bool
	OldAmount   int64
	NewAmount   int64
	Description string
}

type RecurringCreator interface {
	Execute(ctx context.Context, in RecurringCreatorInput) (RecurringCreatorResult, error)
}

type RecurringCreatorInput struct {
	UserID string
	Intent intent.Intent
}

type RecurringCreatorResult struct {
	Persisted    bool
	Direction    string
	AmountCents  int64
	Frequency    string
	DayOfMonth   int
	CategoryPath string
	Description  string
}

type RecurringView struct {
	Direction   string
	AmountCents int64
	Description string
	Frequency   string
	DayOfMonth  int
}

type RecurringLister interface {
	Execute(ctx context.Context, userID string) ([]RecurringView, error)
}

type PendingExpenseConfirmationGateway interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (pendingexpense.Draft, bool, error)
	Save(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) error
	Clear(ctx context.Context, userID uuid.UUID, channel string) error
}
