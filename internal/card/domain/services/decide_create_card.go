package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type CreateCardCommand struct {
	UserID   uuid.UUID
	Nickname valueobjects.Nickname
	Bank     valueobjects.BankCode
	Cycle    valueobjects.BillingCycle
}

type CreateCardDecider struct{}

func NewCreateCardDecider() CreateCardDecider {
	return CreateCardDecider{}
}

func (CreateCardDecider) Decide(cmd CreateCardCommand, cardID uuid.UUID, now time.Time) entities.Card {
	at := now.UTC()
	return entities.HydrateCard(
		cardID,
		cmd.UserID,
		cmd.Nickname,
		cmd.Bank,
		cmd.Cycle,
		at,
		at,
		nil,
	)
}
