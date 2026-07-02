package services

import "time"

type PurchaseDay struct {
	ClosingDay int
	BestDay    int
}

type PurchaseDayService struct{}

func (PurchaseDayService) Decide(dueDay, daysBeforeDue int, now time.Time, tz *time.Location) PurchaseDay {
	ref := now.In(tz)
	due := time.Date(ref.Year(), ref.Month(), clamp(dueDay, ref.Year(), ref.Month()), 0, 0, 0, 0, tz)
	closing := due.AddDate(0, 0, -daysBeforeDue)
	closingDay := closing.Day()
	bestDay := closingDay + 1
	if bestDay > 31 {
		bestDay = 1
	}
	return PurchaseDay{ClosingDay: closingDay, BestDay: bestDay}
}
