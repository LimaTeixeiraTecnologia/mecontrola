package workflows

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type GoalEditSectionsSuite struct {
	suite.Suite
}

func TestGoalEditSectionsSuite(t *testing.T) {
	suite.Run(t, new(GoalEditSectionsSuite))
}

func (s *GoalEditSectionsSuite) TestGoalEditSectionBody() {
	content := "## Objetivo Financeiro\n\nComprar uma casa"
	s.Equal("Comprar uma casa", goalEditSectionBody(content, goalEditSectionHeading))
}

func (s *GoalEditSectionsSuite) TestGoalEditSectionBodyMissing() {
	content := "## Outra Secao\n\nAlgum conteúdo"
	s.Equal("", goalEditSectionBody(content, goalEditSectionHeading))
}

func (s *GoalEditSectionsSuite) TestGoalEditReplaceSectionPreservesOtherSections() {
	content := "## Preferencias\n\nGosta de relatorios semanais\n\n## Objetivo Financeiro\n\nComprar uma casa\n\n## Outra Secao\n\nConteúdo adicional"

	updated := goalEditReplaceSection(content, goalEditSectionHeading, "Viajar pela Europa")

	s.Contains(updated, "## Preferencias")
	s.Contains(updated, "Gosta de relatorios semanais")
	s.Contains(updated, "## Objetivo Financeiro")
	s.Contains(updated, "Viajar pela Europa")
	s.NotContains(updated, "Comprar uma casa")
	s.Contains(updated, "## Outra Secao")
	s.Contains(updated, "Conteúdo adicional")
}

func (s *GoalEditSectionsSuite) TestGoalEditReplaceSectionAppendsWhenMissing() {
	content := "## Preferencias\n\nGosta de relatorios semanais"

	updated := goalEditReplaceSection(content, goalEditSectionHeading, "Viajar pela Europa")

	s.Contains(updated, "## Preferencias")
	s.Contains(updated, "## Objetivo Financeiro")
	s.Contains(updated, "Viajar pela Europa")
}

func (s *GoalEditSectionsSuite) TestGoalEditReplaceSectionEmptyContent() {
	updated := goalEditReplaceSection("", goalEditSectionHeading, "Viajar pela Europa")

	s.Equal("## Objetivo Financeiro\n\nViajar pela Europa", updated)
}

func (s *GoalEditSectionsSuite) TestGoalEditReplaceSectionPreservesLeadingPreamble() {
	content := "Nome de tratamento: João\n\n## Objetivo Financeiro\n\nComprar uma casa"

	updated := goalEditReplaceSection(content, goalEditSectionHeading, "Viajar pela Europa")

	s.Contains(updated, "Nome de tratamento: João")
	s.Contains(updated, "## Objetivo Financeiro")
	s.Contains(updated, "Viajar pela Europa")
	s.NotContains(updated, "Comprar uma casa")
}
