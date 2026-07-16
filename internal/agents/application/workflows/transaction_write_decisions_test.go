package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func TestDecideTransactionPostWrite_GuardAntiFalsoSucesso(t *testing.T) {
	status, stepStatus, err := DecideTransactionPostWrite(agent.ToolOutcomeRouted, uuid.Nil)
	if err == nil {
		t.Fatal("expected error for write accepted without resource")
	}
	if stepStatus != workflow.StepStatusFailed {
		t.Fatalf("expected StepStatusFailed, got %v", stepStatus)
	}
	if status != TransactionWriteStatusActive {
		t.Fatalf("expected TransactionWriteStatusActive, got %v", status)
	}
}

func TestDecideTransactionPostWrite_Replay_NoResourceRequired(t *testing.T) {
	_, stepStatus, err := DecideTransactionPostWrite(agent.ToolOutcomeReplay, uuid.Nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stepStatus != workflow.StepStatusCompleted {
		t.Fatalf("expected StepStatusCompleted, got %v", stepStatus)
	}
}

func TestDecideTransactionPostWrite_Success(t *testing.T) {
	status, stepStatus, err := DecideTransactionPostWrite(agent.ToolOutcomeRouted, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stepStatus != workflow.StepStatusCompleted || status != TransactionWriteStatusCompleted {
		t.Fatalf("unexpected result: %v %v", status, stepStatus)
	}
}

func TestDecideTransactionInitialAwaiting_MapDispatchByOperation(t *testing.T) {
	scenarios := []struct {
		name string
		kind TransactionOperationKind
		args initialAwaitingArgs
		want TransactionAwaitingSlot
	}{
		{
			name: "expense sem categoria",
			kind: TransactionOpRegisterExpense,
			args: initialAwaitingArgs{CategoryAwaiting: TransactionAwaitingCategory},
			want: TransactionAwaitingCategory,
		},
		{
			name: "expense sem forma de pagamento",
			kind: TransactionOpRegisterExpense,
			args: initialAwaitingArgs{CategoryAwaiting: TransactionAwaitingConfirmation, PaymentMethod: ""},
			want: TransactionAwaitingPaymentMethod,
		},
		{
			name: "expense credito sem cartao",
			kind: TransactionOpRegisterExpense,
			args: initialAwaitingArgs{CategoryAwaiting: TransactionAwaitingConfirmation, PaymentMethod: PaymentMethodCreditCard, HasCard: false},
			want: TransactionAwaitingCard,
		},
		{
			name: "expense completo",
			kind: TransactionOpRegisterExpense,
			args: initialAwaitingArgs{CategoryAwaiting: TransactionAwaitingConfirmation, PaymentMethod: "pix"},
			want: TransactionAwaitingConfirmation,
		},
		{
			name: "income sem categoria",
			kind: TransactionOpRegisterIncome,
			args: initialAwaitingArgs{CategoryAwaiting: TransactionAwaitingCategory},
			want: TransactionAwaitingCategory,
		},
		{
			name: "income nao pergunta forma de pagamento",
			kind: TransactionOpRegisterIncome,
			args: initialAwaitingArgs{CategoryAwaiting: TransactionAwaitingConfirmation},
			want: TransactionAwaitingConfirmation,
		},
		{
			name: "edit com multiplos candidatos",
			kind: TransactionOpEditEntry,
			args: initialAwaitingArgs{EditCandidates: 3},
			want: TransactionAwaitingEditCandidate,
		},
		{
			name: "edit com um candidato",
			kind: TransactionOpEditEntry,
			args: initialAwaitingArgs{EditCandidates: 1},
			want: TransactionAwaitingConfirmation,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			got := DecideTransactionInitialAwaiting(scenario.kind, scenario.args)
			if got != scenario.want {
				t.Fatalf("expected %v, got %v", scenario.want, got)
			}
		})
	}
}

func TestDecideTransactionSlotResume_Expire(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC().Add(-time.Hour)}
	decision := DecideTransactionSlotResume(state, "pix", time.Now().UTC())
	if decision.Action != TransactionSlotActionExpire {
		t.Fatalf("expected expire, got %v", decision.Action)
	}
}

func TestDecideTransactionSlotResume_Cancel(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC()}
	decision := DecideTransactionSlotResume(state, "cancelar", time.Now().UTC())
	if decision.Action != TransactionSlotActionCancel {
		t.Fatalf("expected cancel, got %v", decision.Action)
	}
}

func TestDecideTransactionSlotResume_Replace(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC()}
	decision := DecideTransactionSlotResume(state, "gastei R$ 50 no mercado", time.Now().UTC())
	if decision.Action != TransactionSlotActionReplace {
		t.Fatalf("expected replace, got %v", decision.Action)
	}
}

func TestDecideTransactionSlotResume_FillPaymentMethod(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC(), Awaiting: TransactionAwaitingPaymentMethod}
	decision := DecideTransactionSlotResume(state, "pix", time.Now().UTC())
	if decision.Action != TransactionSlotActionFill || decision.FilledValue != "pix" {
		t.Fatalf("expected fill pix, got %+v", decision)
	}
}

func TestDecideTransactionSlotResume_RepromptThenCancel(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC(), Awaiting: TransactionAwaitingPaymentMethod}
	decision := DecideTransactionSlotResume(state, "não sei", time.Now().UTC())
	if decision.Action != TransactionSlotActionReprompt {
		t.Fatalf("expected reprompt, got %v", decision.Action)
	}

	state.RepromptCount = transactionMaxReprompts
	decision = DecideTransactionSlotResume(state, "não sei", time.Now().UTC())
	if decision.Action != TransactionSlotActionCancel {
		t.Fatalf("expected cancel after reprompt cap, got %v", decision.Action)
	}
}

func TestDecideTransactionConfirmation_Accept(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC()}
	action := DecideTransactionConfirmation(state, PendingMessage{Text: "sim"}, time.Now().UTC())
	if action != TransactionConfirmActionAccept {
		t.Fatalf("expected accept, got %v", action)
	}
}

func TestDecideTransactionConfirmation_Cancel(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC()}
	action := DecideTransactionConfirmation(state, PendingMessage{Text: "não"}, time.Now().UTC())
	if action != TransactionConfirmActionCancel {
		t.Fatalf("expected cancel, got %v", action)
	}
}

func TestDecideTransactionConfirmation_Expire(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC().Add(-time.Hour)}
	action := DecideTransactionConfirmation(state, PendingMessage{Text: "sim"}, time.Now().UTC())
	if action != TransactionConfirmActionExpire {
		t.Fatalf("expected expire, got %v", action)
	}
}

func TestDecideTransactionConfirmation_Replay(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC(), ProcessedMessageID: "wamid-1"}
	action := DecideTransactionConfirmation(state, PendingMessage{Text: "sim", MessageID: "wamid-1"}, time.Now().UTC())
	if action != TransactionConfirmActionReplay {
		t.Fatalf("expected replay, got %v", action)
	}
}

func TestDecideTransactionConfirmation_RepromptThenCancel(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC()}
	action := DecideTransactionConfirmation(state, PendingMessage{Text: "talvez"}, time.Now().UTC())
	if action != TransactionConfirmActionReprompt {
		t.Fatalf("expected reprompt, got %v", action)
	}

	state.ConfirmRepromptCount = transactionMaxReprompts
	action = DecideTransactionConfirmation(state, PendingMessage{Text: "talvez"}, time.Now().UTC())
	if action != TransactionConfirmActionCancel {
		t.Fatalf("expected cancel after reprompt cap, got %v", action)
	}
}

func TestDecideEditCandidateChoice(t *testing.T) {
	candidates := []TransactionEditCandidate{
		{TransactionID: uuid.New()},
		{TransactionID: uuid.New()},
	}
	idx, ok := DecideEditCandidateChoice(candidates, "2")
	if !ok || idx != 1 {
		t.Fatalf("expected index 1, got %d ok=%v", idx, ok)
	}

	_, ok = DecideEditCandidateChoice(candidates, "9")
	if ok {
		t.Fatal("expected out-of-range choice to be invalid")
	}

	_, ok = DecideEditCandidateChoice(candidates, "abacate")
	if ok {
		t.Fatal("expected non-numeric choice to be invalid")
	}
}

func TestDecideTransactionCategoryChoice_Selected(t *testing.T) {
	candidates := []PendingCategoryCandidate{
		{RootCategoryID: uuid.New(), SubcategoryID: uuid.New(), SubcategorySlug: "aluguel", Path: "Custo Fixo > Aluguel"},
	}
	decision, err := DecideTransactionCategoryChoice(candidates, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != CategoryChoiceActionSelected {
		t.Fatalf("expected selected, got %v", decision.Action)
	}
}

func TestDecideTransactionCategoryChoice_Reprompt(t *testing.T) {
	decision, err := DecideTransactionCategoryChoice(nil, "categoria inexistente")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != CategoryChoiceActionReprompt {
		t.Fatalf("expected reprompt, got %v", decision.Action)
	}
}

func TestDecideNewTransactionOperationReplacement(t *testing.T) {
	if !DecideNewTransactionOperationReplacement("gastei R$ 30 no uber") {
		t.Fatal("expected true for new complete operation")
	}
	if DecideNewTransactionOperationReplacement("pix") {
		t.Fatal("expected false for slot-fill text")
	}
}
