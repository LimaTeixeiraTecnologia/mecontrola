package observability

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
)

func RedactCardLogFields(card entities.Card) []observability.Field {
	return []observability.Field{
		observability.String("card_id", card.ID.String()),
		observability.String("user_id", card.UserID.String()),
		observability.Int("closing_day", card.Cycle.ClosingDay),
		observability.Int("due_day", card.Cycle.DueDay),
	}
}
