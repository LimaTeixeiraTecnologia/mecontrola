package workflows

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

var ephemeralOrientationMessages = map[string]struct{}{
	MultiItemOrientationMessage: {},
}

type historyHygieneStore struct {
	next memory.MessageStore
}

func NewHistoryHygieneStore(next memory.MessageStore) memory.MessageStore {
	return &historyHygieneStore{next: next}
}

func (s *historyHygieneStore) Append(ctx context.Context, threadPK uuid.UUID, m memory.Message) error {
	return s.next.Append(ctx, threadPK, m)
}

func (s *historyHygieneStore) Recent(ctx context.Context, threadPK uuid.UUID, limit int) ([]memory.Message, error) {
	recent, err := s.next.Recent(ctx, threadPK, limit)
	if err != nil {
		return nil, err
	}
	filtered := make([]memory.Message, 0, len(recent))
	for _, m := range recent {
		if isEphemeralOrientation(m) {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered, nil
}

func isEphemeralOrientation(m memory.Message) bool {
	if m.Role != memory.RoleAssistant {
		return false
	}
	_, ok := ephemeralOrientationMessages[m.Content]
	return ok
}
