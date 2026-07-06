package interfaces

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidEntryKind       = errors.New("agents: entry kind inválido")
	ErrInvalidClassifyOutcome = errors.New("agents: classify outcome inválido")
	ErrInvalidCategoryKind    = errors.New("agents: category kind inválido")
)

type EntryKind uint8

const (
	EntryKindTransaction EntryKind = iota + 1
	EntryKindRecurringTemplate
	EntryKindCard
)

func ParseEntryKind(s string) (EntryKind, error) {
	switch s {
	case "transaction":
		return EntryKindTransaction, nil
	case "recurring_template":
		return EntryKindRecurringTemplate, nil
	case "card":
		return EntryKindCard, nil
	default:
		return 0, fmt.Errorf("agents: %q: %w", s, ErrInvalidEntryKind)
	}
}

func (k EntryKind) String() string {
	switch k {
	case EntryKindTransaction:
		return "transaction"
	case EntryKindRecurringTemplate:
		return "recurring_template"
	case EntryKindCard:
		return "card"
	default:
		return ""
	}
}

func (k EntryKind) IsValid() bool {
	switch k {
	case EntryKindTransaction, EntryKindRecurringTemplate, EntryKindCard:
		return true
	default:
		return false
	}
}

type ClassifyOutcome uint8

const (
	ClassifyOutcomeNoMatch ClassifyOutcome = iota + 1
	ClassifyOutcomeMatched
	ClassifyOutcomeAmbiguous
)

func ParseClassifyOutcome(s string) (ClassifyOutcome, error) {
	switch s {
	case "no_match":
		return ClassifyOutcomeNoMatch, nil
	case "matched":
		return ClassifyOutcomeMatched, nil
	case "ambiguous":
		return ClassifyOutcomeAmbiguous, nil
	default:
		return 0, fmt.Errorf("agents: %q: %w", s, ErrInvalidClassifyOutcome)
	}
}

func (o ClassifyOutcome) String() string {
	switch o {
	case ClassifyOutcomeNoMatch:
		return "no_match"
	case ClassifyOutcomeMatched:
		return "matched"
	case ClassifyOutcomeAmbiguous:
		return "ambiguous"
	default:
		return ""
	}
}

func (o ClassifyOutcome) IsValid() bool {
	switch o {
	case ClassifyOutcomeNoMatch, ClassifyOutcomeMatched, ClassifyOutcomeAmbiguous:
		return true
	default:
		return false
	}
}

type CategoryKind uint8

const (
	CategoryKindIncome CategoryKind = iota + 1
	CategoryKindExpense
)

func ParseCategoryKind(s string) (CategoryKind, error) {
	switch s {
	case "income":
		return CategoryKindIncome, nil
	case "expense":
		return CategoryKindExpense, nil
	default:
		return 0, fmt.Errorf("agents: %q: %w", s, ErrInvalidCategoryKind)
	}
}

func (k CategoryKind) String() string {
	switch k {
	case CategoryKindIncome:
		return "income"
	case CategoryKindExpense:
		return "expense"
	default:
		return ""
	}
}

func (k CategoryKind) IsValid() bool {
	switch k {
	case CategoryKindIncome, CategoryKindExpense:
		return true
	default:
		return false
	}
}

func (r CategorySearchResult) IsWriteEligible() bool {
	if r.Version <= 0 {
		return false
	}
	if r.Outcome != ClassifyOutcomeMatched {
		return false
	}
	if len(r.Candidates) != 1 {
		return false
	}
	candidate := r.Candidates[0]
	if candidate.IsAmbiguous {
		return false
	}
	return candidate.CategoryID != candidate.RootCategoryID
}
