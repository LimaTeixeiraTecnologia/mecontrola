package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type MonthlySummary struct {
	userID       uuid.UUID
	refMonth     valueobjects.RefMonth
	incomeCents  int64
	outcomeCents int64
	totalCents   int64
	version      int64
	updatedAt    *time.Time
}

func NewMonthlySummary(
	userID uuid.UUID,
	refMonth valueobjects.RefMonth,
	incomeCents int64,
	outcomeCents int64,
	version int64,
	updatedAt *time.Time,
) MonthlySummary {
	return MonthlySummary{
		userID:       userID,
		refMonth:     refMonth,
		incomeCents:  incomeCents,
		outcomeCents: outcomeCents,
		totalCents:   incomeCents - outcomeCents,
		version:      version,
		updatedAt:    updatedAt,
	}
}

func (m *MonthlySummary) UserID() uuid.UUID               { return m.userID }
func (m *MonthlySummary) RefMonth() valueobjects.RefMonth { return m.refMonth }
func (m *MonthlySummary) IncomeCents() int64              { return m.incomeCents }
func (m *MonthlySummary) OutcomeCents() int64             { return m.outcomeCents }
func (m *MonthlySummary) TotalCents() int64               { return m.totalCents }
func (m *MonthlySummary) Version() int64                  { return m.version }
func (m *MonthlySummary) UpdatedAt() *time.Time           { return m.updatedAt }
