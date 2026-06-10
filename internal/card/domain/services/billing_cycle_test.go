package services_test

import (
	"math/rand"
	"testing"
	"testing/quick"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

func mustBillingCycle(closing, due int) valueobjects.BillingCycle {
	c, err := valueobjects.NewBillingCycle(closing, due)
	if err != nil {
		panic(err)
	}
	return c
}

func spDate(sp *time.Location, year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, sp)
}

func TestBillingCycle_InvoiceFor_TableDriven(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}

	type fixture struct {
		name             string
		purchase         time.Time
		cycle            valueobjects.BillingCycle
		tz               *time.Location
		wantClosingYear  int
		wantClosingMonth time.Month
		wantClosingDay   int
		wantDueYear      int
		wantDueMonth     time.Month
		wantDueDay       int
	}

	fixtures := []fixture{
		{
			name:             "closing>due itau-style purchase before closing",
			purchase:         spDate(sp, 2026, time.January, 5),
			cycle:            mustBillingCycle(25, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       5,
		},
		{
			name:             "closing>due purchase after closing advances to next month",
			purchase:         spDate(sp, 2026, time.January, 26),
			cycle:            mustBillingCycle(25, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       5,
		},
		{
			name:             "closing<due same month purchase before closing",
			purchase:         spDate(sp, 2026, time.March, 1),
			cycle:            mustBillingCycle(10, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.March,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       20,
		},
		{
			name:             "closing<due purchase after closing advances",
			purchase:         spDate(sp, 2026, time.March, 15),
			cycle:            mustBillingCycle(10, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.April,
			wantDueDay:       20,
		},
		{
			name:             "closing==due closing becomes due-1",
			purchase:         spDate(sp, 2026, time.May, 1),
			cycle:            mustBillingCycle(15, 15),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.May,
			wantClosingDay:   14,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       15,
		},
		{
			name:             "closing==due purchase on computed closing does not advance",
			purchase:         spDate(sp, 2026, time.May, 14),
			cycle:            mustBillingCycle(15, 15),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.May,
			wantClosingDay:   14,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       15,
		},
		{
			name:             "closing==due purchase after computed closing advances",
			purchase:         spDate(sp, 2026, time.May, 15),
			cycle:            mustBillingCycle(15, 15),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.June,
			wantClosingDay:   14,
			wantDueYear:      2026,
			wantDueMonth:     time.June,
			wantDueDay:       15,
		},
		{
			name:             "february 28 non-leap year closing=31",
			purchase:         spDate(sp, 2026, time.February, 1),
			cycle:            mustBillingCycle(31, 10),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   28,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       10,
		},
		{
			name:             "february 29 leap year 2024 closing=31",
			purchase:         spDate(sp, 2024, time.February, 1),
			cycle:            mustBillingCycle(31, 10),
			tz:               sp,
			wantClosingYear:  2024,
			wantClosingMonth: time.February,
			wantClosingDay:   29,
			wantDueYear:      2024,
			wantDueMonth:     time.March,
			wantDueDay:       10,
		},
		{
			name:             "february 29 leap year 2028 closing=31",
			purchase:         spDate(sp, 2028, time.February, 1),
			cycle:            mustBillingCycle(31, 10),
			tz:               sp,
			wantClosingYear:  2028,
			wantClosingMonth: time.February,
			wantClosingDay:   29,
			wantDueYear:      2028,
			wantDueMonth:     time.March,
			wantDueDay:       10,
		},
		{
			name:             "april has 30 days closing=31 clamps to 30",
			purchase:         spDate(sp, 2026, time.April, 1),
			cycle:            mustBillingCycle(31, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       5,
		},
		{
			name:             "june has 30 days closing=31 clamps to 30",
			purchase:         spDate(sp, 2026, time.June, 1),
			cycle:            mustBillingCycle(31, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.June,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.July,
			wantDueDay:       5,
		},
		{
			name:             "september has 30 days closing=31 clamps to 30",
			purchase:         spDate(sp, 2026, time.September, 1),
			cycle:            mustBillingCycle(31, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.September,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.October,
			wantDueDay:       5,
		},
		{
			name:             "november has 30 days closing=31 clamps to 30",
			purchase:         spDate(sp, 2026, time.November, 1),
			cycle:            mustBillingCycle(31, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.November,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.December,
			wantDueDay:       5,
		},
		{
			name:             "december to january year rollover closing>due",
			purchase:         spDate(sp, 2026, time.December, 1),
			cycle:            mustBillingCycle(20, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.December,
			wantClosingDay:   20,
			wantDueYear:      2027,
			wantDueMonth:     time.January,
			wantDueDay:       5,
		},
		{
			name:             "december to january rollover purchase after closing",
			purchase:         spDate(sp, 2026, time.December, 25),
			cycle:            mustBillingCycle(20, 5),
			tz:               sp,
			wantClosingYear:  2027,
			wantClosingMonth: time.January,
			wantClosingDay:   20,
			wantDueYear:      2027,
			wantDueMonth:     time.February,
			wantDueDay:       5,
		},
		{
			name:             "purchase on closing date does not advance",
			purchase:         spDate(sp, 2026, time.March, 10),
			cycle:            mustBillingCycle(10, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.March,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       20,
		},
		{
			name:             "purchase one day after closing advances",
			purchase:         spDate(sp, 2026, time.March, 11),
			cycle:            mustBillingCycle(10, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.April,
			wantDueDay:       20,
		},
		{
			name:             "closing=1 due=15 purchase on day 1",
			purchase:         spDate(sp, 2026, time.June, 1),
			cycle:            mustBillingCycle(1, 15),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.June,
			wantClosingDay:   1,
			wantDueYear:      2026,
			wantDueMonth:     time.June,
			wantDueDay:       15,
		},
		{
			name:             "closing=1 due=15 purchase on day 2 advances",
			purchase:         spDate(sp, 2026, time.June, 2),
			cycle:            mustBillingCycle(1, 15),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.July,
			wantClosingDay:   1,
			wantDueYear:      2026,
			wantDueMonth:     time.July,
			wantDueDay:       15,
		},
		{
			name:             "due=31 in november clamps to 30",
			purchase:         spDate(sp, 2026, time.November, 1),
			cycle:            mustBillingCycle(15, 31),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.November,
			wantClosingDay:   15,
			wantDueYear:      2026,
			wantDueMonth:     time.November,
			wantDueDay:       30,
		},
		{
			name:             "due=31 in february non-leap clamps to 28",
			purchase:         spDate(sp, 2026, time.February, 1),
			cycle:            mustBillingCycle(10, 31),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       28,
		},
		{
			name:             "due=31 in february leap 2024 clamps to 29",
			purchase:         spDate(sp, 2024, time.February, 1),
			cycle:            mustBillingCycle(10, 31),
			tz:               sp,
			wantClosingYear:  2024,
			wantClosingMonth: time.February,
			wantClosingDay:   10,
			wantDueYear:      2024,
			wantDueMonth:     time.February,
			wantDueDay:       29,
		},
		{
			name:             "DST BR start 2018-10-21 purchase before closing",
			purchase:         time.Date(2018, time.October, 20, 12, 0, 0, 0, sp),
			cycle:            mustBillingCycle(25, 10),
			tz:               sp,
			wantClosingYear:  2018,
			wantClosingMonth: time.October,
			wantClosingDay:   25,
			wantDueYear:      2018,
			wantDueMonth:     time.November,
			wantDueDay:       10,
		},
		{
			name:             "DST BR end 2018-11-04 purchase before closing",
			purchase:         time.Date(2018, time.November, 3, 12, 0, 0, 0, sp),
			cycle:            mustBillingCycle(25, 10),
			tz:               sp,
			wantClosingYear:  2018,
			wantClosingMonth: time.November,
			wantClosingDay:   25,
			wantDueYear:      2018,
			wantDueMonth:     time.December,
			wantDueDay:       10,
		},
		{
			name:             "standard time 2026 BR no DST",
			purchase:         time.Date(2026, time.July, 15, 12, 0, 0, 0, sp),
			cycle:            mustBillingCycle(20, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.July,
			wantClosingDay:   20,
			wantDueYear:      2026,
			wantDueMonth:     time.August,
			wantDueDay:       5,
		},
		{
			name:             "closing=31 due=31 closing==due convention on january",
			purchase:         spDate(sp, 2026, time.January, 5),
			cycle:            mustBillingCycle(31, 31),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.January,
			wantDueDay:       31,
		},
		{
			name:             "closing=28 due=28 feb non-leap due-1=27",
			purchase:         spDate(sp, 2026, time.February, 1),
			cycle:            mustBillingCycle(28, 28),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   27,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       28,
		},
		{
			name:             "closing=29 due=10 feb non-leap clamps closing to 28",
			purchase:         spDate(sp, 2026, time.February, 1),
			cycle:            mustBillingCycle(29, 10),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   28,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       10,
		},
		{
			name:             "closing=29 due=10 feb leap 2024",
			purchase:         spDate(sp, 2024, time.February, 1),
			cycle:            mustBillingCycle(29, 10),
			tz:               sp,
			wantClosingYear:  2024,
			wantClosingMonth: time.February,
			wantClosingDay:   29,
			wantDueYear:      2024,
			wantDueMonth:     time.March,
			wantDueDay:       10,
		},
		{
			name:             "purchase on jan 31 closing=31 does not advance",
			purchase:         spDate(sp, 2026, time.January, 31),
			cycle:            mustBillingCycle(31, 10),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   31,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       10,
		},
		{
			name:             "purchase on dec 31 closing=31 does not advance due is jan",
			purchase:         spDate(sp, 2026, time.December, 31),
			cycle:            mustBillingCycle(31, 10),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.December,
			wantClosingDay:   31,
			wantDueYear:      2027,
			wantDueMonth:     time.January,
			wantDueDay:       10,
		},
		{
			name:             "closing=5 due=25 purchase before closing",
			purchase:         spDate(sp, 2026, time.August, 3),
			cycle:            mustBillingCycle(5, 25),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.August,
			wantClosingDay:   5,
			wantDueYear:      2026,
			wantDueMonth:     time.August,
			wantDueDay:       25,
		},
		{
			name:             "closing=5 due=25 purchase after closing",
			purchase:         spDate(sp, 2026, time.August, 6),
			cycle:            mustBillingCycle(5, 25),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.September,
			wantClosingDay:   5,
			wantDueYear:      2026,
			wantDueMonth:     time.September,
			wantDueDay:       25,
		},
		{
			name:             "closing=20 due=5 purchase on jan 1",
			purchase:         spDate(sp, 2026, time.January, 1),
			cycle:            mustBillingCycle(20, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   20,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       5,
		},
		{
			name:             "closing=20 due=5 purchase on jan 20 does not advance",
			purchase:         spDate(sp, 2026, time.January, 20),
			cycle:            mustBillingCycle(20, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   20,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       5,
		},
		{
			name:             "closing=20 due=5 purchase on jan 21 advances",
			purchase:         spDate(sp, 2026, time.January, 21),
			cycle:            mustBillingCycle(20, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   20,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       5,
		},
		{
			name:             "closing=15 due=1 closing>due purchase before closing feb non-leap",
			purchase:         spDate(sp, 2026, time.February, 14),
			cycle:            mustBillingCycle(15, 1),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   15,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       1,
		},
		{
			name:             "closing=31 due=1 jan purchase jan 1 stays in current cycle",
			purchase:         spDate(sp, 2026, time.January, 1),
			cycle:            mustBillingCycle(31, 1),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   31,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       1,
		},
		{
			name:             "end of year nov closing=30 due=15 purchase on 30 stays in current cycle",
			purchase:         spDate(sp, 2026, time.November, 30),
			cycle:            mustBillingCycle(30, 15),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.November,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.December,
			wantDueDay:       15,
		},
		{
			name:             "purchase UTC 02h March 11 is March 10 in SP stays in current cycle",
			purchase:         time.Date(2026, time.March, 11, 2, 0, 0, 0, time.UTC),
			cycle:            mustBillingCycle(10, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.March,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       20,
		},
		{
			name:             "purchase UTC 14h March 11 is March 11 in SP advances",
			purchase:         time.Date(2026, time.March, 11, 14, 0, 0, 0, time.UTC),
			cycle:            mustBillingCycle(10, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   10,
			wantDueYear:      2026,
			wantDueMonth:     time.April,
			wantDueDay:       20,
		},
		{
			name:             "closing=1 due=1 closing==due jan purchase jan 1 advances to next cycle",
			purchase:         spDate(sp, 2026, time.January, 1),
			cycle:            mustBillingCycle(1, 1),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   31,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       1,
		},
		{
			name:             "closing=2 due=2 closing==due purchase on day 1 stays",
			purchase:         spDate(sp, 2026, time.March, 1),
			cycle:            mustBillingCycle(2, 2),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.March,
			wantClosingDay:   1,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       2,
		},
		{
			name:             "closing=2 due=2 closing==due purchase on day 2 advances",
			purchase:         spDate(sp, 2026, time.March, 2),
			cycle:            mustBillingCycle(2, 2),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   1,
			wantDueYear:      2026,
			wantDueMonth:     time.April,
			wantDueDay:       2,
		},
		{
			name:             "closing=10 due=5 itau-style year-end nov purchase after closing",
			purchase:         spDate(sp, 2026, time.November, 15),
			cycle:            mustBillingCycle(10, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.December,
			wantClosingDay:   10,
			wantDueYear:      2027,
			wantDueMonth:     time.January,
			wantDueDay:       5,
		},
		{
			name:             "closing=28 due=5 feb 2026 purchase feb 1",
			purchase:         spDate(sp, 2026, time.February, 1),
			cycle:            mustBillingCycle(28, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.February,
			wantClosingDay:   28,
			wantDueYear:      2026,
			wantDueMonth:     time.March,
			wantDueDay:       5,
		},
		{
			name:             "closing=28 due=5 feb 2024 leap purchase feb 1",
			purchase:         spDate(sp, 2024, time.February, 1),
			cycle:            mustBillingCycle(28, 5),
			tz:               sp,
			wantClosingYear:  2024,
			wantClosingMonth: time.February,
			wantClosingDay:   28,
			wantDueYear:      2024,
			wantDueMonth:     time.March,
			wantDueDay:       5,
		},
		{
			name:             "purchase on last second of closing day stays in current cycle",
			purchase:         time.Date(2026, time.June, 9, 23, 59, 59, 0, sp),
			cycle:            mustBillingCycle(9, 20),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.June,
			wantClosingDay:   9,
			wantDueYear:      2026,
			wantDueMonth:     time.June,
			wantDueDay:       20,
		},
		{
			name:             "closing=30 due=10 april 30 days closing clamped already 30",
			purchase:         spDate(sp, 2026, time.April, 29),
			cycle:            mustBillingCycle(30, 10),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       10,
		},
		{
			name:             "closing=30 due=10 april purchase on 30 does not advance",
			purchase:         spDate(sp, 2026, time.April, 30),
			cycle:            mustBillingCycle(30, 10),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       10,
		},
		{
			name:             "due=31 closing>due clamp with closing==due jan convention",
			purchase:         spDate(sp, 2026, time.January, 5),
			cycle:            mustBillingCycle(31, 31),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   30,
			wantDueYear:      2026,
			wantDueMonth:     time.January,
			wantDueDay:       31,
		},
		{
			name:             "nubank style closing=7 due=17 purchase on 7",
			purchase:         spDate(sp, 2026, time.May, 7),
			cycle:            mustBillingCycle(7, 17),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.May,
			wantClosingDay:   7,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       17,
		},
		{
			name:             "nubank style closing=7 due=17 purchase on 8 advances",
			purchase:         spDate(sp, 2026, time.May, 8),
			cycle:            mustBillingCycle(7, 17),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.June,
			wantClosingDay:   7,
			wantDueYear:      2026,
			wantDueMonth:     time.June,
			wantDueDay:       17,
		},
		{
			name:             "itau style closing=25 due=2 purchase last day dec",
			purchase:         spDate(sp, 2025, time.December, 31),
			cycle:            mustBillingCycle(25, 2),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.January,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.February,
			wantDueDay:       2,
		},
		{
			name:             "itau style closing=25 due=2 purchase dec 24",
			purchase:         spDate(sp, 2025, time.December, 24),
			cycle:            mustBillingCycle(25, 2),
			tz:               sp,
			wantClosingYear:  2025,
			wantClosingMonth: time.December,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.January,
			wantDueDay:       2,
		},
		{
			name:             "closing=25 due=5 purchase on closing date stays",
			purchase:         spDate(sp, 2026, time.March, 25),
			cycle:            mustBillingCycle(25, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.March,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.April,
			wantDueDay:       5,
		},
		{
			name:             "closing=25 due=5 purchase day after closing advances",
			purchase:         spDate(sp, 2026, time.March, 26),
			cycle:            mustBillingCycle(25, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.April,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.May,
			wantDueDay:       5,
		},
		{
			name:             "purchase on last second of closing day in SP does not advance",
			purchase:         time.Date(2026, time.June, 25, 23, 59, 59, 0, sp),
			cycle:            mustBillingCycle(25, 5),
			tz:               sp,
			wantClosingYear:  2026,
			wantClosingMonth: time.June,
			wantClosingDay:   25,
			wantDueYear:      2026,
			wantDueMonth:     time.July,
			wantDueDay:       5,
		},
	}

	bc := services.BillingCycle{}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			inv := bc.InvoiceFor(f.purchase, f.cycle, f.tz)

			gotC := inv.ClosingDate.In(f.tz)
			gotD := inv.DueDate.In(f.tz)

			if gotC.Year() != f.wantClosingYear || gotC.Month() != f.wantClosingMonth || gotC.Day() != f.wantClosingDay {
				t.Errorf("ClosingDate: got %04d-%02d-%02d, want %04d-%02d-%02d",
					gotC.Year(), gotC.Month(), gotC.Day(),
					f.wantClosingYear, f.wantClosingMonth, f.wantClosingDay)
			}
			if gotD.Year() != f.wantDueYear || gotD.Month() != f.wantDueMonth || gotD.Day() != f.wantDueDay {
				t.Errorf("DueDate: got %04d-%02d-%02d, want %04d-%02d-%02d",
					gotD.Year(), gotD.Month(), gotD.Day(),
					f.wantDueYear, f.wantDueMonth, f.wantDueDay)
			}
		})
	}
}

func TestBillingCycle_InvoiceFor_PropertyBased(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}

	bc := services.BillingCycle{}

	baseTime := time.Date(2020, time.January, 1, 12, 0, 0, 0, sp)

	f := func(closingDay, dueDay uint8, purchaseDays uint32) bool {
		cd := int(closingDay%31) + 1
		dd := int(dueDay%31) + 1
		cycle, err := valueobjects.NewBillingCycle(cd, dd)
		if err != nil {
			return true
		}

		purchaseOffset := int(purchaseDays % (365 * 10))
		purchase := baseTime.Add(time.Duration(purchaseOffset) * 24 * time.Hour)

		inv := bc.InvoiceFor(purchase, cycle, sp)

		closingInSP := inv.ClosingDate.In(sp)
		dueInSP := inv.DueDate.In(sp)

		// Invariante (a): due_date >= closing_date
		if dueInSP.Before(closingInSP) {
			return false
		}

		purchaseInSP := purchase.In(sp)
		purchaseDayTime := time.Date(purchaseInSP.Year(), purchaseInSP.Month(), purchaseInSP.Day(), 0, 0, 0, 0, sp)
		dueDayTime := time.Date(dueInSP.Year(), dueInSP.Month(), dueInSP.Day(), 0, 0, 0, 0, sp)

		// Invariante (b): due_date >= purchase_date
		if dueDayTime.Before(purchaseDayTime) {
			return false
		}

		// Invariante (d): closing_date.day == min(closing_day, daysInMonth(closing_date.year, closing_date.month))
		// Excecao: quando closing_day == due_day, a convenção define closing como due_date - 1 dia.
		if cd != dd {
			dim := time.Date(closingInSP.Year(), closingInSP.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			expectedClosingDay := cd
			if expectedClosingDay > dim {
				expectedClosingDay = dim
			}
			if closingInSP.Day() != expectedClosingDay {
				return false
			}
		} else {
			expectedClosing := dueDayTime.AddDate(0, 0, -1)
			if !closingInSP.Equal(expectedClosing) {
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{
		MaxCount: 10000,
		Rand:     rand.New(rand.NewSource(42)),
	}

	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("property-based check failed: %v", err)
	}
}

func TestBillingCycle_InvoiceFor_Idempotence(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}

	bc := services.BillingCycle{}
	cycle := mustBillingCycle(20, 5)
	purchase := spDate(sp, 2026, time.March, 10)

	inv1 := bc.InvoiceFor(purchase, cycle, sp)
	inv2 := bc.InvoiceFor(purchase, cycle, sp)

	if !inv1.ClosingDate.Equal(inv2.ClosingDate) || !inv1.DueDate.Equal(inv2.DueDate) {
		t.Errorf("idempotence violated: first=%v second=%v", inv1, inv2)
	}
}

func TestBillingCycle_InvoiceFor_DueDateNeverBeforeClosingDate(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}

	bc := services.BillingCycle{}

	type tc struct {
		closing int
		due     int
	}

	cases := []tc{
		{1, 1}, {1, 15}, {15, 1}, {10, 20}, {20, 10}, {31, 1}, {1, 31}, {31, 31},
		{15, 15}, {7, 17}, {25, 2}, {28, 28},
	}

	baseDate := time.Date(2024, time.January, 1, 12, 0, 0, 0, sp)

	for _, tc := range cases {
		cycle, err := valueobjects.NewBillingCycle(tc.closing, tc.due)
		if err != nil {
			t.Fatalf("invalid cycle %d/%d: %v", tc.closing, tc.due, err)
		}
		for i := range 366 * 2 {
			purchase := baseDate.Add(time.Duration(i) * 24 * time.Hour)
			inv := bc.InvoiceFor(purchase, cycle, sp)
			if inv.DueDate.Before(inv.ClosingDate) {
				t.Errorf("due %v before closing %v for purchase %v cycle %d/%d",
					inv.DueDate, inv.ClosingDate, purchase, tc.closing, tc.due)
			}
		}
	}
}

func TestBillingCycle_InvoiceFor_DueDateNeverBeforePurchaseDate(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}

	bc := services.BillingCycle{}
	cycle := mustBillingCycle(20, 5)

	baseDate := time.Date(2024, time.January, 1, 12, 0, 0, 0, sp)

	for i := range 366 * 2 {
		purchase := baseDate.Add(time.Duration(i) * 24 * time.Hour)
		inv := bc.InvoiceFor(purchase, cycle, sp)
		purchaseDay := purchase.In(sp)
		purchaseNorm := time.Date(purchaseDay.Year(), purchaseDay.Month(), purchaseDay.Day(), 0, 0, 0, 0, sp)
		if inv.DueDate.Before(purchaseNorm) {
			t.Errorf("due %v before purchase %v", inv.DueDate, purchaseNorm)
		}
	}
}
