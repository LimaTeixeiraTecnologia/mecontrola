package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RefMonthResolver struct{}

func (r RefMonthResolver) From(t time.Time, loc *time.Location) valueobjects.RefMonth {
	return valueobjects.RefMonthFromTime(t, loc)
}
