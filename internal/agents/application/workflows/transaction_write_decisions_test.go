package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
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

func TestRecognizePaymentMethod_AliasesEPreposicoes(t *testing.T) {
	scenarios := []struct {
		text string
		want string
	}{
		{text: "pix", want: "pix"},
		{text: "no pix", want: "pix"},
		{text: "paguei no pix", want: "pix"},
		{text: "foi no débito", want: "debit_card"},
		{text: "com dinheiro", want: "cash"},
		{text: "cartão de crédito", want: "credit_card"},
		{text: "no cartão de débito", want: "debit_card"},
		{text: "vale refeição", want: "vale_refeicao"},
		{text: "vr", want: "vale_refeicao"},
		{text: "va", want: "vale_alimentacao"},
		{text: "pelo boleto", want: "boleto"},
		{text: "Sim", want: ""},
		{text: "não sei", want: ""},
		{text: "qualquer coisa", want: ""},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.text, func(t *testing.T) {
			answer := DecidePaymentAnswer(scenario.text)
			if answer.Method != scenario.want {
				t.Fatalf("DecidePaymentAnswer(%q).Method: expected %q, got %q", scenario.text, scenario.want, answer.Method)
			}
			if answer.CardHint != "" {
				t.Fatalf("DecidePaymentAnswer(%q): hint inesperado %q", scenario.text, answer.CardHint)
			}
		})
	}
}

func TestDecideTransactionSlotResume_SimInvalidoNoSlotDePagamento(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC(), Awaiting: TransactionAwaitingPaymentMethod}
	decision := DecideTransactionSlotResume(state, "Sim", time.Now().UTC())
	if decision.Action != TransactionSlotActionReprompt {
		t.Fatalf("expected reprompt for 'Sim', got %v", decision.Action)
	}

	state.RepromptCount = transactionMaxReprompts
	decision = DecideTransactionSlotResume(state, "Sim", time.Now().UTC())
	if decision.Action != TransactionSlotActionCancel {
		t.Fatalf("expected cancel after reprompt cap, got %v", decision.Action)
	}
}

func TestBuildTransactionSlotReprompt_PaymentMethodMensagemDedicada(t *testing.T) {
	state := TransactionWriteState{Awaiting: TransactionAwaitingPaymentMethod}
	got := buildTransactionSlotReprompt(state)
	want := messages.PaymentMethodReprompt()
	if got != want {
		t.Fatalf("expected reprompt dedicado %q, got %q", want, got)
	}
	if got == messages.ClarificationQuestion(messages.MissingFieldPaymentMethod) {
		t.Fatal("reprompt nao pode ser identico a pergunta inicial")
	}
}

func TestDecideTransactionConfirmation_ReplayComWamidDoResume(t *testing.T) {
	state := TransactionWriteState{
		SuspendedAt:        time.Now().UTC(),
		ProcessedMessageID: "wamid-sim-1",
	}
	action := DecideTransactionConfirmation(state, PendingMessage{Text: "sim", MessageID: "wamid-sim-1"}, time.Now().UTC())
	if action != TransactionConfirmActionReplay {
		t.Fatalf("expected replay for repeated wamid, got %v", action)
	}

	action = DecideTransactionConfirmation(state, PendingMessage{Text: "sim", MessageID: "wamid-sim-2"}, time.Now().UTC())
	if action != TransactionConfirmActionAccept {
		t.Fatalf("expected accept for new wamid, got %v", action)
	}
}

func TestDecidePaymentMethodFromCard(t *testing.T) {
	if got := DecidePaymentMethodFromCard("", true); got != PaymentMethodCreditCard {
		t.Fatalf("cartão presente sem pagamento deve virar credit_card, got %q", got)
	}
	if got := DecidePaymentMethodFromCard("pix", true); got != "pix" {
		t.Fatalf("pagamento explícito deve prevalecer, got %q", got)
	}
	if got := DecidePaymentMethodFromCard("", false); got != "" {
		t.Fatalf("sem cartão e sem pagamento deve permanecer vazio, got %q", got)
	}
}

func TestDecidePaymentAnswer_CartaoComApelido(t *testing.T) {
	scenarios := []struct {
		text     string
		method   string
		cardHint string
	}{
		{text: "Cartão de crédito XP", method: "credit_card", cardHint: "xp"},
		{text: "cartão xp", method: "credit_card", cardHint: "xp"},
		{text: "no cartão de crédito nubank", method: "credit_card", cardHint: "nubank"},
		{text: "crédito roxinho", method: "credit_card", cardHint: "roxinho"},
		{text: "crédito", method: "credit_card", cardHint: ""},
		{text: "paguei no pix", method: "pix", cardHint: ""},
		{text: "qualquer coisa", method: "", cardHint: ""},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.text, func(t *testing.T) {
			answer := DecidePaymentAnswer(scenario.text)
			if answer.Method != scenario.method || answer.CardHint != scenario.cardHint {
				t.Fatalf("DecidePaymentAnswer(%q) = %+v, expected method=%q hint=%q", scenario.text, answer, scenario.method, scenario.cardHint)
			}
		})
	}
}

func TestDecideTransactionSlotResume_FillPaymentWithCardHint(t *testing.T) {
	state := TransactionWriteState{SuspendedAt: time.Now().UTC(), Awaiting: TransactionAwaitingPaymentMethod}
	decision := DecideTransactionSlotResume(state, "Cartão de crédito XP", time.Now().UTC())
	if decision.Action != TransactionSlotActionFillPaymentWithCard {
		t.Fatalf("expected fill payment with card, got %v", decision.Action)
	}
	if decision.FilledValue != PaymentMethodCreditCard || decision.CardHint != "xp" {
		t.Fatalf("unexpected decision %+v", decision)
	}
}

func TestIsNewCompleteOperation_SemPrefixoMonetario(t *testing.T) {
	scenarios := []struct {
		text string
		want bool
	}{
		{text: "Paguei 100 reais no abastecimento do veículo no cartão xp", want: true},
		{text: "Gastei 21,57 no supermercado", want: true},
		{text: "gastei R$ 30 no uber", want: true},
		{text: "comprei 2 contos de bala", want: true},
		{text: "1", want: false},
		{text: "sim", want: false},
		{text: "Custo fixo > combustível", want: false},
		{text: "paguei no crédito em 12x", want: false},
		{text: "crédito", want: false},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.text, func(t *testing.T) {
			if got := isNewCompleteOperation(scenario.text); got != scenario.want {
				t.Fatalf("isNewCompleteOperation(%q) = %v, expected %v", scenario.text, got, scenario.want)
			}
		})
	}
}

func TestDecideTransactionCategoryChoice_NomeDeFolhaEPlural(t *testing.T) {
	leafCandidates := []PendingCategoryCandidate{
		{RootCategoryID: testRootCustoFixoID, RootSlug: "custo-fixo", SubcategoryID: testLeafCombustivel, SubcategorySlug: "combustivel", Path: "Custo Fixo > Combustível"},
		{RootCategoryID: testRootCustoFixoID, RootSlug: "custo-fixo", SubcategoryID: testLeafSupermercado, SubcategorySlug: "supermercado", Path: "Custo Fixo > Supermercado"},
	}

	scenarios := []struct {
		text   string
		action CategoryChoiceAction
		leafID uuid.UUID
	}{
		{text: "Combustível", action: CategoryChoiceActionSelected, leafID: testLeafCombustivel},
		{text: "combustivel", action: CategoryChoiceActionSelected, leafID: testLeafCombustivel},
		{text: "Custo fixo > combustível", action: CategoryChoiceActionSelected, leafID: testLeafCombustivel},
		{text: "custo fixo e combustivel", action: CategoryChoiceActionSelected, leafID: testLeafCombustivel},
		{text: "supermercados", action: CategoryChoiceActionSelected, leafID: testLeafSupermercado},
		{text: "custos fixos", action: CategoryChoiceActionAmbiguous},
		{text: "nada a ver", action: CategoryChoiceActionReprompt},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.text, func(t *testing.T) {
			decision, err := DecideTransactionCategoryChoice(leafCandidates, scenario.text)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Action != scenario.action {
				t.Fatalf("expected %v, got %v", scenario.action, decision.Action)
			}
			if scenario.leafID != uuid.Nil && decision.Candidate.SubcategoryID != scenario.leafID {
				t.Fatalf("expected leaf %s, got %s", scenario.leafID, decision.Candidate.SubcategoryID)
			}
		})
	}
}

func TestDecideTransactionCategoryChoice_ListaDeRaizes(t *testing.T) {
	rootCandidates := []PendingCategoryCandidate{
		{RootCategoryID: testRootCustoFixoID, RootSlug: "custo-fixo", Path: "Custo Fixo"},
		{RootCategoryID: testRootPrazeresID, RootSlug: "prazeres", Path: "Prazeres"},
	}

	scenarios := []struct {
		text   string
		action CategoryChoiceAction
		rootID uuid.UUID
	}{
		{text: "1", action: CategoryChoiceActionRootOnly, rootID: testRootCustoFixoID},
		{text: "custos fixos", action: CategoryChoiceActionRootOnly, rootID: testRootCustoFixoID},
		{text: "Prazeres", action: CategoryChoiceActionRootOnly, rootID: testRootPrazeresID},
		{text: "nada a ver", action: CategoryChoiceActionReprompt},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.text, func(t *testing.T) {
			decision, err := DecideTransactionCategoryChoice(rootCandidates, scenario.text)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Action != scenario.action {
				t.Fatalf("expected %v, got %v", scenario.action, decision.Action)
			}
			if scenario.rootID != uuid.Nil && decision.Candidate.RootCategoryID != scenario.rootID {
				t.Fatalf("expected root %s, got %s", scenario.rootID, decision.Candidate.RootCategoryID)
			}
		})
	}
}
