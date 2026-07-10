package golden

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistrySuite struct {
	suite.Suite
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) TestAllCasesAreValid() {
	err := ValidateAll(AllCases())
	s.NoError(err)
}

func (s *RegistrySuite) TestAllCasesNonEmpty() {
	s.NotEmpty(AllCases())
}

func (s *RegistrySuite) TestAllRequiredCategoriesCovered() {
	required := []Category{
		CategoryExpenseIncome,
		CategoryQuery,
		CategoryCard,
		CategoryBudget,
		CategoryRecurrence,
		CategoryOnboarding,
		CategoryPending,
		CategoryConfirmation,
		CategoryFollowUp,
		CategoryToolError,
		CategoryAmbiguity,
		CategoryWhatsAppFormat,
		CategoryNoInternalTerms,
	}
	for _, category := range required {
		cases := CasesByCategory(category)
		s.NotEmptyf(cases, "categoria %q deve ter ao menos um caso golden", category)
	}
}

func (s *RegistrySuite) TestNoVerbatimProductionArtifacts() {
	forbidden := []string{
		"wamid.",
		"resourceId=",
		"threadId=",
	}
	for _, c := range AllCases() {
		lower := strings.ToLower(c.Input)
		for _, term := range forbidden {
			s.NotContainsf(lower, strings.ToLower(term), "caso %q não pode conter artefato verbatim de produção (%s)", c.Name, term)
		}
	}
}

func (s *RegistrySuite) TestCasesByCategoryFiltersCorrectly() {
	cases := CasesByCategory(CategoryQuery)
	for _, c := range cases {
		s.Equal(CategoryQuery, c.Category)
	}
	s.Len(cases, len(queryCases()))
}
