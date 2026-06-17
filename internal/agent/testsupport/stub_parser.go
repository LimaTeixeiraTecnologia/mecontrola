package testsupport

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type StubParser struct {
	table     map[string]intent.Intent
	defaultFn func() intent.Intent
}

func NewStubParser(table map[string]intent.Intent, defaultFn func() intent.Intent) *StubParser {
	return &StubParser{table: table, defaultFn: defaultFn}
}

func (s *StubParser) Parse(_ context.Context, _ uuid.UUID, text string) (services.ParsedIntent, error) {
	if in, ok := s.table[text]; ok {
		return services.ParsedIntent{Intent: in}, nil
	}
	if s.defaultFn != nil {
		return services.ParsedIntent{Intent: s.defaultFn()}, nil
	}
	unknown, _ := intent.NewUnknown(text)
	return services.ParsedIntent{Intent: unknown}, nil
}
