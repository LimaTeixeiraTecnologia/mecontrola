package testsupport

import (
	"context"

	"github.com/google/uuid"
)

type StubFallback struct {
	DefaultReply string
}

func (s *StubFallback) Reply(_ context.Context, _ uuid.UUID, _, _ string) (string, error) {
	if s.DefaultReply != "" {
		return s.DefaultReply, nil
	}
	return "Não entendi. Pode reformular?", nil
}
