package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type Card struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      valueobjects.CardName
	Nickname  valueobjects.Nickname
	Cycle     valueobjects.BillingCycle
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type NewCardInput struct {
	UserID   uuid.UUID
	Name     valueobjects.CardName
	Nickname valueobjects.Nickname
	Cycle    valueobjects.BillingCycle
}

func NewCard(in NewCardInput) Card {
	now := time.Now().UTC()
	return Card{
		ID:        NewCardID(),
		UserID:    in.UserID,
		Name:      in.Name,
		Nickname:  in.Nickname,
		Cycle:     in.Cycle,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func HydrateCard(
	id uuid.UUID,
	userID uuid.UUID,
	name valueobjects.CardName,
	nickname valueobjects.Nickname,
	cycle valueobjects.BillingCycle,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) Card {
	return Card{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Nickname:  nickname,
		Cycle:     cycle,
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
