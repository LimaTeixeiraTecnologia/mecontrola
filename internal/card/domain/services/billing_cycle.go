package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type Invoice struct {
	ClosingDate time.Time
	DueDate     time.Time
}

func InvoiceFor(purchase time.Time, cycle valueobjects.BillingCycle, tz *time.Location) Invoice {
	p := purchase.In(tz)
	year := p.Year()
	month := p.Month()

	purchaseDay := time.Date(year, month, p.Day(), 0, 0, 0, 0, tz)

	closingDate, dueDate := computeCycle(cycle, year, month, tz)

	if purchaseDay.After(closingDate) {
		nextYear, nextMonth := advanceMonth(year, month)
		closingDate, dueDate = computeCycle(cycle, nextYear, nextMonth, tz)
	}

	return Invoice{
		ClosingDate: closingDate,
		DueDate:     dueDate,
	}
}

func computeCycle(cycle valueobjects.BillingCycle, year int, month time.Month, tz *time.Location) (closingDate, dueDate time.Time) {
	switch {
	case cycle.ClosingDay > cycle.DueDay:
		closingDay := clamp(cycle.ClosingDay, year, month)
		closingDate = time.Date(year, month, closingDay, 0, 0, 0, 0, tz)

		dueYear, dueMonth := advanceMonth(year, month)
		dueDay := clamp(cycle.DueDay, dueYear, dueMonth)
		dueDate = time.Date(dueYear, dueMonth, dueDay, 0, 0, 0, 0, tz)

	case cycle.ClosingDay < cycle.DueDay:
		closingDay := clamp(cycle.ClosingDay, year, month)
		closingDate = time.Date(year, month, closingDay, 0, 0, 0, 0, tz)

		dueDay := clamp(cycle.DueDay, year, month)
		dueDate = time.Date(year, month, dueDay, 0, 0, 0, 0, tz)

	default:
		dueDay := clamp(cycle.DueDay, year, month)
		dueDate = time.Date(year, month, dueDay, 0, 0, 0, 0, tz)
		closingDate = dueDate.AddDate(0, 0, -1)
	}

	return closingDate, dueDate
}

func advanceMonth(year int, month time.Month) (int, time.Month) {
	next := month + 1
	if next > 12 {
		return year + 1, time.January
	}
	return year, next
}

func clamp(day, year int, month time.Month) int {
	dim := daysInMonth(year, month)
	if day > dim {
		return dim
	}
	return day
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
