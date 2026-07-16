package messages

import "testing"

func TestEditConfirmationBlock_Verbatim(t *testing.T) {
	got := EditConfirmationBlock(EditConfirmationView{
		PreviousAmountFormatted: "R$ 90,00",
		PreviousCategory:        "Mercado",
		PreviousPaymentMethod:   "pix",
		NewAmountFormatted:      "R$ 95,00",
	})
	want := "✏️ Lançamento atual: R$ 90,00 em *Mercado* (pix)\n🔄 Novo valor: R$ 95,00\n\nPosso atualizar?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEditConfirmationBlock_RendersNewCategoryWhenCategoryChanges(t *testing.T) {
	got := EditConfirmationBlock(EditConfirmationView{
		PreviousAmountFormatted: "R$ 90,00",
		PreviousCategory:        "Mercado",
		PreviousPaymentMethod:   "pix",
		NewAmountFormatted:      "R$ 90,00",
		NewCategory:             "Casa",
		CategoryChanged:         true,
	})
	want := "✏️ Lançamento atual: R$ 90,00 em *Mercado* (pix)\n🔄 Nova categoria: *Casa*\n\nPosso atualizar?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEditConfirmationBlock_RendersNewPaymentWhenPaymentChanges(t *testing.T) {
	got := EditConfirmationBlock(EditConfirmationView{
		PreviousAmountFormatted: "R$ 90,00",
		PreviousCategory:        "Mercado",
		PreviousPaymentMethod:   "pix",
		NewAmountFormatted:      "R$ 90,00",
		NewPaymentMethod:        "debit_card",
		PaymentChanged:          true,
	})
	want := "✏️ Lançamento atual: R$ 90,00 em *Mercado* (pix)\n🔄 Novo pagamento: debit_card\n\nPosso atualizar?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEditConfirmationBlock_RendersAllChangedFields(t *testing.T) {
	got := EditConfirmationBlock(EditConfirmationView{
		PreviousAmountFormatted: "R$ 90,00",
		PreviousCategory:        "Mercado",
		PreviousPaymentMethod:   "pix",
		NewAmountFormatted:      "R$ 95,00",
		NewCategory:             "Casa",
		NewPaymentMethod:        "debit_card",
		AmountChanged:           true,
		CategoryChanged:         true,
		PaymentChanged:          true,
	})
	want := "✏️ Lançamento atual: R$ 90,00 em *Mercado* (pix)\n🔄 Novo valor: R$ 95,00\n🔄 Nova categoria: *Casa*\n🔄 Novo pagamento: debit_card\n\nPosso atualizar?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRecurrenceConfirmationBlock_Verbatim(t *testing.T) {
	got := RecurrenceConfirmationBlock(RecurrenceConfirmationView{
		AmountFormatted: "R$ 50,00",
		Category:        "Saúde > Academia",
		Frequency:       "monthly",
	})
	want := "✅ Encontrei esta recorrência:\n\n💰 Valor: R$ 50,00\n📂 Categoria: Saúde > Academia\n🔁 Frequência: monthly\n\nPosso configurar?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEditSuccess_DeterministicBySeed(t *testing.T) {
	seed := NewMotivationSeed("wamid-fixed")
	first := EditSuccess(seed)
	second := EditSuccess(seed)
	if first != second {
		t.Fatal("expected deterministic motivational phrase for the same seed")
	}
}

func TestRecurrenceSuccess_DeterministicBySeed(t *testing.T) {
	seed := NewMotivationSeed("wamid-fixed")
	first := RecurrenceSuccess(seed)
	second := RecurrenceSuccess(seed)
	if first != second {
		t.Fatal("expected deterministic motivational phrase for the same seed")
	}
}

func TestWriteCancelled_NotEmpty(t *testing.T) {
	if WriteCancelled() == "" {
		t.Fatal("expected non-empty cancellation message")
	}
}

func TestWriteExpired_NotEmpty(t *testing.T) {
	if WriteExpired() == "" {
		t.Fatal("expected non-empty expiration message")
	}
}

func TestPaymentMethodMigrationBlocked_NotEmpty(t *testing.T) {
	if PaymentMethodMigrationBlocked() == "" {
		t.Fatal("expected non-empty payment method migration blocked message")
	}
}

func TestEditCandidatesPrompt_ListsAllOptions(t *testing.T) {
	got := EditCandidatesPrompt([]string{"R$ 30,00 - Transporte", "R$ 30,00 - Lazer"})
	want := "Encontrei mais de um lançamento compatível. Qual deles você quer editar?\n\n1. R$ 30,00 - Transporte\n2. R$ 30,00 - Lazer"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
