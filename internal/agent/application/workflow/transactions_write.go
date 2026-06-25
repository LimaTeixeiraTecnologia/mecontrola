package workflow

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const TransactionsWriteWorkflowID = "transactions_write"

type TransactionsWriteDeps struct {
	Authorize      steps.AuthorizeFunc
	Replay         steps.ReplayFunc
	Policy         steps.PolicyFunc
	AuditBegin     steps.AuditBeginFunc
	OnSettle       steps.OnSettleRegistered
	Resolver       steps.CategoryResolverFunc
	Persist        steps.PersistFunc
	DenyReply      string
	ReplayReply    string
	AuditFailReply string
	RetryPolicy    platform.RetryPolicy
	MaxAttempts    int
}

func NewTransactionsWriteDefinition(deps TransactionsWriteDeps) platform.Definition[steps.ExpenseState] {
	root := platform.Sequence("transactions_write_seq",
		steps.NewAuthorize(deps.Authorize, deps.DenyReply),
		steps.NewReplay(deps.Replay),
		steps.NewPolicy(deps.Policy),
		platform.Retry(steps.NewAuditBegin(deps.AuditBegin, deps.OnSettle, deps.ReplayReply, deps.AuditFailReply), deps.RetryPolicy),
		platform.Retry(steps.NewResolveCategory(deps.Resolver), deps.RetryPolicy),
		platform.Retry(steps.NewPersist(deps.Persist), deps.RetryPolicy),
		steps.NewFormat(formatExpenseReply),
	)
	if deps.MaxAttempts <= 0 {
		deps.MaxAttempts = 1
	}
	return platform.Definition[steps.ExpenseState]{
		ID:          TransactionsWriteWorkflowID,
		Root:        root,
		Durable:     true,
		MaxAttempts: deps.MaxAttempts,
	}
}

func formatExpenseReply(state steps.ExpenseState) string {
	switch state.Kind {
	case intent.KindRecordIncome:
		return tools.FormatPersistedIncome(state.AmountCents, state.Merchant, state.CategoryPath)
	case intent.KindRecordCardPurchase:
		return tools.FormatPersistedCardPurchase(tools.CardPurchaseLoggerResult{
			AmountCents:  state.AmountCents,
			CategoryPath: state.CategoryPath,
			CardFound:    true,
			CardName:     state.CardName,
			Installments: state.Installments,
		})
	default:
		return tools.FormatPersistedExpense(state.AmountCents, state.Merchant, state.CategoryPath)
	}
}

func ExpenseStateToToolResult(state steps.ExpenseState) tools.ToolResult {
	return tools.ToolResult{
		Reply:   state.Reply,
		Outcome: state.Outcome,
		Kind:    state.Kind,
	}
}

func ExpenseStateFromToolInput(in tools.ToolInput) steps.ExpenseState {
	kind := in.Intent.Kind()
	return steps.ExpenseState{
		UserID:          in.UserID,
		Channel:         in.Channel,
		MessageID:       in.MessageID,
		StepIndex:       in.StepIndex,
		Confidence:      in.Confidence.Value(),
		Kind:            kind,
		TransactionKind: resolveTransactionKind(kind),
		AmountCents:     in.Intent.AmountCents(),
		Merchant:        in.Intent.Merchant(),
		CategoryHint:    in.Intent.CategoryHint(),
		PaymentMethod:   in.Intent.PaymentMethod(),
		Direction:       resolveDirection(kind),
		Installments:    in.Intent.Installments(),
		CardHint:        in.Intent.CardHint(),
		LLMModel:        in.LLMModel,
		PromptSHA256:    in.PromptSHA256,
		DirectReply:     in.DirectReply,
		RawResponse:     in.RawResponse,
	}
}

func resolveTransactionKind(k intent.Kind) pendingexpense.TransactionKind {
	switch k {
	case intent.KindRecordIncome:
		return pendingexpense.TransactionKindIncome
	case intent.KindRecordCardPurchase:
		return pendingexpense.TransactionKindCardPurchase
	default:
		return pendingexpense.TransactionKindExpense
	}
}

func resolveDirection(k intent.Kind) string {
	if k == intent.KindRecordIncome {
		return "income"
	}
	return "outcome"
}
