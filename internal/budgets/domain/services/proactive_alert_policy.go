package services

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	BudgetMissingMaxDayOfMonth     = 5
	BudgetNotReviewedMinDayOfMonth = 3
)

type MissingBudgetSnapshot struct {
	UserID uuid.UUID
}

type UnreviewedBudgetSnapshot struct {
	UserID   uuid.UUID
	BudgetID uuid.UUID
}

func DecideBudgetLifecycleAlerts(
	missing []MissingBudgetSnapshot,
	unreviewed []UnreviewedBudgetSnapshot,
	alreadySent map[ThresholdSentKey]struct{},
	refDay time.Time,
) []DomainAlert {
	day := refDay.UTC().Truncate(24 * time.Hour)
	dayOfMonth := day.Day()
	out := make([]DomainAlert, 0, len(missing)+len(unreviewed))

	if dayOfMonth <= BudgetMissingMaxDayOfMonth {
		for _, snapshot := range missing {
			key := ThresholdSentKey{
				UserID:   snapshot.UserID,
				BudgetID: uuid.Nil,
				Kind:     ThresholdAlertBudgetMissingMonthStart,
				RefDay:   day,
			}
			if _, sent := alreadySent[key]; sent {
				continue
			}
			out = append(out, DomainAlert{
				UserID:   snapshot.UserID,
				BudgetID: uuid.Nil,
				Kind:     ThresholdAlertBudgetMissingMonthStart,
				RefDay:   day,
			})
		}
	}

	if dayOfMonth >= BudgetNotReviewedMinDayOfMonth {
		for _, snapshot := range unreviewed {
			key := ThresholdSentKey{
				UserID:   snapshot.UserID,
				BudgetID: snapshot.BudgetID,
				Kind:     ThresholdAlertBudgetNotReviewedDay3,
				RefDay:   day,
			}
			if _, sent := alreadySent[key]; sent {
				continue
			}
			out = append(out, DomainAlert{
				UserID:   snapshot.UserID,
				BudgetID: snapshot.BudgetID,
				Kind:     ThresholdAlertBudgetNotReviewedDay3,
				RefDay:   day,
			})
		}
	}

	return out
}

type SuppressionReason uint8

const (
	SuppressedByPriority SuppressionReason = iota + 1
	SuppressedByFrequency
	SuppressedNoChannel
	SuppressedOptInMissing
	SuppressedQuietHours
	SuppressedTemplateUnapproved
	SuppressedKindBlocked
)

func (r SuppressionReason) String() string {
	switch r {
	case SuppressedByPriority:
		return "priority"
	case SuppressedByFrequency:
		return "frequency"
	case SuppressedNoChannel:
		return "no_channel"
	case SuppressedOptInMissing:
		return "opt_in_missing"
	case SuppressedQuietHours:
		return "quiet_hours"
	case SuppressedTemplateUnapproved:
		return "template_unapproved"
	case SuppressedKindBlocked:
		return "kind_blocked"
	default:
		return ""
	}
}

func (r SuppressionReason) IsValid() bool {
	return r.String() != ""
}

type TemplateCategory uint8

const (
	TemplateCategoryUtility TemplateCategory = iota + 1
	TemplateCategoryMarketing
)

func (c TemplateCategory) String() string {
	switch c {
	case TemplateCategoryUtility:
		return "UTILITY"
	case TemplateCategoryMarketing:
		return "MARKETING"
	default:
		return ""
	}
}

func (c TemplateCategory) IsValid() bool {
	return c.String() != ""
}

func RequiresExplicitOptIn(c TemplateCategory) bool {
	return c == TemplateCategoryMarketing
}

type TemplateStatus uint8

const (
	TemplateStatusApproved TemplateStatus = iota + 1
	TemplateStatusPending
	TemplateStatusRejected
	TemplateStatusUnknown
)

func (s TemplateStatus) String() string {
	switch s {
	case TemplateStatusApproved:
		return "APPROVED"
	case TemplateStatusPending:
		return "PENDING"
	case TemplateStatusRejected:
		return "REJECTED"
	case TemplateStatusUnknown:
		return "UNKNOWN"
	default:
		return ""
	}
}

func (s TemplateStatus) IsValid() bool {
	return s.String() != ""
}

func AllowsRealSend(s TemplateStatus) bool {
	return s == TemplateStatusApproved
}

type AlertContext struct {
	Kind   ThresholdAlertKind
	SentAt time.Time
}

func IsAlertContextValid(sentAt, now time.Time, ttl time.Duration) bool {
	if sentAt.IsZero() || ttl <= 0 {
		return false
	}
	if now.Before(sentAt) {
		return false
	}
	return now.Sub(sentAt) <= ttl
}

func AlertFollowUpTopic(k ThresholdAlertKind) string {
	switch k {
	case ThresholdAlertCategory80:
		return "detalhar a categoria por subcategoria"
	case ThresholdAlertCategory100:
		return "mostrar o panorama completo do orcamento"
	case ThresholdAlertBudgetMissingMonthStart:
		return "iniciar a criacao do orcamento do mes"
	case ThresholdAlertBudgetNotReviewedDay3:
		return "revisar o orcamento do mes"
	default:
		return ""
	}
}

type SuppressedAlert struct {
	UserID uuid.UUID
	Kind   ThresholdAlertKind
	Reason SuppressionReason
}

func IsReleaseOneEmittable(k ThresholdAlertKind) bool {
	switch k {
	case ThresholdAlertCategory80,
		ThresholdAlertCategory100,
		ThresholdAlertBudgetMissingMonthStart,
		ThresholdAlertBudgetNotReviewedDay3:
		return true
	default:
		return false
	}
}

func KindPriority(k ThresholdAlertKind) int {
	switch k {
	case ThresholdAlertCategory100:
		return 1
	case ThresholdAlertBudgetNotReviewedDay3:
		return 2
	case ThresholdAlertBudgetMissingMonthStart:
		return 3
	case ThresholdAlertCategory80:
		return 4
	default:
		return 0
	}
}

func DecideCategoryKind(spentCents, plannedCents int64) (ThresholdAlertKind, bool) {
	if plannedCents <= 0 {
		return 0, false
	}
	if spentCents*100 >= plannedCents*100 {
		return ThresholdAlertCategory100, true
	}
	if spentCents*100 >= plannedCents*80 {
		return ThresholdAlertCategory80, true
	}
	return 0, false
}

func DecideDailyRoundAlerts(candidates []DomainAlert, alreadyAlertedToday map[uuid.UUID]struct{}) ([]DomainAlert, []SuppressedAlert) {
	selected := make([]DomainAlert, 0, len(candidates))
	suppressed := make([]SuppressedAlert, 0, len(candidates))

	emittable := make([]DomainAlert, 0, len(candidates))
	for _, candidate := range candidates {
		if !IsReleaseOneEmittable(candidate.Kind) {
			suppressed = append(suppressed, SuppressedAlert{
				UserID: candidate.UserID,
				Kind:   candidate.Kind,
				Reason: SuppressedKindBlocked,
			})
			continue
		}
		if _, alerted := alreadyAlertedToday[candidate.UserID]; alerted {
			suppressed = append(suppressed, SuppressedAlert{
				UserID: candidate.UserID,
				Kind:   candidate.Kind,
				Reason: SuppressedByFrequency,
			})
			continue
		}
		emittable = append(emittable, candidate)
	}

	sort.SliceStable(emittable, func(i, j int) bool {
		left, right := emittable[i], emittable[j]
		if left.UserID != right.UserID {
			return left.UserID.String() < right.UserID.String()
		}
		if KindPriority(left.Kind) != KindPriority(right.Kind) {
			return KindPriority(left.Kind) < KindPriority(right.Kind)
		}
		return left.BudgetID.String() < right.BudgetID.String()
	})

	claimed := make(map[uuid.UUID]struct{}, len(emittable))
	for _, candidate := range emittable {
		if _, taken := claimed[candidate.UserID]; taken {
			suppressed = append(suppressed, SuppressedAlert{
				UserID: candidate.UserID,
				Kind:   candidate.Kind,
				Reason: SuppressedByPriority,
			})
			continue
		}
		claimed[candidate.UserID] = struct{}{}
		selected = append(selected, candidate)
	}

	return selected, suppressed
}
