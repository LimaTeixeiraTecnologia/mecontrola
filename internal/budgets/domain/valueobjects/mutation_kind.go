package valueobjects

import (
	"errors"
	"fmt"
)

var ErrMutationKindUnknown = errors.New("budgets: mutation kind desconhecido")

type MutationKind uint8

const (
	MutationKindCreate MutationKind = iota + 1
	MutationKindUpdate
	MutationKindDelete
)

func ParseMutationKind(s string) (MutationKind, error) {
	switch s {
	case "create":
		return MutationKindCreate, nil
	case "update":
		return MutationKindUpdate, nil
	case "delete":
		return MutationKindDelete, nil
	default:
		return 0, fmt.Errorf("budgets: %q: %w", s, ErrMutationKindUnknown)
	}
}

func (m MutationKind) String() string {
	switch m {
	case MutationKindCreate:
		return "create"
	case MutationKindUpdate:
		return "update"
	case MutationKindDelete:
		return "delete"
	default:
		return ""
	}
}
