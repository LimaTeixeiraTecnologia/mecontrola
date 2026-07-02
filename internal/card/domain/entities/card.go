package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

const initialCardVersion int64 = 1

type Card struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Nickname  valueobjects.Nickname
	Bank      valueobjects.BankCode
	Cycle     valueobjects.BillingCycle
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type NewCardInput struct {
	UserID   uuid.UUID
	Nickname valueobjects.Nickname
	Bank     valueobjects.BankCode
	Cycle    valueobjects.BillingCycle
}

func NewCard(in NewCardInput) Card {
	now := time.Now().UTC()
	return Card{
		ID:        NewCardID(),
		UserID:    in.UserID,
		Nickname:  in.Nickname,
		Bank:      in.Bank,
		Cycle:     in.Cycle,
		Version:   initialCardVersion,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func HydrateCard(
	id uuid.UUID,
	userID uuid.UUID,
	nickname valueobjects.Nickname,
	bank valueobjects.BankCode,
	cycle valueobjects.BillingCycle,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) Card {
	return HydrateCardWithVersion(id, userID, nickname, bank, cycle, initialCardVersion, createdAt, updatedAt, deletedAt)
}

func HydrateCardWithVersion(
	id uuid.UUID,
	userID uuid.UUID,
	nickname valueobjects.Nickname,
	bank valueobjects.BankCode,
	cycle valueobjects.BillingCycle,
	version int64,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) Card {
	return Card{
		ID:        id,
		UserID:    userID,
		Nickname:  nickname,
		Bank:      bank,
		Cycle:     cycle,
		Version:   version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		DeletedAt: deletedAt,
	}
}

func (c Card) IsDeleted() bool {
	return c.DeletedAt != nil
}

func NewCardID() uuid.UUID {
	return uuid.New()
}
