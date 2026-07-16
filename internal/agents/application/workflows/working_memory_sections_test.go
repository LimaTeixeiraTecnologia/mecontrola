package workflows

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type WorkingMemorySectionsSuite struct {
	suite.Suite
}

func TestWorkingMemorySectionsSuite(t *testing.T) {
	suite.Run(t, new(WorkingMemorySectionsSuite))
}

func (s *WorkingMemorySectionsSuite) TestReplaceWorkingMemorySectionPreservesSiblingSections() {
	content := "## Nome de Tratamento\n\nStef\n\n## Objetivo Financeiro\n\nComprar uma casa"

	updated := replaceWorkingMemorySection(content, "## Nome de Tratamento", "Stefany")

	s.Contains(updated, "## Nome de Tratamento")
	s.Contains(updated, "Stefany")
	s.NotContains(updated, "\nStef\n")
	s.Contains(updated, "## Objetivo Financeiro")
	s.Contains(updated, "Comprar uma casa")
	s.Equal(goalEditReplaceSection(content, "## Nome de Tratamento", "Stefany"), updated)
}

func (s *WorkingMemorySectionsSuite) TestReplaceWorkingMemorySectionAppendsWhenMissing() {
	content := "## Objetivo Financeiro\n\nComprar uma casa"

	updated := replaceWorkingMemorySection(content, "## Nome de Tratamento", "Stef")

	s.Contains(updated, "## Objetivo Financeiro")
	s.Contains(updated, "## Nome de Tratamento")
	s.Contains(updated, "Stef")
}
