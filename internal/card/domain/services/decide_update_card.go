package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type UpdateCardCommand struct {
	Nickname *string
	Bank     *valueobjects.BankCode
	Cycle    *valueobjects.BillingCycle
}

type UpdateCardDecider struct{}

func NewUpdateCardDecider() UpdateCardDecider {
	return UpdateCardDecider{}
}

func (UpdateCardDecider) Decide(current entities.Card, cmd UpdateCardCommand, now time.Time) (entities.Card, error) {
	nickname := current.Nickname
	if cmd.Nickname != nil {
		nk, err := valueobjects.NewNickname(*cmd.Nickname)
		if err != nil {
			return entities.Card{}, err
		}
		nickname = nk
	}

	bank := current.Bank
	if cmd.Bank != nil {
		bank = *cmd.Bank
	}

	cycle := current.Cycle
	if cmd.Cycle != nil {
		cycle = *cmd.Cycle
	}

	return entities.HydrateCardWithVersion(
		current.ID,
		current.UserID,
		nickname,
		bank,
		cycle,
		current.Version,
		current.CreatedAt,
		now.UTC(),
		current.DeletedAt,
	), nil
}
