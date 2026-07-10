package golden

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type Category string

const (
	CategoryExpenseIncome   Category = "expense_income"
	CategoryQuery           Category = "query"
	CategoryCard            Category = "card"
	CategoryBudget          Category = "budget"
	CategoryRecurrence      Category = "recurrence"
	CategoryOnboarding      Category = "onboarding"
	CategoryPending         Category = "pending"
	CategoryConfirmation    Category = "confirmation"
	CategoryFollowUp        Category = "follow_up"
	CategoryToolError       Category = "tool_error"
	CategoryAmbiguity       Category = "ambiguity"
	CategoryWhatsAppFormat  Category = "whatsapp_format"
	CategoryNoInternalTerms Category = "no_internal_terms"
)

func (c Category) String() string {
	return string(c)
}

func (c Category) IsValid() bool {
	switch c {
	case CategoryExpenseIncome, CategoryQuery, CategoryCard, CategoryBudget, CategoryRecurrence,
		CategoryOnboarding, CategoryPending, CategoryConfirmation, CategoryFollowUp,
		CategoryToolError, CategoryAmbiguity, CategoryWhatsAppFormat, CategoryNoInternalTerms:
		return true
	default:
		return false
	}
}

func AllCategories() []Category {
	return []Category{
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
}

type Turn struct {
	UserMessage string
	ToolResults map[string]string
}

type ResponsePropertyFunc func(response string) bool

type Case struct {
	Name               string
	Category           Category
	Origin             string
	PriorTurns         []Turn
	Input              string
	ToolSubset         []string
	ExpectedTool       string
	ExpectedTools      []string
	ExpectedAnyOfTools []string
	NoToolExpected     bool
	ExpectedArgs       map[string]any
	ExpectedOutcome    agent.ToolOutcome
	ResponseProperty   ResponsePropertyFunc
	ResponseDescribe   string
}

func (c Case) Validate() error {
	return validateCase(c)
}
