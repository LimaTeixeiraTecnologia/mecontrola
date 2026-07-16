package workflows

import "testing"

func TestTransactionWriteStatus_RoundTrip(t *testing.T) {
	values := []TransactionWriteStatus{
		TransactionWriteStatusActive,
		TransactionWriteStatusCompleted,
		TransactionWriteStatusCancelled,
		TransactionWriteStatusExpired,
		TransactionWriteStatusReplaced,
	}
	for _, v := range values {
		if !v.IsValid() {
			t.Fatalf("expected %v to be valid", v)
		}
		parsed, err := ParseTransactionWriteStatus(v.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed != v {
			t.Fatalf("expected %v, got %v", v, parsed)
		}
	}
}

func TestTransactionWriteStatus_Invalid(t *testing.T) {
	if _, err := ParseTransactionWriteStatus("bogus"); err == nil {
		t.Fatal("expected error for invalid status")
	}
	var zero TransactionWriteStatus
	if zero.IsValid() {
		t.Fatal("expected zero value to be invalid")
	}
}

func TestTransactionAwaitingSlot_RoundTrip(t *testing.T) {
	values := []TransactionAwaitingSlot{
		TransactionAwaitingCategory,
		TransactionAwaitingPaymentMethod,
		TransactionAwaitingCard,
		TransactionAwaitingDate,
		TransactionAwaitingEditCandidate,
		TransactionAwaitingConfirmation,
	}
	for _, v := range values {
		if !v.IsValid() {
			t.Fatalf("expected %v to be valid", v)
		}
		parsed, err := ParseTransactionAwaitingSlot(v.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed != v {
			t.Fatalf("expected %v, got %v", v, parsed)
		}
	}
}

func TestTransactionAwaitingSlot_Invalid(t *testing.T) {
	if _, err := ParseTransactionAwaitingSlot("bogus"); err == nil {
		t.Fatal("expected error for invalid slot")
	}
}

func TestTransactionOperationKind_RoundTrip(t *testing.T) {
	values := []TransactionOperationKind{
		TransactionOpRegisterExpense,
		TransactionOpRegisterIncome,
		TransactionOpEditEntry,
		TransactionOpCreateRecurrence,
	}
	for _, v := range values {
		if !v.IsValid() {
			t.Fatalf("expected %v to be valid", v)
		}
		parsed, err := ParseTransactionOperationKind(v.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed != v {
			t.Fatalf("expected %v, got %v", v, parsed)
		}
	}
}

func TestTransactionOperationKind_Invalid(t *testing.T) {
	if _, err := ParseTransactionOperationKind("bogus"); err == nil {
		t.Fatal("expected error for invalid operation kind")
	}
	var zero TransactionOperationKind
	if zero.IsValid() {
		t.Fatal("expected zero value to be invalid")
	}
}
