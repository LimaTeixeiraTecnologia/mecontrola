package services

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type BillingCycleResolver struct{}

func (r BillingCycleResolver) Resolve(
	purchasedAt time.Time,
	snapshot valueobjects.CardBillingSnapshot,
	n valueobjects.InstallmentCount,
) (refMonths []valueobjects.RefMonth, closings []time.Time, dues []time.Time) {
	count := n.Value()
	closingDay := snapshot.ClosingDay().Value()
	dueDay := snapshot.DueDay().Value()

	br := purchasedAt.In(purchasedAt.Location())
	purchaseDay := br.Day()

	var firstYear int
	var firstMonth time.Month

	if purchaseDay <= closingDay {
		firstYear = br.Year()
		firstMonth = br.Month()
	} else {
		next := time.Date(br.Year(), br.Month()+1, 1, 0, 0, 0, 0, br.Location())
		firstYear = next.Year()
		firstMonth = next.Month()
	}

	refMonths = make([]valueobjects.RefMonth, count)
	closings = make([]time.Time, count)
	dues = make([]time.Time, count)

	for i := range count {
		targetMonth := time.Month(int(firstMonth) + i)
		targetYear := firstYear
		for targetMonth > 12 {
			targetMonth -= 12
			targetYear++
		}

		closingEffective := clampDay(closingDay, targetYear, targetMonth)
		closings[i] = time.Date(targetYear, targetMonth, closingEffective, 0, 0, 0, 0, purchasedAt.Location())

		dueYear, dueMonth := targetYear, targetMonth
		if dueDay < closingDay {
			next := time.Date(targetYear, targetMonth+1, 1, 0, 0, 0, 0, time.UTC)
			dueYear, dueMonth = next.Year(), next.Month()
		}
		dueEffective := clampDay(dueDay, dueYear, dueMonth)
		dues[i] = time.Date(dueYear, dueMonth, dueEffective, 0, 0, 0, 0, purchasedAt.Location())

		refMonths[i] = valueobjects.RefMonthFromTime(time.Date(targetYear, targetMonth, 1, 0, 0, 0, 0, purchasedAt.Location()), purchasedAt.Location())
	}

	return refMonths, closings, dues
}

func clampDay(day, year int, month time.Month) int {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		return lastDay
	}
	return day
}
