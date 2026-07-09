package valueobjects

import (
	"errors"
	"fmt"
	"time"
)

var ErrMonthRefKindUnknown = errors.New("budgets: month ref kind desconhecido")

var ErrClarifyReasonUnknown = errors.New("budgets: clarify reason desconhecido")

type MonthRefKind int

const (
	MonthRefCurrent MonthRefKind = iota + 1
	MonthRefPrevious
	MonthRefNext
	MonthRefExplicit
	MonthRefNamedWithoutYear
	MonthRefUnknown
)

func (k MonthRefKind) String() string {
	switch k {
	case MonthRefCurrent:
		return "current"
	case MonthRefPrevious:
		return "previous"
	case MonthRefNext:
		return "next"
	case MonthRefExplicit:
		return "explicit"
	case MonthRefNamedWithoutYear:
		return "named_without_year"
	case MonthRefUnknown:
		return "unknown"
	default:
		return ""
	}
}

func (k MonthRefKind) IsValid() bool {
	switch k {
	case MonthRefCurrent, MonthRefPrevious, MonthRefNext, MonthRefExplicit, MonthRefNamedWithoutYear, MonthRefUnknown:
		return true
	default:
		return false
	}
}

func ParseMonthRefKind(s string) (MonthRefKind, error) {
	switch s {
	case "current":
		return MonthRefCurrent, nil
	case "previous":
		return MonthRefPrevious, nil
	case "next":
		return MonthRefNext, nil
	case "explicit":
		return MonthRefExplicit, nil
	case "named_without_year":
		return MonthRefNamedWithoutYear, nil
	case "unknown":
		return MonthRefUnknown, nil
	default:
		return 0, fmt.Errorf("budgets: %q: %w", s, ErrMonthRefKindUnknown)
	}
}

type ClarifyReason int

const (
	ClarifyNone ClarifyReason = iota + 1
	ClarifyMissingYear
	ClarifyUnrecognized
)

func (r ClarifyReason) String() string {
	switch r {
	case ClarifyNone:
		return "none"
	case ClarifyMissingYear:
		return "missing_year"
	case ClarifyUnrecognized:
		return "unrecognized"
	default:
		return ""
	}
}

func (r ClarifyReason) IsValid() bool {
	switch r {
	case ClarifyNone, ClarifyMissingYear, ClarifyUnrecognized:
		return true
	default:
		return false
	}
}

func ParseClarifyReason(s string) (ClarifyReason, error) {
	switch s {
	case "none":
		return ClarifyNone, nil
	case "missing_year":
		return ClarifyMissingYear, nil
	case "unrecognized":
		return ClarifyUnrecognized, nil
	default:
		return 0, fmt.Errorf("budgets: %q: %w", s, ErrClarifyReasonUnknown)
	}
}

type MonthReference struct {
	Kind  MonthRefKind
	Year  int
	Month int
}

func DecideCompetence(ref MonthReference, now time.Time) (Competence, ClarifyReason, error) {
	switch ref.Kind {
	case MonthRefCurrent:
		return CompetenceFromTime(now, now.Location()), ClarifyNone, nil
	case MonthRefPrevious:
		return CompetenceFromTime(now, now.Location()).Prev(), ClarifyNone, nil
	case MonthRefNext:
		return CompetenceFromTime(now, now.Location()).Next(), ClarifyNone, nil
	case MonthRefExplicit:
		if ref.Year <= 0 {
			return Competence{}, ClarifyMissingYear, nil
		}
		c, err := NewCompetence(fmt.Sprintf("%04d-%02d", ref.Year, ref.Month))
		if err != nil {
			return Competence{}, ClarifyNone, err
		}
		return c, ClarifyNone, nil
	case MonthRefNamedWithoutYear:
		return Competence{}, ClarifyMissingYear, nil
	case MonthRefUnknown:
		return Competence{}, ClarifyUnrecognized, nil
	default:
		return Competence{}, ClarifyNone, fmt.Errorf("budgets: %d: %w", ref.Kind, ErrMonthRefKindUnknown)
	}
}
