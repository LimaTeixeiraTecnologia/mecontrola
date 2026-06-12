package valueobjects_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type GraceWindowSuite struct {
	suite.Suite
}

func TestGraceWindowSuite(t *testing.T) {
	suite.Run(t, new(GraceWindowSuite))
}

func (s *GraceWindowSuite) TestDuration() {
	scenarios := []struct {
		name   string
		window valueobjects.GraceWindow
		want   time.Duration
	}{
		{
			name:   "janela padrao deve ser 72h",
			window: valueobjects.DefaultGraceWindow,
			want:   72 * time.Hour,
		},
		{
			name:   "janela custom deve preservar duracao informada",
			window: valueobjects.GraceWindow(48 * time.Hour),
			want:   48 * time.Hour,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			assert.Equal(s.T(), scenario.want, scenario.window.Duration())
		})
	}
}
