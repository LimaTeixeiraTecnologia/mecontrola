package observability

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
)

type Redactor struct{}

func (Redactor) RedactCardLogFields(card entities.Card) []observability.Field {
	return []observability.Field{
		observability.String("card_id", card.ID.String()),
		observability.String("user_id", card.UserID.String()),
		observability.Int("closing_day", card.Cycle.ClosingDay),
		observability.Int("due_day", card.Cycle.DueDay),
	}
}

func (Redactor) RedactOutputCardLogFields(card output.Card) []observability.Field {
	return []observability.Field{
		observability.String("card_id", card.ID),
		observability.String("user_id", card.UserID),
		observability.Int("closing_day", card.ClosingDay),
		observability.Int("due_day", card.DueDay),
	}
}
