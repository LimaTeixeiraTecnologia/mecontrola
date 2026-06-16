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
	ThresholdAlertCardLimit
)

func (k ThresholdAlertKind) String() string {
	switch k {
	case ThresholdAlertCategory:
		return "category_threshold"
	case ThresholdAlertGoal:
		return "goal_achieved"
	case ThresholdAlertCardLimit:
		return "card_limit_near"
	default:
		return ""
	}
}

type ActiveBudgetSnapshot struct {
	UserID       uuid.UUID
	BudgetID     uuid.UUID
	Kind         ThresholdAlertKind
	CategoryID   uuid.UUID
	CardID       uuid.UUID
	RootSlug     valueobjects.RootSlug
	PlannedCents int64
	SpentCents   int64
}

type ThresholdConfig struct {
	Category valueobjects.ThresholdRatio
	Goal     valueobjects.ThresholdRatio
	Card     valueobjects.ThresholdRatio
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
		ratio, ok := thresholdForKind(s.Kind, thresholds)
		if !ok || ratio.IsZero() {
			continue
		}
		if !crossed(s.SpentCents, s.PlannedCents, ratio) {
			continue
		}
		key := ThresholdSentKey{UserID: s.UserID, BudgetID: s.BudgetID, Kind: s.Kind, RefDay: day}
		if _, sent := alreadySent[key]; sent {
			continue
		}
		out = append(out, DomainAlert{
			UserID:               s.UserID,
			BudgetID:             s.BudgetID,
			Kind:                 s.Kind,
			CategoryID:           s.CategoryID,
			CardID:               s.CardID,
			RootSlug:             s.RootSlug,
			PercentUsedBps:       percentUsedBps(s.SpentCents, s.PlannedCents),
			AmountRemainingCents: s.PlannedCents - s.SpentCents,
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

func thresholdForKind(k ThresholdAlertKind, cfg ThresholdConfig) (valueobjects.ThresholdRatio, bool) {
	switch k {
	case ThresholdAlertCategory:
		return cfg.Category, true
	case ThresholdAlertGoal:
		return cfg.Goal, true
	case ThresholdAlertCardLimit:
		return cfg.Card, true
	default:
		return valueobjects.ThresholdRatio{}, false
	}
}

func crossed(spent, planned int64, ratio valueobjects.ThresholdRatio) bool {
	bps := int64(ratio.Float64()*10000 + 0.5)
	return spent*10000 >= planned*bps
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
