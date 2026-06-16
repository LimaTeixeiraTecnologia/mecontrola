package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

const initialCardVersion int64 = 1

type Card struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       valueobjects.CardName
	Nickname   valueobjects.Nickname
	Cycle      valueobjects.BillingCycle
	LimitCents int64
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

type NewCardInput struct {
	UserID     uuid.UUID
	Name       valueobjects.CardName
	Nickname   valueobjects.Nickname
	Cycle      valueobjects.BillingCycle
	LimitCents int64
}

func NewCard(in NewCardInput) Card {
	now := time.Now().UTC()
	return Card{
		ID:         NewCardID(),
		UserID:     in.UserID,
		Name:       in.Name,
		Nickname:   in.Nickname,
		Cycle:      in.Cycle,
		LimitCents: in.LimitCents,
		Version:    initialCardVersion,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func HydrateCard(
	id uuid.UUID,
	userID uuid.UUID,
	name valueobjects.CardName,
	nickname valueobjects.Nickname,
	cycle valueobjects.BillingCycle,
	limitCents int64,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) Card {
	return HydrateCardWithVersion(id, userID, name, nickname, cycle, limitCents, initialCardVersion, createdAt, updatedAt, deletedAt)
}

func HydrateCardWithVersion(
	id uuid.UUID,
	userID uuid.UUID,
	name valueobjects.CardName,
	nickname valueobjects.Nickname,
	cycle valueobjects.BillingCycle,
	limitCents int64,
	version int64,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) Card {
	return Card{
		ID:         id,
		UserID:     userID,
		Name:       name,
		Nickname:   nickname,
		Cycle:      cycle,
		LimitCents: limitCents,
		Version:    version,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		DeletedAt:  deletedAt,
	}
}

func (c Card) IsDeleted() bool {
	return c.DeletedAt != nil
}

func (c Card) UpdateLimit(newLimit valueobjects.CardLimit, now time.Time) Card {
	c.LimitCents = newLimit.Cents()
	c.UpdatedAt = now.UTC()
	c.Version++
	return c
}

func NewCardID() uuid.UUID {
	return uuid.New()
}
