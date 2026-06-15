package mappers

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

const invoiceDateLayout = "2006-01-02"

func (Mapper) ToInvoiceOutput(invoice services.Invoice, tz *time.Location) output.Invoice {
	return output.Invoice{
		ClosingDate: invoice.ClosingDate.In(tz).Format(invoiceDateLayout),
		DueDate:     invoice.DueDate.In(tz).Format(invoiceDateLayout),
	}
}

func (Mapper) ToCardOutput(c entities.Card) output.Card {
	return output.Card{
		ID:         c.ID.String(),
		UserID:     c.UserID.String(),
		Name:       c.Name.String(),
		Nickname:   c.Nickname.String(),
		ClosingDay: c.Cycle.ClosingDay,
		DueDay:     c.Cycle.DueDay,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		DeletedAt:  c.DeletedAt,
	}
}

func (m Mapper) ToCardListOutput(cards []entities.Card, nextCursor string) output.CardList {
	items := make([]output.Card, 0, len(cards))
	for _, c := range cards {
		items = append(items, m.ToCardOutput(c))
	}
	var next *string
	if nextCursor != "" {
		nc := nextCursor
		next = &nc
	}
	return output.CardList{Items: items, NextCursor: next}
}
