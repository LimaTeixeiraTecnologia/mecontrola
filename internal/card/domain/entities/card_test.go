package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

func mustNickname(v string) valueobjects.Nickname {
	n, err := valueobjects.NewNickname(v)
	if err != nil {
		panic(err)
	}
	return n
}

func mustBankCode(v string) valueobjects.BankCode {
	b, err := valueobjects.NewBankCode(v)
	if err != nil {
		panic(err)
	}
	return b
}

func mustCycle(closing, due int) valueobjects.BillingCycle {
	c, err := valueobjects.NewBillingCycle(closing, due)
	if err != nil {
		panic(err)
	}
	return c
}

func TestNewCard(t *testing.T) {
	userID := uuid.New()
	in := entities.NewCardInput{
		UserID:   userID,
		Nickname: mustNickname("nubank"),
		Bank:     mustBankCode("Nubank"),
		Cycle:    mustCycle(13, 20),
	}

	before := time.Now().UTC().Truncate(time.Second)
	card := entities.NewCard(in)
	after := time.Now().UTC().Add(time.Second)

	if card.ID == (uuid.UUID{}) {
		t.Error("ID must not be zero")
	}
	if card.UserID != userID {
		t.Errorf("UserID: got %v, want %v", card.UserID, userID)
	}
	if card.Nickname.String() != "nubank" {
		t.Errorf("Nickname: got %q", card.Nickname.String())
	}
	if card.Bank.String() != "Nubank" {
		t.Errorf("Bank: got %q", card.Bank.String())
	}
	if card.Cycle.ClosingDay != 13 || card.Cycle.DueDay != 20 {
		t.Errorf("Cycle: got %+v", card.Cycle)
	}
	if card.CreatedAt.Before(before) || card.CreatedAt.After(after) {
		t.Errorf("CreatedAt out of expected range: %v", card.CreatedAt)
	}
	if card.DeletedAt != nil {
		t.Error("DeletedAt must be nil for new card")
	}
	if card.IsDeleted() {
		t.Error("IsDeleted() must be false for new card")
	}
}

func TestHydrateCard(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()
	deletedAt := now.Add(time.Hour)

	card := entities.HydrateCard(
		id, userID,
		mustNickname("itau"),
		mustBankCode("Itaú"),
		mustCycle(2, 10),
		now,
		now,
		&deletedAt,
	)

	if card.ID != id {
		t.Errorf("ID: got %v, want %v", card.ID, id)
	}
	if card.UserID != userID {
		t.Errorf("UserID mismatch")
	}
	if card.DeletedAt == nil || !card.DeletedAt.Equal(deletedAt) {
		t.Errorf("DeletedAt: got %v, want %v", card.DeletedAt, deletedAt)
	}
	if !card.IsDeleted() {
		t.Error("IsDeleted() must be true when DeletedAt is set")
	}
}

func TestHydrateCard_NotDeleted(t *testing.T) {
	card := entities.HydrateCard(
		uuid.New(), uuid.New(),
		mustNickname("card"),
		mustBankCode("Inter"),
		mustCycle(23, 1),
		time.Now().UTC(),
		time.Now().UTC(),
		nil,
	)

	if card.IsDeleted() {
		t.Error("IsDeleted() must be false when DeletedAt is nil")
	}
}

func TestNewCardID(t *testing.T) {
	id1 := entities.NewCardID()
	id2 := entities.NewCardID()

	if id1 == (uuid.UUID{}) {
		t.Error("NewCardID must not return zero UUID")
	}
	if id1 == id2 {
		t.Error("NewCardID must return unique values each call")
	}
}
