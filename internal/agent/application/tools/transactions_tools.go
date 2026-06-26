package tools

import (
	"context"
	"errors"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type RecordExpense struct {
	recorder      *Recorder
	clarification *ClarificationResolver
	expense       ExpenseRecorder
	o11y          observability.Observability
}

func NewRecordExpense(recorder *Recorder, clarification *ClarificationResolver, expense ExpenseRecorder, o11y observability.Observability) *RecordExpense {
	return &RecordExpense{recorder: recorder, clarification: clarification, expense: expense, o11y: o11y}
}

func (t *RecordExpense) Name() string { return "record_expense" }

func (t *RecordExpense) Descriptor() ToolSpec {
	return ToolSpec{Name: "record_expense", IntentKind: intent.KindRecordExpense, Description: "record_expense", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *RecordExpense) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindRecordExpense
	if t.expense == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := t.expense.Execute(ctx, ExpenseRecorderInput{UserID: in.UserID.String(), Intent: in.Intent})
	if err != nil {
		if clarify, ok := t.clarification.ResolveCategory(ctx, in.UserID, in.Channel, kind, in.Intent, err); ok {
			return clarify, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.log_expense_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: registerFailedText(in.Intent.AmountCents(), in.Intent.Merchant()), Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: FormatPersistedExpense(result.AmountCents, in.Intent.Merchant(), result.CategoryPath), Outcome: OutcomeRouted, Kind: kind}, nil
}

type RecordIncome struct {
	recorder      *Recorder
	clarification *ClarificationResolver
	expense       ExpenseRecorder
	o11y          observability.Observability
}

func NewRecordIncome(recorder *Recorder, clarification *ClarificationResolver, expense ExpenseRecorder, o11y observability.Observability) *RecordIncome {
	return &RecordIncome{recorder: recorder, clarification: clarification, expense: expense, o11y: o11y}
}

func (t *RecordIncome) Name() string { return "record_income" }

func (t *RecordIncome) Descriptor() ToolSpec {
	return ToolSpec{Name: "record_income", IntentKind: intent.KindRecordIncome, Description: "record_income", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *RecordIncome) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindRecordIncome
	if t.expense == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := t.expense.Execute(ctx, ExpenseRecorderInput{UserID: in.UserID.String(), Intent: in.Intent})
	if err != nil {
		if clarify, ok := t.clarification.ResolveCategory(ctx, in.UserID, in.Channel, kind, in.Intent, err); ok {
			return clarify, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.log_income_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: registerFailedText(in.Intent.AmountCents(), in.Intent.Merchant()), Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatPersistedIncome(result.AmountCents, in.Intent.Merchant(), result.CategoryPath), Outcome: OutcomeRouted, Kind: kind}, nil
}

type RecordCardPurchase struct {
	recorder      *Recorder
	clarification *ClarificationResolver
	cardPurchase  CardPurchaseLogger
	o11y          observability.Observability
}

func NewRecordCardPurchase(recorder *Recorder, clarification *ClarificationResolver, cardPurchase CardPurchaseLogger, o11y observability.Observability) *RecordCardPurchase {
	return &RecordCardPurchase{recorder: recorder, clarification: clarification, cardPurchase: cardPurchase, o11y: o11y}
}

func (t *RecordCardPurchase) Name() string { return "record_card_purchase" }

func (t *RecordCardPurchase) Descriptor() ToolSpec {
	return ToolSpec{Name: "record_card_purchase", IntentKind: intent.KindRecordCardPurchase, Description: "record_card_purchase", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *RecordCardPurchase) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindRecordCardPurchase
	if t.cardPurchase == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	if in.Intent.AmountCents() == 0 {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
		return ToolResult{Reply: formatCardPurchaseAmountMissing(in.Intent.Merchant()), Outcome: OutcomeClarify, Kind: kind}, nil
	}
	result, err := t.cardPurchase.Execute(ctx, CardPurchaseLoggerInput{UserID: in.UserID.String(), Intent: in.Intent})
	if err != nil {
		if clarify, ok := t.clarification.ResolveCategory(ctx, in.UserID, in.Channel, kind, in.Intent, err); ok {
			return clarify, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.log_card_purchase_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: registerFailedText(in.Intent.AmountCents(), in.Intent.Merchant()), Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	if !result.CardFound {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: formatCardPurchaseCardMissing(in.Intent.CardHint()), Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: FormatPersistedCardPurchase(result), Outcome: OutcomeRouted, Kind: kind}, nil
}

type ListTransactions struct {
	recorder *Recorder
	lister   TransactionLister
	loc      *time.Location
	o11y     observability.Observability
}

func NewListTransactions(recorder *Recorder, lister TransactionLister, loc *time.Location, o11y observability.Observability) *ListTransactions {
	return &ListTransactions{recorder: recorder, lister: lister, loc: loc, o11y: o11y}
}

func (t *ListTransactions) Name() string { return "list_transactions" }

func (t *ListTransactions) Descriptor() ToolSpec {
	return ToolSpec{Name: "list_transactions", IntentKind: intent.KindListTransactions, Description: "list_transactions", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *ListTransactions) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindListTransactions
	if t.lister == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	refMonth := in.Intent.RefMonth()
	if refMonth == "" {
		refMonth = currentCompetence(t.loc)
	}
	list, err := WithReadRetry(ctx, func(ctx context.Context) (TransactionListResult, error) {
		return t.lister.Execute(ctx, TransactionListInput{UserID: in.UserID.String(), RefMonth: refMonth})
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.list_transactions_failed",
			observability.String("ref_month", refMonth),
			observability.Error(err),
		)
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatTransactionList(list), Outcome: OutcomeRouted, Kind: kind}, nil
}

type DeleteLastTransaction struct {
	recorder *Recorder
	lister   TransactionLister
	deleter  LastTransactionDeleter
	loc      *time.Location
	o11y     observability.Observability
}

func NewDeleteLastTransaction(recorder *Recorder, lister TransactionLister, deleter LastTransactionDeleter, loc *time.Location, o11y observability.Observability) *DeleteLastTransaction {
	return &DeleteLastTransaction{recorder: recorder, lister: lister, deleter: deleter, loc: loc, o11y: o11y}
}

func (t *DeleteLastTransaction) Name() string { return "delete_last_transaction" }

func (t *DeleteLastTransaction) Descriptor() ToolSpec {
	return ToolSpec{Name: "delete_last_transaction", IntentKind: intent.KindDeleteLastTransaction, Description: "delete_last_transaction", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *DeleteLastTransaction) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindDeleteLastTransaction
	if t.lister == nil || t.deleter == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	last, found, err := mostRecentTransaction(ctx, t.lister, in.UserID, t.loc, t.o11y)
	if err != nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	if !found {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
		return ToolResult{Reply: noTransactionsText, Outcome: OutcomeRouted, Kind: kind}, nil
	}
	if err := t.deleter.Execute(ctx, in.UserID.String(), last.ID, last.Version); err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.delete_last_transaction_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatDeletedTransaction(last), Outcome: OutcomeRouted, Kind: kind}, nil
}

type EditLastTransaction struct {
	recorder *Recorder
	lister   TransactionLister
	editor   LastTransactionEditor
	loc      *time.Location
	o11y     observability.Observability
}

func NewEditLastTransaction(recorder *Recorder, lister TransactionLister, editor LastTransactionEditor, loc *time.Location, o11y observability.Observability) *EditLastTransaction {
	return &EditLastTransaction{recorder: recorder, lister: lister, editor: editor, loc: loc, o11y: o11y}
}

func (t *EditLastTransaction) Name() string { return "edit_last_transaction" }

func (t *EditLastTransaction) Descriptor() ToolSpec {
	return ToolSpec{Name: "edit_last_transaction", IntentKind: intent.KindEditLastTransaction, Description: "edit_last_transaction", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *EditLastTransaction) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindEditLastTransaction
	if t.lister == nil || t.editor == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	last, found, err := mostRecentTransaction(ctx, t.lister, in.UserID, t.loc, t.o11y)
	if err != nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	if !found {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
		return ToolResult{Reply: noTransactionsText, Outcome: OutcomeRouted, Kind: kind}, nil
	}
	result, err := t.editor.Execute(ctx, EditTransactionInput{UserID: in.UserID.String(), Current: last, NewAmount: in.Intent.AmountCents()})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.edit_last_transaction_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatEditedTransaction(result), Outcome: OutcomeRouted, Kind: kind}, nil
}

type CreateRecurring struct {
	recorder      *Recorder
	clarification *ClarificationResolver
	creator       RecurringCreator
	o11y          observability.Observability
}

func NewCreateRecurring(recorder *Recorder, clarification *ClarificationResolver, creator RecurringCreator, o11y observability.Observability) *CreateRecurring {
	return &CreateRecurring{recorder: recorder, clarification: clarification, creator: creator, o11y: o11y}
}

func (t *CreateRecurring) Name() string { return "create_recurring" }

func (t *CreateRecurring) Descriptor() ToolSpec {
	return ToolSpec{Name: "create_recurring", IntentKind: intent.KindCreateRecurring, Description: "create_recurring", SchemaVersion: "v1", Timeout: 8 * time.Second, AuthzMode: AuthzUserOwned}
}

func (t *CreateRecurring) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindCreateRecurring
	if t.creator == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	result, err := t.creator.Execute(ctx, RecurringCreatorInput{UserID: in.UserID.String(), Intent: in.Intent})
	if err != nil {
		if clarify, ok := t.clarification.ResolveCategory(ctx, in.UserID, in.Channel, kind, in.Intent, err); ok {
			return clarify, nil
		}
		if errors.Is(err, ErrRecurringInvalidDay) {
			t.o11y.Logger().Warn(ctx, "agent.intent_router.create_recurring_invalid_day", observability.Error(err))
			t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeClarify)
			return ToolResult{Reply: recurringInvalidDayText, Outcome: OutcomeClarify, Kind: kind}, nil
		}
		t.o11y.Logger().Warn(ctx, "agent.intent_router.create_recurring_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: registerFailedText(in.Intent.AmountCents(), in.Intent.Merchant()), Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatPersistedRecurring(result), Outcome: OutcomeRouted, Kind: kind}, nil
}

type ListRecurring struct {
	recorder *Recorder
	lister   RecurringLister
	o11y     observability.Observability
}

func NewListRecurring(recorder *Recorder, lister RecurringLister, o11y observability.Observability) *ListRecurring {
	return &ListRecurring{recorder: recorder, lister: lister, o11y: o11y}
}

func (t *ListRecurring) Name() string { return "list_recurring" }

func (t *ListRecurring) Descriptor() ToolSpec {
	return ToolSpec{Name: "list_recurring", IntentKind: intent.KindListRecurring, Description: "list_recurring", SchemaVersion: "v1", Timeout: 5 * time.Second, AuthzMode: AuthzPublic}
}

func (t *ListRecurring) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	kind := intent.KindListRecurring
	if t.lister == nil {
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
		return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
	}
	items, err := WithReadRetry(ctx, func(ctx context.Context) ([]RecurringView, error) {
		return t.lister.Execute(ctx, in.UserID.String())
	})
	if err != nil {
		t.o11y.Logger().Warn(ctx, "agent.intent_router.list_recurring_failed", observability.Error(err))
		t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
		return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
	}
	t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
	return ToolResult{Reply: formatRecurringList(items), Outcome: OutcomeRouted, Kind: kind}, nil
}

func mostRecentTransaction(ctx context.Context, lister TransactionLister, userID uuid.UUID, loc *time.Location, o11y observability.Observability) (TransactionView, bool, error) {
	list, err := lister.Execute(ctx, TransactionListInput{UserID: userID.String(), RefMonth: currentCompetence(loc)})
	if err != nil {
		o11y.Logger().Warn(ctx, "agent.intent_router.most_recent_transaction_failed", observability.Error(err))
		return TransactionView{}, false, err
	}
	return pickMostRecent(list.Transactions)
}
