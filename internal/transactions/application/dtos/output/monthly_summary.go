package output

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type MonthlySummary struct {
	UserID       uuid.UUID  `json:"user_id"`
	RefMonth     string     `json:"ref_month"`
	IncomeCents  int64      `json:"income_cents"`
	OutcomeCents int64      `json:"outcome_cents"`
	TotalCents   int64      `json:"total_cents"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

func MonthlySummaryFrom(s *entities.MonthlySummary) MonthlySummary {
	return MonthlySummary{
		UserID:       s.UserID(),
		RefMonth:     s.RefMonth().String(),
		IncomeCents:  s.IncomeCents(),
		OutcomeCents: s.OutcomeCents(),
		TotalCents:   s.TotalCents(),
		UpdatedAt:    s.UpdatedAt(),
	}
}
