package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type InvoiceDueCandidate struct {
	UserID     uuid.UUID
	CardID     uuid.UUID
	CardName   string
	Cycle      valueobjects.BillingCycle
	LimitCents int64
}

type InvoiceDueAlert struct {
	UserID     uuid.UUID
	CardID     uuid.UUID
	CardName   string
	LimitCents int64
	DueDate    time.Time
	DaysUntil  int
}

type InvoiceDueAlertsDecider struct{}

func NewInvoiceDueAlertsDecider() InvoiceDueAlertsDecider {
	return InvoiceDueAlertsDecider{}
}

func (InvoiceDueAlertsDecider) Decide(candidates []InvoiceDueCandidate, windowDays int, now time.Time, tz *time.Location) []InvoiceDueAlert {
	loc := tz
	if loc == nil {
		loc = time.UTC
	}
	today := now.In(loc)
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)

	out := make([]InvoiceDueAlert, 0, len(candidates))
	for _, c := range candidates {
		dueDate := nextDueDate(c.Cycle, startOfDay, loc)
		days := daysBetween(startOfDay, dueDate)
		if days < 0 || days > windowDays {
			continue
		}
		out = append(out, InvoiceDueAlert{
			UserID:     c.UserID,
			CardID:     c.CardID,
			CardName:   c.CardName,
			LimitCents: c.LimitCents,
			DueDate:    dueDate,
			DaysUntil:  days,
		})
	}
	return out
}

func nextDueDate(cycle valueobjects.BillingCycle, startOfDay time.Time, loc *time.Location) time.Time {
	year := startOfDay.Year()
	month := startOfDay.Month()
	dueDay := clamp(cycle.DueDay, year, month)
	candidate := time.Date(year, month, dueDay, 0, 0, 0, 0, loc)
	if candidate.Before(startOfDay) {
		nextYear, nextMonth := advanceMonth(year, month)
		nextDay := clamp(cycle.DueDay, nextYear, nextMonth)
		candidate = time.Date(nextYear, nextMonth, nextDay, 0, 0, 0, 0, loc)
	}
	return candidate
}

func daysBetween(from, to time.Time) int {
	diff := to.Sub(from)
	return int(diff.Hours() / 24)
}
