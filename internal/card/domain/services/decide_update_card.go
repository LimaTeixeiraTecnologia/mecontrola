package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type UpdateCardCommand struct {
	Name       *string
	Nickname   *string
	ClosingDay *int
	DueDay     *int
}

type UpdateCardDecider struct{}

func NewUpdateCardDecider() UpdateCardDecider {
	return UpdateCardDecider{}
}

func (UpdateCardDecider) Decide(current entities.Card, cmd UpdateCardCommand, now time.Time) (entities.Card, error) {
	name := current.Name
	if cmd.Name != nil {
		n, err := valueobjects.NewCardName(*cmd.Name)
		if err != nil {
			return entities.Card{}, err
		}
		name = n
	}

	nickname := current.Nickname
	if cmd.Nickname != nil {
		nk, err := valueobjects.NewNickname(*cmd.Nickname)
		if err != nil {
			return entities.Card{}, err
		}
		nickname = nk
	}

	cycle := current.Cycle
	if cmd.ClosingDay != nil || cmd.DueDay != nil {
		cd := current.Cycle.ClosingDay
		dd := current.Cycle.DueDay
		if cmd.ClosingDay != nil {
			cd = *cmd.ClosingDay
		}
		if cmd.DueDay != nil {
			dd = *cmd.DueDay
		}
		c, err := valueobjects.NewBillingCycle(cd, dd)
		if err != nil {
			return entities.Card{}, err
		}
		cycle = c
	}

	return entities.HydrateCardWithVersion(
		current.ID,
		current.UserID,
		name,
		nickname,
		cycle,
		current.LimitCents,
		current.Version,
		current.CreatedAt,
		now.UTC(),
		current.DeletedAt,
	), nil
}
