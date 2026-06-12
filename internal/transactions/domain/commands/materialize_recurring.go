package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RawMaterializeRecurring struct {
	TemplateID string
	RefMonth   string
	Today      time.Time
}

type MaterializeRecurring struct {
	TemplateID uuid.UUID
	RefMonth   valueobjects.RefMonth
	Today      time.Time
}

func NewMaterializeRecurring(raw RawMaterializeRecurring) (MaterializeRecurring, error) {
	var errs []error

	templateID, err := uuid.Parse(raw.TemplateID)
	if err != nil {
		errs = append(errs, fmt.Errorf("template_id: inválido"))
	}

	refMonth, err := valueobjects.NewRefMonth(raw.RefMonth)
	if err != nil {
		errs = append(errs, fmt.Errorf("ref_month: %w", err))
	}

	if raw.Today.IsZero() {
		errs = append(errs, ErrCommandMissingOccurredAt)
	}

	if len(errs) > 0 {
		return MaterializeRecurring{}, fmt.Errorf("commands/materialize_recurring: %w", errors.Join(errs...))
	}

	return MaterializeRecurring{
		TemplateID: templateID,
		RefMonth:   refMonth,
		Today:      raw.Today,
	}, nil
}
