package capability

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"

const (
	workflowTransactions = "transactions"
	workflowBudget       = "budget"
	workflowCards        = "cards"
	whatsAppChannel      = "whatsapp"
)

func BuildCatalog() (*Catalog, error) {
	return NewCatalog(
		newSpec(workflowTransactions, intent.KindRecordExpense, ModeWrite, false, true, true),
		newSpec(workflowTransactions, intent.KindRecordIncome, ModeWrite, false, true, true),
		newSpec(workflowTransactions, intent.KindRecordCardPurchase, ModeWrite, false, true, true),
		newSpec(workflowTransactions, intent.KindListTransactions, ModeRead, false, false, false),
		newSpec(workflowTransactions, intent.KindCreateRecurring, ModeWrite, false, false, false),
		newSpec(workflowTransactions, intent.KindListRecurring, ModeRead, false, false, false),
		newSpec(workflowTransactions, intent.KindQueryIncomeSummary, ModeRead, false, false, false),
		newSpec(workflowBudget, intent.KindMonthlySummary, ModeRead, false, false, false),
		newSpec(workflowBudget, intent.KindHowAmIDoing, ModeRead, false, false, false),
		newSpec(workflowBudget, intent.KindQueryCategory, ModeRead, false, false, false),
		newSpec(workflowBudget, intent.KindQueryGoal, ModeRead, false, false, false),
		newSpec(workflowBudget, intent.KindQueryCard, ModeRead, false, false, false),
		newSpec(workflowBudget, intent.KindConfigureBudget, ModeWrite, false, true, true),
		newSpec(workflowBudget, intent.KindEditCategoryPercentage, ModeWrite, false, false, false),
		newSpec(workflowBudget, intent.KindBudgetRecurrence, ModeWrite, false, false, false),
		newSpec(workflowCards, intent.KindListCards, ModeRead, false, false, false),
		newSpec(workflowCards, intent.KindCreateCard, ModeWrite, false, false, false),
		newSpec(workflowCards, intent.KindCountCards, ModeRead, false, false, false),
		newSpec(workflowCards, intent.KindUpdateCard, ModeWrite, false, false, false),
		newUnknownSpec(),
		newSpec(workflowTransactions, intent.KindDeleteLastTransaction, ModeWrite, true, true, true),
		newSpec(workflowTransactions, intent.KindEditLastTransaction, ModeWrite, true, true, true),
		newSpec(workflowCards, intent.KindDeleteCard, ModeWrite, true, true, true),
		newSpec(workflowTransactions, intent.KindDeleteTransactionByRef, ModeWrite, true, true, true),
		newSpec(workflowTransactions, intent.KindEditTransactionByRef, ModeWrite, true, true, true),
	)
}

func newSpec(workflowID string, kind intent.Kind, mode CapabilityMode, requiresConfirmation, supportsSuspend, supportsResume bool) CapabilitySpec {
	label := kind.String()
	return CapabilitySpec{
		ID:                   workflowID + "." + label,
		Description:          label,
		Kind:                 kind,
		WorkflowID:           workflowID,
		ToolName:             label,
		Mode:                 mode,
		RequiresConfirmation: requiresConfirmation,
		SupportsSuspend:      supportsSuspend,
		SupportsResume:       supportsResume,
		Channels:             []string{whatsAppChannel},
		MetricsKey:           label,
	}
}

func newUnknownSpec() CapabilitySpec {
	return CapabilitySpec{
		ID:          workflowConversational + ".unknown",
		Description: workflowConversational,
		Kind:        intent.KindUnknown,
		WorkflowID:  workflowConversational,
		Mode:        ModeRead,
		Channels:    []string{whatsAppChannel},
	}
}
