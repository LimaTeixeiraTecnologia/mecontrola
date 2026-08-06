package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdAlertKind uint8

const (
	ThresholdAlertCategory ThresholdAlertKind = iota + 1
	ThresholdAlertGoal
	ThresholdAlertCategory80
	ThresholdAlertCategory100
	ThresholdAlertCategory90
	ThresholdAlertBudgetMissingMonthStart
	ThresholdAlertBudgetNotReviewedDay3
)

func (k ThresholdAlertKind) String() string {
	switch k {
	case ThresholdAlertCategory:
		return "category_threshold"
	case ThresholdAlertGoal:
		return "goal_achieved"
	case ThresholdAlertCategory80:
		return "category_threshold_80"
	case ThresholdAlertCategory100:
		return "category_threshold_100"
	case ThresholdAlertCategory90:
		return "category_threshold_90"
	case ThresholdAlertBudgetMissingMonthStart:
		return "budget_missing_month_start"
	case ThresholdAlertBudgetNotReviewedDay3:
		return "budget_not_reviewed_day_3"
	default:
		return ""
	}
}

func (k ThresholdAlertKind) IsValid() bool {
	return k.String() != ""
}

type ActiveBudgetSnapshot struct {
	UserID       uuid.UUID
	BudgetID     uuid.UUID
	CategoryID   uuid.UUID
	CardID       uuid.UUID
	RootSlug     valueobjects.RootSlug
	PlannedCents int64
	SpentCents   int64
}

type ThresholdConfig struct {
	Category valueobjects.ThresholdRatio
	Goal     valueobjects.ThresholdRatio
}

type DomainAlert struct {
	UserID               uuid.UUID
	BudgetID             uuid.UUID
	Kind                 ThresholdAlertKind
	CategoryID           uuid.UUID
	CardID               uuid.UUID
	RootSlug             valueobjects.RootSlug
	PercentUsedBps       int32
	AmountRemainingCents int64
	PlannedCents         int64
	SpentCents           int64
	RefDay               time.Time
}

type ThresholdWorkflow struct{}

func (ThresholdWorkflow) DecideAlerts(
	snapshots []ActiveBudgetSnapshot,
	thresholds ThresholdConfig,
	alreadySent map[ThresholdSentKey]struct{},
	refDay time.Time,
) []DomainAlert {
	day := refDay.UTC().Truncate(24 * time.Hour)
	out := make([]DomainAlert, 0, len(snapshots))
	for _, s := range snapshots {
		if s.PlannedCents <= 0 {
			continue
		}
		if s.RootSlug == valueobjects.RootSlugMetas {
			continue
		}
		kind, ok := DecideCategoryKind(s.SpentCents, s.PlannedCents)
		if !ok {
			continue
		}
		key := ThresholdSentKey{UserID: s.UserID, BudgetID: s.BudgetID, Kind: kind, RefDay: day}
		if _, sent := alreadySent[key]; sent {
			continue
		}
		out = append(out, DomainAlert{
			UserID:               s.UserID,
			BudgetID:             s.BudgetID,
			Kind:                 kind,
			CategoryID:           s.CategoryID,
			CardID:               s.CardID,
			RootSlug:             s.RootSlug,
			PercentUsedBps:       percentUsedBps(s.SpentCents, s.PlannedCents),
			AmountRemainingCents: s.PlannedCents - s.SpentCents,
			PlannedCents:         s.PlannedCents,
			SpentCents:           s.SpentCents,
			RefDay:               day,
		})
	}
	return out
}

type ThresholdSentKey struct {
	UserID   uuid.UUID
	BudgetID uuid.UUID
	Kind     ThresholdAlertKind
	RefDay   time.Time
}

func percentUsedBps(spent, planned int64) int32 {
	if planned <= 0 {
		return 0
	}
	v := (spent * 10000) / planned
	if v > 2_147_483_647 {
		return 2_147_483_647
	}
	return int32(v)
}
